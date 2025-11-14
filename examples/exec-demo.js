#!/usr/bin/env node

/**
 * Example Node.js application demonstrating gaia exec usage
 *
 * Usage:
 *   1. Add some common secrets:
 *      gaia secrets add common production database-url "postgres://localhost:5432/mydb"
 *      gaia secrets add common production api-key "secret123"
 *
 *   2. Run this app with gaia exec:
 *      gaia exec -- node examples/exec-demo.js
 */

console.log('=== Gaia Exec Demo ===\n');

// Check for Gaia-injected environment variables
const secrets = Object.entries(process.env)
  .filter(([key]) => key.startsWith('GAIA_'))
  .sort();

if (secrets.length === 0) {
  console.log('❌ No Gaia secrets found in environment.');
  console.log('\nMake sure to:');
  console.log('  1. Add secrets to the common namespace');
  console.log('  2. Run this script with: gaia exec -- node examples/exec-demo.js');
  process.exit(1);
}

console.log(`✅ Found ${secrets.length} Gaia secret(s):\n`);

// Display secrets (masking the values for security)
secrets.forEach(([key, value]) => {
  const maskedValue = value.length > 4
    ? value.substring(0, 4) + '*'.repeat(Math.min(value.length - 4, 20))
    : '****';
  console.log(`  ${key} = ${maskedValue}`);
});

console.log('\n=== Example Usage ===\n');

// Example: Using the secrets
const dbUrl = process.env.GAIA_PRODUCTION_DATABASE_URL;
const apiKey = process.env.GAIA_PRODUCTION_API_KEY;

if (dbUrl) {
  console.log('📊 Database URL:', dbUrl.substring(0, 20) + '...');
}

if (apiKey) {
  console.log('🔑 API Key:', apiKey.substring(0, 10) + '...');
}

console.log('\n✨ Application would start here with secure secrets!\n');

