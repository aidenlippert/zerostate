pub mod error;
pub mod types;

pub use error::{Error, Result};
pub use types::*;

// Re-export AgentCard types for convenience
pub use agentcard::{AgentCard, DID};
