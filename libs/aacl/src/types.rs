use agentcard::DID;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// AACL Message types
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum MessageType {
    Request,
    Response,
    Query,
    Notification,
    Negotiation,
    Error,
    Acknowledgment,
    WorkflowRequest,
    WorkflowResponse,
    WorkflowStatus,
    WorkflowError,
    ClarificationRequest,
    SessionControl,
    CapabilityQuery,
    CapabilityQueryResponse,
    PriceNegotiation,
    AsyncRequest,
    AsyncResponse,
    StreamingResponse,
    BatchRequest,
}

/// Intent action vocabulary
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Intent {
    pub action: String,
    pub goal: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub natural_language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parsed: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub confidence: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub capabilities_required: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub constraints: Option<HashMap<String, serde_json::Value>>,
    pub parameters: HashMap<String, serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub context_reference: Option<ContextReference>,
}

impl Intent {
    pub fn new(action: impl Into<String>, goal: impl Into<String>) -> Self {
        Self {
            action: action.into(),
            goal: goal.into(),
            natural_language: None,
            parsed: None,
            confidence: None,
            capabilities_required: None,
            constraints: None,
            parameters: HashMap::new(),
            context_reference: None,
        }
    }

    pub fn with_param(mut self, key: impl Into<String>, value: serde_json::Value) -> Self {
        self.parameters.insert(key.into(), value);
        self
    }

    pub fn with_capability(mut self, capability: impl Into<String>) -> Self {
        self.capabilities_required
            .get_or_insert_with(Vec::new)
            .push(capability.into());
        self
    }

    pub fn with_constraint(
        mut self,
        key: impl Into<String>,
        value: serde_json::Value,
    ) -> Self {
        self.constraints
            .get_or_insert_with(HashMap::new)
            .insert(key.into(), value);
        self
    }
}

/// Context reference for accessing previous conversation state
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContextReference {
    pub message_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub field: Option<String>,
}

/// Message metadata
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Metadata {
    pub priority: String,
    pub timeout_ms: u64,
    pub language: String,
    pub user_agent: String,
}

impl Default for Metadata {
    fn default() -> Self {
        Self {
            priority: "normal".to_string(),
            timeout_ms: 5000,
            language: "en".to_string(),
            user_agent: "ainur-sdk/1.0.0".to_string(),
        }
    }
}

/// Cryptographic signature for AACL messages
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Signature {
    #[serde(rename = "type")]
    pub sig_type: String,
    pub created: DateTime<Utc>,
    pub verification_method: String,
    pub proof_purpose: String,
    pub proof_value: String,
}

/// Base AACL message structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AACLMessage {
    #[serde(rename = "@context")]
    pub context: String,
    #[serde(rename = "@type")]
    pub message_type: String,
    pub id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub conversation_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub in_reply_to: Option<String>,
    pub timestamp: DateTime<Utc>,
    pub from: DID,
    pub to: DID,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub intent: Option<Intent>,
    pub payload: serde_json::Value,
    pub metadata: Metadata,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<Signature>,
}

impl AACLMessage {
    pub fn builder() -> AACLMessageBuilder {
        AACLMessageBuilder::default()
    }

    /// Convert to JSON string
    pub fn to_json(&self) -> Result<String, serde_json::Error> {
        serde_json::to_string_pretty(self)
    }

    /// Parse from JSON string
    pub fn from_json(json: &str) -> Result<Self, serde_json::Error> {
        serde_json::from_str(json)
    }
}

#[derive(Default)]
pub struct AACLMessageBuilder {
    message_type: Option<String>,
    conversation_id: Option<String>,
    in_reply_to: Option<String>,
    from: Option<DID>,
    to: Option<DID>,
    intent: Option<Intent>,
    payload: serde_json::Value,
    metadata: Option<Metadata>,
}

impl AACLMessageBuilder {
    pub fn message_type(mut self, msg_type: impl Into<String>) -> Self {
        self.message_type = Some(msg_type.into());
        self
    }

    pub fn conversation_id(mut self, id: impl Into<String>) -> Self {
        self.conversation_id = Some(id.into());
        self
    }

    pub fn in_reply_to(mut self, id: impl Into<String>) -> Self {
        self.in_reply_to = Some(id.into());
        self
    }

    pub fn from(mut self, did: DID) -> Self {
        self.from = Some(did);
        self
    }

    pub fn to(mut self, did: DID) -> Self {
        self.to = Some(did);
        self
    }

    pub fn intent(mut self, intent: Intent) -> Self {
        self.intent = Some(intent);
        self
    }

