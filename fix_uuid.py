#!/usr/bin/env python3
import re

# Read the file
with open('libs/api/agent_upload_handlers.go', 'r') as f:
    content = f.read()

# Add helper function after constants
helper = '''
// getUserIDString extracts user ID from gin context and converts to string
func getUserIDString(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	
	switch v := userID.(type) {
	case string:
		return v, true
	case uuid.UUID:
		return v.String(), true
	default:
		return "", false
	}
}
'''

# Insert helper after constants section
content = content.replace(
    'MaxAgentFormSize = 60 * 1024 * 1024\n)',
    'MaxAgentFormSize = 60 * 1024 * 1024\n)' + helper
)

# Replace getUserID patterns in functions
# Pattern 1: Get userID from context
content = re.sub(
    r'(\n\t// Get user ID from context.*?\n)\tuserID, exists := c\.Get\("user_id"\)',
    r'\1\tuserIDStr, exists := getUserIDString(c)',
    content
)

# Pattern 2: Replace userID.(string) with userIDStr
content = re.sub(
    r'userID\.\(string\)',
    'userIDStr',
    content
)

# Write back
with open('libs/api/agent_upload_handlers.go', 'w') as f:
    f.write(content)

print("Fixed!")
