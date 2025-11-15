use crate::{error::Result, types::*};
use chrono::Utc;
use ed25519_dalek::{Signature, Signer, SigningKey, Verifier, VerifyingKey};
use sha2::{Digest, Sha256};

/// Sign an AgentCard with an Ed25519 private key
pub fn sign_agentcard(card: &mut AgentCard, signing_key: &SigningKey) -> Result<()> {
    // Create canonical representation for signing
    let mut card_for_signing = card.clone();
    card_for_signing.proof = None; // Remove any existing proof
    
    let canonical_json = serde_json::to_string(&card_for_signing.credential_subject)?;
    
    // Sign the canonical JSON
    let message = canonical_json.as_bytes();
    let signature = signing_key.sign(message);
    
    // Encode signature as base58
    let proof_value = bs58::encode(signature.to_bytes()).into_string();
    
    // Create verification method DID
    let verification_method = format!("{}#keys-1", card.credential_subject.id.as_str());
    
    // Add proof to card
    card.proof = Some(Proof {
        proof_type: "Ed25519Signature2020".to_string(),
        created: Utc::now(),
        verification_method,
        proof_purpose: "assertionMethod".to_string(),
        proof_value,
    });
    
    Ok(())
}

/// Verify an AgentCard signature
pub fn verify_agentcard(card: &AgentCard, public_key: &VerifyingKey) -> Result<bool> {
    let proof = card.proof.as_ref().ok_or_else(|| {
        crate::error::Error::Verification("AgentCard has no proof".to_string())
    })?;
    
    // Decode signature from base58
    let signature_bytes = bs58::decode(&proof.proof_value)
        .into_vec()
        .map_err(|e| crate::error::Error::Verification(format!("Invalid signature encoding: {}", e)))?;
    
    let signature = Signature::from_bytes(&signature_bytes.try_into().map_err(|_| {
        crate::error::Error::Verification("Invalid signature length".to_string())
    })?);
    
    // Recreate canonical representation
    let mut card_for_verification = card.clone();
    card_for_verification.proof = None;
    
    let canonical_json = serde_json::to_string(&card_for_verification.credential_subject)?;
    let message = canonical_json.as_bytes();
    
    // Verify signature
    public_key
        .verify(message, &signature)
        .map(|_| true)
        .map_err(|e| crate::error::Error::Verification(format!("Signature verification failed: {}", e)))
}

/// Generate a new Ed25519 keypair for signing
pub fn generate_keypair() -> (SigningKey, VerifyingKey) {
    use rand::RngCore;
    let mut csprng = rand::rngs::OsRng;
    let mut secret_bytes = [0u8; 32];
    csprng.fill_bytes(&mut secret_bytes);
    let signing_key = SigningKey::from_bytes(&secret_bytes);
    let verifying_key = signing_key.verifying_key();
    (signing_key, verifying_key)
}

/// Hash an AgentCard to create a unique identifier
pub fn hash_agentcard(card: &AgentCard) -> Result<String> {
    let json = serde_json::to_string(&card.credential_subject)?;
    let mut hasher = Sha256::new();
    hasher.update(json.as_bytes());
    let hash = hasher.finalize();
    Ok(format!("sha256:{}", hex::encode(hash)))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sign_and_verify() {
        let (signing_key, verifying_key) = generate_keypair();
        
        let mut card = AgentCard::builder()
            .agent_did(DID::new_agent("test-001"))
            .name("Test Agent")
            .description("Test agent for signing")
            .capabilities(Capabilities::builder()
                .domain("test")
                .interface("ari-v1")
                .max_input_size(1024)
                .max_execution_time_ms(1000)
                .build())
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
        
        // Sign
        sign_agentcard(&mut card, &signing_key).unwrap();
        assert!(card.proof.is_some());
        
        // Verify
        let valid = verify_agentcard(&card, &verifying_key).unwrap();
        assert!(valid);
    }
}
