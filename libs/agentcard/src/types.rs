use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// DID (Decentralized Identifier) for Ainur agents
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct DID(pub String);

impl DID {
    /// Create a new agent DID with format: did:ainur:agent:<identifier>
    pub fn new_agent(identifier: &str) -> Self {
        Self(format!("did:ainur:agent:{}", identifier))
    }

    /// Create a new user DID with format: did:ainur:user:<identifier>
    pub fn new_user(identifier: &str) -> Self {
        Self(format!("did:ainur:user:{}", identifier))
    }

    /// Create a new network DID with format: did:ainur:network:<identifier>
    pub fn new_network(identifier: &str) -> Self {
        Self(format!("did:ainur:network:{}", identifier))
    }

    /// Get the DID string
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl std::fmt::Display for DID {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

/// Operation definition for an agent capability
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Operation {
    pub name: String,
    pub category: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_schema: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_schema: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub complexity: Option<String>,
    pub gas_estimate: u64,
}

/// Capability constraints
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CapabilityConstraints {
    pub max_input_size: u64,
    pub max_execution_time_ms: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub concurrent_tasks: Option<u32>,
}

/// Agent capabilities declaration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Capabilities {
    pub domains: Vec<String>,
    pub operations: Vec<Operation>,
    pub constraints: CapabilityConstraints,
    pub interfaces: Vec<String>,
}

impl Capabilities {
    pub fn builder() -> CapabilitiesBuilder {
        CapabilitiesBuilder::default()
    }
}

#[derive(Default)]
pub struct CapabilitiesBuilder {
    domains: Vec<String>,
    operations: Vec<Operation>,
    max_input_size: u64,
    max_execution_time_ms: u64,
    concurrent_tasks: Option<u32>,
    interfaces: Vec<String>,
}

impl CapabilitiesBuilder {
    pub fn domain(mut self, domain: impl Into<String>) -> Self {
        self.domains.push(domain.into());
        self
    }

    pub fn operation(mut self, op: Operation) -> Self {
        self.operations.push(op);
        self
    }

    pub fn max_input_size(mut self, size: u64) -> Self {
        self.max_input_size = size;
        self
    }

    pub fn max_execution_time_ms(mut self, ms: u64) -> Self {
        self.max_execution_time_ms = ms;
        self
    }

    pub fn concurrent_tasks(mut self, count: u32) -> Self {
        self.concurrent_tasks = Some(count);
        self
    }

    pub fn interface(mut self, interface: impl Into<String>) -> Self {
        self.interfaces.push(interface.into());
        self
    }

    pub fn build(self) -> Capabilities {
        Capabilities {
            domains: self.domains,
            operations: self.operations,
            constraints: CapabilityConstraints {
                max_input_size: self.max_input_size,
                max_execution_time_ms: self.max_execution_time_ms,
                concurrent_tasks: self.concurrent_tasks,
            },
            interfaces: self.interfaces,
        }
    }
}

/// Runtime execution environment configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionEnvironment {
    pub memory_limit_mb: u32,
    pub cpu_quota_ms: u32,
    pub network_enabled: bool,
    pub filesystem_enabled: bool,
}

/// Runtime endpoint configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Endpoint {
    pub protocol: String,
    pub address: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tls: Option<bool>,
}

/// Runtime information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuntimeInfo {
    pub protocol: String,
    pub implementation: String,
    pub version: String,
    pub wasm_engine: String,
    pub wasm_version: String,
    pub module_hash: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub module_url: Option<String>,
    pub execution_environment: ExecutionEnvironment,
    pub endpoints: Vec<Endpoint>,
}

/// Reputation badge
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Badge {
    #[serde(rename = "type")]
    pub badge_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub threshold: Option<String>,
    pub issued_by: DID,
    pub issued_at: DateTime<Utc>,
}

