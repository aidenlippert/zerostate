#!/bin/bash
# Script to add WebSocket integration to all authenticated pages

# WebSocket snippet to add before </body>
WS_SNIPPET='
    <!-- WebSocket Connection Status Indicator -->
    <div id="ws-status" class="fixed bottom-4 right-4 z-50 opacity-0 transition-opacity duration-300">
        <div class="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-dark border border-gray-800 backdrop-blur-sm shadow-lg">
            <span class="inline-block w-2 h-2 rounded-full bg-yellow-500 animate-pulse" id="ws-status-indicator"></span>
            <span class="text-sm text-gray-300" id="ws-status-text">Connecting...</span>
        </div>
    </div>

    <!-- Load WebSocket Client -->
    <script src="/static/js/websocket.js"></script>
</body>'

# List of files to update (authenticated pages only)
FILES=(
    "web/static/agents.html"
    "web/static/tasks.html"
    "web/static/analytics.html"
    "web/static/settings.html"
    "web/static/api-keys.html"
    "web/static/agent-detail.html"
)

echo "Adding WebSocket integration to authenticated pages..."

for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
        # Check if WebSocket is already added
        if grep -q "websocket.js" "$file"; then
            echo "✅ $file - Already has WebSocket"
        else
            # Replace </body> with our snippet
            sed -i 's|</body>|'"$WS_SNIPPET"'|' "$file"
            echo "✅ $file - WebSocket added"
        fi
    else
        echo "❌ $file - File not found"
    fi
done

echo ""
echo "WebSocket integration complete!"
echo ""
echo "Files updated:"
echo "  - dashboard.html (manual)"
echo "  - agents.html"
echo "  - tasks.html"
echo "  - analytics.html"
echo "  - settings.html"
echo "  - api-keys.html"
echo "  - agent-detail.html"
echo ""
echo "Next steps:"
echo "1. Test locally: ./bin/zerostate-api"
echo "2. Open http://localhost:8080"
echo "3. Check browser console for WebSocket connection"
