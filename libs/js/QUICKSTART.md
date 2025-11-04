# Quick Start Guide - Gaia JS/TS Client

## Prerequisites

1. Gaia daemon must be running
2. Valid mTLS certificates (CA, client cert, client key)
3. Node.js >= 14.x

## Installation

### From Workspace (Development)

```bash
cd /path/to/gaia
make build-js-client
```

### From npm (Once Published)

```bash
npm install @stain-win/gaia-client
```

## 5-Minute Quick Start

### 1. Import the Library

```typescript
import { createClient } from '@stain-win/gaia-client';
```

Or with CommonJS:

```javascript
const { createClient } = require('@stain-win/gaia-client');
```

### 2. Connect to Gaia

```typescript
const client = await createClient({
  address: 'localhost:50051',
  caCertFile: './certs/ca.crt',
  clientCertFile: './certs/client.crt',
  clientKeyFile: './certs/client.key'
});
```

### 3. Use the Client

```typescript
try {
  // Get a secret
  const secret = await client.getSecret('production', 'database_url');
  console.log(secret);

  // Get all common secrets
  const secrets = await client.getCommonSecrets();
  console.log(secrets);

  // Load into environment
  await client.loadEnv();
  console.log(process.env.GAIA_PRODUCTION_DATABASE_URL);

} finally {
  await client.close();
}
```

## Complete Example

```typescript
import { createClient } from '@stain-win/gaia-client';

async function main() {
  // Connect
  const client = await createClient({
    address: 'localhost:50051',
    caCertFile: './certs/ca.crt',
    clientCertFile: './certs/client.crt',
    clientKeyFile: './certs/client.key'
  });

  try {
    // Check if daemon is unlocked
    const status = await client.getStatus();
    if (status === 'locked') {
      console.log('Daemon is locked!');
      return;
    }

    // Fetch a secret
    const dbUrl = await client.getSecret('production', 'database_url');
    
    // Use the secret
    connectToDatabase(dbUrl);

  } catch (error) {
    console.error('Error:', error.message);
  } finally {
    await client.close();
  }
}

main();
```

## Common Patterns

### With Express.js

```typescript
import express from 'express';
import { createClient } from '@stain-win/gaia-client';

const app = express();
let gaiaClient;

// Initialize on startup
async function init() {
  gaiaClient = await createClient({
    address: process.env.GAIA_ADDRESS || 'localhost:50051',
    caCertFile: './certs/ca.crt',
    clientCertFile: './certs/client.crt',
    clientKeyFile: './certs/client.key'
  });

  await gaiaClient.loadEnv();
}

// Use secrets
app.get('/api/data', async (req, res) => {
  const dbUrl = process.env.GAIA_PRODUCTION_DATABASE_URL;
  // ... use dbUrl
});

// Cleanup on shutdown
process.on('SIGTERM', async () => {
  await gaiaClient.close();
  process.exit(0);
});

init().then(() => {
  app.listen(3000, () => console.log('Server started'));
});
```

### With Error Handling

```typescript
import { createClient } from '@stain-win/gaia-client';
import * as grpc from '@grpc/grpc-js';

const client = await createClient({ /* config */ });

try {
  const secret = await client.getSecret('production', 'api_key');
} catch (error) {
  if (error.code === grpc.status.NOT_FOUND) {
    console.error('Secret not found');
  } else if (error.code === grpc.status.PERMISSION_DENIED) {
    console.error('Access denied');
  } else if (error.code === grpc.status.UNAVAILABLE) {
    console.error('Daemon not available');
  } else {
    console.error('Error:', error.message);
  }
}
```

## Testing Your Setup

Run the included example:

```bash
cd libs/js
npx ts-node examples/basic-usage.ts
```

Expected output:
```
🔐 Gaia Client Example

Connecting to Gaia daemon...

📊 Checking daemon status...
   Status: unlocked

📂 Available namespaces:
   - common
   - production

🔑 Fetching specific secret...
   production/database_url: postgres://...

✅ Example completed successfully!
```

## Troubleshooting

### "Failed to connect to Gaia daemon"

- Check if daemon is running: `./gaia daemon status`
- Verify address is correct (default: localhost:50051)
- Check firewall settings

### "Certificate validation failed"

- Verify certificate paths are correct
- Ensure certificates are readable
- Check certificate expiration
- Verify CA certificate matches daemon's CA

### "Permission denied"

- Verify client has access to the namespace
- Check client registration: `./gaia clients list`
- Ensure client certificate CN matches registered client

### "Daemon is locked"

- Unlock the daemon: `./gaia daemon unlock`
- Or unlock from TUI: `./gaia` and select "Unlock Gaia"

## Next Steps

1. Read the full [README.md](./README.md)
2. Check [examples/basic-usage.ts](./examples/basic-usage.ts)
3. Review the TypeScript types for IntelliSense
4. Integrate with your application

## Support

- GitHub Issues: https://github.com/stain-win/gaia/issues
- Documentation: See README.md

---

**Ready to use!** The Gaia JS/TS client is production-ready and fully featured. 🚀

