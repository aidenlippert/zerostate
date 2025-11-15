# Ainur Protocol - Data Flow Architecture

This document details the data flows within the Ainur Protocol ecosystem, including user interactions, system processes, and cross-component communication patterns.

## Core Data Flow Patterns

### 1. Agent Registration Flow

```
Agent Owner                API Gateway               Orchestrator              Blockchain
     │                         │                         │                        │
     │─── POST /agents ────────▶│                         │                        │
     │    (metadata)            │─── Validate ──────────▶│                        │
     │                         │                         │─── Check DID ────────▶│
     │                         │                         │                        │
     │                         │                         │◀── DID Valid ─────────│
     │                         │                         │                        │
     │                         │                         │─── Generate Agent ID   │
     │                         │                         │                        │
     │                         │◀── Agent Created ──────│                        │
     │◀── Agent ID ────────────│                         │                        │
     │                         │                         │                        │
     │─── POST /agents/{id}/binary ──────────────────────▶│                        │
     │    (WASM file)          │                         │                        │
     │                         │                         │─── Validate WASM       │
     │                         │                         │─── Store in R2         │
     │                         │                         │─── Update Registry ───▶│
     │                         │                         │                        │
     │◀── Upload Complete ────│◀── Registration Done ──│◀── Confirm Txn ───────│
```

### 2. Task Submission & Auction Flow

```
Task Requester          API Gateway              Orchestrator              Blockchain              Agents
       │                     │                        │                        │                     │
       │── Submit Task ─────▶│                        │                        │                     │
       │   (requirements)    │── Validate ──────────▶│                        │                     │
       │                     │                        │── Create Auction ────▶│                     │
       │                     │                        │                        │                     │
       │                     │                        │◀── Auction ID ────────│                     │
       │◀── Task ID ─────────│◀── Task Created ──────│                        │                     │
       │                     │                        │                        │                     │
       │                     │                        │───── P2P Broadcast ──────────────────────▶│
       │                     │                        │      (auction info)                        │
       │                     │                        │                        │                     │
       │                     │                        │◀────── Submit Bids ──────────────────────│
       │                     │                        │                        │                     │
       │                     │                        │── Collect & Validate   │                     │
       │                     │                        │   Bids                  │                     │
       │                     │                        │                        │                     │
       │                     │─ WebSocket: New Bid ──▶│                        │                     │
       │◀─ Bid Notification ─│                        │                        │                     │
       │                     │                        │                        │                     │
       │                     │                        │── Auction Timeout      │                     │
       │                     │                        │── Determine Winner ───▶│                     │
       │                     │                        │                        │                     │
       │                     │                        │◀── Winner Confirmed ──│                     │
       │                     │─ WebSocket: Winner ───▶│                        │                     │
       │◀─ Auction Result ───│                        │                        │                     │
```

### 3. Task Execution Flow

```
Orchestrator            Winner Agent            WASM Runtime           Storage (R2)           Blockchain
     │                      │                       │                      │                    │
     │─── Execution Request ─▶│                       │                      │                    │
     │                      │─── Download Binary ──────────────────────────▶│                    │
     │                      │                       │                      │                    │
     │                      │◀──── Binary Data ────────────────────────────│                    │
     │                      │                       │                      │                    │
     │                      │─── Load & Validate ──▶│                      │                    │
     │                      │                       │                      │                    │
     │                      │◀──── Runtime Ready ───│                      │                    │
     │                      │                       │                      │                    │
     │─── Task Data ────────▶│                       │                      │                    │
     │                      │─── Execute ──────────▶│                      │                    │
     │                      │                       │                      │                    │
     │                      │◀──── Result ──────────│                      │                    │
     │◀──── Task Result ────│                       │                      │                    │
     │                      │                       │                      │                    │
     │─── Store Result ─────────────────────────────────────────────────▶│                    │
     │                      │                       │                      │                    │
     │─── Update Status ──────────────────────────────────────────────────────────────────────▶│
     │                      │                       │                      │                    │
     │◀─── Confirm Update ────────────────────────────────────────────────────────────────────│
```

