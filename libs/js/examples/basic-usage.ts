/**
 * Example usage of the Gaia TypeScript client.
 *
 * Prerequisites:
 * 1. Gaia daemon must be running
 * 2. You need valid mTLS certificates
 * 3. Install dependencies: npm install
 * 4. Build the library: npm run build
 *
 * Run this example:
 *   npx ts-node examples/basic-usage.ts
 */

import { createClient, GaiaClient } from '../src';

async function main() {
  console.log('🔐 Gaia Client Example\n');

  // Create and connect to Gaia daemon
  console.log('Connecting to Gaia daemon...');
  const client: GaiaClient = await createClient({
    address: 'localhost:50051',
    caCertFile: '../../certs/ca.crt',
    clientCertFile: '../../certs/client.crt',
    clientKeyFile: '../../certs/client.key',
    timeout: 10000,
  });

  try {
    // Check daemon status
    console.log('\n📊 Checking daemon status...');
    const status = await client.getStatus();
    console.log(`   Status: ${status}`);

    if (status === 'locked') {
      console.log('   ⚠️  Daemon is locked. Please unlock it first.');
      return;
    }

    // List available namespaces
    console.log('\n📂 Available namespaces:');
    const namespaces = await client.getNamespaces();
    namespaces.forEach(ns => console.log(`   - ${ns}`));

    // Fetch a specific secret
    console.log('\n🔑 Fetching specific secret...');
    try {
      const secret = await client.getSecret('production', 'database_url');
      console.log(`   production/database_url: ${secret}`);
    } catch (error: any) {
      console.log(`   ❌ Error: ${error.message}`);
    }

    // Get all common secrets
    console.log('\n🌍 Fetching common secrets...');
    const commonSecrets = await client.getCommonSecrets();

    for (const [namespace, secrets] of Object.entries(commonSecrets)) {
      console.log(`\n   Namespace: ${namespace}`);
      for (const [key, value] of Object.entries(secrets)) {
        // Mask the value for security
        const masked = value.substring(0, 3) + '*'.repeat(Math.min(value.length - 3, 10));
        console.log(`     ${key}: ${masked}`);
      }
    }

    // Load secrets into environment
    console.log('\n🌐 Loading secrets into environment...');
    await client.loadEnv();
    console.log('   ✓ Secrets loaded into process.env');

    // Show some environment variables (masked)
    const envVars = Object.keys(process.env)
      .filter(key => key.startsWith('GAIA_'))
      .slice(0, 5);

    if (envVars.length > 0) {
      console.log('\n   Sample environment variables:');
      envVars.forEach(key => {
        const value = process.env[key] || '';
        const masked = value.substring(0, 3) + '*'.repeat(Math.min(value.length - 3, 10));
        console.log(`     ${key}=${masked}`);
      });
    }

    console.log('\n✅ Example completed successfully!');

  } catch (error: any) {
    console.error('\n❌ Error:', error.message);
    if (error.code) {
      console.error(`   gRPC Error Code: ${error.code}`);
    }
  } finally {
    // Always close the connection
    console.log('\n🔌 Closing connection...');
    await client.close();
    console.log('   Connection closed.\n');
  }
}

// Run the example
main().catch(error => {
  console.error('Fatal error:', error);
  process.exit(1);
});

