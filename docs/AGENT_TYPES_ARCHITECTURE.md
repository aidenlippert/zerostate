# ZeroState Agent Types Architecture

## Overview

ZeroState supports **4 types of AI agents** to maximize flexibility and adoption:

1. **WASM Agents** - Sandboxed, portable, runs anywhere
2. **API/Endpoint Agents** - CrewAI, LangChain, or custom hosted services
3. **Container Agents** - Docker images for complex agents
4. **Hybrid Agents** - Combination of WASM + API calls

---

## Agent Type Comparison

| Feature | WASM | API/Endpoint | Container | Hybrid |
|---------|------|--------------|-----------|---------|
| **Portability** | ✅ Excellent | ⚠️ Depends on hosting | ⚠️ Requires runtime | ✅ Good |
| **Security** | ✅ Sandboxed | ⚠️ Network calls | ⚠️ Isolation needed | ⚠️ Mixed |
| **Performance** | ✅ Fast | ⚠️ Network latency | ✅ Native speed | ⚠️ Mixed |
| **Ease of Use** | ⚠️ Compile to WASM | ✅ Just provide URL | ✅ Deploy anywhere | ⚠️ Complex |
| **Cost** | 💰 Low | 💰💰 Medium | 💰💰💰 High | 💰💰 Medium |
| **Best For** | Simple logic, data processing | Existing LLM pipelines | Heavy ML models | Complex workflows |

---

## 1. WASM Agents (Current Implementation)

### How It Works
```
User → Upload .wasm file → ZeroState stores in R2 → Execute in WASI runtime
```

### Registration
```bash
curl -X POST /api/v1/agents/register \
  -H "Authorization: Bearer $TOKEN" \
  -F "wasm_binary=@agent.wasm" \
  -F 'agent={
    "name": "My WASM Agent",
    "type": "wasm",
    "capabilities": ["text-processing"],
    "pricing": {"per_execution": 0.001}
  }'
```

### Pros
- ✅ Sandboxed (secure by default)
- ✅ Platform-independent
- ✅ No hosting costs for users
- ✅ Fast execution

### Cons
- ⚠️ Requires compilation to WASM
- ⚠️ Limited to WASI capabilities
- ⚠️ Not ideal for large ML models

---

## 2. API/Endpoint Agents (NEW - For CrewAI/LangChain)

### How It Works
```
User → Register endpoint URL → ZeroState validates → Routes tasks to endpoint
```

### Use Cases
- **CrewAI agents** running on Railway/Render/Fly.io
- **LangChain chains** deployed as FastAPI services
- **Locally hosted LLMs** (with ngrok/Cloudflare Tunnel)
- **Custom AI services** (AutoGPT, BabyAGI, etc.)

### Registration
```bash
curl -X POST /api/v1/agents/register \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CrewAI Research Agent",
    "type": "endpoint",
    "endpoint": {
      "url": "https://my-crewai-agent.railway.app/execute",
      "method": "POST",
      "auth_type": "bearer",  // or "api_key", "none"
      "timeout": 300,
      "health_check_url": "https://my-crewai-agent.railway.app/health"
    },
    "capabilities": ["research", "web-search", "summarization"],
    "pricing": {"per_execution": 0.01, "per_second": 0.001}
  }'
```

### Request/Response Format (Standard)
```json
// ZeroState → Agent Endpoint
POST /execute
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "query": "Research quantum computing trends",
  "context": {"max_results": 10},
  "timeout": 300
}

// Agent Endpoint → ZeroState
{
  "status": "success",
  "result": "Research findings...",
  "metadata": {
    "execution_time_ms": 12500,
    "tokens_used": 3400,
    "sources": ["arxiv.org", "nature.com"]
  }
}
```

### Locally Hosted AI Setup

#### Option 1: Cloudflare Tunnel (Recommended)
```bash
# Install cloudflared
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o cloudflared
chmod +x cloudflared

# Run your local agent
python my_crewai_agent.py --port 8000 &

# Create tunnel
cloudflared tunnel --url http://localhost:8000

# Register with ZeroState using the https://...trycloudflare.com URL
```

#### Option 2: ngrok
```bash
# Start your agent
python langchain_agent.py --port 5000 &

# Expose via ngrok
ngrok http 5000

# Use the ngrok HTTPS URL for registration
```

