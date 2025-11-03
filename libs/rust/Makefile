.PHONY: help build test clean fmt clippy doc check all proto

# Default target
help:
	@echo "Gaia Rust Client Library - Available Commands:"
	@echo ""
	@echo "  make build       - Build the library"
	@echo "  make test        - Run tests"
	@echo "  make check       - Check code without building"
	@echo "  make fmt         - Format code with rustfmt"
	@echo "  make clippy      - Run clippy lints"
	@echo "  make doc         - Generate and open documentation"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make proto       - Copy proto files from root"
	@echo "  make all         - Format, check, test, and build"
	@echo "  make publish     - Publish to crates.io (requires CARGO_TOKEN)"
	@echo ""

# Build the library
build:
	@echo "Building Rust client library..."
	cargo build --release

# Run all tests
test:
	@echo "Running tests..."
	cargo test

# Check code without building
check:
	@echo "Checking code..."
	cargo check

# Format code
fmt:
	@echo "Formatting code..."
	cargo fmt

# Run clippy
clippy:
	@echo "Running clippy..."
	cargo clippy -- -D warnings

# Generate and open documentation
doc:
	@echo "Generating documentation..."
	cargo doc --no-deps --open

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cargo clean
	rm -rf target/

# Copy proto files from root
proto:
	@echo "Copying proto files from root..."
	cp ../../proto/gaia-client.proto proto/
	@echo "Proto files updated!"

# Run all quality checks
all: fmt check clippy test build
	@echo "All checks passed!"

# Publish to crates.io
publish:
	@echo "Publishing to crates.io..."
	@if [ -z "$$CARGO_TOKEN" ]; then \
		echo "Error: CARGO_TOKEN environment variable not set"; \
		echo "Get token from: https://crates.io/settings/tokens"; \
		exit 1; \
	fi
	cargo publish --token $$CARGO_TOKEN

# Development workflow
dev: fmt check
	@echo "Development checks complete!"

# CI workflow (what GitHub Actions runs)
ci: check test clippy
	@echo "CI checks complete!"

# Install dependencies (for new contributors)
setup:
	@echo "Setting up development environment..."
	@command -v rustc >/dev/null 2>&1 || { echo "Error: Rust not installed. Visit https://rustup.rs"; exit 1; }
	@command -v protoc >/dev/null 2>&1 || { echo "Error: protoc not installed. Install via: brew install protobuf (macOS) or apt-get install protobuf-compiler (Linux)"; exit 1; }
	@echo "Installing development tools..."
	cargo install cargo-edit
	@echo "Setup complete!"

# Run examples
example-simple:
	cargo run --example simple_client

example-fetch:
	cargo run --example fetch_secrets

# Run specific test
test-unit:
	cargo test --lib

test-integration:
	cargo test --test integration_test

test-doc:
	cargo test --doc

# Watch mode (requires cargo-watch)
watch:
	@command -v cargo-watch >/dev/null 2>&1 || { echo "Installing cargo-watch..."; cargo install cargo-watch; }
	cargo watch -x check -x test

