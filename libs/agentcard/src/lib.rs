pub mod error;
pub mod signing;
pub mod types;

pub use error::{Error, Result};
pub use signing::{generate_keypair, hash_agentcard, sign_agentcard, verify_agentcard};
pub use types::*;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_agentcard_builder() {
        let card = AgentCard::builder()
            .agent_did(DID::new_agent("math-specialist-001"))
            .name("Math Specialist Agent")
            .description("High-precision mathematical computation agent")
            .version("1.0.0")
            .capabilities(
                Capabilities::builder()
                    .domain("math")
                    .domain("computation")
                    .operation(Operation {
                        name: "add".to_string(),
                        category: "math.arithmetic".to_string(),
                        input_schema: Some(serde_json::json!({
                            "type": "object",
                            "properties": {
                                "a": {"type": "number"},
                                "b": {"type": "number"}
                            }
                        })),
                        output_schema: Some(serde_json::json!({"type": "number"})),
                        complexity: Some("O(1)".to_string()),
                        gas_estimate: 100,
                    })
                    .max_input_size(1048576)
                    .max_execution_time_ms(5000)
                    .concurrent_tasks(10)
                    .interface("ari-v1")
                    .interface("grpc")
                    .build(),
            )
            .runtime(RuntimeInfo {
                protocol: "ari-v1".to_string(),
                implementation: "reference-runtime-v1".to_string(),
                version: "1.0.0".to_string(),
                wasm_engine: "wasmtime".to_string(),
                wasm_version: "24.0.0".to_string(),
                module_hash: "sha256:a1b2c3d4e5f6...".to_string(),
                module_url: Some("https://r2.ainur.network/agents/math.wasm".to_string()),
                execution_environment: ExecutionEnvironment {
                    memory_limit_mb: 128,
                    cpu_quota_ms: 1000,
                    network_enabled: false,
                    filesystem_enabled: false,
                },
                endpoints: vec![Endpoint {
                    protocol: "grpc".to_string(),
                    address: "localhost:9001".to_string(),
                    tls: Some(false),
                }],
            })
            .reputation(Reputation {
                trust_score: 95.5,
                total_tasks: 10234,
                successful_tasks: 10102,
                failed_tasks: 132,
                success_rate: 0.987,
                average_execution_time_ms: 125,
                uptime_percentage: 99.8,
                peer_endorsements: 47,
                violations: 0,
                created_at: chrono::Utc::now(),
                last_active: chrono::Utc::now(),
                badges: vec![],
                slashing_history: vec![],
            })
            .economic(Economic {
                pricing_model: "per_operation".to_string(),
                base_price_uainur: 100,
                surge_pricing: None,
                discounts: vec![],
                payment_methods: vec!["ainur".to_string()],
                escrow_required: false,
                refund_policy: "full_refund_on_failure".to_string(),
            })
            .network(Network {
                p2p: P2PConfig {
                    peer_id: "12D3KooWMPqCdc16e9zFNK9SveMUn4swBC1evJ6qYqpW8v3hsarw".to_string(),
                    listen_addresses: vec!["/ip4/0.0.0.0/tcp/4001".to_string()],
                    announce_addresses: vec![],
                    protocols: vec!["/ainur/presence/1.0.0".to_string()],
                },
                discovery: Discovery {
                    methods: vec!["mdns".to_string(), "dht".to_string()],
                    bootstrap_nodes: vec![],
                },
                availability: Availability {
                    regions: vec!["us-east".to_string()],
                    latency_targets: LatencyTargets {
                        p50_ms: 50,
                        p95_ms: 200,
                        p99_ms: 500,
                    },
                },
            })
            .issuer(DID::new_agent("math-specialist-001"))
            .expiration_days(365)
            .build()
            .unwrap();

        assert_eq!(card.credential_subject.name, "Math Specialist Agent");
        assert_eq!(card.credential_subject.capabilities.domains.len(), 2);
        assert_eq!(card.credential_subject.capabilities.operations.len(), 1);
    }

    #[test]
    fn test_agentcard_serialization() {
        let card = AgentCard::builder()
            .agent_did(DID::new_agent("test-001"))
            .name("Test Agent")
            .description("Test")
            .capabilities(
                Capabilities::builder()
                    .domain("test")
                    .interface("ari-v1")
                    .max_input_size(1024)
                    .max_execution_time_ms(1000)
                    .build(),
            )
            .runtime(RuntimeInfo {
                protocol: "ari-v1".to_string(),
                implementation: "test".to_string(),
                version: "1.0.0".to_string(),
                wasm_engine: "wasmtime".to_string(),
                wasm_version: "24.0.0".to_string(),
                module_hash: "sha256:test".to_string(),
                module_url: None,
                execution_environment: ExecutionEnvironment {
                    memory_limit_mb: 128,
                    cpu_quota_ms: 1000,
                    network_enabled: false,
                    filesystem_enabled: false,
                },
                endpoints: vec![],
            })
            .network(Network {
                p2p: P2PConfig {
                    peer_id: "12D3KooW...".to_string(),
                    listen_addresses: vec![],
                    announce_addresses: vec![],
                    protocols: vec![],
                },
                discovery: Discovery {
                    methods: vec!["mdns".to_string()],
                    bootstrap_nodes: vec![],
                },
                availability: Availability {
                    regions: vec!["local".to_string()],
                    latency_targets: LatencyTargets {
                        p50_ms: 50,
                        p95_ms: 200,
                        p99_ms: 500,
                    },
                },
            })
            .build()
            .unwrap();

        let json = card.to_json().unwrap();
        assert!(json.contains("@context"));
        assert!(json.contains("VerifiableCredential"));
        assert!(json.contains("AgentCard"));

        let parsed = AgentCard::from_json(&json).unwrap();
        assert_eq!(parsed.credential_subject.name, "Test Agent");
    }
}
