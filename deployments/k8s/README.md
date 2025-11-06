# Zerostate Kubernetes Deployment

This directory contains Kubernetes manifests for deploying the zerostate network.

## Prerequisites

- Kubernetes 1.24+
- kubectl configured
- Metrics Server (for HPA)
- Optional: Prometheus Operator for monitoring
- Optional: Jaeger Operator for distributed tracing

## Quick Start

```bash
# Create namespace
kubectl apply -f namespace.yaml

# Deploy bootnode
kubectl apply -f bootnode.yaml

# Wait for bootnode to be ready
kubectl wait --for=condition=ready pod -l app=bootnode -n zerostate --timeout=60s

# Deploy edge nodes
kubectl apply -f edge-node.yaml

# Deploy relay nodes
kubectl apply -f relay.yaml

# Verify deployment
kubectl get pods -n zerostate
kubectl get services -n zerostate
```

## Architecture

- **Bootnode**: Single replica deployment with LoadBalancer service for external connectivity
- **Edge Nodes**: StatefulSet with 3-10 replicas (HPA enabled), headless service for peer discovery
- **Relay Nodes**: Deployment with 2 replicas, LoadBalancer service

## Scaling

Edge nodes auto-scale based on CPU (70%) and memory (80%) utilization:

```bash
# Manual scaling
kubectl scale statefulset edge-node --replicas=5 -n zerostate

# View HPA status
kubectl get hpa edge-node-hpa -n zerostate
```

## Monitoring

Services expose Prometheus metrics on port 8080:

```bash
# Port forward to access metrics
kubectl port-forward svc/bootnode 8080:8080 -n zerostate
curl http://localhost:8080/metrics
```

## Health Checks

All services implement:
- `/healthz` - Liveness probe (pod restart if fails)
- `/readyz` - Readiness probe (removes from service if fails)

## Resource Limits

| Component | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-----------|-------------|-----------|----------------|--------------|
| Bootnode  | 100m        | 500m      | 128Mi          | 512Mi        |
| Edge Node | 200m        | 1000m     | 256Mi          | 1Gi          |
| Relay     | 150m        | 750m      | 192Mi          | 768Mi        |

## Troubleshooting

```bash
# Check pod logs
kubectl logs -f deployment/bootnode -n zerostate
kubectl logs -f statefulset/edge-node -n zerostate

# Describe pod for events
kubectl describe pod <pod-name> -n zerostate

# Check resource usage
kubectl top pods -n zerostate

# Get peer connections
kubectl exec -it edge-node-0 -n zerostate -- wget -qO- localhost:8080/metrics | grep peer_connections
```

## Cleanup

```bash
kubectl delete -f relay.yaml
kubectl delete -f edge-node.yaml
kubectl delete -f bootnode.yaml
kubectl delete -f namespace.yaml
```

## Production Considerations

1. **Bootnode Peer ID**: The bootstrap peer ID is hardcoded. In production, use:
   - ConfigMap update after bootnode starts
   - Init container to fetch peer ID
   - DNS-based peer discovery

2. **Persistent Storage**: Add PersistentVolumeClaims for DHT data persistence

3. **Network Policies**: Add NetworkPolicy resources to restrict traffic

4. **Security**: 
   - Use Pod Security Standards
   - Enable RBAC
   - Use secrets for sensitive data
   - Run as non-root (already configured in containers)

5. **Observability**: 
   - Deploy Prometheus Operator
   - Deploy Jaeger Operator
   - Configure ServiceMonitor resources
   - Set up alerts and dashboards
