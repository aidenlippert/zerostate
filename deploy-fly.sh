#!/bin/bash

# Deploy Ainur to Fly.io
# This script sets up secrets and deploys the application

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║        🚀 DEPLOYING AINUR TO FLY.IO 🚀                       ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Load environment variables
if [ -f .env ]; then
    echo "📦 Loading environment from .env..."
    source .env
    echo "  ✅ Environment loaded"
    echo ""
else
    echo "❌ .env file not found!"
    exit 1
fi

# Check if fly CLI is installed
if ! command -v fly &> /dev/null; then
    echo "❌ Fly CLI not installed!"
    echo "Install with: curl -L https://fly.io/install.sh | sh"
    exit 1
fi

echo "🔐 Setting Fly.io Secrets..."
echo ""

# Set database secret
if [ -n "$DATABASE_URL" ]; then
    echo "  ⏳ Setting DATABASE_URL..."
    fly secrets set DATABASE_URL="$DATABASE_URL" --app zerostate-api
    echo "  ✅ DATABASE_URL set"
else
    echo "  ⚠️  DATABASE_URL not found in .env"
fi

# Set JWT secret
if [ -n "$JWT_SECRET" ]; then
    echo "  ⏳ Setting JWT_SECRET..."
    fly secrets set JWT_SECRET="$JWT_SECRET" --app zerostate-api
    echo "  ✅ JWT_SECRET set"
else
    echo "  ⚠️  JWT_SECRET not found in .env"
fi

# Set R2 secrets
if [ -n "$R2_ACCESS_KEY_ID" ] && [ -n "$R2_SECRET_ACCESS_KEY" ] && [ -n "$R2_ENDPOINT" ] && [ -n "$R2_BUCKET_NAME" ]; then
    echo "  ⏳ Setting R2 credentials..."
    fly secrets set \
        R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID" \
        R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY" \
        R2_ENDPOINT="$R2_ENDPOINT" \
        R2_BUCKET_NAME="$R2_BUCKET_NAME" \
        --app zerostate-api
    echo "  ✅ R2 credentials set"
else
    echo "  ⚠️  R2 credentials not found in .env"
fi

# Set Groq API key
if [ -n "$GROQ_API_KEY" ]; then
    echo "  ⏳ Setting GROQ_API_KEY..."
    fly secrets set GROQ_API_KEY="$GROQ_API_KEY" --app zerostate-api
    echo "  ✅ GROQ_API_KEY set"
else
    echo "  ⚠️  GROQ_API_KEY not found in .env"
fi

# Optional: Set Gemini API key
if [ -n "$GEMINI_API_KEY" ]; then
    echo "  ⏳ Setting GEMINI_API_KEY..."
    fly secrets set GEMINI_API_KEY="$GEMINI_API_KEY" --app zerostate-api
    echo "  ✅ GEMINI_API_KEY set"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 Deploying to Fly.io..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Deploy
fly deploy --app zerostate-api

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ DEPLOYMENT COMPLETE!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📊 Your Ainur API is now live at:"
echo "  https://zerostate-api.fly.dev"
echo ""
echo "🧪 Test the health endpoint:"
echo "  curl https://zerostate-api.fly.dev/health"
echo ""
echo "📝 View logs:"
echo "  fly logs --app zerostate-api"
echo ""
echo "📊 Check status:"
echo "  fly status --app zerostate-api"
echo ""
echo "🎉 Phase 1 & 2 are now LIVE in production!"
