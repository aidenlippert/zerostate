# Authentication Layer for DHT Writes

## Overview

The auth layer provides signature verification for Agent Card updates in the DHT. It ensures that only the owner of a DID (the holder of the corresponding private key) can publish or modify their Agent Card.

## Problem

Without authentication, the DHT accepts any writes:
- Attackers could publish fake agent cards
- Malicious actors could modify other agents' cards
- No way to verify card authenticity
- DHT poisoning attacks possible

## Solution

**Ed25519 Signature Verification**
- Cards are signed with Ed25519 private keys
- Signatures are verified before DHT writes
- DID is derived from public key (did:zs:<peerID>)
- Only matching DID/key pairs can update cards

## Architecture

```
┌─────────────┐         ┌──────────────┐         ┌─────────┐
│Agent Creates│────────▶│ Sign Card    │────────▶│ Publish │
│  Card JSON  │         │ with PrivKey │         │ to DHT  │
└─────────────┘         └──────────────┘         └─────────┘
                               │
                               ▼
                      ┌──────────────────┐
                      │ SignedAgentCard  │
                      │  - card (JSON)   │
                      │  - signature     │
                      │  - timestamp     │
                      │  - public_key    │
                      └──────────────────┘
                               │
                               ▼
                      ┌──────────────────┐
                      │ DHT Validator    │
                      │ 1. Check timestamp│
                      │ 2. Verify signature│
                      │ 3. Check DID match│
                      └──────────────────┘
                               │
                       ┌───────┴───────┐
                       ▼               ▼
                   ✅ Accept      ❌ Reject
```

## Usage

### Signing a Card

```go
import "github.com/zerostate/libs/p2p"

validator := p2p.NewAgentCardValidator(logger, true)

// Create agent card
cardData := map[string]interface{}{
    "did": "did:zs:12D3KooW...",
    "capabilities": []string{"text-generation"},
}
cardJSON, _ := json.Marshal(cardData)

// Sign with private key
signedCard, err := validator.SignCard(cardJSON, privKey)
if err != nil {
    return err
}

// signedCard contains:
// - Card: original JSON
// - Signature: hex-encoded Ed25519 signature
// - Timestamp: Unix timestamp
// - PublicKey: hex-encoded public key
```

### Verifying a Card

```go
// Verify signature and DID
err := validator.VerifySignedCard(ctx, signedCard)
if err != nil {
    // Verification failed: invalid signature, expired, or DID mismatch
    return err
}

// Card is authentic - safe to use
```

### Publishing with Authentication

```go
// Validate before DHT publish
err := validator.ValidatePublish(ctx, signedCard)
if err != nil {
    // Reject publication
    return fmt.Errorf("auth failed: %w", err)
}

// Publish to DHT
cid, err := node.PublishSignedCard(ctx, signedCard)
```

## Security Features

### 1. Timestamp Validation
- Cards expire after 1 hour (configurable)
- Prevents replay attacks with old signatures
- Rejects cards with future timestamps (clock skew tolerance: 5 min)

```go
validator.SetMaxAge(30 * time.Minute) // Custom expiration
```

### 2. DID-Key Binding
- DID is cryptographically derived from public key
- Format: `did:zs:<libp2p-peer-id>`
- Peer ID is derived from Ed25519 public key
- Impossible to forge without private key

### 3. Signature Verification
- Ed25519 provides 128-bit security
- Message signed: `card_json + timestamp`
- Constant-time verification (no timing attacks)

### 4. Tamper Detection
- Any modification invalidates signature
- Changing DID, capabilities, or metadata requires new signature
- Timestamp prevents replays

## Configuration

### Default Settings

```go
validator := p2p.NewAgentCardValidator(logger, true)
// enableAuth: true
// maxAge: 1 hour
```

### Custom Configuration

```go
validator := p2p.NewAgentCardValidator(logger, true)

// Change expiration
validator.SetMaxAge(2 * time.Hour)

// Disable auth (for testing)
validator.Disable()

// Re-enable
validator.Enable()
```

## Metrics

### Prometheus Metrics

```prometheus
# Verification attempts
zerostate_auth_verifications_total{result="success"}
zerostate_auth_verifications_total{result="failure_signature"}
zerostate_auth_verifications_total{result="failure_did_mismatch"}
zerostate_auth_verifications_total{result="failure_expired"}
zerostate_auth_verifications_total{result="failure_future"}
zerostate_auth_verifications_total{result="skipped"}

# Publish attempts
zerostate_auth_publish_attempts_total{result="allowed"}
zerostate_auth_publish_attempts_total{result="rejected"}
```

### Example Queries

```promql
# Verification success rate
rate(zerostate_auth_verifications_total{result="success"}[5m])
/ rate(zerostate_auth_verifications_total[5m])

# Rejection reasons
sum by (result) (
  rate(zerostate_auth_verifications_total{result=~"failure_.*"}[5m])
)

# Publish acceptance rate
rate(zerostate_auth_publish_attempts_total{result="allowed"}[5m])
```

