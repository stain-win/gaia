# Set the current directory for the application
APP_DIR := apps/gaia
LIBS_DIR := libs

# Protobuf directories and files
PROTO_DIR := proto
PROTO_FILE := $(PROTO_DIR)/gaia.proto
PROTO_CLIENT_FILE := $(PROTO_DIR)/gaia-client.proto

# Go binaries and output paths
GO_BIN_DIR := bin
GAIA_BIN_NAME := gaia
GO_OS_LIST := linux darwin windows
GO_ARCH_LIST := amd64 arm64

# --- Commands ---

.PHONY: all build protoc clean test cross-build

# Default command to run everything
all: protoc build

protoc-client-go:
	@echo "Compiling client protobuf files..."
	protoc --proto_path=$(PROTO_DIR) --go_out=$(LIBS_DIR)/go/proto --go-grpc_out=$(LIBS_DIR)/go/proto --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative $(PROTO_CLIENT_FILE)
# Compile the .proto files into Go code
protoc:
	@echo "Compiling protobuf files..."
	protoc --proto_path=$(PROTO_DIR) --go_out=$(APP_DIR)/proto --go-grpc_out=$(APP_DIR)/proto --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative $(PROTO_FILE)

# Build the application for the current OS
build: protoc
	@echo "Building Gaia for $(shell go env GOOS)/$(shell go env GOARCH)..."
	cd $(APP_DIR) && go build -o ../../$(GO_BIN_DIR)/$(GO_BIN_NAME) ./main.go

# Run tests
test:
	@echo "Running tests..."
	cd $(APP_DIR) && go test ./...

# Cross-compile for multiple platforms
cross-build: protoc
	@echo "Cross-compiling Gaia..."
	@mkdir -p $(GO_BIN_DIR)/cross-build
	@for GOOS in $(GO_OS_LIST); do \
		for GOARCH in $(GO_ARCH_LIST); do \
			if [ "$$GOOS" = "windows" ]; then \
				BIN_SUFFIX=".exe"; \
			else \
				BIN_SUFFIX=""; \
			fi; \
			echo "--> Building for $$GOOS/$$GOARCH..."; \
			GOOS=$$GOOS GOARCH=$$GOARCH go build -o $(GO_BIN_DIR)/cross-build/$(GAIA_BIN_NAME)-$$GOOS-$$GOARCH$(BIN_SUFFIX) $(APP_DIR)/main.go; \
		done; \
	done

# Clean up build artifacts
clean:
	@echo "Cleaning up build artifacts..."
	rm -f $(GO_BIN_DIR)/$(GAIA_BIN_NAME)
	rm -rf $(GO_BIN_DIR)/cross-build
	rm -rf $(APP_DIR)/proto/*.pb.go
