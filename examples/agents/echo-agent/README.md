# Echo Agent

Simple echo agent for testing and learning ZeroState agent development.

## Features

- **Echo Capability** - Echoes back any input data
- **Test Capability** - Basic testing functionality
- **Low Cost** - Only $0.10 per task
- **High Performance** - Handles 100 TPS, 10 concurrent tasks

## Quick Start

### 1. Build WASM Binary

```bash
chmod +x build.sh
./build.sh
```

This creates `dist/echo-agent.wasm` ready for deployment.

### 2. Test Locally

```bash
go run main.go
```

### 3. Register on Network

First, get an auth token:

```bash
# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -d '{"email":"your@email.com","password":"yourpassword"}' | jq -r '.access_token')
```

Then register the agent:

```bash
curl -X POST http://localhost:8080/api/v1/agents/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "wasm_binary=@dist/echo-agent.wasm" \
  -F "name=EchoAgent" \
  -F "description=Simple echo agent for testing" \
  -F "version=1.0.0" \
  -F "capabilities=echo,test" \
  -F "price=0.10"
```

### 4. Test Agent

Submit a test task:

```bash
curl -X POST http://localhost:8080/api/v1/tasks/submit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "capabilities": ["echo"],
    "input": {
      "message": "Hello, ZeroState!",
      "data": {"test": 123}
    },
    "budget": 1.0
  }'
```

Expected response:

```json
{
  "echo": {
    "message": "Hello, ZeroState!",
    "data": {"test": 123}
  },
  "message": "Successfully echoed your input!",
  "timestamp": 1704067200,
  "agent_name": "EchoAgent",
  "agent_version": "1.0.0"
}
```

## Agent Configuration

```go
Name:        "EchoAgent"
Version:     "1.0.0"
Price:       $0.10 per task
MinBudget:   $0.05
MaxConcurrent: 10 tasks
TaskTimeout:  30 seconds
```

## Capabilities

### echo (v1.0.0)
- **Description**: Echoes back input data
- **Cost**: $0.10 per task
- **Limits**: 100 TPS, 10 concurrent

### test (v1.0.0)
- **Description**: Test capability for development
- **Cost**: $0.01 per task

## Code Structure

```
echo-agent/
├── main.go       # Agent implementation
├── build.sh      # Build script for WASM
├── README.md     # This file
└── dist/         # Build output (created by build.sh)
    └── echo-agent.wasm
```

## Development

### Running Tests

```bash
go test -v
```

### Local Development

```bash
# Run agent locally
go run main.go

# Build for testing
go build -o echo-agent main.go
./echo-agent
```

### Debugging

Enable debug logging by setting `LogLevel: "debug"` in config.

## Example Use Cases

1. **Testing Network** - Verify agent registration and communication
2. **Load Testing** - Benchmark agent selection and auction system
3. **Learning** - Understand agent development workflow
4. **Integration Testing** - Test task submission and result handling

## Next Steps

- Try modifying the echo logic
- Add custom capabilities
- Build more complex agents (see `../image-processor/`)
- Test agent-to-agent communication

## License

See [LICENSE](../../../LICENSE)
