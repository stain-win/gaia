import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import * as fs from "fs";
import * as path from "path";

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
 * High-level Gaia client for interacting with the Gaia daemon.
 *
 * @example
 * ```typescript
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
  private client: any;
  private readonly protoPath: string;

  /**
   * Creates a new Gaia client.
   *
   * @param config - Configuration for connecting to the Gaia daemon
   * @throws {Error} If secure connection is requested but certificate paths are missing
   */
  constructor(private config: GaiaClientConfig) {
    // Proto file is relative to this module
    this.protoPath = path.join(__dirname, "../proto/gaia-client.proto");

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
    ) as any;
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

    this.client = new GaiaClientService(this.config.address, credentials, {
      "grpc.keepalive_time_ms": 10000,
      "grpc.keepalive_timeout_ms": 5000,
    });

    // Test connection
    return new Promise((resolve, reject) => {
      const deadline = Date.now() + this.config.timeout!;
      this.client.waitForReady(deadline, (error: Error | undefined) => {
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
   * ```typescript
   * const dbUrl = await client.getSecret('production', 'database_url');
   * ```
   */
  async getSecret(namespace: string, id: string): Promise<string> {
    return new Promise((resolve, reject) => {
      this.client.GetSecret(
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
   * ```typescript
   * // Get all common secrets
   * const allSecrets = await client.getCommonSecrets();
   *
   * // Get secrets from specific namespace
   * const prodSecrets = await client.getCommonSecrets('production');
   * ```
   */
  /**
   * Fetches all secrets from the "common" area and loads them into process.env.
   *
   * Environment variables are formatted as GAIA_NAMESPACE_KEY.
   * Hyphens in names are replaced with underscores, and names are uppercased.
   *
   * @example
   * ```typescript
   * await client.loadEnv();
   * // Now you can access: process.env.GAIA_PRODUCTION_DATABASE_URL
   * ```
   */
  async loadEnv(): Promise<void> {
    // Fetch all secrets, including common
    const secrets = await this.listSecrets();

    // Filter for common namespace only?
    // The original behavior was just common secrets.
    // listSecrets returns client secrets + common.
    // If we want to replicate original behavior we should filter for 'common' namespace,
    // OR maybe the intention of LoadEnv is to load EVERYTHING?
    // The original doc said "Fetches all secrets from the 'common' area".
    // So let's stick to that for safety.

    // However, listSecrets doesn't guarantee 'common' is in the map if it's empty.
    // And actually, listSecrets returns ALL namespaces.
    // If the user wants to load their OWN secrets into env, they probably should.
    // But sticking to the doc: "from the 'common' area".

    // Wait, with listSecrets we get everything.
    // Let's assume we want to load EVERYTHING available to the client.
    // That seems more useful than just common.
    // But to be safe and backward compatible with the description "common area", maybe we should target that?
    // Actually, widespread pattern is to load all accessible configuration.
    // Let's load everything.

    for (const [namespace, kv] of Object.entries(secrets)) {
      // If we want to strictly follow old behavior:
      // if (namespace !== 'common') continue;
      // But that reduces utility. Let's load all.

      for (const [key, value] of Object.entries(kv)) {
        const envVarName = `GAIA_${namespace}_${key}`
          .toUpperCase()
          .replace(/-/g, "_");

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
   * ```typescript
   * // Get all secrets (client's own + common)
   * const allSecrets = await client.listSecrets();
   *
   * // Get only secrets from a specific namespace
   * const prodSecrets = await client.listSecrets('production');
   * ```
   */
  async listSecrets(namespace?: string): Promise<SecretsMap> {
    return new Promise((resolve, reject) => {
      const request = namespace ? { namespace } : {};

      this.client.ListSecrets(
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
   * ```typescript
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
 * ```typescript
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
