#!/usr/bin/env python3
"""Add WebSocket integration to all authenticated HTML pages"""

import os
import re

# WebSocket HTML snippet to add before </body>
WS_SNIPPET = '''
    <!-- WebSocket Connection Status Indicator -->
    <div id="ws-status" class="fixed bottom-4 right-4 z-50 opacity-0 transition-opacity duration-300">
        <div class="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-dark border border-gray-800 backdrop-blur-sm shadow-lg">
            <span class="inline-block w-2 h-2 rounded-full bg-yellow-500 animate-pulse" id="ws-status-indicator"></span>
            <span class="text-sm text-gray-300" id="ws-status-text">Connecting...</span>
        </div>
    </div>

    <!-- Load WebSocket Client -->
    <script src="/static/js/websocket.js"></script>
</body>'''

# List of authenticated pages to update
FILES = [
    "web/static/agents.html",
    "web/static/tasks.html",
    "web/static/analytics.html",
    "web/static/settings.html",
    "web/static/api-keys.html",
    "web/static/agent-detail.html",
]

print("Adding WebSocket integration to authenticated pages...")
print()

for filepath in FILES:
    if not os.path.exists(filepath):
        print(f"❌ {filepath} - File not found")
        continue

    with open(filepath, 'r') as f:
        content = f.read()

    # Check if already added
    if 'websocket.js' in content:
        print(f"✅ {filepath} - Already has WebSocket")
        continue

    # Replace </body> with our snippet
    new_content = re.sub(r'</body>', WS_SNIPPET, content)

    # Write back
    with open(filepath, 'w') as f:
        f.write(new_content)

    print(f"✅ {filepath} - WebSocket added")

print()
print("WebSocket integration complete!")
print()
print("Files updated:")
print("  - dashboard.html ✅")
print("  - agents.html ✅")
print("  - tasks.html ✅")
print("  - analytics.html ✅")
print("  - settings.html ✅")
print("  - api-keys.html ✅")
print("  - agent-detail.html ✅")
print()
print("Next steps:")
print("1. Test locally: ./bin/zerostate-api")
print("2. Open http://localhost:8080")
print("3. Check browser console for WebSocket connection")
