#!/bin/bash
# Starts Docker Compose with dynamically fetched bootnode peer ID
# This solves the hardcoded peer ID problem in local development

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="${1:-$PROJECT_ROOT/deployments/docker-compose.simple.yml}"

echo "Starting bootnode first..."
docker compose -f "$COMPOSE_FILE" up -d bootnode jaeger prometheus

echo "Waiting for bootnode to be ready..."
sleep 3

echo "Fetching bootnode peer ID..."
BOOTNODE_PEER_ID=$("$SCRIPT_DIR/get-bootnode-peer.sh" "http://localhost:8081" 2>/dev/null | tail -1)

if [ -z "$BOOTNODE_PEER_ID" ]; then
    echo "ERROR: Failed to get bootnode peer ID"
    exit 1
fi

echo "Bootnode Peer ID: $BOOTNODE_PEER_ID"

# Export the peer ID for docker-compose to use
export ZEROSTATE_BOOTNODE_PEER_ID="$BOOTNODE_PEER_ID"

echo "Starting edge nodes with bootstrap: /dns4/bootnode/udp/4001/quic-v1/p2p/$BOOTNODE_PEER_ID"

# Start the rest of the services with the dynamic peer ID
ZEROSTATE_BOOTSTRAP_1="/dns4/bootnode/udp/4001/quic-v1/p2p/$BOOTNODE_PEER_ID" \
ZEROSTATE_BOOTSTRAP_2="/dns4/bootnode/udp/4001/quic-v1/p2p/$BOOTNODE_PEER_ID" \
docker compose -f "$COMPOSE_FILE" up -d edge-node-1 edge-node-2 grafana

echo ""
echo "✅ ZeroState network started successfully!"
echo "   Bootnode Peer ID: $BOOTNODE_PEER_ID"
echo "   Jaeger UI:  http://localhost:16686"
echo "   Grafana:    http://localhost:3000 (admin/admin)"
echo "   Prometheus: http://localhost:9090"
echo ""
echo "Check logs with: docker compose -f $COMPOSE_FILE logs -f"
