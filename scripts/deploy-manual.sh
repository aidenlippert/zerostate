#!/bin/bash

# Manual Deployment with Live Progress Tracking
# Use this if you want to see exactly what's happening at each step

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║        🚀 MANUAL DEPLOYMENT WITH LIVE PROGRESS 🚀            ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${YELLOW}This script will deploy with full verbose output so you can see progress!${NC}"
echo ""
read -p "Press Enter to start deployment..."

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📦 Step 1: Docker Build${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Building Docker image with progress...${NC}"
echo ""

# Build with progress output
docker build --progress=plain -t zerostate-api:latest . 2>&1 | while IFS= read -r line; do
    # Show only important lines
    if echo "$line" | grep -E "(Step|COPY|RUN|Downloading|Building|Compiling|Finished)" > /dev/null; then
        echo "$line"
    fi
done

echo ""
echo -e "${GREEN}✅ Docker build complete!${NC}"
echo ""

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🚀 Step 2: Deploy to Fly.io${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Deploying to Fly.io with live progress...${NC}"
echo ""

# Deploy with verbose output
fly deploy --app zerostate-api --verbose

echo ""
echo -e "${GREEN}✅ Deployment complete!${NC}"
echo ""

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🧪 Step 3: Health Check${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Waiting 10 seconds for app to start...${NC}"
sleep 10

echo -e "${YELLOW}Testing health endpoint...${NC}"
if curl -f https://zerostate-api.fly.dev/health 2>/dev/null | grep -q "ok"; then
    echo -e "${GREEN}✅ Health check passed!${NC}"
else
    echo -e "${YELLOW}⚠️  Health check failed (app might still be starting)${NC}"
fi

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                                                              ║${NC}"
echo -e "${GREEN}║        ✨ DEPLOYMENT COMPLETE! ✨                             ║${NC}"
echo -e "${GREEN}║                                                              ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}📍 Your API:${NC} https://zerostate-api.fly.dev"
echo -e "${YELLOW}📊 Check logs:${NC} fly logs --app zerostate-api"
echo -e "${YELLOW}📈 Check status:${NC} fly status --app zerostate-api"
echo ""