### 4. Payment & Escrow Flow

```
Task Requester          Orchestrator            Blockchain Escrow          Agent                Database
      │                      │                        │                     │                    │
      │── Fund Task ────────▶│                        │                     │                    │
      │   (amount)           │── Create Escrow ─────▶│                     │                    │
      │                      │                        │                     │                    │
      │                      │◀── Escrow Created ────│                     │                    │
      │                      │── Store Escrow Info ─────────────────────────────────────────────▶│
      │                      │                        │                     │                    │
      │◀── Escrow ID ───────│                        │                     │                    │
      │                      │                        │                     │                    │
      │                      │  ╔══ Task Execution ══════════════════════════════════════════════╗
      │                      │  ║                                                                ║
      │                      │  ║  [Task completed successfully]                                 ║
      │                      │  ║                                                                ║
      │                      │  ╚════════════════════════════════════════════════════════════════╝
      │                      │                        │                     │                    │
      │                      │── Release Conditions ─▶│                     │                    │
      │                      │   Check                 │                     │                    │
      │                      │                        │                     │                    │
      │                      │◀── Conditions Met ─────│                     │                    │
      │                      │                        │                     │                    │
      │                      │── Release Payment ────▶│                     │                    │
      │                      │                        │── Transfer Funds ──▶│                    │
      │                      │                        │                     │                    │
      │                      │                        │◀── Confirm Receipt ─│                    │
      │                      │── Update Payment ─────────────────────────────────────────────────▶│
      │                      │   Status               │                     │                    │
      │                      │                        │                     │                    │
      │── Payment Complete ──│                        │                     │                    │
```

### 5. Reputation Update Flow

```
System Evaluator        Orchestrator            Blockchain             Database           P2P Network
       │                      │                      │                     │                 │
       │── Task Complete ────▶│                      │                     │                 │
       │                      │── Calculate Metrics  │                     │                 │
       │                      │   (quality, time)    │                     │                 │
       │                      │                      │                     │                 │
       │                      │── Store Local ──────────────────────────────▶│                 │
       │                      │   Metrics            │                     │                 │
       │                      │                      │                     │                 │
       │                      │── Submit Reputation ─▶│                     │                 │
       │                      │   Update              │                     │                 │
       │                      │                      │                     │                 │
       │                      │◀── Update Confirmed ──│                     │                 │
       │                      │                      │                     │                 │
       │                      │── Broadcast Update ──────────────────────────────────────────▶│
       │                      │   (P2P gossip)       │                     │                 │
       │                      │                      │                     │                 │
       │                      │── Aggregate & ───────────────────────────────▶│                 │
       │                      │   Recalculate        │                     │                 │
       │                      │   Reputation         │                     │                 │
       │                      │                      │                     │                 │
       │◀── Rep Score ────────│◀── Final Score ──────────────────────────────│                 │
```

## Database Transaction Flows

### 1. User Registration Transaction

```sql
BEGIN TRANSACTION;

-- Create user record
INSERT INTO users (username, email, password_hash, created_at)
VALUES ($1, $2, $3, NOW())
RETURNING id;

-- Initialize user preferences
INSERT INTO user_preferences (user_id, notification_settings, privacy_settings)
VALUES ($user_id, $default_notifications, $default_privacy);

-- Create initial reputation entry
INSERT INTO user_reputation (user_id, score, created_at)
VALUES ($user_id, 50.0, NOW());

COMMIT;
```

### 2. Agent Registration Transaction

```sql
BEGIN TRANSACTION;

-- Create agent record
INSERT INTO agents (id, did, name, description, capabilities, owner_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
RETURNING id;

-- Store agent metadata
INSERT INTO agent_metadata (agent_id, runtime_info, resource_requirements)
VALUES ($agent_id, $runtime_info, $resource_reqs);

-- Initialize agent reputation
INSERT INTO agent_reputation (agent_id, overall_score, created_at)
VALUES ($agent_id, 0.0, NOW());

-- Log registration event
INSERT INTO agent_events (agent_id, event_type, metadata, created_at)
VALUES ($agent_id, 'registered', $event_metadata, NOW());

COMMIT;
```