    pub fn payload(mut self, payload: serde_json::Value) -> Self {
        self.payload = payload;
        self
    }

    pub fn metadata(mut self, metadata: Metadata) -> Self {
        self.metadata = Some(metadata);
        self
    }

    pub fn build(self) -> Result<AACLMessage, crate::error::Error> {
        let message_type = self
            .message_type
            .ok_or(crate::error::Error::MissingField("message_type"))?;
        let from = self.from.ok_or(crate::error::Error::MissingField("from"))?;
        let to = self.to.ok_or(crate::error::Error::MissingField("to"))?;

        let message_id = format!("msg:{}", uuid::Uuid::new_v4());

        Ok(AACLMessage {
            context: "https://ainur.network/contexts/aacl/v1".to_string(),
            message_type,
            id: message_id,
            conversation_id: self.conversation_id,
            in_reply_to: self.in_reply_to,
            timestamp: Utc::now(),
            from,
            to,
            intent: self.intent,
            payload: self.payload,
            metadata: self.metadata.unwrap_or_default(),
            signature: None,
        })
    }
}

/// Request message
pub type Request = AACLMessage;

/// Response result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseResult {
    pub value: serde_json::Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub confidence: f64,
}

/// Execution metadata
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionMetadata {
    pub duration_ms: u64,
    pub gas_used: u64,
    pub cost_uainur: u64,
}

/// Response message payload
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponsePayload {
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<ResponseResult>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<ErrorInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub execution_metadata: Option<ExecutionMetadata>,
}

/// Error information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorInfo {
    pub code: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub recovery: Option<Recovery>,
}

/// Recovery options for errors
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Recovery {
    pub action: String,
    pub suggestion: String,
}

/// Workflow step definition
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkflowStep {
    pub step_id: String,
    pub agent_capability: String,
    pub intent: Intent,
    pub outputs: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub inputs: Option<Vec<String>>,
}

/// Workflow definition
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Workflow {
    pub workflow_id: String,
    pub goal: String,
    pub steps: Vec<WorkflowStep>,
    pub dependencies: HashMap<String, Vec<String>>,
}

/// Conversation context
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConversationContext {
    pub conversation_id: String,
    pub participants: Vec<DID>,
    pub created_at: DateTime<Utc>,
    pub topic: String,
    pub previous_results: Vec<PreviousResult>,
    pub shared_state: HashMap<String, serde_json::Value>,
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PreviousResult {
    pub message_id: String,
    pub result: serde_json::Value,
    pub timestamp: DateTime<Utc>,
}

/// Capability query
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CapabilityQuery {
    pub capabilities: CapabilityFilter,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub constraints: Option<HashMap<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CapabilityFilter {
    pub domains: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub operations: Option<Vec<String>>,
}

/// Agent match from capability query
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentMatch {
    pub agent_did: DID,
    pub agent_name: String,
    pub trust_score: f64,
    pub price_uainur: u64,
    pub estimated_time_ms: u64,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_intent_builder() {
        let intent = Intent::new("compute", "Calculate 5 + 7")
            .with_param("operation", serde_json::json!("add"))
            .with_param("a", serde_json::json!(5))
            .with_param("b", serde_json::json!(7))
            .with_capability("math");

        assert_eq!(intent.action, "compute");
        assert_eq!(intent.parameters.len(), 3);
    }

    #[test]
    fn test_aacl_message_builder() {
        let intent = Intent::new("compute", "Calculate 5 + 7")
            .with_param("operation", serde_json::json!("add"))
            .with_param("a", serde_json::json!(5))
            .with_param("b", serde_json::json!(7));

        let msg = AACLMessage::builder()
            .message_type("Request")
            .from(DID::new_user("alice"))
            .to(DID::new_agent("math-001"))
            .intent(intent)
            .payload(serde_json::json!({}))
            .build()
            .unwrap();

        assert_eq!(msg.message_type, "Request");
        assert!(msg.intent.is_some());
    }

    #[test]
    fn test_aacl_serialization() {
        let intent = Intent::new("compute", "Calculate 5 + 7")
            .with_param("operation", serde_json::json!("add"));

        let msg = AACLMessage::builder()
            .message_type("Request")
            .from(DID::new_user("alice"))
            .to(DID::new_agent("math-001"))
            .intent(intent)
            .payload(serde_json::json!({}))
            .build()
            .unwrap();

        let json = msg.to_json().unwrap();
        assert!(json.contains("@context"));
        assert!(json.contains("ainur.network/contexts/aacl/v1"));

        let parsed = AACLMessage::from_json(&json).unwrap();
        assert_eq!(parsed.message_type, "Request");
    }
}
