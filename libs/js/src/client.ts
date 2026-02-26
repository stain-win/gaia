import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import * as fs from "fs";
import * as path from "path";

// Typed shape for the generated gRPC client.
type GaiaGrpcClient = {
  waitForReady: (deadline: number, callback: (error?: Error | null) => void) => void;
  GetSecret: (
    request: { namespace: string; id: string },
    callback: (error: grpc.ServiceError | null, response: Secret) => void,
  ) => void;
  ListSecrets: (
    request: { namespace?: string },
    callback: (error: grpc.ServiceError | null, response: { namespaces: Namespace[] }) => void,
  ) => void;
  close: () => void;
};

type GaiaClientServiceCtor = new (
  address: string,
  credentials: grpc.ChannelCredentials,
  options?: Record<string, number | string>,
) => GaiaGrpcClient;

type GaiaProtoDescriptor = {
  gaia: {
    GaiaClient: GaiaClientServiceCtor;
  };
};

/**
 * Configuration for connecting to the Gaia daemon.
 */
export interface GaiaClientConfig {
  /**
   * Address of the Gaia gRPC server (e.g., "localhost:50051").
   */
  address: string;

  /**
   * Path to the CA certificate file.
   */
  caCertFile?: string;

  /**
   * Path to the client's certificate file.
   */
  clientCertFile?: string;

  /**
   * Path to the client's private key file.
   */
  clientKeyFile?: string;

  /**
   * Timeout in milliseconds for the initial connection.
   * Default: 5000ms
   */
  timeout?: number;

  /**
   * Allow connecting without TLS. For development only.
   * Default: false
   */
  insecure?: boolean;

  /**
   * Override the path to the gaia-client.proto file.
   * Useful in environments like Next.js standalone where __dirname resolution fails.
   */
  protoPath?: string;

  /**
   * Override the expected server hostname for TLS certificate validation.
   * Useful in Docker Compose where the hostname is 'gaia' but the cert is for 'localhost'.
   */
  hostOverride?: string;
}

/**
 * Represents a secret with an ID and value.
 */
export interface Secret {
  id: string;
  value: string;
}

/**
 * Represents a namespace containing secrets.
 */
export interface Namespace {
  name: string;
  secrets: Secret[];
}

/**
 * Map of namespace names to their secrets (key-value pairs).
 */
export type SecretsMap = Record<string, Record<string, string>>;

/**
 * Options for loading secrets into the environment.
 */
export interface LoadEnvOptions {
  /**
   * Prefix to prepend to environment variable names.
   * If undefined or empty, no prefix is added.
   */
  prefix?: string;

  /**
   * Whether to include the namespace in the environment variable name.
   * Default: false
   */
  useNamespace?: boolean;
}

/**
 * High-level Gaia client for interacting with the Gaia daemon.
 *
 * @example
 * ```TypeScript
 * const client = new GaiaClient({
 *   address: 'localhost:50051',
 *   caCertFile: './certs/ca.crt',
 *   clientCertFile: './certs/client.crt',
 *   clientKeyFile: './certs/client.key'
 * });
 *
 * try {
 *   const secret = await client.getSecret('production', 'database_url');
 *   console.log('Secret value:', secret);
 * } finally {
 *   await client.close();
 * }
 * ```
 */
export class GaiaClient {
  private client: GaiaGrpcClient | null = null;
  private readonly protoPath: string;

  /**
   * Creates a new Gaia client.
   *
   * @param config - Configuration for connecting to the Gaia daemon
   * @throws {Error} If secure connection is requested but certificate paths are missing
   */
  constructor(private config: GaiaClientConfig) {
    // Proto file is relative to this module by default but can be overridden
    this.protoPath = config.protoPath || path.join(__dirname, "../proto/gaia-client.proto");

    if (!config.insecure) {
      if (
        !config.caCertFile ||
        !config.clientCertFile ||
        !config.clientKeyFile
      ) {
        throw new Error(
          "For secure connections, caCertFile, clientCertFile, and clientKeyFile are required",
        );
      }
    }

    this.config.timeout = config.timeout || 5000;
  }

  /**
   * Initializes the gRPC connection to the Gaia daemon.
   * Must be called before using any other methods.
   *
   * @throws {Error} If connection fails
   */
  async connect(): Promise<void> {
    const packageDefinition = await protoLoader.load(this.protoPath, {
      keepCase: true,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    });

    const protoDescriptor = grpc.loadPackageDefinition(
      packageDefinition,
    ) as unknown as GaiaProtoDescriptor;
    const GaiaClientService = protoDescriptor.gaia.GaiaClient;

    let credentials: grpc.ChannelCredentials;

    if (this.config.insecure) {
      credentials = grpc.credentials.createInsecure();
    } else {
      const caCert = fs.readFileSync(this.config.caCertFile!);
      const clientCert = fs.readFileSync(this.config.clientCertFile!);
      const clientKey = fs.readFileSync(this.config.clientKeyFile!);

      credentials = grpc.credentials.createSsl(caCert, clientKey, clientCert);
    }

    const options: Record<string, number | string> = {
      "grpc.keepalive_time_ms": 10000,
      "grpc.keepalive_timeout_ms": 5000,
    };

    if (this.config.hostOverride) {
      options["grpc.ssl_target_name_override"] = this.config.hostOverride;
    }

    this.client = new GaiaClientService(this.config.address, credentials, options);

    // Test connection
    const connectedClient = this.client;
    return new Promise((resolve, reject) => {
      const deadline = Date.now() + this.config.timeout!;
      connectedClient.waitForReady(deadline, (error?: Error | null) => {
        if (error) {
          reject(
            new Error(`Failed to connect to Gaia daemon: ${error.message}`),
          );
        } else {
          resolve();
        }
      });
    });
  }