### 3. Task Execution Transaction

```sql
BEGIN TRANSACTION;

-- Update task status to executing
UPDATE tasks
SET status = 'executing',
    assigned_agent_id = $agent_id,
    started_at = NOW()
WHERE id = $task_id AND status = 'auction_won';

-- Create execution record
INSERT INTO task_executions (task_id, agent_id, started_at, status)
VALUES ($task_id, $agent_id, NOW(), 'running');

-- Update agent status
UPDATE agents
SET status = 'busy',
    current_task_id = $task_id,
    updated_at = NOW()
WHERE id = $agent_id;

COMMIT;
```

### 4. Payment Transaction

```sql
BEGIN TRANSACTION;

-- Create payment record
INSERT INTO payments (id, payer_id, payee_id, amount, currency, task_id, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, 'pending', NOW());

-- Update escrow status
UPDATE escrow_accounts
SET status = 'releasing',
    payment_id = $payment_id,
    updated_at = NOW()
WHERE task_id = $task_id AND status = 'funded';

-- Record transaction
INSERT INTO payment_transactions (payment_id, transaction_type, amount, status, created_at)
VALUES ($payment_id, 'release', $amount, 'completed', NOW());

COMMIT;
```

## WebSocket Event Flows

### 1. Real-time Task Updates

```
WebSocket Connection Flow:

Client                     WebSocket Server            Task Manager
  │                             │                         │
  │─── Connect ────────────────▶│                         │
  │    (JWT auth)               │── Validate Token        │
  │                             │                         │
  │◀── Connection OK ───────────│                         │
  │                             │                         │
  │─── Subscribe ──────────────▶│── Register Client       │
  │    (task updates)           │   for task events       │
  │                             │                         │
  │                             │◀── Task Status Change ──│
  │                             │                         │
  │◀── Event: Task Started ────│── Broadcast to          │
  │                             │   subscribed clients    │
  │                             │                         │
  │◀── Event: Progress 50% ────│◀── Progress Update ─────│
  │                             │                         │
  │◀── Event: Task Complete ───│◀── Completion Event ────│
```

### 2. Auction Bidding Updates

```
Auction Event Flow:

Bidder A        Bidder B        WebSocket Hub        Auction Manager
    │               │                 │                     │
    │◀────── Auction Created ─────────│◀─── New Auction ───│
    │               │                 │                     │
    │─── Submit Bid ─────────────────▶│──── Validate ─────▶│
    │               │                 │      Bid            │
    │               │                 │                     │
    │               │◀─── New Bid ───│◀──── Bid Valid ────│
    │◀─── Bid Confirmed ─────────────│      (broadcast)    │
    │               │                 │                     │
    │               │─── Submit Bid ─▶│──── Higher Bid ───▶│
    │               │                 │                     │
    │◀─── Outbid Alert ──────────────│◀──── Update All ───│
    │               │◀─── Lead Bid ───│      (broadcast)    │
```

## P2P Network Data Flows

### 1. Agent Discovery Flow

```
Seeking Node            DHT Network            Target Agent Node
     │                      │                        │
     │── Publish Agent ─────▶│                        │
     │   Capability          │                        │
     │                      │◀─── Advertise ────────│
     │                      │     Capability         │
     │                      │                        │
     │── Query: "math" ─────▶│                        │
     │   capability          │                        │
     │                      │── Route Query ────────▶│
     │                      │                        │
     │                      │◀── Agent Info ─────────│
     │◀── Found Agents ─────│                        │
     │   List               │                        │
```

### 2. Gossip Protocol Flow

