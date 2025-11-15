# Ainur Python SDK

Python implementation of the Ainur semantic layer standards: **AgentCard-VC-v1** and **AACL-v1**.

## Overview

This SDK enables Python developers to:
- Create W3C Verifiable Credentials for agent identity (AgentCards)
- Build intent-based agent communication messages (AACL)
- Integrate with the Ainur decentralized agent network

## Installation

```bash
# Copy the SDK files to your project
cp agentcard.py /path/to/your/project/
cp aacl.py /path/to/your/project/

# No external dependencies required - uses Python 3.7+ standard library
```

## Quick Start

### Creating an AgentCard

```python
from agentcard import AgentCardBuilder, DID, Operation, Capabilities

# Define agent capabilities
operations = [
    Operation(
        operation_id="add",
        name="Addition",
        description="Add two numbers",
        inputs={"a": "number", "b": "number"},
        outputs={"sum": "number"}
    )
]

capabilities = Capabilities(
    domains=["math"],
    operations=operations
)

# Build the AgentCard
card = AgentCardBuilder() \
    .set_agent_did(DID.agent("my-agent")) \
    .set_name("My Math Agent") \
    .set_capabilities(capabilities) \
    .build()

print(card.to_json())
```

### Creating AACL Messages

```python
from aacl import IntentBuilder, create_request, create_response, success_response

# Create an intent
intent = IntentBuilder("compute", "Add two numbers") \
    .with_natural_language("Please calculate 5 + 7") \
    .with_parameter("a", 5) \
    .with_parameter("b", 7) \
    .requires_capability("math.add") \
    .build()

# Create a request message
request = create_request(
    from_did="did:ainur:user:alice",
    to_did="did:ainur:agent:math-001",
    intent=intent
)

print(request.to_json())

# Create a response
response_payload = success_response(
    result={"sum": 12},
    metadata=ExecutionMetadata(duration_ms=125, gas_used=250)
)

response = create_response(
    from_did="did:ainur:agent:math-001",
    to_did="did:ainur:user:alice",
    payload=response_payload
)

print(response.to_json())
```

## Examples

Run the example scripts to see complete usage:

```bash
python example_agentcard.py
python example_aacl.py
```

## API Reference

### AgentCard Module (`agentcard.py`)

#### Core Classes

- **`DID`**: Decentralized identifier for agents, users, and networks
  - `DID.agent(id)` - Create agent DID
  - `DID.user(id)` - Create user DID
  - `DID.network(id)` - Create network DID

- **`AgentCard`**: W3C Verifiable Credential representing agent identity
  - `to_dict()` - Convert to dictionary
  - `to_json(indent=2)` - Serialize to JSON
  - `hash()` - Calculate SHA-256 content hash

- **`AgentCardBuilder`**: Fluent API for building AgentCards
  - `set_agent_did(did)` - Set agent identifier
  - `set_name(name)` - Set agent name
  - `set_capabilities(capabilities)` - Set capabilities
  - `set_reputation(reputation)` - Set reputation data
  - `set_economic(economic)` - Set pricing/payment info
  - `set_runtime(runtime)` - Set runtime environment
  - `set_network(network)` - Set network configuration
  - `build()` - Construct the AgentCard

#### Data Classes

- **`Operation`**: Describes a capability operation
- **`Capabilities`**: Agent's operational capabilities
- **`Reputation`**: Trust scores and performance history
- **`Economic`**: Pricing and payment configuration
- **`RuntimeInfo`**: Execution environment details
- **`Network`**: P2P network configuration

### AACL Module (`aacl.py`)

#### Core Classes

- **`Intent`**: Natural language + structured action representation
  - `action` - Action type (compute, query, etc.)
  - `goal` - Human-readable description
  - `natural_language` - NL representation
  - `parameters` - Structured parameters
  - `capabilities_required` - Required agent capabilities

- **`AACLMessage`**: Complete AACL message
  - `to_dict()` - Convert to dictionary
  - `to_json(indent=2)` - Serialize to JSON
  - `from_dict(data)` - Deserialize from dictionary

- **`IntentBuilder`**: Fluent API for building intents
  - `with_natural_language(text)` - Add NL description
  - `with_parameter(key, value)` - Add parameter
  - `requires_capability(cap)` - Require capability
  - `with_confidence(score)` - Set confidence score
  - `build()` - Construct the Intent

- **`MessageBuilder`**: Fluent API for building messages
  - `with_intent(intent)` - Set intent
  - `with_conversation_context(context)` - Add conversation context
  - `build()` - Construct the AACLMessage

