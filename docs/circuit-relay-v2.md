# Circuit Relay v2

## Overview

ZeroState uses libp2p's Circuit Relay v2 protocol to enable connectivity between peers behind NATs and firewalls. The relay service acts as a bridge, allowing peers that cannot directly connect to communicate through the relay.

## Features

### Resource Management
- **Reservation limits**: Maximum 256 active reservations per relay
- **Per-peer limits**: Up to 2 reservations per peer
- **Per-IP limits**: Up to 4 reservations per IP address
- **Time limits**: Reservations expire after 1 hour
- **Data limits**: 1 MB transfer limit per reservation

### Circuit Constraints
- **Max circuits**: 64 simultaneous circuits
- **Buffer size**: 2048 bytes for relay connections
- **Auto-refresh**: Clients automatically refresh expiring reservations

## Architecture

```
┌─────────┐                  ┌─────────┐                  ┌─────────┐
│ Client1 │─────────────────▶│  Relay  │◀─────────────────│ Client2 │
│ (NAT)   │  Reserve slot    │ (Public)│  Reserve slot    │ (NAT)   │
└─────────┘                  └─────────┘                  └─────────┘
     │                             │                             │
     │      Circuit connection     │                             │
     └────────────────────────────▶│◀────────────────────────────┘
                                   │
                          Relay traffic between
                          Client1 ↔ Client2
```

## Configuration

### Default Configuration

```go
relayCfg := p2p.DefaultRelayConfig()
// MaxReservations: 256
// MaxCircuits: 64
// MaxReservationsPerPeer: 2
// MaxReservationsPerIP: 4
// ReservationTTL: 1 hour
// Limit.Duration: 2 hours
// Limit.Data: 1 MB
```

### Custom Configuration

```go
relayCfg := p2p.DefaultRelayConfig()
relayCfg.Resources.MaxReservations = 512
relayCfg.Resources.MaxCircuits = 128
relayCfg.Resources.ReservationTTL = 30 * time.Minute
```

## Usage

### Running a Relay Node

```bash
# Using Docker Compose
make dev-up

# Direct execution
./bin/relay \
  --listen /ip4/0.0.0.0/udp/4004/quic-v1 \
  --bootstrap /ip4/bootnode/udp/4001/quic-v1/p2p/12D3KooW...
```

### Client Configuration

Edge nodes automatically discover and use available relays:

```go
// Relay client is automatically enabled in edge-node
cfg := &p2p.Config{
    ListenAddrs:    []string{"/ip4/0.0.0.0/udp/4001/quic-v1"},
    EnableAutoRelay: true,  // Automatically use discovered relays
}
```

### Connecting via Circuit Relay

Manual circuit relay connection:

```go
// Build relayed multiaddr
relayedAddr := fmt.Sprintf(
    "%s/p2p/%s/p2p-circuit/p2p/%s",
    relayAddr,
    relayPeerID,
    targetPeerID,
)

// Connect through relay
targetInfo := peer.AddrInfo{
    ID:    targetPeerID,
    Addrs: []multiaddr.Multiaddr{relayedAddr},
}
err := host.Connect(ctx, targetInfo)
```

## Monitoring

### Prometheus Metrics

```
# Active reservations
zerostate_relay_reservations_total

# Active circuits
zerostate_relay_connections_total

# Bytes transferred
zerostate_relay_bytes_transferred_total{direction="inbound"}
zerostate_relay_bytes_transferred_total{direction="outbound"}

# Connection acceptance/rejection
zerostate_relay_connections_accepted_total
zerostate_relay_connections_rejected_total{reason="resource_limit"}
zerostate_relay_connections_rejected_total{reason="ip_limit"}
zerostate_relay_connections_rejected_total{reason="peer_limit"}
```

### HTTP Endpoints

```bash
# Relay info
curl http://relay:8080/relay-info
{
  "version": "v0.1.0",
  "protocol": "circuit-relay-v2",
  "peer_id": "12D3KooW...",
  "max_reservations": 256,
  "max_circuits": 64
}

# Health check
curl http://relay:8080/healthz

# Readiness
curl http://relay:8080/readyz

# Metrics
curl http://relay:8080/metrics
```

## Deployment

### Docker

```yaml
relay:
  image: zerostate/relay:latest
  environment:
    - ZEROSTATE_LISTEN=/ip4/0.0.0.0/udp/4004/quic-v1
    - ZEROSTATE_BOOTSTRAP=/ip4/bootnode/...
  ports:
    - "4004:4004/udp"
    - "8080:8080"
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: relay
spec:
  replicas: 2  # For high availability
  template:
    spec:
      containers:
      - name: relay
        image: zerostate/relay:latest
        env:
        - name: ZEROSTATE_LISTEN
          value: "/ip4/0.0.0.0/udp/4004/quic-v1"
        ports:
        - containerPort: 4004
          protocol: UDP
        - containerPort: 8080
          protocol: TCP
```

## Performance Tuning

### For High-Traffic Relays

```go
cfg := p2p.DefaultRelayConfig()

// Increase limits for production
cfg.Resources.MaxReservations = 1024
cfg.Resources.MaxCircuits = 256
cfg.Resources.MaxReservationsPerPeer = 4
cfg.Resources.ReservationTTL = 2 * time.Hour
cfg.Resources.BufferSize = 4096
```

### For Resource-Constrained Nodes

```go
cfg := p2p.DefaultRelayConfig()

// Reduce for lower resource usage
cfg.Resources.MaxReservations = 64
cfg.Resources.MaxCircuits = 16
cfg.Resources.MaxReservationsPerPeer = 1
cfg.Resources.ReservationTTL = 15 * time.Minute
```

## Troubleshooting

### Reservation Failures

```bash
# Check relay metrics
curl http://relay:8080/metrics | grep relay_connections_rejected

# Common reasons:
# - resource_limit: Relay at capacity
# - ip_limit: Too many reservations from your IP
# - peer_limit: Your peer has too many reservations
```

### Circuit Connection Issues

```bash
# Verify relay is reachable
telnet relay-host 4004

# Check relay logs
docker logs relay-1 | grep "reservation\|circuit"

# Test direct connection first
libp2p-connect /ip4/relay/udp/4004/quic-v1/p2p/12D3KooW...
```

## Best Practices

1. **Deploy Multiple Relays**: For redundancy and load distribution
2. **Monitor Limits**: Track reservation/circuit usage via metrics
3. **Geographic Distribution**: Place relays in different regions
4. **Firewall Configuration**: Open UDP port for QUIC transport
5. **Resource Allocation**: Reserve adequate bandwidth and memory
6. **Auto-scaling**: Scale relay count based on demand

## Security Considerations

- Relay nodes see metadata but not content (end-to-end encryption)
- Per-IP limits prevent DoS attacks
- Per-peer limits prevent resource exhaustion
- Data limits prevent bandwidth abuse
- Short TTLs minimize stale reservations

## Circuit Relay vs Direct Connection

| Aspect | Direct | Circuit Relay |
|--------|--------|---------------|
| Latency | Lower | Higher (extra hop) |
| Bandwidth | Full | Limited by relay |
| Privacy | Peer-to-peer | Metadata visible to relay |
| NAT traversal | May fail | Always works |
| Resource usage | Lower | Higher (relay overhead) |

## Future Enhancements

- [ ] DCUtR (Direct Connection Upgrade through Relay)
- [ ] Relay discovery via DHT
- [ ] Relay reputation system
- [ ] Bandwidth accounting and quotas
- [ ] Relay incentivization mechanism
