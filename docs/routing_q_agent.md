# Relay Q-Routing Agent — Pseudocode and Interface

A lightweight RL agent runs on regional relays to choose next hops for messages/streams, balancing latency, reliability, and cost/energy. Edges stay simple; relays learn.

## Contract (I/O)

- Inputs per decision:
  - srcPeerId, dstKey (destination DID/AgentId or content key)
  - candidateNextHops: list of neighbor PeerIds with live metrics
  - features for each hop: { rttMs, loss, jitter, pricePerMB, hopReputation, regionMatch }
  - workload: { sizeBytes, qClass (best-effort|regional|backbone), ttlMs }
- Output:
  - chosenNextHop (PeerId)
  - policyScore per hop (for observability)
- Learning signal:
  - reward = - (latency_weight * endToEndRtt + loss_penalty * dropRate + price_weight * cost) + success_bonus

## Q-update (tabular or linear approx)

We maintain Q(s, a) where state s is a compact feature vector and action a is nextHop.

Q(s, a) ← (1 - α) Q(s, a) + α [ r + γ max_a' Q(s', a') ]

- α: learning rate (e.g., 0.1)
- γ: discount (e.g., 0.8 for near-term focus)
- ε-greedy exploration with ε decay per peer/region

## Feature engineering (compact)

s = [
  norm(rttMs),
  norm(loss),
  norm(jitter),
  norm(pricePerMB),
  hopReputation,
  regionMatch (0/1),
  qClassOneHot(3),
  log(sizeBytes),
  ttlMsBucket
]

## Pseudocode

```python
class QRouter:
    def __init__(self, neighbors, config):
        self.neighbors = neighbors  # neighbor -> stats
        self.Q = {}  # dict[(state_hash, neighbor)] -> value
        self.cfg = config  # weights, alpha, gamma, eps

    def select_next_hop(self, src, dst, candidates, workload):
        feats = [self._features(hop, workload) for hop in candidates]
        state = self._encode_state(feats, workload)

        # ε-greedy
        if random() < self.cfg.epsilon:
            choice = random_choice(candidates)
            scores = {hop: 0.0 for hop in candidates}
            scores[choice] = 1.0
            return choice, scores

        # exploit
        scores = {}
        best_hop, best_q = None, -float('inf')
        for hop in candidates:
            q = self.Q.get((state, hop), 0.0)
            scores[hop] = q
            if q > best_q:
                best_q, best_hop = q, hop
        return best_hop or random_choice(candidates), scores

    def update(self, prev_state, action_hop, reward, next_state, next_candidates):
        q_sa = self.Q.get((prev_state, action_hop), 0.0)
        next_max = max([self.Q.get((next_state, h), 0.0) for h in next_candidates] or [0.0])
        td_target = reward + self.cfg.gamma * next_max
        self.Q[(prev_state, action_hop)] = (1 - self.cfg.alpha) * q_sa + self.cfg.alpha * td_target

    def _features(self, hop, workload):
        m = self.neighbors[hop].metrics  # rtt, loss, jitter, price, rep, region
        return [
            norm(m.rtt),
            norm(m.loss),
            norm(m.jitter),
            norm(m.price_per_mb),
            m.reputation,
            1.0 if m.region == workload.region_hint else 0.0,
            *one_hot(workload.qclass, ["best-effort", "regional", "backbone"]),
            log1p(workload.size_bytes),
            bucket(workload.ttl_ms)
        ]

    def _encode_state(self, feats_list, workload):
        # pooled representation, e.g., mean and min across candidates
        mean_feats = mean_pool(feats_list)
        min_feats = min_pool(feats_list)
        return hash_tuple((*mean_feats, *min_feats))
```

## Reward shaping

- success_bonus = +c when dst reached or stream stable for X seconds
- endToEndRtt measured via probes or piggybacked ACKs
- cost computed from pricePerMB * sizeBytes + relay internal costs
- add penalty for policy violations (latency P95 > target, cross-region if forbidden)

## Integration points

- libp2p hook: before dialing next hop, ask QRouter for choice
- Observability: emit chosen hop, ε, Q-values to metrics
- Safety: maintain allow/deny lists; respect privacy/guild boundaries

## Persistence & cold-start

- Persist Q-table per region; decay old entries (EMA)
- Cold-start: initialize per-hop prior using regionMatch and reputation

## Config (defaults)

- alpha=0.1, gamma=0.8, epsilon_start=0.2, epsilon_min=0.02, epsilon_decay=1e-5
- latency_weight=1.0, loss_penalty=3.0, price_weight=0.2, success_bonus=2.0

## Extensions

- Linear function approximation or shallow NN for Q(s,a)
- Contextual bandits for fast adaptation under non-stationarity
- Per-SLA policy heads tuned to each qClass
