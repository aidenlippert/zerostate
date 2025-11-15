# Frontend Documentation Package

## Instructions for Frontend Developer

This package contains all the documentation and resources needed to build the Ainur Protocol documentation site. Please read the `FRONTEND_DEVELOPER_BRIEF.md` first for detailed requirements.

## File Structure

```
ainur-docs/
├── docusaurus.config.ts    # Current Docusaurus configuration
├── package.json            # Dependencies and scripts
├── docs/                   # All markdown documentation
│   ├── README.md          # Main overview (use as base for homepage)
│   ├── GETTING_STARTED.md # Developer quickstart
│   ├── 00_AINUR_PROTOCOL_OVERVIEW.md
│   ├── 01_TEMPORAL_LEDGER.md
│   ├── L3-Aether-Topics-v1.md
│   ├── L5-ARI-v1.md
│   ├── COMPLETE_SPRINT_ROADMAP.md
│   ├── COMPREHENSIVE_FEATURE_BRAINSTORM.md
│   ├── PLANETARY_AI_PROTOCOL_COMPLETE_ARCHITECTURE.md
│   ├── AI_COLLABORATION_BRIEF.md
│   └── [other technical docs...]
└── [other Docusaurus files]
```

## Key Technical Concepts to Highlight

### 1. The 9-Layer Protocol Stack
This is the core architecture that should be visualized prominently:

```
L6: Koinos (Economy) - VCG Auctions, Escrow, Payments
L5.5: Warden (Verification) - TEE + ZK Proofs  
L5: Cognition (Execution) - WASM, ARI Runtime Interface
L4.5: Nexus (HMARL) - Hierarchical Multi-Agent RL
L4: Concordat (Market) - AACL Protocol, Negotiation
L3: Aether (Transport) - P2P Topics, CQ-Routing
L2: Verity (Identity) - DID, Verifiable Credentials
L1.5: Fractal (Sharding) - Horizontal Scaling
L1: Temporal (Blockchain) - Substrate Pallets, Consensus
```

### 2. Core Value Propositions

**For Developers**:
- Build agents in any language (WASM, Python, Docker)
- Fair economic mechanisms (VCG auctions)
- Built-in identity and reputation

**For Enterprises**:
- Private agent networks
- Verifiable execution (TEE + ZK)
- Compliance ready

**For Researchers**:
- Open protocol for experimentation
- State-of-the-art MARL algorithms
- Academic collaboration opportunities

### 3. Key Differentiators

1. **Runtime Agnostic**: Supports WASM, Python, Docker, and more
2. **Strategy-Proof Economics**: VCG auctions ensure fair pricing
3. **Verifiable Execution**: TEE + ZK proofs for trustless operation
4. **Horizontal Scalability**: Sharding by agent capabilities

## Design Inspiration

The documentation should feel:
- **Technical but accessible**: Like Stripe docs
- **Visually sophisticated**: Like Vercel's design system
- **Information-dense but scannable**: Like Rust documentation
- **Interactive where helpful**: Like MDN Web Docs

## Code Examples to Feature

### Quick Start Example
```go
import "github.com/vegalabs/ainur/libs/agentsdk"

type MathAgent struct {
    agentsdk.BaseAgent
}

func (a *MathAgent) HandleTask(task *agentsdk.Task) (*agentsdk.TaskResult, error) {
    // Parse mathematical expression
    result := evaluate(task.Input)
    return &agentsdk.TaskResult{
        Status: "completed",
        Output: result,
    }, nil
}
```

### Runtime Interface Example
```protobuf
service AgentRuntime {
    rpc GetManifest(Empty) returns (AgentManifest);
    rpc ExecuteTask(TaskRequest) returns (TaskResponse);
    rpc GetHealth(Empty) returns (HealthStatus);
}
```

## Interactive Components Ideas

1. **Protocol Layer Explorer**
   - Click each layer to see details
   - Show message flow between layers
   - Highlight current implementation status

2. **Agent Builder Wizard**
   - Step-by-step agent creation
   - Language selection
   - Capability definition
   - Deploy to testnet

3. **Economic Simulator**
   - Visualize VCG auction mechanics
   - Show bid strategies
   - Demonstrate truthful bidding

## Content Priorities

### Must Have
1. Clear explanation of what Ainur is
2. Getting started in under 5 minutes
3. API reference
4. Architecture overview
5. Examples repository link

### Nice to Have
1. Video tutorials
2. Interactive demos
3. Community showcase
4. Benchmarks/performance data

## Technical Requirements

### Search
- Full-text search across all docs
- Algolia DocSearch integration preferred
- Search suggestions and filters

### Navigation
- Persistent sidebar
- Breadcrumbs
- Previous/Next navigation
- Version dropdown (for future)

### Code Blocks
- Syntax highlighting for Go, Rust, Python, TypeScript
- Copy button
- Line highlighting capability
- Filename display

### Responsive Design
- Mobile-first approach
- Tablet optimization
- Desktop with wide margins for readability

## Deployment Notes

- Currently deployed on Netlify
- Domain: ainur.network
- Build command: `npm run build`
- Publish directory: `build`
- Node version: 18+

## Contact for Questions

If you need clarification on any technical concepts or architectural decisions, refer to:
1. The `AI_COLLABORATION_BRIEF.md` for overall vision
2. The whitepaper series for deep technical details
3. The `PLANETARY_AI_PROTOCOL_COMPLETE_ARCHITECTURE.md` for research foundations

Remember: The goal is to make this documentation so good that developers choose Ainur not just for the technology, but because they trust and understand it from day one.
