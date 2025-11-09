# ZeroState Quick Start Guide

Get up and running with ZeroState agent development in 5 minutes!

## Prerequisites

- Docker & Docker Compose
- Go 1.21+
- netcat (nc) - for service health checks
- jq - for JSON parsing in scripts

## Quick Start (3 Commands)

```bash
# 1. Setup local network (PostgreSQL, Redis, API)
./scripts/setup-local-network.sh

# 2. Register the Echo agent
./scripts/register-agent.sh examples/agents/echo-agent/dist/echo-agent.wasm

# 3. Test agent communication
./scripts/test-agent.sh
```

That's it! Your first agent is now running on the ZeroState network.

## What Just Happened?

### 1. Network Setup
- ✅ Started PostgreSQL database
- ✅ Started Redis cache
- ✅ Built and started ZeroState API
- ✅ Created test user account
- ✅ Generated authentication token

### 2. Agent Registration
- ✅ Uploaded Echo Agent WASM binary (5.8MB)
- ✅ Registered agent with capabilities: `echo`, `test`
- ✅ Set pricing: $0.10 per task
- ✅ Agent is now discoverable on network

### 3. Agent Testing
- ✅ Submitted test task with sample data
- ✅ Agent processed task and echoed back input
- ✅ Verified task completion and result

## Next Steps

### Test the Auction System

```bash
./scripts/test-auction.sh
```

This will:
- Register 3 agents with different prices ($0.50, $1.50, $3.00)
- Submit a task requiring `echo` capability
- MetaAgent runs auction with multi-criteria scoring
- Shows which agent won and why

Expected result: **CheapAgent wins** (lowest price with 30% weight)

## Support

- Issues: https://github.com/aidenlippert/zerostate/issues
- Documentation: [docs/](docs/)
- Examples: [examples/agents/](examples/agents/)

Happy building! 🚀
