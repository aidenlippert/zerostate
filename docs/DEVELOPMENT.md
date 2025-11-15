# Ainur Protocol Development Guide

This guide covers development workflows, project structure, contribution guidelines, and best practices for the Ainur Protocol ecosystem.

## Table of Contents

1. [Project Structure Overview](#project-structure-overview)
2. [Development Environment Setup](#development-environment-setup)
3. [Building and Testing Locally](#building-and-testing-locally)
4. [Architecture Overview](#architecture-overview)
5. [Code Contribution Guidelines](#code-contribution-guidelines)
6. [Adding New Pallets](#adding-new-pallets)
7. [Testing Guidelines](#testing-guidelines)
8. [Architecture Decision Records](#architecture-decision-records)
9. [Development Workflows](#development-workflows)
10. [Debugging and Profiling](#debugging-and-profiling)

## Project Structure Overview

```
zerostate/
├── chain-v2/                    # Substrate blockchain
│   ├── node/                    # Node implementation
│   ├── runtime/                 # Runtime configuration
│   ├── pallets/                 # Custom pallets
│   │   ├── did/                 # Decentralized Identity
│   │   ├── registry/            # Agent registry
│   │   ├── reputation/          # Reputation system
│   │   ├── vcg-auction/        # VCG auction mechanism
│   │   └── escrow/             # Payment escrow
│   └── docs/                   # Substrate-specific docs
├── libs/                       # Shared Go libraries
│   ├── api/                    # HTTP/WebSocket API
│   ├── orchestration/          # Task orchestration
│   ├── p2p/                    # Peer-to-peer networking
│   ├── substrate/              # Blockchain integration
│   ├── execution/              # WASM execution
│   ├── storage/                # R2/S3 storage
│   ├── database/               # PostgreSQL integration
│   ├── identity/               # DID and signing
│   ├── economic/               # Economic mechanisms
│   ├── reputation/             # Reputation calculations
│   ├── payment/                # Payment processing
│   ├── websocket/              # Real-time communication
│   ├── metrics/                # Prometheus metrics
│   └── llm/                    # LLM integrations
├── cmd/                        # Entry points
│   ├── api/                    # Main API server
│   └── test-blockchain/        # Blockchain testing
├── web/                        # React frontend
│   ├── src/                    # React components
│   ├── public/                 # Static assets
│   └── package.json            # Dependencies
├── scripts/                    # Deployment/utility scripts
├── tests/                      # Integration tests
├── docs/                       # Documentation
│   ├── API.md                  # API documentation
│   ├── DEPLOYMENT.md           # Deployment guide
│   ├── OPERATIONS.md           # Operations guide
│   └── architecture/           # Architecture diagrams
├── examples/                   # SDK examples
├── agents/                     # Example WASM agents
├── contracts/                  # Smart contracts
└── deployments/               # Deployment configurations
```

### Core Components

#### 1. Substrate Blockchain (chain-v2/)
- **Runtime**: Custom runtime with specialized pallets
- **Node**: Network node implementation with RPC endpoints
- **Pallets**: Custom business logic modules

#### 2. Orchestrator API (libs/api/, cmd/api/)
- **HTTP API**: RESTful endpoints for web and mobile clients
- **WebSocket**: Real-time communication for task updates
- **Middleware**: Authentication, rate limiting, logging

#### 3. P2P Network (libs/p2p/)
- **Discovery**: Agent discovery and network topology
- **Messaging**: Protocol for inter-node communication
- **Gossip**: Market information propagation

#### 4. Economic Layer (libs/economic/)
- **Auctions**: VCG auction implementation
- **Escrow**: Payment security mechanisms
- **Reputation**: Trust and quality scoring

## Development Environment Setup

### Prerequisites

#### System Requirements
```bash
# Operating System
# - Linux (Ubuntu 20.04+ recommended)
# - macOS (Big Sur+ recommended)
# - Windows (WSL2 required)

# Hardware Minimum
# - 4 CPU cores
# - 8GB RAM
# - 20GB free disk space
# - Stable internet connection
```

#### Required Software

**1. Rust Development Environment**
```bash
# Install rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env

# Configure Rust toolchain
rustup default stable
rustup update
rustup update nightly
rustup target add wasm32-unknown-unknown --toolchain nightly

# Install additional tools
cargo install cargo-watch
cargo install cargo-audit
cargo install substrate-contracts-node
```

**2. Go Development Environment**
```bash
# Install Go 1.21+
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
go version
```

**3. Node.js Development Environment**
```bash
# Install Node.js 18+
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs

# Install Yarn (optional)
npm install -g yarn

# Verify installation
node --version
npm --version
```

**4. Database Setup**
```bash
# Install PostgreSQL
sudo apt install postgresql postgresql-contrib

# Start PostgreSQL service
sudo systemctl start postgresql
sudo systemctl enable postgresql

# Create development database
sudo -u postgres createuser --interactive $USER
sudo -u postgres createdb ainur_dev
```

**5. Development Tools**
```bash
# Git (version control)
sudo apt install git

# Docker (for containerization)
sudo apt install docker.io docker-compose
sudo usermod -aG docker $USER

# Additional tools
sudo apt install jq curl wget build-essential pkg-config libssl-dev
```

### Environment Configuration

#### 1. Clone Repository
```bash
git clone https://github.com/aidenlippert/zerostate.git
cd zerostate

# Set up Git hooks
git config core.hooksPath .githooks
chmod +x .githooks/*
```

#### 2. Development Environment File
Create `.env.development`:
```bash
# Database
DATABASE_URL="postgresql://username:password@localhost:5432/ainur_dev"

# JWT
JWT_SECRET="development-secret-key-change-in-production"

# Server
HOST="localhost"
PORT="8080"
LOG_LEVEL="debug"

# Storage (use local/test values)
R2_ENDPOINT="http://localhost:9000"  # MinIO for local development
R2_ACCESS_KEY_ID="minioadmin"
R2_SECRET_ACCESS_KEY="minioadmin"
R2_BUCKET_NAME="ainur-dev"

# LLM (optional for development)
GROQ_API_KEY="your-groq-key-for-testing"

# Blockchain
SUBSTRATE_RPC_URL="ws://localhost:9944"
```

#### 3. IDE Configuration

**Visual Studio Code Setup**
```json
// .vscode/settings.json
{
  "rust-analyzer.checkOnSave.command": "clippy",
  "rust-analyzer.cargo.allFeatures": true,
  "go.toolsManagement.checkForUpdates": "local",
  "go.useLanguageServer": true,
  "typescript.preferences.quoteStyle": "single",
  "editor.formatOnSave": true,
  "editor.rulers": [100],
  "files.exclude": {
    "**/target": true,
    "**/node_modules": true
  }
}
```

**Recommended Extensions**
```json
// .vscode/extensions.json
{
  "recommendations": [
    "rust-lang.rust-analyzer",
    "golang.go",
    "bradlc.vscode-tailwindcss",
    "esbenp.prettier-vscode",
    "ms-vscode.vscode-json",
    "redhat.vscode-yaml"
  ]
}
```

### Local Services Setup

#### 1. Start PostgreSQL
```bash
# Start PostgreSQL
sudo systemctl start postgresql

# Create development database
createdb ainur_dev

# Verify connection
psql ainur_dev -c "SELECT version();"
```

#### 2. Start MinIO (S3-compatible storage)
```bash
# Using Docker
docker run -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"

# Create bucket
mc alias set local http://localhost:9000 minioadmin minioadmin
mc mb local/ainur-dev
```

## Building and Testing Locally

### Building the Blockchain

#### 1. Build Substrate Node
```bash
cd chain-v2

# Debug build (faster compilation)
cargo build

# Release build (optimized)
cargo build --release

# Build with runtime benchmarks (for testing)
cargo build --release --features runtime-benchmarks
```

#### 2. Run Development Node
```bash
# Start development chain
./target/release/solochain-template-node --dev --rpc-cors all

# Or with detailed logging
RUST_BACKTRACE=1 ./target/release/solochain-template-node --dev -ldebug --rpc-cors all

# The node will start on:
# - WebSocket: ws://localhost:9944
# - HTTP RPC: http://localhost:9933
# - P2P: /ip4/127.0.0.1/tcp/30333
```

#### 3. Verify Blockchain
```bash
# Check node health
curl -H "Content-Type: application/json" \
     -d '{"id":1, "jsonrpc":"2.0", "method":"system_health","params":[]}' \
     http://localhost:9933

# Get node info
curl -H "Content-Type: application/json" \
     -d '{"id":1, "jsonrpc":"2.0", "method":"system_name","params":[]}' \
     http://localhost:9933
```

### Building the Orchestrator API

#### 1. Build Go Application
```bash
# Build debug version
cd cmd/api
go build -o ../../bin/api

# Build with race detection (for testing)
go build -race -o ../../bin/api-race

# Build optimized for production
CGO_ENABLED=0 go build -ldflags "-s -w" -o ../../bin/api-optimized
```

#### 2. Run API Server
```bash
# Run with development settings
./bin/api -debug -workers 3

# Or with specific configuration
./bin/api -host localhost -port 8080 -workers 5 -debug

# Check API health
curl http://localhost:8080/health
```

### Building the Frontend

#### 1. Install Dependencies
```bash
cd web
npm install

# Or using Yarn
yarn install
```

#### 2. Development Server
```bash
# Start development server
npm start

# Or with specific configuration
PORT=3000 REACT_APP_API_URL=http://localhost:8080 npm start

# Build for production
npm run build
```

### Running Integration Tests

#### 1. Blockchain Tests
```bash
cd chain-v2

# Run unit tests
cargo test

# Run tests with output
cargo test -- --nocapture

# Run specific test
cargo test test_reputation_system

# Run tests with coverage
cargo tarpaulin --all-features --workspace
```

#### 2. API Tests
```bash
# Run Go tests
go test ./libs/... -v

# Run tests with race detection
go test -race ./libs/...

# Run specific package tests
go test ./libs/api -v

# Run benchmarks
go test -bench=. ./libs/orchestration
```

#### 3. End-to-End Tests
```bash
# Start all services first
./scripts/start-dev-stack.sh

# Run E2E tests
cd tests
go test -v ./...

# Run specific test suite
go test -v ./integration
```

### Development Workflow Scripts

#### 1. Start Development Stack
```bash
#!/bin/bash
# scripts/start-dev-stack.sh

echo "Starting Ainur development stack..."

# Start PostgreSQL
sudo systemctl start postgresql

# Start MinIO
docker run -d --name minio-dev -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"

# Start Substrate node
cd chain-v2
./target/release/solochain-template-node --dev --rpc-cors all &
SUBSTRATE_PID=$!
cd ..

# Wait for blockchain to start
sleep 10

# Start API server
cd cmd/api
go run . -debug &
API_PID=$!
cd ../..

# Start frontend
cd web
npm start &
FRONTEND_PID=$!
cd ..

echo "Development stack started!"
echo "Blockchain: ws://localhost:9944"
echo "API: http://localhost:8080"
echo "Frontend: http://localhost:3000"
echo "MinIO Console: http://localhost:9001"

# Store PIDs for cleanup
echo $SUBSTRATE_PID > .dev-substrate.pid
echo $API_PID > .dev-api.pid
echo $FRONTEND_PID > .dev-frontend.pid
```

#### 2. Stop Development Stack
```bash
#!/bin/bash
# scripts/stop-dev-stack.sh

echo "Stopping Ainur development stack..."

# Kill processes
if [ -f .dev-substrate.pid ]; then
  kill $(cat .dev-substrate.pid) 2>/dev/null
  rm .dev-substrate.pid
fi

if [ -f .dev-api.pid ]; then
  kill $(cat .dev-api.pid) 2>/dev/null
  rm .dev-api.pid
fi

if [ -f .dev-frontend.pid ]; then
  kill $(cat .dev-frontend.pid) 2>/dev/null
  rm .dev-frontend.pid
fi

# Stop MinIO
docker stop minio-dev 2>/dev/null
docker rm minio-dev 2>/dev/null

echo "Development stack stopped!"
```

## Architecture Overview

### System Architecture

The Ainur Protocol follows a modular, microservices-inspired architecture with clear separation of concerns:

#### Layer 1: Blockchain (Substrate)
```rust
// Runtime composition
pub struct Runtime {
    // Core Substrate pallets
    pub System: frame_system,
    pub Timestamp: pallet_timestamp,
    pub Balances: pallet_balances,

    // Ainur custom pallets
    pub DID: pallet_did,
    pub Registry: pallet_registry,
    pub Reputation: pallet_reputation,
    pub VCGAuction: pallet_vcg_auction,
    pub Escrow: pallet_escrow,
}
```

#### Layer 2: Orchestration (Go)
```go
// Core orchestrator components
type Orchestrator struct {
    taskQueue    *TaskQueue
    agentPool    *AgentPool
    auctioneer   *VCGAuctioneer
    reputation   *ReputationManager
    blockchain   *SubstrateClient
    p2pNetwork   *P2PNetwork
}
```

#### Layer 3: API Gateway (HTTP/WebSocket)
```go
// API server structure
type Server struct {
    router       *gin.Engine
    handlers     *Handlers
    wsHub        *websocket.Hub
    middleware   []gin.HandlerFunc
    metrics      *prometheus.Registry
}
```

### Design Patterns

#### 1. Hexagonal Architecture
```go
// Domain layer (core business logic)
type TaskService interface {
    SubmitTask(ctx context.Context, task *Task) (*TaskResult, error)
    ExecuteTask(ctx context.Context, taskID string) error
}

// Application layer (orchestration)
type TaskOrchestrator struct {
    taskService   TaskService
    auctionService AuctionService
    paymentService PaymentService
}

// Infrastructure layer (external dependencies)
type PostgresTaskRepository struct {
    db *sql.DB
}

func (r *PostgresTaskRepository) SubmitTask(ctx context.Context, task *Task) error {
    // Database implementation
}
```

#### 2. Event-Driven Architecture
```go
// Event system for loose coupling
type EventBus interface {
    Publish(event Event) error
    Subscribe(eventType string, handler EventHandler) error
}

// Example events
type TaskSubmittedEvent struct {
    TaskID    string
    UserID    string
    Timestamp time.Time
}

type AuctionCompletedEvent struct {
    AuctionID string
    Winner    string
    Price     int64
}
```

#### 3. Repository Pattern
```go
// Generic repository interface
type Repository[T any] interface {
    Create(ctx context.Context, entity *T) error
    GetByID(ctx context.Context, id string) (*T, error)
    Update(ctx context.Context, entity *T) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filters Filters) ([]*T, error)
}

// Concrete implementation
type AgentRepository struct {
    db *database.Database
}

func (r *AgentRepository) Create(ctx context.Context, agent *Agent) error {
    query := `INSERT INTO agents (id, did, name, capabilities) VALUES ($1, $2, $3, $4)`
    _, err := r.db.ExecContext(ctx, query, agent.ID, agent.DID, agent.Name, agent.Capabilities)
    return err
}
```

### Key Architectural Decisions

#### Decision 1: Substrate vs Custom Blockchain
**Status**: Accepted
**Context**: Need for flexible, upgradeable blockchain with custom business logic
**Decision**: Use Substrate framework with custom pallets
**Consequences**:
- Pro: Rich ecosystem, proven consensus mechanisms
- Pro: Easy runtime upgrades
- Con: Learning curve for Rust/Substrate
- Con: Larger binary size

#### Decision 2: Go vs Rust for Orchestrator
**Status**: Accepted
**Context**: Need for high-performance, concurrent orchestration layer
**Decision**: Use Go for orchestrator, Rust for blockchain
**Consequences**:
- Pro: Go excellent for concurrent network services
- Pro: Rich ecosystem for web services
- Pro: Team expertise in Go
- Con: Two languages to maintain

#### Decision 3: WebSocket + REST vs GraphQL
**Status**: Accepted
**Context**: Need for real-time updates and flexible queries
**Decision**: REST for commands, WebSocket for real-time events
**Consequences**:
- Pro: Simple, well-understood patterns
- Pro: Easy to implement and debug
- Con: More endpoints to maintain
- Con: Over/under-fetching in some cases

## Code Contribution Guidelines

### Development Process

#### 1. Git Workflow
```bash
# Create feature branch
git checkout -b feature/reputation-enhancements

# Make changes and commit
git add .
git commit -m "feat: add reputation decay mechanism

- Implement time-based reputation decay
- Add decay rate configuration
- Update reputation calculation tests
- Add migration for new reputation table fields"

# Push and create pull request
git push origin feature/reputation-enhancements
```

#### 2. Commit Message Format
```
type(scope): brief description

Longer description of what changed and why.

- Bullet point 1
- Bullet point 2

Closes #123
```

**Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`
**Scopes**: `api`, `blockchain`, `frontend`, `orchestration`, `p2p`, etc.

#### 3. Pull Request Guidelines

**PR Title Format**
```
[Type] Brief description of changes
```

**PR Description Template**
```markdown
## Summary
Brief description of what this PR does.

## Changes
- [ ] Added new feature X
- [ ] Fixed bug Y
- [ ] Updated documentation

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Breaking Changes
- [ ] None
- [ ] Listed below:

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Tests added for new functionality
- [ ] Documentation updated
```

### Code Quality Standards

#### 1. Rust Code Standards
```rust
// Use descriptive names
fn calculate_reputation_score(agent_id: &str, reviews: &[Review]) -> Result<f64, ReputationError> {
    // Implementation
}

// Prefer explicit error handling
match reputation_service.get_score(agent_id) {
    Ok(score) => Ok(score),
    Err(ReputationError::NotFound) => Ok(0.0), // Default score for new agents
    Err(e) => Err(e),
}

// Use appropriate visibility
pub struct Agent {
    pub id: String,
    pub name: String,
    capabilities: Vec<String>, // Private field
}

// Document public APIs
/// Calculates the reputation score for an agent based on historical reviews.
///
/// # Arguments
/// * `agent_id` - The unique identifier of the agent
/// * `reviews` - Vector of reviews for the agent
///
/// # Returns
/// Returns the calculated reputation score between 0.0 and 100.0
pub fn calculate_reputation_score(agent_id: &str, reviews: &[Review]) -> Result<f64, ReputationError> {
    // Implementation
}
```

#### 2. Go Code Standards
```go
// Use descriptive package and function names
package orchestration

// Context as first parameter
func (o *Orchestrator) ExecuteTask(ctx context.Context, taskID string) error {
    // Implementation
}

// Proper error handling
func (s *TaskService) SubmitTask(ctx context.Context, task *Task) (*TaskResult, error) {
    if err := s.validateTask(task); err != nil {
        return nil, fmt.Errorf("task validation failed: %w", err)
    }

    result, err := s.repository.Create(ctx, task)
    if err != nil {
        return nil, fmt.Errorf("failed to create task: %w", err)
    }

    return result, nil
}

// Interface segregation
type TaskReader interface {
    GetTask(ctx context.Context, id string) (*Task, error)
}

type TaskWriter interface {
    CreateTask(ctx context.Context, task *Task) error
    UpdateTask(ctx context.Context, task *Task) error
}

type TaskRepository interface {
    TaskReader
    TaskWriter
}
```

#### 3. TypeScript/React Standards
```typescript
// Use descriptive component names
interface AgentCardProps {
  agent: Agent;
  onSelect: (agent: Agent) => void;
}

const AgentCard: React.FC<AgentCardProps> = ({ agent, onSelect }) => {
  const handleClick = useCallback(() => {
    onSelect(agent);
  }, [agent, onSelect]);

  return (
    <div className="agent-card" onClick={handleClick}>
      <h3>{agent.name}</h3>
      <p>{agent.description}</p>
      <div className="capabilities">
        {agent.capabilities.map(cap => (
          <span key={cap} className="capability-tag">{cap}</span>
        ))}
      </div>
    </div>
  );
};

// Use custom hooks for complex state
const useAgentList = () => {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchAgents = async () => {
      try {
        const response = await api.agents.list();
        setAgents(response.data);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchAgents();
  }, []);

  return { agents, loading, error };
};
```

### Testing Requirements

#### 1. Unit Tests
```rust
// Rust unit tests
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_reputation_calculation() {
        let reviews = vec![
            Review { score: 5, comment: "Excellent".to_string() },
            Review { score: 4, comment: "Good".to_string() },
        ];

        let score = calculate_reputation_score("agent_123", &reviews).unwrap();
        assert_eq!(score, 4.5);
    }

    #[test]
    fn test_reputation_with_no_reviews() {
        let reviews = vec![];
        let score = calculate_reputation_score("agent_123", &reviews).unwrap();
        assert_eq!(score, 0.0);
    }
}
```

```go
// Go unit tests
func TestTaskSubmission(t *testing.T) {
    ctx := context.Background()
    mockRepo := &MockTaskRepository{}
    service := NewTaskService(mockRepo)

    task := &Task{
        ID:          "task_123",
        Description: "Test task",
        Type:        "computation",
    }

    result, err := service.SubmitTask(ctx, task)

    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, "task_123", result.ID)

    mockRepo.AssertExpectations(t)
}
```

#### 2. Integration Tests
```go
// Integration test example
func TestTaskExecutionFlow(t *testing.T) {
    // Setup test database
    db := setupTestDB()
    defer cleanupTestDB(db)

    // Start test server
    server := setupTestServer(db)
    defer server.Close()

    // Submit task
    task := &Task{Description: "Integration test task"}
    resp, err := http.Post(server.URL+"/api/v1/tasks/submit",
        "application/json",
        bytes.NewReader(mustMarshalJSON(task)))

    require.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)

    // Verify task was created
    var result TaskResponse
    json.NewDecoder(resp.Body).Decode(&result)
    assert.NotEmpty(t, result.ID)
}
```

## Adding New Pallets

### Creating a New Pallet

#### 1. Generate Pallet Structure
```bash
cd chain-v2/pallets

# Create new pallet directory
mkdir my-pallet
cd my-pallet

# Create Cargo.toml
cat > Cargo.toml << EOF
[package]
name = "pallet-my-pallet"
version = "1.0.0"
edition = "2021"

[dependencies]
frame-support = { default-features = false, version = "40.1.0" }
frame-system = { default-features = false, version = "40.1.0" }
codec = { package = "parity-scale-codec", version = "3.7.4", default-features = false }
scale-info = { version = "2.11.6", default-features = false }
sp-std = { default-features = false, version = "14.0.0" }

[features]
default = ["std"]
std = [
    "frame-support/std",
    "frame-system/std",
    "codec/std",
    "scale-info/std",
    "sp-std/std",
]
EOF

# Create source directory
mkdir src
```

#### 2. Implement Pallet Logic
```rust
// src/lib.rs
#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

#[frame_support::pallet]
pub mod pallet {
    use frame_support::{
        dispatch::DispatchResult,
        pallet_prelude::*,
        traits::{Get, Currency},
    };
    use frame_system::pallet_prelude::*;
    use sp_std::vec::Vec;

    #[pallet::pallet]
    pub struct Pallet<T>(_);

    #[pallet::config]
    pub trait Config: frame_system::Config {
        type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

        /// Maximum length for pallet data
        #[pallet::constant]
        type MaxLength: Get<u32>;
    }

    #[pallet::storage]
    #[pallet::getter(fn my_data)]
    pub type MyData<T: Config> = StorageMap<
        _,
        Blake2_128Concat,
        T::AccountId,
        Vec<u8>,
        ValueQuery,
    >;

    #[pallet::event]
    #[pallet::generate_deposit(pub(super) fn deposit_event)]
    pub enum Event<T: Config> {
        /// Data was stored
        DataStored { who: T::AccountId, data: Vec<u8> },
        /// Data was removed
        DataRemoved { who: T::AccountId },
    }

    #[pallet::error]
    pub enum Error<T> {
        /// Data too long
        DataTooLong,
        /// Data not found
        DataNotFound,
    }

    #[pallet::call]
    impl<T: Config> Pallet<T> {
        /// Store some data
        #[pallet::call_index(0)]
        #[pallet::weight(10_000)]
        pub fn store_data(
            origin: OriginFor<T>,
            data: Vec<u8>,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            ensure!(
                data.len() <= T::MaxLength::get() as usize,
                Error::<T>::DataTooLong
            );

            MyData::<T>::insert(&who, &data);

            Self::deposit_event(Event::DataStored { who, data });
            Ok(())
        }

        /// Remove stored data
        #[pallet::call_index(1)]
        #[pallet::weight(10_000)]
        pub fn remove_data(origin: OriginFor<T>) -> DispatchResult {
            let who = ensure_signed(origin)?;

            ensure!(
                MyData::<T>::contains_key(&who),
                Error::<T>::DataNotFound
            );

            MyData::<T>::remove(&who);

            Self::deposit_event(Event::DataRemoved { who });
            Ok(())
        }
    }
}
```

#### 3. Add Pallet to Runtime
```rust
// runtime/src/lib.rs

// Add to construct_runtime macro
construct_runtime!(
    pub struct Runtime {
        // Existing pallets...
        MyPallet: pallet_my_pallet,
    }
);

// Add pallet configuration
impl pallet_my_pallet::Config for Runtime {
    type RuntimeEvent = RuntimeEvent;
    type MaxLength = frame_support::traits::ConstU32<100>;
}
```

#### 4. Update Dependencies
```toml
# runtime/Cargo.toml
[dependencies]
# Add your pallet
pallet-my-pallet = { path = "../pallets/my-pallet", default-features = false }

[features]
std = [
    # Add std feature
    "pallet-my-pallet/std",
]
```

#### 5. Write Tests
```rust
// src/tests.rs
use super::*;
use frame_support::{
    assert_ok, assert_noop,
    traits::{OnInitialize, OnFinalize},
};

type MyPalletModule = Pallet<Test>;

#[test]
fn store_data_works() {
    new_test_ext().execute_with(|| {
        let data = vec![1, 2, 3, 4];

        assert_ok!(MyPalletModule::store_data(
            RuntimeOrigin::signed(1),
            data.clone()
        ));

        assert_eq!(MyData::<Test>::get(1), data);

        System::assert_last_event(Event::DataStored {
            who: 1,
            data
        }.into());
    });
}

#[test]
fn store_data_too_long_fails() {
    new_test_ext().execute_with(|| {
        let data = vec![1; 101]; // Exceeds MaxLength of 100

        assert_noop!(
            MyPalletModule::store_data(RuntimeOrigin::signed(1), data),
            Error::<Test>::DataTooLong
        );
    });
}
```

### Pallet Best Practices

#### 1. Storage Optimization
```rust
// Use appropriate storage types
#[pallet::storage]
pub type SingleValue<T> = StorageValue<_, u64>; // Single global value

#[pallet::storage]
pub type MappedData<T: Config> = StorageMap<
    _,
    Blake2_128Concat, // Use appropriate hasher
    T::AccountId,     // Key type
    UserData,         // Value type
    OptionQuery,      // Query type (OptionQuery/ValueQuery)
>;

#[pallet::storage]
pub type DoubleMap<T: Config> = StorageDoubleMap<
    _,
    Blake2_128Concat, T::AccountId,  // First key
    Blake2_128Concat, u64,           // Second key
    Balance,                         // Value
    ValueQuery,
>;
```

#### 2. Event Design
```rust
#[pallet::event]
#[pallet::generate_deposit(pub(super) fn deposit_event)]
pub enum Event<T: Config> {
    /// Use descriptive documentation
    /// [who, amount]
    TokensDeposited { who: T::AccountId, amount: Balance },

    /// Include relevant data for indexing
    TransferCompleted {
        from: T::AccountId,
        to: T::AccountId,
        amount: Balance,
        transaction_id: u64,
    },
}
```

#### 3. Error Handling
```rust
#[pallet::error]
pub enum Error<T> {
    /// Use descriptive error names
    InsufficientBalance,

    /// Account does not exist
    AccountNotFound,

    /// Operation would cause arithmetic overflow
    ArithmeticOverflow,
}
```

## Testing Guidelines

### Test Strategy

#### 1. Unit Tests (Fast, Isolated)
- Test individual functions and methods
- Mock external dependencies
- Focus on edge cases and error conditions

#### 2. Integration Tests (Moderate Speed)
- Test component interactions
- Use real databases/services in test mode
- Verify end-to-end workflows

#### 3. End-to-End Tests (Slow, Comprehensive)
- Test complete user journeys
- Use production-like environment
- Validate system behavior

### Test Organization

#### 1. Rust Tests
```rust
// Unit tests in same file
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_function_name() {
        // Test implementation
    }
}

// Integration tests in tests/ directory
// tests/integration_test.rs
use my_crate::*;

#[tokio::test]
async fn test_full_workflow() {
    // Integration test implementation
}
```

#### 2. Go Tests
```bash
# Test file naming convention
agent_service.go      # Source file
agent_service_test.go # Test file

# Test organization
pkg/
  agent/
    service.go
    service_test.go       # Unit tests
    integration_test.go   # Integration tests
  testutil/               # Test utilities
    fixtures.go
    mocks.go
```

### Test Utilities

#### 1. Test Fixtures
```go
// testutil/fixtures.go
func CreateTestAgent() *Agent {
    return &Agent{
        ID:           "test-agent-" + uuid.New().String(),
        Name:         "Test Agent",
        Capabilities: []string{"math", "test"},
        Status:       "online",
        CreatedAt:    time.Now(),
    }
}

func CreateTestTask() *Task {
    return &Task{
        ID:          "test-task-" + uuid.New().String(),
        Type:        "computation",
        Description: "Test task description",
        Status:      "pending",
        CreatedAt:   time.Now(),
    }
}
```

#### 2. Mock Services
```go
// testutil/mocks.go
type MockTaskRepository struct {
    mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, task *Task) error {
    args := m.Called(ctx, task)
    return args.Error(0)
}

func (m *MockTaskRepository) GetByID(ctx context.Context, id string) (*Task, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*Task), args.Error(1)
}
```

### Performance Testing

#### 1. Benchmark Tests
```go
// Performance benchmarks
func BenchmarkReputationCalculation(b *testing.B) {
    reviews := generateTestReviews(1000) // Generate test data

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        calculateReputation(reviews)
    }
}

func BenchmarkTaskSubmission(b *testing.B) {
    service := setupTestService()
    task := createTestTask()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        service.SubmitTask(context.Background(), task)
    }
}
```

#### 2. Load Testing
```bash
# Using wrk for HTTP load testing
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/agents

# Using k6 for more complex scenarios
k6 run --vus 100 --duration 30s load-test-script.js
```

## Architecture Decision Records

### ADR Template
```markdown
# ADR-001: Use Substrate for Blockchain Layer

## Status
Accepted

## Context
We need a blockchain platform that supports custom business logic, has strong consensus mechanisms, and allows for runtime upgrades without hard forks.

## Decision
Use Substrate framework with custom pallets for the Ainur Protocol blockchain.

## Rationale
- Proven consensus algorithms (Aura + GRANDPA)
- Runtime upgrades without hard forks
- Rich ecosystem and tooling
- Strong security guarantees
- Flexible pallet system

## Consequences
- Team needs to learn Rust and Substrate
- Larger binary size compared to minimal implementations
- Dependency on Polkadot ecosystem
- Complex deployment process

## Implementation
- Create custom runtime with business logic pallets
- Use Substrate node template as starting point
- Implement custom pallets for DID, reputation, auctions, escrow
```

### Current ADRs

1. **ADR-001**: Substrate for Blockchain Layer
2. **ADR-002**: Go for Orchestrator Layer
3. **ADR-003**: PostgreSQL for Primary Database
4. **ADR-004**: Cloudflare R2 for Object Storage
5. **ADR-005**: WebSocket + REST vs GraphQL
6. **ADR-006**: VCG Auction Mechanism
7. **ADR-007**: JWT for Authentication
8. **ADR-008**: Prometheus + Grafana for Monitoring

## Development Workflows

### Feature Development Workflow

1. **Planning Phase**
   - Create GitHub issue with requirements
   - Design discussion in issue comments
   - Architecture review if significant changes

2. **Implementation Phase**
   - Create feature branch from main
   - Implement changes with tests
   - Regular commits with descriptive messages

3. **Review Phase**
   - Create pull request with detailed description
   - Code review by team members
   - Address review feedback

4. **Testing Phase**
   - All CI tests must pass
   - Manual testing in development environment
   - Performance testing if applicable

5. **Deployment Phase**
   - Merge to main branch
   - Deploy to staging environment
   - Production deployment after validation

### Hotfix Workflow

1. **Immediate Response**
   - Create hotfix branch from main
   - Implement minimal fix
   - Fast-track review process

2. **Testing**
   - Essential tests only
   - Manual verification in staging
   - Rollback plan prepared

3. **Deployment**
   - Deploy to production immediately
   - Monitor for issues
   - Follow up with comprehensive fix

### Release Workflow

1. **Pre-Release**
   - Create release branch
   - Finalize changelog
   - Update version numbers
   - Complete integration testing

2. **Release**
   - Tag release version
   - Build and test release artifacts
   - Deploy to production
   - Update documentation

3. **Post-Release**
   - Monitor system health
   - Collect user feedback
   - Plan next iteration

## Debugging and Profiling

### Debugging Rust Code

#### 1. Logging
```rust
use log::{info, debug, error};

fn my_function() {
    debug!("Starting function execution");

    match some_operation() {
        Ok(result) => {
            info!("Operation successful: {:?}", result);
        },
        Err(e) => {
            error!("Operation failed: {:?}", e);
        }
    }
}
```

#### 2. Using GDB with Rust
```bash
# Build with debug symbols
cargo build

# Start debugging
gdb ./target/debug/my-program
(gdb) break main
(gdb) run
(gdb) bt  # Backtrace
```

#### 3. Substrate-Specific Debugging
```bash
# Enable detailed logging
RUST_LOG=debug ./target/release/solochain-template-node --dev

# Specific pallet logging
RUST_LOG=pallet_reputation=debug ./target/release/solochain-template-node --dev
```

### Debugging Go Code

#### 1. Using Delve Debugger
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Start debugging
dlv debug ./cmd/api
(dlv) break main.main
(dlv) continue
```

#### 2. Profiling with pprof
```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()

    // Your application code
}
```

```bash
# CPU profiling
go tool pprof http://localhost:6060/debug/pprof/profile

# Memory profiling
go tool pprof http://localhost:6060/debug/pprof/heap

# View profile in browser
go tool pprof -http=:8080 profile.pb.gz
```

#### 3. Trace Analysis
```go
import "go.opentelemetry.io/otel/trace"

func myFunction(ctx context.Context) {
    tracer := otel.Tracer("my-service")
    ctx, span := tracer.Start(ctx, "my-function")
    defer span.End()

    // Function implementation
}
```

### Performance Monitoring

#### 1. Application Metrics
```go
// Custom metrics
var (
    taskDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "task_duration_seconds",
            Help: "Time spent processing tasks",
        },
        []string{"task_type", "status"},
    )
)

func processTask(taskType string) {
    timer := prometheus.NewTimer(taskDuration.WithLabelValues(taskType, "processing"))
    defer timer.ObserveDuration()

    // Process task
}
```

#### 2. Database Performance
```sql
-- Enable query logging
ALTER SYSTEM SET log_min_duration_statement = '1000'; -- Log slow queries
ALTER SYSTEM SET log_statement = 'all'; -- Log all statements (dev only)

-- Query performance analysis
SELECT
    query,
    calls,
    total_time,
    mean_time
FROM pg_stat_statements
ORDER BY total_time DESC;
```

---

This development guide provides a comprehensive foundation for contributing to the Ainur Protocol. For deployment procedures, see the [Deployment Guide](DEPLOYMENT.md), and for operational procedures, see the [Operations Guide](OPERATIONS.md).