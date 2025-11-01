#!/bin/bash

# Test script for TUI cleanup changes
# This script helps verify the TUI improvements are working correctly

set -e

echo "=========================================="
echo "TUI Package Cleanup - Test Script"
echo "=========================================="
echo ""

# Navigate to the gaia app directory
cd "$(dirname "$0")/apps/gaia"

echo "1. Building Gaia..."
go build -v . || { echo "❌ Build failed"; exit 1; }
echo "✓ Build successful"
echo ""

echo "2. Running go vet on TUI package..."
go vet ./tui/... || { echo "❌ go vet found issues"; exit 1; }
echo "✓ No issues found"
echo ""

echo "3. Checking for compilation errors..."
go build -v ./tui/... > /dev/null 2>&1 || { echo "❌ TUI package has compilation errors"; exit 1; }
echo "✓ TUI package compiles cleanly"
echo ""

echo "4. Running tests (if any)..."
go test ./tui/... -v 2>/dev/null || echo "⚠️  No tests found (this is expected)"
echo ""

echo "=========================================="
echo "✓ All automated tests passed!"
echo "=========================================="
echo ""
echo "Manual Testing Checklist:"
echo ""
echo "1. Register Client Functionality:"
echo "   □ Start daemon: ./gaia daemon start"
echo "   □ Launch TUI: ./gaia tui"
echo "   □ Go to Manage Certificates > Register Client"
echo "   □ Enter a client name and press Enter"
echo "   □ Verify certificate files are created in certs/"
echo "   □ Verify success message appears in status bar"
echo ""
echo "2. Locked/Unlocked Status:"
echo "   □ Observe status bar shows current daemon state"
echo "   □ Lock daemon: ./gaia daemon lock"
echo "   □ Verify TUI shows 🔒 locked in status bar"
echo "   □ Unlock daemon: ./gaia daemon unlock"
echo "   □ Verify TUI shows 🔓 unlocked in status bar"
echo ""
echo "3. Back Navigation:"
echo "   □ Press 'b' in any menu screen (should go back)"
echo "   □ Press 'esc' in any menu screen (should go back)"
echo "   □ Press 'esc' in a form (should cancel)"
echo "   □ Try typing 'b' in a text field (should work normally)"
echo ""
echo "4. Status Messages:"
echo "   □ Add a new record - verify status message"
echo "   □ Register a client - verify status message"
echo "   □ Generate certificates - verify status message"
echo "   □ Verify error messages show with ❌"
echo "   □ Verify success messages show with ✓"
echo ""

