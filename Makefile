# Set the current directory for the application
APP_DIR := apps/gaia
LIBS_DIR := libs
BUILD_DIR := build

# Protobuf directories and files
PROTO_DIR := proto
PROTO_FILE := $(PROTO_DIR)/gaia.proto
PROTO_CLIENT_FILE := $(PROTO_DIR)/gaia-client.proto

# Go binaries and output paths
GO_BIN_DIR := bin
GAIA_BIN_NAME := gaia
GO_OS_LIST := linux darwin windows
GO_ARCH_LIST := amd64 arm64

VERSION_PKG := github.com/stain-win/gaia/apps/gaia/cmd

GIT_VERSION := $(shell git describe --tags --always --dirty)

LDFLAGS := -s -w -X '$(VERSION_PKG).version=$(GIT_VERSION)'

# --- Commands ---

.PHONY: all build protoc clean test cross-build protoc-client-go protoc-client-rust protoc-client-js build-js-client help

# Default command to run everything
all: protoc build

# Show help
help:
	@echo "Gaia Makefile - Available Targets:"
	@echo ""
	@echo "  make all              - Build protobuf and Gaia binary"
	@echo "  make build            - Build Gaia binary"
	@echo "  make test             - Run all tests"
	@echo "  make protoc           - Compile protobuf files"
	@echo "  make cross-build      - Build for multiple platforms"
	@echo "  make clean            - Remove build artifacts"
	@echo ""
	@echo "Client Libraries:"
	@echo "  make build-js-client  - Build JavaScript/TypeScript client"
	@echo "  make protoc-client-go - Generate Go client proto"
	@echo "  make protoc-client-js - Copy proto for JS client"
	@echo ""
	@echo "Other:"
	@echo "  make debug_build      - Build with debug flags"
	@echo ""

# Build the JavaScript/TypeScript client library
build-js-client: protoc-client-js
	@echo "Building JavaScript/TypeScript client library..."
	cd $(LIBS_DIR)/js && npm install && npm run build
	@echo "✓ JavaScript/TypeScript client library built successfully!"

protoc-client-go:
	@echo "Compiling client protobuf files for Go..."
	@# The output directory is the root of the Go module (libs/go).
	@# The 'module' option tells protoc the Go module path, so it can correctly map
	@# the go_package to a directory structure within the output dir.
	protoc --proto_path=$(PROTO_DIR) \
	       --go_out=$(LIBS_DIR)/go \
	       --go-grpc_out=$(LIBS_DIR)/go \
	       --go_opt=module=github.com/stain-win/gaia/libs/go \
	       --go-grpc_opt=module=github.com/stain-win/gaia/libs/go \
	       $(PROTO_CLIENT_FILE)

protoc-client-rust:
	@echo "Compiling client protobuf files for Rust..."
	mkdir -p $(LIBS_DIR)/rust
	protoc --proto_path=$(PROTO_DIR) --rust_out=$(LIBS_DIR)/rust $(PROTO_CLIENT_FILE)

protoc-client-js:
	@echo "Copying client protobuf files for JavaScript/TypeScript..."
	@echo "Note: The JS/TS client uses @grpc/proto-loader for dynamic proto loading"
	mkdir -p $(LIBS_DIR)/js/proto
	cp $(PROTO_CLIENT_FILE) $(LIBS_DIR)/js/proto/
	@echo "Proto file copied to $(LIBS_DIR)/js/proto/"
	@echo "Run 'cd $(LIBS_DIR)/js && npm install && npm run build' to build the client library"

# Compile the .proto files into Go code
protoc:
	@echo "Compiling protobuf files..."
	protoc --proto_path=$(PROTO_DIR) --go_out=$(APP_DIR)/proto --go-grpc_out=$(APP_DIR)/proto --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative $(PROTO_FILE)

# Build the application for the current OS
build: protoc
	@echo "Building Gaia for $(shell go env GOOS)/$(shell go env GOARCH)..."
	cd $(APP_DIR) && go build -ldflags="$(LDFLAGS)" -o ../../$(GO_BIN_DIR)/$(GAIA_BIN_NAME) ./main.go

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
			GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags="-X '$(VERSION_PKG).version=$(GIT_VERSION)'" -o $(GO_BIN_DIR)/cross-build/$(GAIA_BIN_NAME)-$$GOOS-$$GOARCH$(BIN_SUFFIX) $(APP_DIR)/main.go; \
		done; \
	done

# Clean up build artifacts
clean:
	@echo "Cleaning up build artifacts..."
	rm -f $(GO_BIN_DIR)/$(GAIA_BIN_NAME)
	rm -rf $(GO_BIN_DIR)/cross-build
	rm -rf $(APP_DIR)/proto/*.pb.go
	rm -rf $(LIBS_DIR)/go/proto
	rm -rf $(LIBS_DIR)/rust
	rm -rf $(LIBS_DIR)/js

debug_build:
	@echo "Building Gaia with debug flags..."
	cd $(BUILD_DIR) && ./build_gaia
