#!/bin/bash

# Ainur Bootstrap Node - 1-Click Community Deploy
# For Founding Members who want to help decentralize the network

set -e

echo "🚀 Ainur Foundation - Community Bootstrap Node Deployment"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "This script will deploy YOUR bootstrap node to help decentralize Ainur."
echo "Cost: $0 (uses Fly.io free tier)"
echo "Time: 5 minutes"
echo ""
read -p "Ready to become a Founding Member? (y/n) " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "No problem! Come back when you're ready."
    exit 0
fi

# Check if fly CLI is installed
if ! command -v fly &> /dev/null; then
    echo "📦 Installing Fly CLI..."
    curl -L https://fly.io/install.sh | sh
    echo ""
    echo "✅ Fly CLI installed!"
    echo "   Run this script again to continue."
    exit 0
fi

# Login to Fly.io
echo ""
echo "🔐 Step 1: Login to Fly.io"
echo "   (If you don't have an account, it will create one - it's free!)"
fly auth login

# Get community member name
echo ""
echo "🎯 Step 2: Choose your node name"
echo "   This will be YOUR unique bootstrap node identity."
echo "   Examples: ainur-community-alice, ainur-community-tokyo, ainur-community-berlin"
echo ""
read -p "Enter your node name (lowercase, no spaces): " NODE_NAME

# Validate name
if [[ ! $NODE_NAME =~ ^[a-z0-9-]+$ ]]; then
    echo "❌ Invalid name. Use only lowercase letters, numbers, and hyphens."
    exit 1
fi

# Create app
echo ""
echo "🏗️  Step 3: Creating your Fly.io app..."
cd "$(dirname "$0")"
fly apps create "$NODE_NAME" || {
    echo "⚠️  App name taken. Try a different name."
    exit 1
}

# Deploy
echo ""
echo "🚀 Step 4: Deploying your bootstrap node..."
fly deploy --app "$NODE_NAME"

# Get multiaddresses
echo ""
echo "🎉 DEPLOYMENT COMPLETE!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "✅ Your bootstrap node is LIVE at: $NODE_NAME.fly.dev"
echo ""
echo "📋 Next Steps:"
echo "   1. Copy your multiaddress from the logs below"
echo "   2. Share it in the Ainur Discord (#bootstrap-nodes channel)"
echo "   3. The Foundation will add it to the official bootstrap list"
echo ""
echo "🏆 Congratulations, Founding Member!"
echo ""
echo "📡 Fetching your node info..."
sleep 3
fly logs --app "$NODE_NAME"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎁 Rewards for Founding Members:"
echo "   • Listed on ainur.foundation website"
echo "   • 'Founding Member' Discord role"
echo "   • Early access to new features"
echo "   • Eternal gratitude of the community ❤️"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
