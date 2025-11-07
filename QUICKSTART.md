# ZeroState Platform - Quick Start Guide

Get the ZeroState AI orchestration platform running in under 2 minutes! 🚀

---

## Prerequisites

- **Go 1.24+** installed
- **Git** for cloning the repository
- **Modern web browser** (Chrome, Firefox, Safari, or Edge)

---

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/aidenlippert/zerostate.git
cd zerostate
```

### 2. Install Dependencies

```bash
go mod tidy
```

---

## Running the Server

### Quick Start (Default Configuration)

```bash
go run cmd/api/main.go
```

The server will start with:
- **Host**: 0.0.0.0
- **Port**: 8080
- **Workers**: 5 orchestrator workers
- **Logging**: Production mode

### Custom Configuration

```bash
# Change port
go run cmd/api/main.go -port 3000

# Enable debug logging
go run cmd/api/main.go -debug

# Custom workers
go run cmd/api/main.go -workers 10

# Combine options
go run cmd/api/main.go -port 3000 -workers 10 -debug
```

### Available Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | 0.0.0.0 | Server bind address |
| `-port` | 8080 | Server port |
| `-workers` | 5 | Number of orchestrator workers |
| `-debug` | false | Enable debug logging |

---

## Accessing the Platform

Once the server is running, you'll see a beautiful startup banner:

```
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║              🚀 ZeroState API Server Running 🚀              ║
║                                                              ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  Web UI:          http://localhost:8080                     ║
║  API Endpoints:   http://localhost:8080/api/v1              ║
║  Health Check:    http://localhost:8080/health              ║
║  Metrics:         http://localhost:8080/metrics             ║
║                                                              ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  Orchestrator:    5 workers active                          ║
║  P2P Node:        12D3KooWABC123...                         ║
║  DID:             did:key:z6Mkr...                          ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
```

### Access Points

1. **Web UI**: Open [http://localhost:8080](http://localhost:8080) in your browser
2. **API Documentation**: See [docs/](docs/) folder for API details
3. **Health Check**: [http://localhost:8080/health](http://localhost:8080/health)
4. **Prometheus Metrics**: [http://localhost:8080/metrics](http://localhost:8080/metrics)

---

## Using the Web UI

### 1. Dashboard

Navigate to the homepage to see:
- Real-time orchestrator metrics
- Recent task activity
- System health status

### 2. Submit a Task

Click "Tasks" → "New Task" or navigate to `/submit-task`:

1. Enter your query (e.g., "Analyze sentiment of this text...")
2. Select priority (Low, Normal, High, Critical)
3. Set budget (e.g., 1.00)
4. Set timeout (optional, default: 30 seconds)
5. Click "Submit Task"

### 3. Monitor Tasks

Navigate to "Tasks" to:
- View all submitted tasks
- See task status (Pending, Running, Completed, Failed)
- Click any task to see detailed information

### 4. View Metrics

Navigate to "Metrics" to see:
- Tasks processed, succeeded, failed
- Success rate percentage
- Average execution time
- Active workers count

---

## Example: Your First Task

### Using the Web UI

1. **Start the server**:
   ```bash
   go run cmd/api/main.go
   ```

2. **Open browser** to [http://localhost:8080](http://localhost:8080)

3. **Click "Tasks"** in the navigation

4. **Click "New Task"** button

5. **Fill in the form**:
   - Query: "What is the capital of France?"
   - Priority: Normal
   - Budget: 1.00
   - Timeout: 30

6. **Click "Submit Task"**

7. **View your task** in the task list - it will show "Queued" → "Running" → "Completed"

8. **Click the task** to see the result!

### Using the API

```bash
# Submit a task
curl -X POST http://localhost:8080/api/v1/tasks/submit \
  -H "Content-Type: application/json" \
  -d '{
    "query": "What is the capital of France?",
    "priority": "normal",
    "budget": 1.00,
    "timeout": 30
  }'

# Response:
# {
#   "task_id": "550e8400-e29b-41d4-a716-446655440000",
#   "status": "queued"
# }

# Get task status
curl http://localhost:8080/api/v1/tasks/{task_id}/status

