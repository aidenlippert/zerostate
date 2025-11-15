use thiserror::Error;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Missing required field: {0}")]
    MissingField(&'static str),

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("Parse error: {0}")]
    Parse(String),

    #[error("Invalid intent: {0}")]
    InvalidIntent(String),

    #[error("Invalid workflow: {0}")]
    InvalidWorkflow(String),

    #[error("AgentCard error: {0}")]
    AgentCard(#[from] agentcard::Error),
}

pub type Result<T> = std::result::Result<T, Error>;
