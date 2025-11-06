#!/bin/bash
# Fetches the bootnode peer ID from the running bootnode's HTTP endpoint
# Usage: ./scripts/get-bootnode-peer.sh [bootnode-url]

BOOTNODE_URL="${1:-http://localhost:8081}"

# Wait for bootnode to be ready (max 30 seconds)
for i in {1..30}; do
    if curl -sf "$BOOTNODE_URL/healthz" > /dev/null 2>&1; then
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Bootnode not ready after 30 seconds" >&2
        exit 1
    fi
    sleep 1
done

# Fetch peer ID
PEER_ID=$(curl -sf "$BOOTNODE_URL/peer-id")
if [ -z "$PEER_ID" ]; then
    echo "ERROR: Failed to fetch peer ID from $BOOTNODE_URL/peer-id" >&2
    exit 1
fi

echo "$PEER_ID"
