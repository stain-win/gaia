#!/bin/bash
# Regenerate Rust protobuf code from proto files
# Usage: ./scripts/regenerate-rust-proto.sh

set -e

echo "🔄 Regenerating Rust protobuf code..."

# Navigate to project root
cd "$(dirname "$0")/.."

# Copy proto file to Rust lib
echo "📄 Copying proto file to libs/rust/proto/..."
mkdir -p libs/rust/proto
cp proto/gaia-client.proto libs/rust/proto/
echo "✓ Proto file copied"

# Regenerate proto code
echo "⚙️  Regenerating proto code..."
cd libs/rust
REGENERATE_PROTO=1 cargo build --features regenerate-proto

# Find and copy generated proto
echo "📝 Copying generated code to src/proto.rs..."
PROTO_FILE=$(find target/debug/build/gaia-client-*/out/gaia.rs -type f -newer ../../../proto/gaia-client.proto 2>/dev/null | head -1)

if [ -z "$PROTO_FILE" ]; then
    # Fallback: get the most recent one
    PROTO_FILE=$(find target/debug/build/gaia-client-*/out/gaia.rs -type f | head -1)
fi

if [ -n "$PROTO_FILE" ]; then
    cp "$PROTO_FILE" src/proto.rs
    echo "✓ Generated proto copied from: $PROTO_FILE"
else
    echo "✗ Error: Generated proto file not found"
    echo "Make sure protoc is installed and the build succeeded"
    exit 1
fi

# Show summary
echo ""
echo "✅ Proto code regeneration complete!"
echo ""
echo "📊 Summary:"
echo "   - Proto file: proto/gaia-client.proto"
echo "   - Generated: src/proto.rs ($(wc -l < src/proto.rs) lines)"
echo ""
echo "📌 Next steps:"
echo "   1. Review the changes: git diff src/proto.rs"
echo "   2. Test the library: cargo test"
echo "   3. Commit changes: git add src/proto.rs && git commit -m 'chore: regenerate proto code'"
echo ""