/// Agent reputation information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Reputation {
    pub trust_score: f64,
    pub total_tasks: u64,
    pub successful_tasks: u64,
    pub failed_tasks: u64,
    pub success_rate: f64,
    pub average_execution_time_ms: u64,
    pub uptime_percentage: f64,
    pub peer_endorsements: u32,
    pub violations: u32,
    pub created_at: DateTime<Utc>,
    pub last_active: DateTime<Utc>,
    pub badges: Vec<Badge>,
    pub slashing_history: Vec<serde_json::Value>,
}

impl Default for Reputation {
    fn default() -> Self {
        let now = Utc::now();
        Self {
            trust_score: 50.0,
            total_tasks: 0,
            successful_tasks: 0,
            failed_tasks: 0,
            success_rate: 0.0,
            average_execution_time_ms: 0,
            uptime_percentage: 100.0,
            peer_endorsements: 0,
            violations: 0,
            created_at: now,
            last_active: now,
            badges: Vec::new(),
            slashing_history: Vec::new(),
        }
    }
}

/// Pricing discount
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Discount {
    #[serde(rename = "type")]
    pub discount_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub min_tasks: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub min_trust_score: Option<f64>,
    pub discount_percentage: f64,
}

/// Surge pricing configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SurgePricing {
    pub enabled: bool,
    pub multiplier_max: f64,
    pub demand_threshold: f64,
}

/// Economic parameters
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Economic {
    pub pricing_model: String,
    pub base_price_uainur: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub surge_pricing: Option<SurgePricing>,
    pub discounts: Vec<Discount>,
    pub payment_methods: Vec<String>,
    pub escrow_required: bool,
    pub refund_policy: String,
}

impl Default for Economic {
    fn default() -> Self {
        Self {
            pricing_model: "per_operation".to_string(),
            base_price_uainur: 100,
            surge_pricing: None,
            discounts: Vec::new(),
            payment_methods: vec!["ainur".to_string()],
            escrow_required: false,
            refund_policy: "full_refund_on_failure".to_string(),
        }
    }
}

/// P2P network configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct P2PConfig {
    pub peer_id: String,
    pub listen_addresses: Vec<String>,
    pub announce_addresses: Vec<String>,
    pub protocols: Vec<String>,
}

/// Discovery methods
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Discovery {
    pub methods: Vec<String>,
    pub bootstrap_nodes: Vec<String>,
}

/// Latency targets
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LatencyTargets {
    pub p50_ms: u64,
    pub p95_ms: u64,
    pub p99_ms: u64,
}

/// Availability information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Availability {
    pub regions: Vec<String>,
    pub latency_targets: LatencyTargets,
}

/// Network information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Network {
    pub p2p: P2PConfig,
    pub discovery: Discovery,
    pub availability: Availability,
}

/// Credential subject (the agent's actual data)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CredentialSubject {
    pub id: DID,
    #[serde(rename = "type")]
    pub subject_type: String,
    pub name: String,
    pub description: String,
    pub version: String,
    pub capabilities: Capabilities,
    pub runtime: RuntimeInfo,
    pub reputation: Reputation,
    pub economic: Economic,
    pub network: Network,
}

/// Cryptographic proof for the AgentCard
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Proof {
    #[serde(rename = "type")]
    pub proof_type: String,
    pub created: DateTime<Utc>,
    pub verification_method: String,
    pub proof_purpose: String,
    pub proof_value: String,
}

/// Complete AgentCard (W3C Verifiable Credential)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentCard {
    #[serde(rename = "@context")]
    pub context: Vec<String>,
    pub id: String,
    #[serde(rename = "type")]
    pub card_type: Vec<String>,
    pub issuer: DID,
    #[serde(rename = "issuanceDate")]
    pub issuance_date: DateTime<Utc>,
    #[serde(rename = "expirationDate")]
    pub expiration_date: DateTime<Utc>,
    #[serde(rename = "credentialSubject")]
    pub credential_subject: CredentialSubject,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub proof: Option<Proof>,
}

impl AgentCard {
    pub fn builder() -> AgentCardBuilder {
        AgentCardBuilder::default()
    }

