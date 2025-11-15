#!/bin/bash

# Simple Gemini API Test Script (using curl)

set -e

API_KEY="${GEMINI_API_KEY:-AIzaSyA8yY8larEI5biwY1ZTiyTl23lKCfBzIiQ}"
MODEL="gemini-2.0-flash-exp"
API_URL="https://generativelanguage.googleapis.com/v1beta/models/${MODEL}:generateContent?key=${API_KEY}"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║       🧠 GEMINI API TEST (curl) 🧠                           ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

echo "📊 Configuration:"
echo "  API Key: ${API_KEY:0:10}...${API_KEY: -10}"
echo "  Model: $MODEL"
echo ""

# Test 1: Simple question
echo "🧪 Test 1: Simple Question"
echo "  Prompt: What is 2 + 2?"
echo ""

REQUEST_BODY='{
  "contents": [{
    "role": "user",
    "parts": [{"text": "What is 2 + 2? Answer in one sentence."}]
  }],
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 100
  }
}'

RESPONSE=$(curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "$REQUEST_BODY")

# Extract text from response
TEXT=$(echo "$RESPONSE" | jq -r '.candidates[0].content.parts[0].text')
TOKENS=$(echo "$RESPONSE" | jq -r '.usageMetadata.totalTokenCount')

if [ "$TEXT" != "null" ] && [ -n "$TEXT" ]; then
  echo "  ✅ Response (${TOKENS} tokens):"
  echo "  $TEXT"
  echo ""
else
  echo "  ❌ Error: No response from API"
  echo "  Full response:"
  echo "$RESPONSE" | jq '.'
  exit 1
fi

# Test 2: Task decomposition
echo "🧪 Test 2: Task Decomposition"
echo "  Prompt: Calculate factorial of 5 and multiply by 7"
echo ""

SYSTEM_INSTRUCTION="You are an intelligent task orchestrator. Break down complex tasks into sequential steps. Return ONLY valid JSON with this structure: {\"plan\": [{\"step\": 1, \"description\": \"...\", \"agent\": \"math-agent-v1.0\", \"function\": \"factorial\", \"args\": [\"5\"]}]}. No explanation, just JSON."

REQUEST_BODY='{
  "contents": [{
    "role": "user",
    "parts": [{"text": "Available agents: [math-agent-v1.0 with functions: add, multiply, factorial, fibonacci]\n\nUser task: Calculate the factorial of 5 and then multiply the result by 7\n\nCreate a sequential execution plan in JSON format."}]
  }],
  "systemInstruction": {
    "parts": [{"text": "'"$SYSTEM_INSTRUCTION"'"}]
  },
  "generationConfig": {
    "temperature": 0.3,
    "maxOutputTokens": 1000
  }
}'

RESPONSE=$(curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "$REQUEST_BODY")

TEXT=$(echo "$RESPONSE" | jq -r '.candidates[0].content.parts[0].text')
TOKENS=$(echo "$RESPONSE" | jq -r '.usageMetadata.totalTokenCount')

if [ "$TEXT" != "null" ] && [ -n "$TEXT" ]; then
  echo "  ✅ Plan generated (${TOKENS} tokens):"
  echo "$TEXT" | jq '.' 2>/dev/null || echo "$TEXT"
  echo ""
else
  echo "  ❌ Error: No response from API"
  echo "  Full response:"
  echo "$RESPONSE" | jq '.'
  exit 1
fi

# Test 3: System instruction (pirate mode)
echo "🧪 Test 3: System Instruction"
echo "  Prompt: Tell me about AI"
echo "  System: You are a pirate"
echo ""

REQUEST_BODY='{
  "contents": [{
    "role": "user",
    "parts": [{"text": "Tell me about artificial intelligence in one sentence."}]
  }],
  "systemInstruction": {
    "parts": [{"text": "You are a friendly pirate captain. Always respond in pirate speak with '\''Arrr!'\'' and sea references."}]
  },
  "generationConfig": {
    "temperature": 0.9,
    "maxOutputTokens": 100
  }
}'

RESPONSE=$(curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "$REQUEST_BODY")

TEXT=$(echo "$RESPONSE" | jq -r '.candidates[0].content.parts[0].text')
TOKENS=$(echo "$RESPONSE" | jq -r '.usageMetadata.totalTokenCount')

if [ "$TEXT" != "null" ] && [ -n "$TEXT" ]; then
  echo "  ✅ Response (${TOKENS} tokens):"
  echo "  $TEXT"
  echo ""
else
  echo "  ❌ Error: No response from API"
  exit 1
fi

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 ALL GEMINI API TESTS PASSED!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "✅ Gemini API is working correctly!"
echo "✅ Task decomposition is operational"
echo "✅ System instructions are working"
echo ""
echo "🚀 Ready to integrate into Go orchestrator!"
