# Kubernetes Deployment Guide

This guide explains how to deploy ZeroState on Kubernetes with dynamic peer discovery.

## Architecture

The K8s deployment uses **Init Containers** to fetch the bootnode peer ID dynamically before starting edge nodes and relays. This eliminates hardcoded peer IDs.

### How It Works

1. **Bootnode** starts first and exposes peer ID via HTTP endpoint `/peer-id`
2. **Init Container** (`fetch-bootstrap-peer`):
   - Waits for bootnode to be ready (polls `/healthz`)
   - Fetches peer ID from `http://bootnode:8080/peer-id`
   - Writes bootstrap multiaddr to `/etc/zerostate/bootstrap-peer`
3. **Main Container** reads bootstrap address from file and starts

## Prerequisites

- Kubernetes cluster (1.25+)
- kubectl configured
- Docker images built and pushed to registry

## Build Images

```bash
cd /home/rocz/vegalabs/zerostate

# Build binaries
make build

# Build Docker images
make docker-build

# Tag and push to your registry
docker tag zerostate-bootnode:latest your-registry/zerostate-bootnode:latest
docker tag zerostate-edge-node:latest your-registry/zerostate-edge-node:latest
docker tag zerostate-relay:latest your-registry/zerostate-relay:latest

docker push your-registry/zerostate-bootnode:latest
docker push your-registry/zerostate-edge-node:latest
docker push your-registry/zerostate-relay:latest
```

## Deploy to Kubernetes

```bash
# Create namespace
kubectl apply -f deployments/k8s/namespace.yaml

# Deploy bootnode (starts first)
kubectl apply -f deployments/k8s/bootnode.yaml

# Wait for bootnode to be ready
kubectl wait --for=condition=ready pod -l app=bootnode -n zerostate --timeout=60s

# Verify bootnode peer ID is accessible
kubectl run curl-test --rm -it --restart=Never -n zerostate --image=curlimages/curl -- \
  curl http://bootnode.zerostate.svc.cluster.local:8080/peer-id

# Deploy edge nodes (with dynamic bootstrap)
kubectl apply -f deployments/k8s/edge-node.yaml

# Deploy relays (with dynamic bootstrap)
kubectl apply -f deployments/k8s/relay.yaml

# Check deployment status
kubectl get pods -n zerostate
```

## Verify Dynamic Bootstrap

```bash
# Check init container logs to see peer ID fetch
kubectl logs -n zerostate edge-node-0 -c fetch-bootstrap-peer

# Should show:
# Waiting for bootnode service...
# Fetching bootnode peer ID...
# Bootnode Peer ID: 12D3KooW...
# Bootstrap address written to /etc/zerostate/bootstrap-peer

# Check main container logs
kubectl logs -n zerostate edge-node-0 -c edge-node | head -20

# Should show:
# Using dynamic bootstrap: /dns4/bootnode.zerostate.svc.cluster.local/udp/4001/quic-v1/p2p/12D3KooW...
```

## Monitoring

```bash
# Check metrics
kubectl port-forward -n zerostate svc/bootnode 8080:8080
curl http://localhost:8080/metrics | grep zerostate_

# Check health
kubectl port-forward -n zerostate svc/edge-node 8080:8080
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Scaling

```bash
# Scale edge nodes manually
kubectl scale statefulset edge-node -n zerostate --replicas=5

# Or let HPA handle it (configured for CPU/memory)
kubectl get hpa -n zerostate
```

## Troubleshooting

### Init Container Fails

```bash
# Check init container logs
kubectl logs -n zerostate edge-node-0 -c fetch-bootstrap-peer

# Common issues:
# 1. Bootnode not ready - wait longer
# 2. Network policy blocking traffic
# 3. DNS resolution issues
```

### Pods Not Connecting

```bash
# Check if bootstrap address is set
kubectl exec -n zerostate edge-node-0 -- cat /etc/zerostate/bootstrap-peer

# Check DHT metrics
kubectl exec -n zerostate edge-node-0 -- curl localhost:8080/metrics | grep peer_connections
```

### Restart Bootnode (Peer ID Changes)

```bash
# Delete bootnode pod to get new peer ID
kubectl delete pod -n zerostate -l app=bootnode

# Edge nodes will fail health checks and restart automatically
# Init containers will fetch the new peer ID
```

## Clean Up

```bash
# Delete all resources
kubectl delete -f deployments/k8s/

# Or delete namespace
kubectl delete namespace zerostate
```

## Production Considerations

1. **Persistent Bootnode Identity**: Mount a PVC for `/home/nonroot/.zerostate/keystore/` to keep bootnode peer ID stable across restarts

2. **Multiple Bootnodes**: Deploy 2-3 bootnodes with a service to load balance peer ID requests

3. **Resource Limits**: Adjust CPU/memory in manifests based on workload

4. **Network Policies**: Add NetworkPolicies to restrict traffic

5. **TLS/mTLS**: Configure ingress with TLS for HTTP endpoints

6. **Monitoring**: Deploy Prometheus + Grafana stack to scrape metrics

## Files Modified

- `deployments/k8s/edge-node.yaml` - Added init container and volume
- `deployments/k8s/relay.yaml` - Added init container and volume  
- `services/bootnode/main.go` - Added `/peer-id` and `/readyz` endpoints
- `services/relay/main.go` - Added `/readyz` endpoint

## Next Steps

- Task #6: Implement HNSW semantic search
- Task #7: Circuit relay v2
- Task #8: Auth layer for DHT writes