  /**
   * Fetches a single secret for the authenticated client from a specific namespace.
   *
   * @param namespace - The namespace containing the secret
   * @param id - The secret ID
   * @returns The secret value
   * @throws {Error} If the secret is not found or access is denied
   *
   * @example
   * ```TypeScript
   * const dbUrl = await client.getSecret('production', 'database_url');
   * ```
   */
  async getSecret(namespace: string, id: string): Promise<string> {
    return new Promise((resolve, reject) => {
      this.client!.GetSecret(
        { namespace, id },
        (error: grpc.ServiceError | null, response: Secret) => {
          if (error) {
            reject(error);
          } else {
            resolve(response.value);
          }
        },
      );
    });
  }

  /**
   * Fetches secrets from the "common" area.
   *
   * @param namespace - Optional. If provided, fetches secrets only for this namespace.
   *                    If omitted, fetches secrets from all namespaces in the common area.
   * @returns Map of namespace names to their secrets (key-value pairs)
   *
   * @example
   * ```TypeScript
   * // Get all common secrets
   * const allSecrets = await client.getCommonSecrets();
   *
   * // Get secrets from a specific namespace
   * const prodSecrets = await client.getCommonSecrets('production');
   * ```
   */
  /**
   * Fetches all accessible secrets and loads them into process.env.
   *
   * By default, environment variables are named after the secret key, converted to uppercase with hyphens replaced by underscores.
   * Optional prefix and namespace inclusion can be configured via LoadEnvOptions.
   *
   * @param options - Configure prefix and namespace inclusion in env var names.
   * 
   * @example
   * ```TypeScript
   * await client.loadEnv();
   * // Now you can access: process.env.DATABASE_URL
   * 
   * await client.loadEnv({ prefix: 'GAIA', useNamespace: true });
   * // Now you can access: process.env.GAIA_PRODUCTION_DATABASE_URL
   * ```
   */
  async loadEnv(options?: LoadEnvOptions): Promise<void> {
    const secrets = await this.listSecrets();

    for (const [namespace, kv] of Object.entries(secrets)) {
      for (const [key, value] of Object.entries(kv)) {
        const parts: string[] = [];

        if (options?.prefix) {
          parts.push(options.prefix.toUpperCase().replace(/-/g, "_"));
        }

        if (options?.useNamespace) {
          parts.push(namespace.toUpperCase().replace(/-/g, "_"));
        }

        parts.push(key.toUpperCase().replace(/-/g, "_"));

        const envVarName = parts.join("_");
        process.env[envVarName] = value;
      }
    }
  }

  /**
   * Lists all secrets for the authenticated client.
   *
   * Returns secrets from the client's own namespaces plus the common namespace.
   * If a namespace is provided, filters to only that namespace.
   *
   * @param namespace - Optional namespace filter
   * @returns Map of namespace names to their secrets (key-value pairs)
   *
   * @example
   * ```TypeScript
   * // Get all secrets (client's own and common)
   * const allSecrets = await client.listSecrets();
   *
   * // Get only secrets from a specific namespace
   * const prodSecrets = await client.listSecrets('production');
   * ```
   */
  async listSecrets(namespace?: string): Promise<SecretsMap> {
    return new Promise((resolve, reject) => {
      const request = namespace ? { namespace } : {};

      this.client!.ListSecrets(
        request,
        (
          error: grpc.ServiceError | null,
          response: { namespaces: Namespace[] },
        ) => {
          if (error) {
            reject(error);
          } else {
            const secrets: SecretsMap = {};

            for (const ns of response.namespaces) {
              secrets[ns.name] = {};
              for (const secret of ns.secrets) {
                secrets[ns.name][secret.id] = secret.value;
              }
            }

            resolve(secrets);
          }
        },
      );
    });
  }

  /**
   * Closes the client's connection to the Gaia daemon.
   * Should be called when done using the client.
   *
   * @example
   * ```TypeScript
   * try {
   *   // ... use client
   * } finally {
   *   await client.close();
   * }
   * ```
   */
  async close(): Promise<void> {
    if (this.client) {
      this.client.close();
    }
  }
}

/**
 * Creates and connects a Gaia client in one step.
 * Convenience function for quick setup.
 *
 * @param config - Configuration for connecting to the Gaia daemon
 * @returns Connected GaiaClient instance
 *
 * @example
 * ```TypeScript
 * const client = await createClient({
 *   address: 'localhost:50051',
 *   caCertFile: './certs/ca.crt',
 *   clientCertFile: './certs/client.crt',
 *   clientKeyFile: './certs/client.key'
 * });
 * ```
 */
export async function createClient(
  config: GaiaClientConfig,
): Promise<GaiaClient> {
  const client = new GaiaClient(config);
  await client.connect();
  return client;
}
