# @gaia/client

TypeScript/JavaScript client library for [Gaia](https://github.com/stain-win/gaia) - a secure, self-hosted secret management daemon.

## Installation

```bash
npm install @gaia/client
```

## Quick Start

```typescript
import { createClient } from '@gaia/client';

// Connect to Gaia daemon
const client = await createClient({
  address: 'localhost:50051',
  caCertFile: './certs/ca.crt',
  clientCertFile: './certs/client.crt',
  clientKeyFile: './certs/client.key'
});

try {
  // Fetch a secret
  const dbUrl = await client.getSecret('production', 'database_url');
  console.log('Database URL:', dbUrl);

  // Get all common secrets
  const secrets = await client.getCommonSecrets();
  console.log('All secrets:', secrets);

  // Load secrets into environment variables
  await client.loadEnv();
  // Now accessible as: process.env.GAIA_PRODUCTION_DATABASE_URL

} finally {
  await client.close();
}
```

## API Reference

### `createClient(config: GaiaClientConfig): Promise<GaiaClient>`

Creates and connects a Gaia client in one step.

**Parameters:**
- `config.address` (string, required): Address of the Gaia gRPC server (e.g., "localhost:50051")
- `config.caCertFile` (string, optional): Path to the CA certificate file
- `config.clientCertFile` (string, optional): Path to the client's certificate file
- `config.clientKeyFile` (string, optional): Path to the client's private key file
- `config.timeout` (number, optional): Connection timeout in milliseconds (default: 5000)
- `config.insecure` (boolean, optional): Allow insecure connections (default: false, **for development only**)

**Returns:** Connected `GaiaClient` instance

**Example:**
```typescript
const client = await createClient({
  address: 'localhost:50051',
  caCertFile: './certs/ca.crt',
  clientCertFile: './certs/client.crt',
  clientKeyFile: './certs/client.key',
  timeout: 10000
});
```

---

### `GaiaClient`

High-level client for interacting with the Gaia daemon.

#### Constructor

```typescript
new GaiaClient(config: GaiaClientConfig)
```

Creates a new Gaia client. You must call `connect()` before using other methods.

#### Methods

##### `connect(): Promise<void>`

Establishes connection to the Gaia daemon.

```typescript
const client = new GaiaClient(config);
await client.connect();
```

##### `getSecret(namespace: string, id: string): Promise<string>`

Fetches a single secret from a specific namespace.

**Parameters:**
- `namespace`: The namespace containing the secret
- `id`: The secret ID

**Returns:** The secret value

**Example:**
```typescript
const apiKey = await client.getSecret('production', 'api_key');
```

##### `getCommonSecrets(namespace?: string): Promise<SecretsMap>`

Fetches secrets from the "common" area.

**Parameters:**
- `namespace` (optional): If provided, returns secrets only for this namespace

**Returns:** Map of namespace names to their secrets (key-value pairs)

**Example:**
```typescript
// Get all common secrets
const allSecrets = await client.getCommonSecrets();
// { production: { api_key: '...', db_url: '...' }, staging: { ... } }

// Get secrets from specific namespace
const prodSecrets = await client.getCommonSecrets('production');
// { production: { api_key: '...', db_url: '...' } }
```

##### `loadEnv(): Promise<void>`

Loads all common secrets into `process.env`.

Environment variables are formatted as `GAIA_NAMESPACE_KEY`.
- Namespace and key names are uppercased
- Hyphens are replaced with underscores

**Example:**
```typescript
await client.loadEnv();

// Secrets are now available as environment variables:
console.log(process.env.GAIA_PRODUCTION_DATABASE_URL);
console.log(process.env.GAIA_PRODUCTION_API_KEY);
```

##### `getStatus(): Promise<string>`

Checks the current operational status of the Gaia daemon.

**Returns:** Status string (e.g., "locked", "unlocked", "offline")

**Example:**
```typescript
const status = await client.getStatus();
console.log('Daemon status:', status);
```

##### `getNamespaces(): Promise<string[]>`

Lists all namespaces the authenticated client has access to.

**Returns:** Array of namespace names

**Example:**
```typescript
const namespaces = await client.getNamespaces();
console.log('Available namespaces:', namespaces);
// ['common', 'production', 'staging']
```

##### `close(): Promise<void>`

Closes the connection to the Gaia daemon. Should be called when done.

```typescript
await client.close();
```

---

## TypeScript Types

```typescript
interface GaiaClientConfig {
  address: string;
  caCertFile?: string;
  clientCertFile?: string;
  clientKeyFile?: string;
  timeout?: number;
  insecure?: boolean;
}

interface Secret {
  id: string;
  value: string;
}

interface Namespace {
  name: string;
  secrets: Secret[];
}

type SecretsMap = Record<string, Record<string, string>>;
```

---

## Advanced Usage

### Manual Connection Management

```typescript
import { GaiaClient } from '@gaia/client';

const client = new GaiaClient({
  address: 'localhost:50051',
  caCertFile: './certs/ca.crt',
  clientCertFile: './certs/client.crt',
  clientKeyFile: './certs/client.key'
});

try {
  await client.connect();
  
  // Use client...
  const secret = await client.getSecret('production', 'api_key');
  
} catch (error) {
  console.error('Failed:', error);
} finally {
  await client.close();
}
```

### Insecure Connection (Development Only)

```typescript
const client = await createClient({
  address: 'localhost:50051',
  insecure: true  // WARNING: For development only!
});
```

### Error Handling

```typescript
try {
  const secret = await client.getSecret('production', 'api_key');
} catch (error) {
  if (error.code === grpc.status.NOT_FOUND) {
    console.error('Secret not found');
  } else if (error.code === grpc.status.PERMISSION_DENIED) {
    console.error('Access denied');
  } else {
    console.error('Error:', error.message);
  }
}
```

### Using with Express.js

```typescript
import express from 'express';
import { createClient } from '@gaia/client';

const app = express();

// Load secrets on startup
const client = await createClient({
  address: 'localhost:50051',
  caCertFile: './certs/ca.crt',
  clientCertFile: './certs/client.crt',
  clientKeyFile: './certs/client.key'
});

await client.loadEnv();

// Access secrets via environment variables
const dbUrl = process.env.GAIA_PRODUCTION_DATABASE_URL;

app.listen(3000, () => {
  console.log('Server started');
});

// Cleanup on shutdown
process.on('SIGTERM', async () => {
  await client.close();
  process.exit(0);
});
```

---

## Requirements

- Node.js >= 14.x
- Gaia daemon running and accessible
- Valid mTLS certificates (unless using insecure mode)

---

## License

MIT

---

## Links

- [Gaia Repository](https://github.com/stain-win/gaia)
- [Issues](https://github.com/stain-win/gaia/issues)