#### Option 3: Direct IP (Not Recommended for Production)
```bash
# Only for testing - requires public IP and firewall config
# Use Cloudflare Tunnel or ngrok instead!
```

### Validation & Health Checks
- ZeroState pings `health_check_url` every 5 minutes
- If 3 consecutive failures → Agent marked as "unavailable"
- User notified via email/dashboard
- Agent auto-reactivated when health check passes

---

## 3. Container Agents (For Complex ML Models)

### How It Works
```
User → Provide Dockerfile/image → ZeroState builds/pulls → Run on-demand
```

### Use Cases
- **Hugging Face models** (too large for WASM)
- **Stable Diffusion** image generation
- **Whisper** speech recognition
- **Custom LLMs** (Llama, Mistral, etc.)

### Registration
```bash
curl -X POST /api/v1/agents/register \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Stable Diffusion XL Agent",
    "type": "container",
    "container": {
      "image": "ghcr.io/myusername/sdxl-agent:latest",
      "registry_auth": "ghp_xxxxxxxxxxxx",
      "resources": {
        "cpu_limit": "2000m",
        "memory_limit": "4Gi",
        "gpu_required": true
      },
      "env_vars": {
        "MODEL_PATH": "/models/sdxl",
        "BATCH_SIZE": "1"
      }
    },
    "capabilities": ["image-generation"],
    "pricing": {"per_execution": 0.25}
  }'
```

### Execution Flow
1. Task received → Check if container running
2. If not running → Pull image, start container (cold start ~30s)
3. Send task via HTTP to container's `/execute` endpoint
4. Container processes and returns result
5. Keep container warm for 5 minutes (configurable)

---

## 4. Hybrid Agents (Best of Both Worlds)

### How It Works
```
WASM handles orchestration → Calls external APIs for heavy lifting
```

### Example: Research Agent
- **WASM part**: Query parsing, result aggregation, caching
- **API calls**: Google Search, OpenAI, specialized databases

### Registration
```bash
curl -X POST /api/v1/agents/register \
  -H "Authorization: Bearer $TOKEN" \
  -F "wasm_binary=@orchestrator.wasm" \
  -F 'agent={
    "name": "Hybrid Research Agent",
    "type": "hybrid",
    "external_apis": [
      {"name": "openai", "url": "https://api.openai.com/v1/chat/completions"},
      {"name": "serp", "url": "https://serpapi.com/search"}
    ],
    "capabilities": ["research", "llm-reasoning"],
    "pricing": {"per_execution": 0.05, "per_api_call": 0.002}
  }'
```

---

## Database Schema Updates

### agents table (enhanced)
```sql
CREATE TABLE agents (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,

  -- Agent Type (new)
  agent_type VARCHAR(20) NOT NULL, -- 'wasm', 'endpoint', 'container', 'hybrid'

  -- WASM-specific fields
  wasm_hash VARCHAR(64),
  wasm_url TEXT,
  wasm_size_bytes BIGINT,

  -- Endpoint-specific fields (new)
  endpoint_url TEXT,
  endpoint_method VARCHAR(10), -- 'POST', 'GET'
  endpoint_auth_type VARCHAR(20), -- 'bearer', 'api_key', 'none'
  endpoint_auth_secret TEXT, -- encrypted
  health_check_url TEXT,
  last_health_check TIMESTAMP,
  health_status VARCHAR(20), -- 'healthy', 'unhealthy', 'unknown'

  -- Container-specific fields (new)
  container_image TEXT,
  container_registry TEXT,
  container_registry_auth TEXT, -- encrypted
  container_resources JSONB,
  container_env_vars JSONB,

  -- Hybrid-specific fields (new)
  external_apis JSONB,

  -- Common fields
  capabilities TEXT[],
  pricing JSONB NOT NULL,
  resources JSONB,
  metadata JSONB,
  status VARCHAR(20) DEFAULT 'active',

  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_agents_type ON agents(agent_type);
CREATE INDEX idx_agents_user_type ON agents(user_id, agent_type);
CREATE INDEX idx_agents_health ON agents(health_status) WHERE agent_type = 'endpoint';
```

---

## Implementation Plan

### Phase 1: API/Endpoint Agents (Week 1-2) ✅ PRIORITY
**Why First?**: Easiest adoption - users already have CrewAI/LangChain agents running