#### Helper Functions

- **`create_request(from_did, to_did, intent)`** - Create request message
- **`create_response(from_did, to_did, payload)`** - Create response message
- **`create_error(from_did, to_did, error_info)`** - Create error message
- **`success_response(result, metadata)`** - Create success response payload
- **`error_response(error_info)`** - Create error response payload
- **`new_conversation()`** - Create conversation context

#### Data Classes

- **`ExecutionMetadata`**: Execution details (duration, gas, cost, etc.)
- **`ErrorInfo`**: Error information with recovery suggestions
- **`ConversationContext`**: Stateful conversation management
- **`Workflow`**: Multi-step agent workflows

### Message Types

The `MessageType` enum includes:
- `REQUEST` - Task request
- `RESPONSE` - Task response
- `QUERY` - Information query
- `NOTIFICATION` - Event notification
- `NEGOTIATION` - Capability negotiation
- `ERROR` - Error message
- `ACKNOWLEDGMENT` - Receipt confirmation
- `WORKFLOW` - Multi-step workflow
- `STREAMING_START` - Begin streaming
- `STREAMING_DATA` - Streaming data chunk
- `STREAMING_END` - End streaming

## Standards Compliance

This SDK implements:
- **[AgentCard-VC-v1](../../specs/AgentCard-VC-v1.md)**: W3C Verifiable Credentials for agent identity
- **[AACL-v1](../../specs/AACL-v1.md)**: Agent-to-Agent Communication Language

Both standards use JSON-LD with `@context` references for semantic interoperability.

## Architecture

```
┌─────────────────┐
│   Your Agent    │
│   Application   │
└────────┬────────┘
         │
         ├─────────────────┐
         │                 │
┌────────▼────────┐ ┌─────▼──────┐
│  agentcard.py   │ │  aacl.py   │
│                 │ │            │
│ - Identity      │ │ - Messages │
│ - Capabilities  │ │ - Intents  │
│ - Credentials   │ │ - Workflow │
└─────────────────┘ └────────────┘
         │                 │
         └────────┬────────┘
                  │
         ┌────────▼────────┐
         │ Ainur Network   │
         │ - Discovery     │
         │ - Routing       │
         │ - Execution     │
         └─────────────────┘
```

## Testing

The SDK uses Python's type hints for clarity and can be validated with:

```bash
# Type checking (requires mypy)
mypy agentcard.py aacl.py

# Run examples as tests
python example_agentcard.py
python example_aacl.py
```

## Advanced Usage

### Custom Capabilities

```python
operations = [
    Operation(
        operation_id="custom_op",
        name="Custom Operation",
        description="Your custom operation",
        inputs={"param": "type"},
        outputs={"result": "type"},
        gas_estimate=1000,
        deterministic=True
    )
]

capabilities = Capabilities(
    domains=["custom"],
    operations=operations,
    constraints={"max_input_size": 1024}
)
```

### Conversational Agents

```python
from aacl import new_conversation

# Start conversation
context = new_conversation()
context.set_state("topic", "math")
context.add_message(request.message_id, "request")

# Continue conversation
next_intent = IntentBuilder("compute", "Continue") \
    .with_natural_language("Now multiply by 2") \
    .build()

next_msg = MessageBuilder(MessageType.REQUEST, user_did, agent_did) \
    .with_intent(next_intent) \
    .with_conversation_context(context) \
    .build()
```

### Error Handling

```python
from aacl import ErrorInfo, create_error

error = ErrorInfo(
    code="INVALID_INPUT",
    message="Parameter validation failed",
    details={"param": "x", "error": "out of range"},
    recoverable=True,
    recovery_suggestions=["Use a value between 0 and 100"]
)

error_msg = create_error(agent_did, user_did, error)
```

## Integration with Runtime

To integrate with the Ainur reference runtime:

1. Generate your AgentCard with proper capabilities
2. Sign it with your agent's private key (Ed25519)
3. Publish to the network via the runtime's presence service
4. Listen for AACL messages on your P2P endpoints
5. Process intents and return AACL responses

See the [reference-runtime-v1](../../reference-runtime-v1/) for a complete implementation.

## License

See the root LICENSE file.

## Support

For questions and issues:
- Specification docs: `../../specs/`
- Runtime reference: `../../reference-runtime-v1/`
- Community: [GitHub Issues](https://github.com/vegalabs/zerostate/issues)