    /// Convert to JSON string
    pub fn to_json(&self) -> Result<String, serde_json::Error> {
        serde_json::to_string_pretty(self)
    }

    /// Convert to JSON value
    pub fn to_json_value(&self) -> Result<serde_json::Value, serde_json::Error> {
        serde_json::to_value(self)
    }

    /// Parse from JSON string
    pub fn from_json(json: &str) -> Result<Self, serde_json::Error> {
        serde_json::from_str(json)
    }
}

#[derive(Default)]
pub struct AgentCardBuilder {
    agent_did: Option<DID>,
    name: Option<String>,
    description: Option<String>,
    version: Option<String>,
    capabilities: Option<Capabilities>,
    runtime: Option<RuntimeInfo>,
    reputation: Option<Reputation>,
    economic: Option<Economic>,
    network: Option<Network>,
    issuer: Option<DID>,
    expiration_days: u64,
}

impl AgentCardBuilder {
    pub fn agent_did(mut self, did: DID) -> Self {
        self.agent_did = Some(did);
        self
    }

    pub fn name(mut self, name: impl Into<String>) -> Self {
        self.name = Some(name.into());
        self
    }

    pub fn description(mut self, desc: impl Into<String>) -> Self {
        self.description = Some(desc.into());
        self
    }

    pub fn version(mut self, version: impl Into<String>) -> Self {
        self.version = Some(version.into());
        self
    }

    pub fn capabilities(mut self, cap: Capabilities) -> Self {
        self.capabilities = Some(cap);
        self
    }

    pub fn runtime(mut self, runtime: RuntimeInfo) -> Self {
        self.runtime = Some(runtime);
        self
    }

    pub fn reputation(mut self, rep: Reputation) -> Self {
        self.reputation = Some(rep);
        self
    }

    pub fn economic(mut self, econ: Economic) -> Self {
        self.economic = Some(econ);
        self
    }

    pub fn network(mut self, net: Network) -> Self {
        self.network = Some(net);
        self
    }

    pub fn issuer(mut self, issuer: DID) -> Self {
        self.issuer = Some(issuer);
        self
    }

    pub fn expiration_days(mut self, days: u64) -> Self {
        self.expiration_days = days;
        self
    }

    pub fn build(self) -> Result<AgentCard, crate::error::Error> {
        let agent_did = self.agent_did.ok_or(crate::error::Error::MissingField("agent_did"))?;
        let name = self.name.ok_or(crate::error::Error::MissingField("name"))?;
        let description = self.description.ok_or(crate::error::Error::MissingField("description"))?;
        let version = self.version.unwrap_or_else(|| "1.0.0".to_string());
        let capabilities = self.capabilities.ok_or(crate::error::Error::MissingField("capabilities"))?;
        let runtime = self.runtime.ok_or(crate::error::Error::MissingField("runtime"))?;
        let reputation = self.reputation.unwrap_or_default();
        let economic = self.economic.unwrap_or_default();
        let network = self.network.ok_or(crate::error::Error::MissingField("network"))?;
        let issuer = self.issuer.unwrap_or_else(|| agent_did.clone());

        let now = Utc::now();
        let expiration_days = if self.expiration_days == 0 { 365 } else { self.expiration_days };
        let expiration = now + chrono::Duration::days(expiration_days as i64);

        let card_id = format!("did:ainur:agentcard:{}", uuid::Uuid::new_v4());

        Ok(AgentCard {
            context: vec![
                "https://www.w3.org/2018/credentials/v1".to_string(),
                "https://ainur.network/contexts/agentcard/v1".to_string(),
            ],
            id: card_id,
            card_type: vec!["VerifiableCredential".to_string(), "AgentCard".to_string()],
            issuer,
            issuance_date: now,
            expiration_date: expiration,
            credential_subject: CredentialSubject {
                id: agent_did,
                subject_type: "AutonomousAgent".to_string(),
                name,
                description,
                version,
                capabilities,
                runtime,
                reputation,
                economic,
                network,
            },
            proof: None,
        })
    }
}