# Get task result (when completed)
curl http://localhost:8080/api/v1/tasks/{task_id}/result
```

---

## Stopping the Server

Press **Ctrl+C** in the terminal running the server. You'll see:

```
🛑 Shutting down gracefully...
   ⏸  Stopping orchestrator...
   ✅ Orchestrator stopped
   ⏸  Stopping API server...
   ✅ API server stopped
   ⏸  Closing task queue...
   ✅ Task queue closed
   ⏸  Closing p2p host...
   ✅ P2P host closed

✨ Shutdown complete. Goodbye!
```

The server performs graceful shutdown, ensuring:
- All in-flight tasks complete
- Workers shut down cleanly
- Connections close properly

---

## Building for Production

### Create Binary

```bash
# Build optimized binary
go build -o bin/zerostate-api cmd/api/main.go

# Run the binary
./bin/zerostate-api
```

### Production Configuration

```bash
# Run with production settings
./bin/zerostate-api \
  -host 0.0.0.0 \
  -port 8080 \
  -workers 10
```

---

## Troubleshooting

### Port Already in Use

If port 8080 is already in use:

```bash
# Use a different port
go run cmd/api/main.go -port 3000
```

### Cannot Access Web UI

1. Check the server is running
2. Verify the port in the startup banner
3. Try `http://127.0.0.1:8080` instead of `localhost`
4. Check firewall settings

### Tasks Not Processing

1. Check orchestrator workers are active (see startup banner)
2. Enable debug logging: `go run cmd/api/main.go -debug`
3. Check logs for errors
4. Verify task queue is not full (default capacity: 1000)

### Build Errors

```bash
# Clean and rebuild
go clean -modcache
go mod tidy
go build cmd/api/main.go
```

---

## Next Steps

### Learn More

- **API Documentation**: See [docs/SPRINT_7_WEEK2_COMPLETE.md](docs/SPRINT_7_WEEK2_COMPLETE.md)
- **Orchestrator Details**: See [docs/SPRINT_7_WEEK3_COMPLETE.md](docs/SPRINT_7_WEEK3_COMPLETE.md)
- **Web UI Guide**: See [web/README.md](web/README.md)
- **Architecture**: See [docs/SPRINT_7_COMPLETE.md](docs/SPRINT_7_COMPLETE.md)

### Advanced Topics

- **Agent Registration**: Coming in Sprint 8
- **User Authentication**: Coming in Sprint 8
- **Database Integration**: Coming in Sprint 8
- **WebSocket Real-Time Updates**: Coming in Sprint 8

### Contributing

Want to contribute? Check out:
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [docs/](docs/) - Complete documentation
- [GitHub Issues](https://github.com/aidenlippert/zerostate/issues) - Report bugs or suggest features

---

## System Requirements

### Minimum
- **OS**: Linux, macOS, or Windows
- **CPU**: 2 cores
- **RAM**: 512MB
- **Disk**: 100MB

### Recommended
- **OS**: Linux (Ubuntu 22.04+) or macOS
- **CPU**: 4+ cores
- **RAM**: 2GB+
- **Disk**: 1GB+

---

## Support

Need help?

- **Documentation**: Check the [docs/](docs/) folder
- **Issues**: [GitHub Issues](https://github.com/aidenlippert/zerostate/issues)
- **Discussions**: [GitHub Discussions](https://github.com/aidenlippert/zerostate/discussions)

---

## Quick Reference Card

```bash
# Start server (default settings)
go run cmd/api/main.go

# Start with custom port
go run cmd/api/main.go -port 3000

# Enable debug mode
go run cmd/api/main.go -debug

# More workers for higher throughput
go run cmd/api/main.go -workers 10

# Build production binary
go build -o bin/zerostate-api cmd/api/main.go

# Run production binary
./bin/zerostate-api

# Stop server
Ctrl+C
```

**Web UI**: http://localhost:8080
**API**: http://localhost:8080/api/v1
**Health**: http://localhost:8080/health
**Metrics**: http://localhost:8080/metrics

---

**Welcome to ZeroState!** 🎉

You're now running a production-ready AI orchestration platform. Enjoy! 🚀