```
Node A                  Node B                  Node C                  Node D
  │                       │                       │                       │
  │─── Reputation ───────▶│                       │                       │
  │    Update             │                       │                       │
  │                       │── Forward Update ───▶│                       │
  │                       │                       │                       │
  │                       │                       │── Forward Update ───▶│
  │                       │                       │                       │
  │◀──── Ack ─────────────│◀──── Ack ────────────│◀──── Ack ────────────│
  │                       │                       │                       │
  │                       │                       │                       │
  │◀──────────────── Reputation Consensus Reached ─────────────────────▶│
```

## Error Handling Data Flows

### 1. Task Execution Failure

```
WASM Runtime            Agent                Orchestrator            Database
     │                    │                      │                     │
     │─── Execution ─────▶│                      │                     │
     │    Error           │                      │                     │
     │                    │── Report Error ─────▶│                     │
     │                    │                      │── Log Failure ────▶│
     │                    │                      │                     │
     │                    │                      │── Check Retry       │
     │                    │                      │   Policy            │
     │                    │                      │                     │
     │                    │◀── Retry Request ────│                     │
     │◀── Retry ──────────│                      │                     │
     │   Execution        │                      │                     │
     │                    │                      │                     │
     │─── Timeout ───────▶│                      │                     │
     │                    │── Final Failure ────▶│                     │
     │                    │                      │── Update Status ───▶│
     │                    │                      │   (failed)          │
```

### 2. Payment Dispute Flow

```
Task Requester          Agent                Orchestrator            Arbitrator
       │                  │                      │                      │
       │─── Dispute ─────▶│                      │                      │
       │    Payment       │                      │                      │
       │                  │                      │── Create Dispute     │
       │                  │                      │   Record             │
       │                  │                      │                      │
       │                  │                      │── Select ──────────▶│
       │                  │                      │   Arbitrator         │
       │                  │                      │                      │
       │                  │                      │◀── Accept Role ─────│
       │                  │                      │                      │
       │◀───── Request Evidence ────────────────│◀── Investigation ───│
       │                  │                      │    Request           │
       │                  │                      │                      │
       │── Submit ────────▶│                      │                      │
       │   Evidence       │── Submit ───────────▶│── Forward ─────────▶│
       │                  │   Counter-evidence   │   Evidence           │
       │                  │                      │                      │
       │                  │                      │◀── Final Decision ──│
       │◀── Ruling ───────────────────────────────│                      │
       │                  │◀── Ruling ───────────│                      │
```

## Caching & Performance Data Flows

### 1. Multi-Level Caching

```
Client                  CDN                API Gateway           Database
  │                      │                    │                    │
  │─── GET Agent ───────▶│                    │                    │
  │    Details           │                    │                    │
  │                      │─── Cache Miss ────▶│                    │
  │                      │                    │                    │
  │                      │                    │─── Query ────────▶│
  │                      │                    │                    │
  │                      │                    │◀── Data ──────────│
  │                      │◀── Response ───────│                    │
  │                      │    (Cache)         │                    │
  │◀── Cached Response ──│                    │                    │
  │                      │                    │                    │
  │                      │                    │                    │
  │─── Next Request ────▶│                    │                    │
  │◀── Cache Hit ────────│                    │                    │
```

### 2. Database Connection Pooling

```
API Requests           Connection Pool        Database
     │                      │                    │
     │─── Request 1 ────────▶│                    │
     │                      │─── Acquire ──────▶│
     │                      │    Connection      │
     │                      │                    │
     │─── Request 2 ────────▶│                    │
     │                      │─── Reuse Conn ────▶│
     │                      │                    │
     │─── Request N ────────▶│                    │
     │                      │─── Queue Request   │
     │                      │    (Pool Full)     │
     │                      │                    │
     │                      │◀── Release ────────│
     │                      │    Connection      │
     │◀── Response ─────────│                    │
```

This data flow architecture ensures efficient, reliable, and scalable operations across all components of the Ainur Protocol ecosystem, with proper error handling, performance optimization, and real-time capabilities.