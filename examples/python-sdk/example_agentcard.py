#!/usr/bin/env python3
"""
Example: Creating an AgentCard

This demonstrates how to use the AgentCard Python SDK to create
a W3C Verifiable Credential for an agent.
"""

import sys
sys.path.insert(0, '.')

from agentcard import (
    AgentCardBuilder, DID, Operation, Capabilities, CapabilityConstraints,
    RuntimeInfo, ExecutionEnvironment, Endpoint, P2PConfig, Discovery,
    Network, Availability, LatencyTargets, Reputation, Economic
)


def main():
    print("=" * 70)
    print(" Creating AgentCard-VC-v1 Example")
    print("=" * 70)
    print()
    
    # 1. Create agent DID
    agent_did = DID.agent("math-agent-001")
    print(f"✓ Agent DID: {agent_did}")
    
    # 2. Define capabilities
    capabilities = Capabilities(
        domains=["mathematics", "computation"],
        operations=[
            Operation(
                name="add",
                category="arithmetic",
                gas_estimate=100
            ),
            Operation(
                name="multiply",
                category="arithmetic",
                gas_estimate=150
            ),
            Operation(
                name="divide",
                category="arithmetic",
                gas_estimate=120
            )
        ],
        constraints=CapabilityConstraints(
            max_input_size=1048576,  # 1MB
            max_execution_time_ms=5000  # 5 seconds
        ),
        interfaces=["http", "grpc", "p2p"]
    )
    print(f"✓ Capabilities: {len(capabilities.operations)} operations")
    
    # 3. Define runtime
    runtime = RuntimeInfo(
        protocol="ari-v1",
        implementation="reference-runtime-v1",
        version="1.0.0",
        wasm_engine="wasmtime",
        wasm_version="23.0.0",
        module_hash="sha256:abc123...",
        module_url="https://storage.example.com/agents/math-agent.wasm",
        execution_environment=ExecutionEnvironment(
            memory_limit_mb=128,
            cpu_quota_ms=1000,
            network_enabled=True,
            filesystem_enabled=False
        ),
        endpoints=[
            Endpoint(protocol="grpc", address="grpc://agent.example.com:9000"),
            Endpoint(protocol="http", address="https://agent.example.com/api", tls=True)
        ]
    )
    print(f"✓ Runtime: {runtime.implementation} v{runtime.version}")
    
    # 4. Define network configuration
    network = Network(
        p2p=P2PConfig(
            peer_id="12D3KooWExample123",
            listen_addresses=[
                "/ip4/0.0.0.0/tcp/4001",
                "/ip6/::/tcp/4001"
            ],
            announce_addresses=[
                "/ip4/203.0.113.1/tcp/4001/p2p/12D3KooWExample123"
            ]
        ),
        discovery=Discovery(
            methods=["mdns", "dht", "bootstrap"],
            bootstrap_nodes=[
                "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
            ]
        ),
        availability=Availability(
            regions=["us-west", "eu-central"],
            latency_targets=LatencyTargets(p50_ms=50, p95_ms=100, p99_ms=200)
        )
    )
    print(f"✓ Network: P2P peer {network.p2p.peer_id[:20]}...")
    
    # 5. Build AgentCard
    card = AgentCardBuilder() \
        .set_agent_did(agent_did) \
        .set_name("Math Agent") \
        .set_description("High-performance mathematical computation agent") \
        .set_version("1.0.0") \
        .set_capabilities(capabilities) \
        .set_runtime(runtime) \
        .set_network(network) \
        .set_expiration_days(365) \
        .build()
    
    print(f"✓ AgentCard created: {card.id}")
    print()
    
    # 6. Display card information
    print("-" * 70)
    print("AgentCard Details:")
    print("-" * 70)
    print(f"ID:          {card.id}")
    print(f"Type:        {', '.join(card.card_type)}")
    print(f"Issuer:      {card.issuer}")
    print(f"Subject:     {card.credential_subject.id}")
    print(f"Name:        {card.credential_subject.name}")
    print(f"Version:     {card.credential_subject.version}")
    print(f"Issued:      {card.issuance_date}")
    print(f"Expires:     {card.expiration_date}")
    print()
    print(f"Capabilities:")
    for op in card.credential_subject.capabilities.operations:
        print(f"  - {op.category}.{op.name} (gas: {op.gas_estimate})")
    print()
    print(f"Reputation:")
    rep = card.credential_subject.reputation
    print(f"  Trust Score:    {rep.trust_score}")
    print(f"  Total Tasks:    {rep.total_tasks}")
    print(f"  Success Rate:   {rep.success_rate:.1%}")
    print(f"  Uptime:         {rep.uptime_percentage:.1%}")
    print()
    
    # 7. Calculate hash
    card_hash = card.hash()
    print(f"Content Hash: {card_hash}")
    print()
    
    # 8. Export to JSON
    json_output = card.to_json(indent=2)
    print("-" * 70)
    print("JSON-LD Output (first 500 chars):")
    print("-" * 70)
    print(json_output[:500] + "...")
    print()
    
    # Save to file
    with open('agentcard-example.json', 'w') as f:
        f.write(json_output)
    print("✓ Saved to: agentcard-example.json")
    print()
    
    print("=" * 70)
    print(" AgentCard Created Successfully! ")
    print("=" * 70)


if __name__ == "__main__":
    main()
