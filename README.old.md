# zerostate — Hybrid P2P Network (Blueprint)

A practical, buildable architecture for a planetary-scale agent network:
- P2P-first at the edge (identity, discovery, message passing)
- Regional relays for low-latency caching, vector search shards, and Q-routing
- Backbone nodes for archival storage, consensus zones, and GPU compute

Branding: project name is "zerostate".

## Start here

- Architecture overview: `docs/architecture.md`
- Q-routing (relay policy): `docs/routing_q_agent.md`
- Schemas:
  - Agent Card: `specs/agent_card.schema.json` (example: `examples/agent_card.example.json`)
  - Task Manifest: `specs/task_manifest.schema.json` (example: `examples/task_manifest.example.json`)
- Planning:
  - MVP sprint plan: `docs/plan/sprint_plan.md`
  - Sprint 1 task breakdown: `docs/plan/sprint1_tasks.md`

## What’s included

- Mermaid diagrams of the network tiers and guild lifecycle
- JSON Schemas for agent identity/ads and task requests
- Pseudocode for a relay-side Q-routing agent (RL-based)

## Next deliverables (proposed)

- MVP sprint plan (12 weeks, epics and milestones)
- Sybil resistance & staking economic model (slashing rules, parameters)
- Federated HNSW query protocol sketch (ANN routing + DHT validation)

## Contributing

- Open issues for questions/feedback
- Prefer small, focused PRs; include examples/tests where possible

## License

TBD (MIT or Apache-2 suggested)
