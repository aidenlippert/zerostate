#!/bin/bash

# Real-time Docker Build Progress Monitor
# Run this in a separate terminal while deploying

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║        📊 DOCKER BUILD PROGRESS MONITOR 📊                   ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Monitoring Docker build progress..."
echo "This will show you what's happening in real-time!"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Watch Docker build progress
docker build --progress=plain . 2>&1 | \
    grep --line-buffered -E "(Step [0-9]+/[0-9]+|COPY|RUN|go mod download|go build|Building|Compiling|Downloading)" | \
    while IFS= read -r line; do
        # Highlight important steps
        if echo "$line" | grep -q "Step"; then
            echo -e "\033[1;34m$line\033[0m"  # Blue for steps
        elif echo "$line" | grep -q "go mod download"; then
            echo -e "\033[1;33m📦 Downloading Go dependencies...\033[0m"  # Yellow
        elif echo "$line" | grep -q "go build"; then
            echo -e "\033[1;32m🔨 Building application...\033[0m"  # Green
        else
            echo "$line"
        fi
    done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Build monitoring complete!"
