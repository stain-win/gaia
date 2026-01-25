# Gaia Go Client Library

This library provides a simple and convenient way to interact with the Gaia secrets management daemon from your Go applications.

## Features

-   High-level client for all `GaiaClient` RPCs.
-   Automatic handling of secure mTLS connections.
-   Convenience function to load secrets directly into your application's environment.

## Installation

To use this library, you'll need to have the Gaia repository accessible in your Go workspace. You can then import it into your project:

```go
import "github.com/stain-win/gaia/libs/go/client"
```

## Usage

### Initializing the Client

To get started, you need to create a new Gaia client. The client handles the gRPC connection and authentication for you.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/stain-win/gaia/libs/go/client"
)

func main() {
	cfg := client.Config{
		Address:        "localhost:50051",
		CACertFile:     "path/to/your/ca.crt",
		ClientCertFile: "path/to/your/client.crt",
		ClientKeyFile:  "path/to/your/client.key",
		Timeout:        5 * time.Second,
	}

	gaiaClient, err := client.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Gaia client: %v", err)
	}
	defer gaiaClient.Close()

	// You can now use the client to interact with Gaia.
	status, err := gaiaClient.GetStatus(context.Background())
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}

	fmt.Printf("Gaia daemon status: %s\n", status)
}
```

### Fetching a Secret

You can fetch a single secret from a specific namespace that your client is authorized to access.

```go
secret, err := gaiaClient.GetSecret(context.Background(), "my-app-namespace", "database-password")
if err != nil {
    log.Fatalf("Failed to get secret: %v", err)
}
fmt.Printf("The secret is: %s\n", secret)
```

### Loading Secrets into the Environment

Gaia can automatically fetch all secrets from the "common" area and load them as environment variables in your application. This is a powerful way to provide configuration to your application without hardcoding values.

Environment variables are formatted as `GAIA_NAMESPACE_KEY`.

```go
if err := gaiaClient.LoadEnv(context.Background()); err != nil {
    log.Fatalf("Failed to load environment: %v", err)
}

// Your application can now access the secrets via os.Getenv()
apiKey := os.Getenv("GAIA_THIRD_PARTY_API_KEY")
```

### Fetching Common Secrets

You can also fetch all common secrets as a map, which gives you more control over how you use them.

```go
// Fetch all common secrets
allSecrets, err := gaiaClient.GetCommonSecrets(context.Background())
if err != nil {
    log.Fatalf("Failed to get common secrets: %v", err)
}

// Fetch secrets for a specific common namespace
specificSecrets, err := gaiaClient.GetCommonSecrets(context.Background(), "billing-service")
if err != nil {
    log.Fatalf("Failed to get specific common secrets: %v", err)
}
```

### Listing All Secrets ⭐ NEW

The `ListSecrets` method fetches all secrets accessible to the authenticated client, including both the client's own namespaces and the common namespace. This is the most efficient way to fetch all secrets at once.

```go
// Get all secrets (client's own + common)
allSecrets, err := gaiaClient.ListSecrets(context.Background())
if err != nil {
    log.Fatalf("Failed to list secrets: %v", err)
}

// Iterate through all namespaces and secrets
for namespace, secrets := range allSecrets {
    fmt.Printf("Namespace: %s\n", namespace)
    for key, value := range secrets {
        fmt.Printf("  %s: %s\n", key, value)
    }
}

// Filter by specific namespace
productionSecrets, err := gaiaClient.ListSecrets(context.Background(), "production")
if err != nil {
    log.Fatalf("Failed to list production secrets: %v", err)
}
```

## API Reference

### Client Methods

#### `NewClient(cfg Config) (*Client, error)`

Creates a new Gaia client with the provided configuration.

#### `Close() error`

Closes the client's connection to the Gaia daemon.

#### `GetStatus(ctx context.Context) (string, error)`

Checks the current operational status of the Gaia daemon ("locked" or "unlocked").

#### `GetSecret(ctx context.Context, namespace, id string) (string, error)`

Fetches a single secret for the authenticated client from a specific namespace.

#### `GetCommonSecrets(ctx context.Context, namespace ...string) (map[string]map[string]string, error)`

Fetches secrets from the "common" area. If a namespace is provided, filters to that namespace only.

#### `GetNamespaces(ctx context.Context) ([]string, error)`

Lists all namespaces the authenticated client has access to.

#### `ListSecrets(ctx context.Context, namespace ...string) (map[string]map[string]string, error)` ⭐ NEW

Fetches all secrets for the authenticated client (own namespaces + common). Optional namespace filter available.

Returns: `map[namespace]map[key]value`

#### `LoadEnv(ctx context.Context) error`

Fetches all secrets from the "common" area and loads them into the current process's environment. Environment variables are formatted as `GAIA_NAMESPACE_KEY`.

## Configuration

The `Config` struct accepts the following fields:

```go
type Config struct {
    Address        string        // Gaia gRPC server address (e.g., "localhost:50051")
    CACertFile     string        // Path to the CA certificate file
    ClientCertFile string        // Path to the client's certificate file
    ClientKeyFile  string        // Path to the client's private key file
    Timeout        time.Duration // Timeout for the initial connection
    Insecure       bool          // Allow connecting without TLS (development only)
}
```

## Examples

See the [examples](./examples) directory for more usage examples.

## License

This library is part of the Gaia project. See the main repository for license information.

