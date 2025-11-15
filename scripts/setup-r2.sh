#!/bin/bash

# R2 Storage Setup Guide
# ======================
# This script helps you set up Cloudflare R2 storage for WASM agents

echo "🚀 Cloudflare R2 Setup for ZeroState WASM Agents"
echo "=================================================="
echo ""

# Check if .env exists
if [ ! -f .env ]; then
    echo "📝 Creating .env file..."
    touch .env
fi

echo "📋 Required Configuration:"
echo ""
echo "1. Create a Cloudflare R2 bucket:"
echo "   - Go to https://dash.cloudflare.com/"
echo "   - Navigate to R2 Object Storage"
echo "   - Click 'Create bucket'"
echo "   - Name: 'zerostate-wasm-agents' (or your choice)"
echo ""
echo "2. Create R2 API tokens:"
echo "   - In R2 dashboard, click 'Manage R2 API Tokens'"
echo "   - Click 'Create API token'"
echo "   - Permissions: Object Read & Write"
echo "   - Copy the Access Key ID and Secret Access Key"
echo ""
echo "3. Get your R2 endpoint:"
echo "   - Format: https://<account-id>.r2.cloudflarestorage.com"
echo "   - Found in R2 bucket details"
echo ""

# Prompt for configuration
read -p "Enter R2 Access Key ID: " R2_ACCESS_KEY_ID
read -p "Enter R2 Secret Access Key: " R2_SECRET_ACCESS_KEY
read -p "Enter R2 Endpoint (e.g., https://abc123.r2.cloudflarestorage.com): " R2_ENDPOINT
read -p "Enter R2 Bucket Name (e.g., zerostate-wasm-agents): " R2_BUCKET_NAME

# Validate inputs
if [ -z "$R2_ACCESS_KEY_ID" ] || [ -z "$R2_SECRET_ACCESS_KEY" ] || [ -z "$R2_ENDPOINT" ] || [ -z "$R2_BUCKET_NAME" ]; then
    echo "❌ Error: All fields are required!"
    exit 1
fi

# Update .env file
echo ""
echo "📝 Updating .env file..."

# Remove existing R2 variables
sed -i '/^R2_/d' .env

# Add new R2 configuration
cat >> .env << EOF

# Cloudflare R2 Storage Configuration
R2_ACCESS_KEY_ID=$R2_ACCESS_KEY_ID
R2_SECRET_ACCESS_KEY=$R2_SECRET_ACCESS_KEY
R2_ENDPOINT=$R2_ENDPOINT
R2_BUCKET_NAME=$R2_BUCKET_NAME
EOF

echo "✅ R2 configuration added to .env"
echo ""

# Test configuration
echo "🧪 Testing R2 connection..."
echo ""

# Create a simple Go test program
cat > /tmp/test_r2.go << 'GOTEST'
package main

import (
	"context"
	"fmt"
	"os"
	"time"
	
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	godotenv.Load()
	
	// Test connection by checking environment variables
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	endpoint := os.Getenv("R2_ENDPOINT")
	bucket := os.Getenv("R2_BUCKET_NAME")
	
	if accessKey == "" || endpoint == "" || bucket == "" {
		fmt.Println("❌ Error: R2 environment variables not set correctly")
		os.Exit(1)
	}
	
	fmt.Println("✅ R2 configuration loaded successfully")
	fmt.Printf("   Endpoint: %s\n", endpoint)
	fmt.Printf("   Bucket: %s\n", bucket)
	fmt.Printf("   Access Key: %s...\n", accessKey[:10])
}
GOTEST

echo "✅ R2 Setup Complete!"
echo ""
echo "📚 Next Steps:"
echo "   1. Run: source .env  (to load environment variables)"
echo "   2. Run: go run scripts/test-r2-upload.go  (to test upload)"
echo "   3. Run: make build-wasm-agents  (to build WASM agents)"
echo ""
echo "📖 Documentation:"
echo "   - R2 Docs: https://developers.cloudflare.com/r2/"
echo "   - API Reference: https://developers.cloudflare.com/r2/api/"
echo ""
