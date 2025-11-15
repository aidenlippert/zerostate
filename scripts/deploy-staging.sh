#!/bin/bash

# Deploy ZeroState to Staging Environment
# This script deploys to Fly.io staging with all necessary configurations

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║        🚀 DEPLOYING ZEROSTATE TO STAGING 🚀                  ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Step 1: Check prerequisites
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📋 Step 1/8: Checking Prerequisites${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check if fly CLI is installed
if ! command -v fly &> /dev/null; then
    echo -e "${RED}❌ Fly CLI not installed!${NC}"
    echo -e "${YELLOW}Install with: curl -L https://fly.io/install.sh | sh${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Fly CLI installed${NC}"

# Check if logged in
if ! fly auth whoami &> /dev/null; then
    echo -e "${RED}❌ Not logged in to Fly.io!${NC}"
    echo -e "${YELLOW}Login with: fly auth login${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Logged in to Fly.io${NC}"

# Check if .env exists
if [ ! -f .env ]; then
    echo -e "${RED}❌ .env file not found!${NC}"
    echo -e "${YELLOW}Copy .env.example to .env and fill in your values${NC}"
    exit 1
fi
echo -e "${GREEN}✅ .env file exists${NC}"

# Load environment variables
source .env

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🏗️  Step 2/8: Building Docker Image${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Test Docker build locally (optional)
read -p "Do you want to test Docker build locally first? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}⏳ Building Docker image locally...${NC}"
    docker build -t zerostate-api:test .
    echo -e "${GREEN}✅ Docker build successful${NC}"
    echo ""
    echo -e "${YELLOW}🧪 Testing image...${NC}"
    docker run --rm -p 8080:8080 -e DATABASE_URL=./test.db -e JWT_SECRET=test-secret zerostate-api:test &
    DOCKER_PID=$!
    sleep 5
    
    if curl -f http://localhost:8080/health &> /dev/null; then
        echo -e "${GREEN}✅ Health check passed${NC}"
    else
        echo -e "${RED}❌ Health check failed${NC}"
        kill $DOCKER_PID
        exit 1
    fi
    
    kill $DOCKER_PID
    echo -e "${GREEN}✅ Local Docker test passed${NC}"
else
    echo -e "${YELLOW}⏩ Skipping local Docker test${NC}"
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🎯 Step 3/8: Checking Fly.io App Status${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check if app exists
if fly status --app zerostate-api &> /dev/null; then
    echo -e "${GREEN}✅ App 'zerostate-api' exists${NC}"
else
    echo -e "${YELLOW}⚠️  App 'zerostate-api' does not exist${NC}"
    echo -e "${YELLOW}Creating new app...${NC}"
    fly launch --name zerostate-api --region sjc --copy-config --now=false
    echo -e "${GREEN}✅ App created${NC}"
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🔐 Step 4/8: Setting Secrets${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Database URL (required)
if [ -z "$DATABASE_URL" ]; then
    echo -e "${RED}❌ DATABASE_URL not set in .env${NC}"
    echo -e "${YELLOW}Please set DATABASE_URL to your Supabase connection string${NC}"
    exit 1
fi
echo -e "${YELLOW}⏳ Setting DATABASE_URL...${NC}"
fly secrets set DATABASE_URL="$DATABASE_URL" --app zerostate-api
echo -e "${GREEN}✅ DATABASE_URL set${NC}"

# JWT Secret (required)
if [ -z "$JWT_SECRET" ]; then
    echo -e "${YELLOW}⚠️  JWT_SECRET not set, generating random one...${NC}"
    JWT_SECRET=$(openssl rand -base64 32)
fi
echo -e "${YELLOW}⏳ Setting JWT_SECRET...${NC}"
fly secrets set JWT_SECRET="$JWT_SECRET" --app zerostate-api
echo -e "${GREEN}✅ JWT_SECRET set${NC}"

# R2 Storage (optional but recommended)
if [ -n "$R2_ACCESS_KEY_ID" ] && [ -n "$R2_SECRET_ACCESS_KEY" ]; then
    echo -e "${YELLOW}⏳ Setting R2 credentials...${NC}"
    fly secrets set \
        R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID" \
        R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY" \
        R2_ENDPOINT="$R2_ENDPOINT" \
        R2_BUCKET_NAME="$R2_BUCKET_NAME" \
        --app zerostate-api
    echo -e "${GREEN}✅ R2 credentials set${NC}"
else
    echo -e "${YELLOW}⚠️  R2 credentials not found, skipping (WASM execution will be limited)${NC}"
fi

# Gemini API Key (optional)
if [ -n "$GEMINI_API_KEY" ]; then
    echo -e "${YELLOW}⏳ Setting GEMINI_API_KEY...${NC}"
    fly secrets set GEMINI_API_KEY="$GEMINI_API_KEY" --app zerostate-api
    echo -e "${GREEN}✅ GEMINI_API_KEY set${NC}"
else
    echo -e "${YELLOW}⚠️  GEMINI_API_KEY not found (LLM features will be limited)${NC}"
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🚀 Step 5/8: Deploying to Fly.io${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}⏳ Starting deployment (this will show live progress)...${NC}"
echo ""

# Deploy with verbose output to show progress
fly deploy --app zerostate-api --verbose

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🧪 Step 6/8: Testing Deployment${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Wait for deployment to be ready
echo -e "${YELLOW}⏳ Waiting for deployment to be ready...${NC}"
sleep 10

# Test health endpoint
echo -e "${YELLOW}🧪 Testing health endpoint...${NC}"
if curl -f https://zerostate-api.fly.dev/health 2>/dev/null | grep -q "ok"; then
    echo -e "${GREEN}✅ Health check passed!${NC}"
else
    echo -e "${RED}❌ Health check failed!${NC}"
    echo -e "${YELLOW}Check logs with: fly logs --app zerostate-api${NC}"
    exit 1
fi

# Test database connection
echo -e "${YELLOW}🧪 Testing database connection...${NC}"
RESPONSE=$(curl -s https://zerostate-api.fly.dev/api/v1/agents)
if echo "$RESPONSE" | grep -q "agents"; then
    echo -e "${GREEN}✅ Database connection working!${NC}"
else
    echo -e "${YELLOW}⚠️  Database might not be initialized yet (this is normal for first deploy)${NC}"
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📊 Step 7/8: Deployment Information${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${GREEN}🎉 Deployment successful!${NC}"
echo ""
echo -e "${YELLOW}📍 API URLs:${NC}"
echo "  Health:  https://zerostate-api.fly.dev/health"
echo "  API:     https://zerostate-api.fly.dev/api/v1"
echo "  Metrics: https://zerostate-api.fly.dev/metrics"
echo ""
echo -e "${YELLOW}🔍 Management Commands:${NC}"
echo "  Logs:    fly logs --app zerostate-api"
echo "  Status:  fly status --app zerostate-api"
echo "  Scale:   fly scale count 2 --app zerostate-api"
echo "  SSH:     fly ssh console --app zerostate-api"
echo ""

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📝 Step 8/8: Next Steps${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${YELLOW}1. Deploy Frontend to Vercel:${NC}"
echo "   cd web && vercel deploy"
echo ""
echo -e "${YELLOW}2. Set Frontend Environment Variable:${NC}"
echo "   API_BASE_URL=https://zerostate-api.fly.dev/api/v1"
echo ""
echo -e "${YELLOW}3. Update CORS Settings:${NC}"
echo "   fly secrets set ALLOWED_ORIGINS='https://your-vercel-app.vercel.app' --app zerostate-api"
echo ""
echo -e "${YELLOW}4. Test End-to-End:${NC}"
echo "   Visit your Vercel URL and try submitting a task!"
echo ""

echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                                                              ║${NC}"
echo -e "${GREEN}║        ✨ STAGING DEPLOYMENT COMPLETE! ✨                     ║${NC}"
echo -e "${GREEN}║                                                              ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
