#!/usr/bin/env python3

"""
Example Python application demonstrating gaia exec usage

Usage:
  1. Add some common secrets:
     gaia secrets add common production database-url "postgres://localhost:5432/mydb"
     gaia secrets add common production api-key "secret123"

  2. Run this app with gaia exec:
     gaia exec -- python examples/exec-demo.py
"""

import os
import sys


def main():
    print("=== Gaia Exec Demo (Python) ===\n")

    # Check for Gaia-injected environment variables
    secrets = {k: v for k, v in os.environ.items() if k.startswith('GAIA_')}

    if not secrets:
        print("❌ No Gaia secrets found in environment.")
        print("\nMake sure to:")
        print("  1. Add secrets to the common namespace")
        print("  2. Run this script with: gaia exec -- python examples/exec-demo.py")
        sys.exit(1)

    print(f"✅ Found {len(secrets)} Gaia secret(s):\n")

    # Display secrets (masking the values for security)
    for key in sorted(secrets.keys()):
        value = secrets[key]
        masked_value = value[:4] + ('*' * min(len(value) - 4, 20)) if len(value) > 4 else '****'
        print(f"  {key} = {masked_value}")

    print("\n=== Example Usage ===\n")

    # Example: Using the secrets
    db_url = os.getenv('GAIA_PRODUCTION_DATABASE_URL')
    api_key = os.getenv('GAIA_PRODUCTION_API_KEY')

    if db_url:
        print(f"📊 Database URL: {db_url[:20]}...")

    if api_key:
        print(f"🔑 API Key: {api_key[:10]}...")

    print("\n✨ Application would start here with secure secrets!\n")


if __name__ == "__main__":
    main()

