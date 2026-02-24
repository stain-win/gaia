# Gaia Playground

This directory contains a pre-configured Docker Compose playground to demonstrate how easy it is to securely deliver secrets into a Next.js (SSR) application using Gaia.

The playground spins up:
1. **Gaia Daemon**: Automatically initialized and unlocked using `GAIA_PASSPHRASE` from the environment.
2. **Next.js Web Frontend**: A simple SSR React application that connects to Gaia over mTLS.

## Quick Start

1. Start the playground:
```bash
docker-compose up -d --build
```
*Note: Make sure your `gaia` executable isn't already running locally on port `50051`.*

2. Visit `http://localhost:3000` to view the beautiful Gaia Playground web page. Initially, it will connect to the daemon but display no secrets.

## Interacting with the Playground

You can use the local Gaia CLI (via `docker-compose exec gaia`) to add secrets and dynamically see them appear on the web page upon refresh. No server restarts are required!

### Add a Secret

First, ensure the client identity is registered on the daemon:

```bash
docker-compose exec gaia gaia clients register gaia_client
```

Then, you can dynamically add secrets. Gaia imports secrets via a structured JSON:

```bash
docker-compose exec -T gaia sh -c "echo '
{
  \"gaia_client\": {
    \"production\": {
      \"database_url\": \"postgresql://dbuser:playgroundpass@localhost:5432/mydb\"
    }
  }
}' > /tmp/secrets.json"

docker-compose exec gaia gaia secrets import /tmp/secrets.json
```

Refresh the web page (`http://localhost:3000`), and you'll immediately see the new `database_url` secret under the `production` namespace!

### Add Another Secret

```bash
docker-compose exec -T gaia sh -c "echo '
{
  \"gaia_client\": {
    \"staging\": {
      \"api_key\": \"sk_test_123456789\"
    }
  }
}' > /tmp/secrets_more.json"

docker-compose exec gaia gaia secrets import --overwrite /tmp/secrets_more.json
```

Refresh again to see it populate.

## How it works
- The `web` container uses the `@stain-win/gaia-client` locally.
- It shares the `gaia_certs` Docker volume directly from the `gaia` container to authenticate securely via mTLS.
- By fetching secrets dynamically inside a Next.js React Server Component (`src/app/page.tsx`), sensitive data is never hardcoded or sent to the client bundle—it remains completely secure on the server!

## Stopping the Playground

```bash
docker-compose down -v
```
*(The `-v` flag deletes the volumes, giving you a fresh slate next time).*
