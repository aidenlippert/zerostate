# 🚀 Ainur Genesis Bootstrap Node

**Status**: Foundation Infrastructure  
**Cost**: $0 (Free Tier)  
**Purpose**: Provide stable bootstrap peers for the Ainur P2P mesh

---

## What is This?

This is a **bootstrap node** (aka "rendezvous point") for the Ainur decentralized agent network.

Think of it as a "phone book" at a party. New guests call you and ask, "Who's here?" You just tell them about 2-3 other people, and they take it from there.

**It does NOT**:
- Store data
- Execute tasks
- Run WASM
- Need a database

**It ONLY**:
- Listens for P2P connections
- Relays gossip messages
- Helps new peers discover each other

**Resource Usage**: ~5 MB RAM, negligible CPU

---

## 🏗️ Foundation Deployment (You Run 2-3 Nodes)

### Option 1: Fly.io (Recommended - Free)

```bash
# Install Fly CLI
curl -L https://fly.io/install.sh | sh

# Login
fly auth login

# Deploy Genesis Node 1 (Virginia)
cd services/bootstrap-node
fly launch --name ainur-genesis-1 --region iad
fly deploy

# Deploy Genesis Node 2 (Amsterdam)
fly launch --name ainur-genesis-2 --region ams
fly deploy

# Deploy Genesis Node 3 (Tokyo)
fly launch --name ainur-genesis-3 --region nrt
fly deploy
```

After deployment, check logs to get your **multiaddresses**:

```bash
fly logs --app ainur-genesis-1
```

You'll see output like:
```
📡 Multiaddresses:
  /ip4/66.241.125.123/tcp/4001/p2p/12D3KooWABC123...
  /ip4/66.241.125.123/udp/4001/quic/p2p/12D3KooWABC123...
```

**Copy these!** These are your "Genesis Multiaddresses" that every runtime will use.

---

### Option 2: Railway.app (Alternative)

```bash
# Install Railway CLI
npm install -g @railway/cli

# Login
railway login

# Deploy
railway up
```

---

### Option 3: Run Locally (For Testing)

```bash
cd services/bootstrap-node
go run main.go
```

---

## 🌍 Community Deployment (Community Runs 7+ Nodes)

### For Community Members

Want to help decentralize Ainur? Run a bootstrap node! It costs $0 and takes 5 minutes.

**Requirements**:
- Free Fly.io or Railway account
- 5 minutes

**Steps**:
1. Fork this repo
2. `cd services/bootstrap-node`
3. `fly launch` (pick your own name like `ainur-community-yourname`)
4. Share your multiaddress in Discord!

The Foundation will add your node to the official bootstrap list.

---

## 📝 Configuration

### Add Bootstrap Nodes to Runtime Config

Edit `reference-runtime-v1/testdata/math-agent.yaml`:

```yaml
p2p:
  enabled: true
  bootstrap:
    - "/dns4/ainur-genesis-1.fly.dev/tcp/4001/p2p/12D3KooW..."
    - "/dns4/ainur-genesis-2.fly.dev/tcp/4001/p2p/12D3KooW..."
    - "/dns4/ainur-genesis-3.fly.dev/tcp/4001/p2p/12D3KooW..."
  presence_topic: "ainur/v1/global/l3_aether/presence"
  heartbeat_interval: 30
```

### Add Bootstrap Nodes to Orchestrator

Set environment variable:

```bash
export P2P_BOOTSTRAP="/dns4/ainur-genesis-1.fly.dev/tcp/4001/p2p/12D3KooW...,/dns4/ainur-genesis-2.fly.dev/tcp/4001/p2p/12D3KooW..."
```

---

## 🎯 The "10 Node" Strategy

### Foundation Nodes (You)
- **Genesis 1** (Virginia) - `ainur-genesis-1.fly.dev`
- **Genesis 2** (Amsterdam) - `ainur-genesis-2.fly.dev`  
- **Genesis 3** (Tokyo) - `ainur-genesis-3.fly.dev`

**Cost**: $0 (Fly.io free tier)

### Community Nodes (Beta Testers)
- 7+ nodes run by early adopters
- Geographic diversity (Africa, South America, India, etc.)
- Each on free-tier cloud

**Cost**: $0 (Community provides)

### Result
- 10+ globally distributed bootstrap nodes
- Zero infrastructure cost
- True decentralization
- Resilient to any single failure

---

## 🔍 Monitoring

Check node health:

```bash
# Fly.io
fly status --app ainur-genesis-1
fly logs --app ainur-genesis-1

# Local
curl http://localhost:4001/debug/pprof/
```

---

## 🚨 Troubleshooting

**Q: My node keeps crashing**  
A: Check logs. Usually a port conflict. Try changing the port in fly.toml.

**Q: No peers connecting**  
A: Make sure TCP port 4001 is exposed. Check firewall rules.

**Q: High memory usage**  
A: Bootstrap nodes should use <50 MB. If higher, there may be a gossip loop. Restart.

---

## 📚 Learn More

- [libp2p Bootstrap Docs](https://docs.libp2p.io/concepts/discovery-routing/bootstrap/)
- [GossipSub Spec](https://github.com/libp2p/specs/blob/master/pubsub/gossipsub/README.md)
- [Fly.io Docs](https://fly.io/docs/)

---

## 🎉 Become a Founding Member

Run a bootstrap node and get:
- Listed on the official Ainur website
- "Founding Member" Discord role  
- NFT badge (coming soon)
- Eternal gratitude of the decentralized agent community

**Deploy now**: `fly launch --name ainur-community-yourname`

---

**Built with ❤️ by the Ainur Foundation**
