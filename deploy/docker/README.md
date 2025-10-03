# Gaia Docker Environment Setup

This guide explains how to set up and run the Gaia ecosystem using Docker Compose. It assumes you have already created the local configuration directories as described in the main project `README.md`.

## 1. Generate and Place Certificates

Before you can run the services, you need to generate the necessary TLS certificates for secure communication between the daemon and clients.

**A. Generate Initial Certificates**

From the **root of the Gaia project**, run the `certs generate` command. This creates the Certificate Authority (CA), the server certificate, and an initial administrative client certificate.

```sh
# Run from the project root
go run ./apps/gaia certs generate
```

This will create a `certs` directory in your project root.

**B. Copy Certificates to Docker Config**

Next, copy the generated files into the correct `config` directories within `deploy/docker`.

1.  **Copy Server and CA Certificates:**

    ```sh
    # Make sure the target directory exists
    mkdir -p config/daemon/certs

    # Copy the files
    cp ../../certs/ca.crt ../../certs/server.crt ../../certs/server.key config/daemon/certs/
    ```

2.  **Copy Admin Client Certificates:**

    The `docker-compose.yml` is configured to use a client certificate named `admin-client`. Let's rename the default `gaia-cli` certificate for clarity.

    ```sh
    # Make sure the target directory exists
    mkdir -p config/client/certs

    # Copy the files, renaming the client cert
    cp ../../certs/ca.crt config/client/certs/
    cp ../../certs/gaia-cli.crt config/client/certs/admin-client.crt
    cp ../../certs/gaia-cli.key config/client/certs/admin-client.key
    ```

## 2. Build and Run the Services

Now you are ready to build the Docker images and launch the services.

1.  **Build the Images:**

    From within the `deploy/docker` directory, run:
    ```sh
    docker-compose build
    ```

2.  **Initialize the Daemon:**

    Run the `init` command using the `client` service. This will create the encrypted database inside the `gaia-data` Docker volume. You will be prompted to enter your master passphrase.

    ```sh
    docker-compose run --rm client init
    ```

3.  **Start the Daemon:**

    Start the Gaia daemon in detached mode.
    ```sh
    docker-compose up -d daemon
    ```

Your Gaia daemon is now running in the background. You can view its logs with `docker-compose logs -f daemon`.

## 3. Using the Client to Manage Gaia

To run administrative commands, use `docker-compose run` with the `client` service. This creates a temporary container that connects to your running daemon, executes the command, and then exits.

### Example Commands:

*   **Check the daemon status:**
    ```sh
    docker-compose run --rm client status
    ```

*   **List registered clients:**
    ```sh
    docker-compose run --rm client clients list
    ```

*   **Register a new client named `my-app`:**
    The generated certificates will be saved inside the `deploy/docker/config/client/certs` directory on your host machine.
    ```sh
    docker-compose run --rm client clients register my-app --output-dir /etc/gaia/certs
    ```

*   **Add a new secret:**
    ```sh
    docker-compose run --rm client secrets add my-app production DATABASE_URL "postgres://user:pass@host:port/db"
    ```

*   **Retrieve a secret:**
    ```sh
    docker-compose run --rm client secrets get my-app production DATABASE_URL
    ```
