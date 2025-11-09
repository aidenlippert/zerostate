#!/bin/bash

# Echo Agent Build Script
# Compiles agent to WASM for deployment on ZeroState network

set -e

echo "Building Echo Agent for WASM..."

# Create output directory
mkdir -p dist

# Build WASM binary
echo "Compiling to WebAssembly..."
GOOS=js GOARCH=wasm go build -o dist/echo-agent.wasm main.go

# Verify WASM binary
if [ -f "dist/echo-agent.wasm" ]; then
    echo "✅ Build successful!"
    echo "📦 Output: dist/echo-agent.wasm"
    echo "📊 Size: $(du -h dist/echo-agent.wasm | cut -f1)"
    echo ""
    file dist/echo-agent.wasm
else
    echo "❌ Build failed!"
    exit 1
fi

echo ""
echo "Next steps:"
echo "1. Test locally: go run main.go"
echo "2. Register agent: ./register.sh"