1. Update database schema
2. Create `/api/v1/agents/register` endpoint variant for type="endpoint"
3. Implement health check worker (checks every 5 min)
4. Update task router to support endpoint calls
5. Add retry logic + circuit breaker
6. Document CrewAI/LangChain integration examples

### Phase 2: Fix WASM Upload Size (Week 1) ✅ CRITICAL
1. Increase `MaxWASMSize` in agent_handlers.go
2. Test with real 5.8MB WASM file
3. Verify R2 upload works end-to-end

### Phase 3: Container Agents (Week 3-4)
1. Integrate with Fly.io Machines API for on-demand containers
2. Implement cold start optimization
3. Add GPU support detection
4. Create container registry integration (GHCR, Docker Hub)

### Phase 4: Hybrid Agents (Week 5-6)
1. Extend WASM runtime with HTTP client capability
2. Add API call metering
3. Implement credential management for external APIs

---

## Security Considerations

### Endpoint Agents
- ✅ HTTPS required (reject HTTP endpoints)
- ✅ Rate limiting per agent (prevent abuse)
- ✅ Timeout enforcement (max 5 minutes)
- ✅ Response size limits (max 10MB)
- ✅ Credential encryption (API keys, tokens)
- ⚠️ SSRF protection (block private IPs, localhost)
- ⚠️ Request signing (verify responses aren't tampered)

### Container Agents
- ✅ Sandboxed execution (no network access to ZeroState internals)
- ✅ Resource limits (CPU, memory, GPU time)
- ✅ Registry authentication
- ⚠️ Image scanning for vulnerabilities
- ⚠️ Secrets management (env vars encrypted at rest)

---

## Pricing Models by Type

### WASM Agents
- **Per Execution**: Fixed cost (e.g., $0.001)
- **Per Second**: For long-running tasks
- **Per MB**: For data processing

### Endpoint Agents
- **Per Execution**: Base cost
- **Per Second**: API hosting costs
- **Per Token**: If using LLMs
- **Tiered**: Volume discounts

### Container Agents
- **Per Execution**: Includes compute + cold start amortization
- **GPU Minutes**: Separate pricing for GPU usage
- **Storage**: For model caching

---

## Example: Registering a CrewAI Agent

```python
# 1. Build your CrewAI agent (runs locally or on Railway)
from crewai import Agent, Task, Crew
from flask import Flask, request, jsonify

app = Flask(__name__)

researcher = Agent(
    role='Senior Research Analyst',
    goal='Uncover cutting-edge developments in AI',
    tools=[search_tool, scraper_tool]
)

@app.route('/health')
def health():
    return jsonify({"status": "healthy"})

@app.route('/execute', methods=['POST'])
def execute():
    data = request.json
    task = Task(description=data['query'], agent=researcher)
    crew = Crew(agents=[researcher], tasks=[task])
    result = crew.kickoff()

    return jsonify({
        "status": "success",
        "result": str(result),
        "metadata": {"execution_time_ms": 5000}
    })

if __name__ == '__main__':
    app.run(port=8000)

# 2. Deploy to Railway/Render/Fly.io (or use Cloudflare Tunnel for local)

# 3. Register with ZeroState
import requests

response = requests.post(
    'https://zerostate-api.fly.dev/api/v1/agents/register',
    headers={'Authorization': f'Bearer {token}'},
    json={
        'name': 'AI Research Crew',
        'type': 'endpoint',
        'endpoint': {
            'url': 'https://my-crewai-agent.railway.app/execute',
            'method': 'POST',
            'auth_type': 'none',
            'health_check_url': 'https://my-crewai-agent.railway.app/health'
        },
        'capabilities': ['research', 'web-search'],
        'pricing': {'per_execution': 0.05}
    }
)

print(response.json())
# {"agent_id": "abc-123", "status": "active", "health_status": "healthy"}
```

---

## Next Steps

1. **Immediate**: Fix WASM upload size issue (increase MaxWASMSize)
2. **Week 1-2**: Implement endpoint agent support (highest value for users)
3. **Week 3**: Test end-to-end with CrewAI + LangChain examples
4. **Week 4**: Container agent support (for ML models)
5. **Week 5-6**: Hybrid agents + advanced features

**This architecture makes ZeroState the most flexible AI agent marketplace!**
