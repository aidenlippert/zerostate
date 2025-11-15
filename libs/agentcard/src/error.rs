use thiserror::Error;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Missing required field: {0}")]
    MissingField(&'static str),

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("Signing error: {0}")]
    Signing(String),

    #[error("Verification error: {0}")]
    Verification(String),

    #[error("Invalid DID format: {0}")]
    InvalidDID(String),

    #[error("Validation error: {0}")]
    Validation(String),
}

pub type Result<T> = std::result::Result<T, Error>;