## Error Handling

### Common Errors

**Expired Card**
```
Error: card expired: age=1h5m0s, max=1h0m0s
Solution: Generate new signature with current timestamp
```

**Future Timestamp**
```
Error: card timestamp in future: 2025-11-06 15:30:00
Solution: Check system clock, ensure NTP sync
```

**Signature Verification Failed**
```
Error: signature verification failed
Causes:
  - Wrong private key used
  - Card modified after signing
  - Corrupted signature
Solution: Re-sign card with correct key
```

**DID Mismatch**
```
Error: DID mismatch: card=did:zs:ABC, expected=did:zs:XYZ
Causes:
  - Signed with different key than DID
  - Attacker trying to impersonate
Solution: Ensure DID matches public key
```

## Integration Examples

### Edge Node Auto-Publishing

```go
// In edge-node startup
validator := p2p.NewAgentCardValidator(logger, true)

// Create card
card := identity.AgentCard{
    DID: signer.DID(),
    Capabilities: []identity.Capability{
        {Name: "text-generation"},
    },
}

cardJSON, _ := json.Marshal(card)

// Sign with persistent identity
signedCard, err := validator.SignCard(cardJSON, signer.PrivateKey())
if err != nil {
    return err
}

// Publish with auth
err = validator.ValidatePublish(ctx, signedCard)
if err != nil {
    return fmt.Errorf("validation failed: %w", err)
}

cid, err := node.PublishSignedCard(ctx, signedCard)
```

### Card Resolution with Verification

```go
// Resolve card from DHT
signedCard, err := node.ResolveSignedCard(ctx, did)
if err != nil {
    return nil, err
}

// Verify before use
err = validator.VerifySignedCard(ctx, signedCard)
if err != nil {
    return nil, fmt.Errorf("untrusted card: %w", err)
}

// Card is verified - safe to use
return signedCard.Card, nil
```

## Testing

### Unit Tests

```go
func TestSignAndVerify(t *testing.T) {
    validator := NewAgentCardValidator(logger, true)
    
    // Generate test key
    _, privKey, _ := ed25519.GenerateKey(nil)
    
    // Sign card
    signed, err := validator.SignCard(cardJSON, privKey)
    require.NoError(t, err)
    
    // Verify (will fail due to DID mismatch in test)
    err = validator.VerifySignedCard(ctx, signed)
    // In production, DID would be derived from key
}
```

### E2E Tests

```bash
cd tests/e2e
go test -v -run TestE2E_AuthenticatedPublish
```

## Best Practices

1. **Always Enable Auth in Production**
   ```go
   validator := NewAgentCardValidator(logger, true)
   ```

2. **Use Persistent Keys**
   - Store keys securely (~/.zerostate/keystore/identity.key)
   - Same key = same DID across restarts

3. **Monitor Metrics**
   - Track verification failures
   - Alert on high rejection rates
   - Investigate DID mismatches

4. **Handle Expiration**
   - Re-sign cards before expiration
   - Implement automatic refresh (e.g., every 30 min)

5. **Clock Synchronization**
   - Use NTP for accurate timestamps
   - Monitor clock skew
   - Allow small tolerance (5 min)

## Security Considerations

### Threat Model

**Protected Against:**
- ✅ Card forgery (requires private key)
- ✅ Replay attacks (timestamp validation)
- ✅ Impersonation (DID-key binding)
- ✅ Tampering (signature invalidation)
- ✅ DHT poisoning (authenticated writes)

**Not Protected Against:**
- ❌ Compromised private keys (use secure key storage)
- ❌ DoS attacks (rate limiting needed separately)
- ❌ Sybil attacks (reputation system needed)

### Key Management

**DO:**
- Use hardware security modules (HSM) for production
- Encrypt keys at rest
- Implement key rotation
- Backup keys securely

**DON'T:**
- Share private keys
- Store keys in version control
- Transmit keys unencrypted
- Use weak key derivation

## Migration Path

### Phase 1: Optional Auth (Current)
```go
// Auth enabled but not enforced globally
validator := NewAgentCardValidator(logger, true)
```

### Phase 2: Enforce on Publish
```go
// Reject unsigned cards
if signedCard == nil || signedCard.Signature == "" {
    return errors.New("signature required")
}
```

### Phase 3: Enforce on Resolution
```go
// Verify all resolved cards
err := validator.VerifySignedCard(ctx, card)
if err != nil {
    return nil, fmt.Errorf("untrusted card rejected: %w", err)
}
```

## Future Enhancements

- [ ] Key rotation support
- [ ] Multi-signature cards (delegation)
- [ ] Revocation lists
- [ ] Hardware key support (YubiKey, TPM)
- [ ] Threshold signatures
- [ ] Card versioning with upgrade paths
