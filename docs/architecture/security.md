# Ainur Protocol - Security Architecture

This document outlines the comprehensive security model for the Ainur Protocol, covering authentication, authorization, cryptographic security, network security, and threat mitigation strategies.

## Security Design Principles

### 1. Defense in Depth
Multiple layers of security controls to protect against various attack vectors.

### 2. Zero Trust Architecture
Never trust, always verify - all components must authenticate and authorize every interaction.

### 3. Least Privilege Access
Users, agents, and services receive minimum required permissions for their functions.

### 4. Cryptographic Security
Strong cryptographic primitives for identity, signatures, and data protection.

### 5. Transparency & Auditability
All security-relevant operations are logged and auditable.

## Authentication Architecture

### 1. Multi-Factor Authentication System

```
                    AUTHENTICATION FLOW
    ┌─────────────────────────────────────────────────────────────────┐
    │                    User Registration                            │
    ├─────────────────────────────────────────────────────────────────┤
    │  Email/Password → Email Verification → Optional 2FA Setup      │
    └─────────────────────────────────────────────────────────────────┘
                                   │
    ┌─────────────────────────────────────────────────────────────────┐
    │                      Login Process                              │
    ├─────────────────────────────────────────────────────────────────┤
    │  Credentials → Rate Limiting → 2FA Check → JWT Generation      │
    └─────────────────────────────────────────────────────────────────┘
                                   │
    ┌─────────────────────────────────────────────────────────────────┐
    │                   Request Authentication                        │
    ├─────────────────────────────────────────────────────────────────┤
    │  JWT Token → Signature Verification → Expiry Check → Success   │
    └─────────────────────────────────────────────────────────────────┘
```

### 2. JWT Token Structure

```json
{
  "header": {
    "alg": "EdDSA",
    "typ": "JWT",
    "kid": "key-id-2024"
  },
  "payload": {
    "sub": "user_12345",
    "iss": "ainur-protocol",
    "aud": "ainur-api",
    "iat": 1699920000,
    "exp": 1699923600,
    "roles": ["user"],
    "permissions": ["task:submit", "agent:view"],
    "session_id": "sess_abcdef",
    "rate_limit_tier": "standard"
  }
}
```

### 3. DID-Based Identity

```
DID Authentication Flow:

Agent/User                 DID Registry              Verification Service
    │                          │                           │
    │── Generate Key Pair       │                           │
    │   (Ed25519)              │                           │
    │                          │                           │
    │── Create DID ──────────▶│                           │
    │   did:ainur:12345        │                           │
    │                          │                           │
    │                          │── Store Public Key        │
    │                          │                           │
    │◀── DID Document ────────│                           │
    │                          │                           │
    │                          │                           │
    │── Sign Challenge ───────────────────────────────────▶│
    │   (Authentication)       │                           │
    │                          │                           │
    │                          │◀── Verify Signature ─────│
    │◀── Authentication Proof ────────────────────────────│
```

## Authorization Framework

### 1. Role-Based Access Control (RBAC)

```
Permission Hierarchy:

System Admin
├── Read/Write System Configuration
├── Manage User Accounts
├── View All Analytics
└── Emergency Controls

Agent Owner
├── Manage Own Agents
├── View Earnings & Analytics
├── Configure Agent Settings
└── Access Agent Logs

Task Requester
├── Submit Tasks
├── View Task Status
├── Manage Payments
└── Leave Reviews

Public User
├── Browse Agent Marketplace
├── View Public Metrics
└── Access Documentation
```

### 2. Resource-Based Permissions

```go
// Permission system implementation
type Permission struct {
    Resource string // "agent", "task", "payment"
    Action   string // "create", "read", "update", "delete"
    Scope    string // "own", "public", "all"
}

type Role struct {
    Name        string
    Permissions []Permission
}

// Example role definitions
var UserRole = Role{
    Name: "user",
    Permissions: []Permission{
        {"task", "create", "own"},
        {"task", "read", "own"},
        {"agent", "read", "public"},
        {"payment", "read", "own"},
    },
}

var AgentOwnerRole = Role{
    Name: "agent_owner",
    Permissions: []Permission{
        {"agent", "create", "own"},
        {"agent", "read", "own"},
        {"agent", "update", "own"},
        {"agent", "delete", "own"},
        {"earnings", "read", "own"},
    },
}
```

### 3. API Endpoint Security

```
Endpoint Protection Matrix:

Public Endpoints (No Auth Required):
├── GET /health
├── GET /api/v1/agents (public listing)
├── GET /api/v1/metrics (basic)
└── GET /docs/*

Protected Endpoints (JWT Required):
├── POST /api/v1/tasks/submit
├── GET /api/v1/users/me
├── PUT /api/v1/agents/{id}
└── POST /api/v1/payments/*

Admin Endpoints (Admin Role + IP Whitelist):
├── GET /admin/users
├── POST /admin/system/config
├── DELETE /admin/emergency/stop
└── GET /admin/analytics/full
```

## Cryptographic Security

### 1. Key Management Architecture

```
Key Hierarchy:

Root Keys (Hardware Security Module)
├── JWT Signing Key (EdDSA)
│   ├── Primary Key (Active)
│   └── Secondary Key (Rotation)
├── DID Master Key (Ed25519)
│   ├── Agent Identity Keys
│   └── User Identity Keys
├── Database Encryption Key (AES-256)
│   ├── Column Encryption
│   └── Backup Encryption
└── TLS Certificate Key (RSA/ECDSA)
    ├── API Endpoints
    └── Internal Communication
```

### 2. Signature & Verification

```rust
// Agent signature verification
use ed25519_dalek::{Signature, Verifier, VerifyingKey};

pub fn verify_agent_signature(
    agent_did: &str,
    message: &[u8],
    signature: &[u8],
) -> Result<bool, SignatureError> {
    // Get public key from DID registry
    let public_key = did_registry.get_public_key(agent_did)?;

    // Parse signature
    let signature = Signature::from_slice(signature)?;

    // Parse public key
    let verifying_key = VerifyingKey::from_bytes(&public_key)?;

    // Verify signature
    verifying_key.verify(message, &signature).map(|_| true)
}

// Task result signing
pub fn sign_task_result(
    private_key: &SigningKey,
    task_id: &str,
    result: &[u8],
) -> Result<Vec<u8>, SignatureError> {
    let message = format!("task:{}:result:", task_id);
    let mut full_message = message.into_bytes();
    full_message.extend_from_slice(result);

    let signature = private_key.sign(&full_message);
    Ok(signature.to_bytes().to_vec())
}
```

### 3. Encryption at Rest

```sql
-- Database column encryption
CREATE TABLE sensitive_data (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    encrypted_data BYTEA, -- AES-256-GCM encrypted
    nonce BYTEA,          -- 96-bit nonce
    created_at TIMESTAMP DEFAULT NOW()
);

-- Application layer encryption
CREATE OR REPLACE FUNCTION encrypt_sensitive_data(data TEXT, key BYTEA)
RETURNS BYTEA AS $$
BEGIN
    RETURN pgp_sym_encrypt(data::TEXT, encode(key, 'base64'));
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION decrypt_sensitive_data(encrypted_data BYTEA, key BYTEA)
RETURNS TEXT AS $$
BEGIN
    RETURN pgp_sym_decrypt(encrypted_data, encode(key, 'base64'));
END;
$$ LANGUAGE plpgsql;
```

## Network Security

### 1. Network Architecture

```
Network Security Layers:

Internet (Untrusted)
    │
    ▼
┌─────────────────────────┐
│     CDN/WAF Layer      │  ← DDoS Protection, SSL Termination
│   (Cloudflare)         │
└─────────────────────────┘
    │
    ▼
┌─────────────────────────┐
│   Load Balancer        │  ← Rate Limiting, Health Checks
│   (Fly.io Proxy)       │
└─────────────────────────┘
    │
    ▼
┌─────────────────────────┐
│   API Gateway          │  ← Authentication, Authorization
│   (Internal Network)    │
└─────────────────────────┘
    │
    ▼
┌─────────────────────────┐
│  Private Services      │  ← Database, Storage, Blockchain
│  (VPC/Private Network) │
└─────────────────────────┘
```

### 2. TLS/mTLS Configuration

```yaml
# TLS Configuration
tls_config:
  min_version: "1.3"
  ciphers:
    - "TLS_AES_256_GCM_SHA384"
    - "TLS_CHACHA20_POLY1305_SHA256"
    - "TLS_AES_128_GCM_SHA256"
  cert_rotation: "30d"
  hsts_max_age: "31536000"  # 1 year

# mTLS for service-to-service
mtls_config:
  require_client_cert: true
  verify_client_cert: true
  ca_cert_path: "/etc/ssl/ca/ainur-ca.crt"
  client_cert_path: "/etc/ssl/client/service.crt"
  client_key_path: "/etc/ssl/client/service.key"
```

### 3. P2P Network Security

```
P2P Security Measures:

Peer Authentication:
├── Peer ID derived from public key
├── Challenge-response authentication
├── Reputation-based peer scoring
└── IP/GeoLocation verification

Message Security:
├── End-to-end encryption (Noise protocol)
├── Message signing with peer identity
├── Anti-replay protection (nonces)
└── Message size limits

Network Resilience:
├── Sybil attack resistance
├── Eclipse attack mitigation
├── DHT pollution protection
└── Bootstrap node security
```

## Threat Model & Mitigations

### 1. Attack Vectors & Countermeasures

```
STRIDE Threat Model:

Spoofing:
├── Threat: Impersonation of legitimate users/agents
├── Mitigation: Strong cryptographic identity (DID)
├── Mitigation: Multi-factor authentication
└── Monitoring: Failed authentication attempts

Tampering:
├── Threat: Modification of task results or payments
├── Mitigation: Cryptographic signatures on all data
├── Mitigation: Immutable blockchain records
└── Monitoring: Signature verification failures

Repudiation:
├── Threat: Denial of actions or transactions
├── Mitigation: Comprehensive audit logs
├── Mitigation: Blockchain transaction records
└── Monitoring: Log integrity verification

Information Disclosure:
├── Threat: Unauthorized access to sensitive data
├── Mitigation: Encryption at rest and in transit
├── Mitigation: Access control and permissions
└── Monitoring: Data access patterns

Denial of Service:
├── Threat: Service disruption attacks
├── Mitigation: Rate limiting and DDoS protection
├── Mitigation: Circuit breakers and auto-scaling
└── Monitoring: Traffic patterns and response times

Elevation of Privilege:
├── Threat: Privilege escalation attacks
├── Mitigation: Principle of least privilege
├── Mitigation: Regular permission audits
└── Monitoring: Permission changes and admin actions
```

### 2. Smart Contract Security

```rust
// Escrow pallet security measures
#[pallet::call]
impl<T: Config> Pallet<T> {
    /// Create an escrow with safety checks
    #[pallet::call_index(0)]
    #[pallet::weight(10_000)]
    pub fn create_escrow(
        origin: OriginFor<T>,
        beneficiary: T::AccountId,
        amount: BalanceOf<T>,
        conditions: Vec<u8>,
    ) -> DispatchResult {
        let who = ensure_signed(origin)?;

        // Security checks
        ensure!(amount > Zero::zero(), Error::<T>::InvalidAmount);
        ensure!(conditions.len() <= T::MaxConditionsLength::get(), Error::<T>::ConditionsTooLong);
        ensure!(!Self::is_blacklisted(&who), Error::<T>::AccountBlacklisted);

        // Check sufficient balance
        let balance = T::Currency::free_balance(&who);
        ensure!(balance >= amount, Error::<T>::InsufficientBalance);

        // Rate limiting
        let current_block = <frame_system::Pallet<T>>::block_number();
        let last_escrow = Self::last_escrow_block(&who);
        ensure!(
            current_block.saturating_sub(last_escrow) >= T::MinBlocksBetweenEscrows::get(),
            Error::<T>::TooManyEscrows
        );

        // Create escrow with overflow protection
        let escrow_id = Self::next_escrow_id()
            .checked_add(&1u32.into())
            .ok_or(Error::<T>::EscrowIdOverflow)?;

        // Reserve funds
        T::Currency::reserve(&who, amount)?;

        // Store escrow
        Escrows::<T>::insert(&escrow_id, EscrowAccount {
            creator: who.clone(),
            beneficiary: beneficiary.clone(),
            amount,
            conditions,
            status: EscrowStatus::Created,
            created_at: current_block,
        });

        // Update tracking
        LastEscrowBlock::<T>::insert(&who, current_block);
        NextEscrowId::<T>::set(escrow_id);

        // Emit event
        Self::deposit_event(Event::EscrowCreated {
            escrow_id,
            creator: who,
            beneficiary,
            amount,
        });

        Ok(())
    }
}
```

### 3. Input Validation & Sanitization

```go
// Input validation middleware
func ValidateInput(validator interface{}) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Rate limiting check
        if !rateLimiter.Allow(c.ClientIP()) {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "rate_limit_exceeded",
            })
            c.Abort()
            return
        }

        // Content-Type validation
        contentType := c.GetHeader("Content-Type")
        if !isValidContentType(contentType) {
            c.JSON(http.StatusUnsupportedMediaType, gin.H{
                "error": "invalid_content_type",
            })
            c.Abort()
            return
        }

        // Request size limits
        if c.Request.ContentLength > MaxRequestSize {
            c.JSON(http.StatusRequestEntityTooLarge, gin.H{
                "error": "request_too_large",
            })
            c.Abort()
            return
        }

        // JSON validation
        var input interface{}
        if err := c.ShouldBindJSON(&input); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "invalid_json",
                "details": sanitizeError(err.Error()),
            })
            c.Abort()
            return
        }

        // Custom validation
        if err := validateStruct(input, validator); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "validation_failed",
                "details": err.Error(),
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

// SQL injection prevention
func sanitizeSQL(query string, args ...interface{}) (string, []interface{}) {
    // Use parameterized queries only
    // Never concatenate user input directly
    return query, args
}

// XSS prevention
func sanitizeHTML(input string) string {
    p := bluemonday.StrictPolicy()
    return p.Sanitize(input)
}
```

## Security Monitoring & Incident Response

### 1. Security Event Monitoring

```go
// Security event logger
type SecurityEvent struct {
    Timestamp   time.Time `json:"timestamp"`
    EventType   string    `json:"event_type"`
    Severity    string    `json:"severity"`
    UserID      string    `json:"user_id,omitempty"`
    IP          string    `json:"ip_address"`
    UserAgent   string    `json:"user_agent"`
    Resource    string    `json:"resource"`
    Action      string    `json:"action"`
    Success     bool      `json:"success"`
    Details     map[string]interface{} `json:"details"`
    CorrelationID string  `json:"correlation_id"`
}

// Security events to monitor
const (
    EventTypeAuthFailure     = "auth_failure"
    EventTypePermissionDenied = "permission_denied"
    EventTypeRateLimitExceeded = "rate_limit_exceeded"
    EventTypeSuspiciousActivity = "suspicious_activity"
    EventTypeDataAccess      = "data_access"
    EventTypeAdminAction     = "admin_action"
)

func LogSecurityEvent(eventType, severity string, details map[string]interface{}) {
    event := SecurityEvent{
        Timestamp: time.Now(),
        EventType: eventType,
        Severity:  severity,
        Details:   details,
    }

    // Send to security monitoring system
    securityLogger.Log(event)

    // Trigger alerts for high severity events
    if severity == "HIGH" || severity == "CRITICAL" {
        alertManager.TriggerAlert(event)
    }
}
```

### 2. Automated Threat Detection

```yaml
# Security rules configuration
security_rules:
  authentication:
    max_failed_attempts: 5
    lockout_duration: "15m"
    suspicious_locations: true

  rate_limiting:
    requests_per_minute: 100
    burst_limit: 200
    per_endpoint_limits:
      "/api/v1/tasks/submit": 10
      "/api/v1/agents/upload": 2

  anomaly_detection:
    unusual_access_patterns: true
    geographic_anomalies: true
    time_based_anomalies: true

  data_protection:
    pii_detection: true
    sensitive_data_access_logging: true
    data_exfiltration_detection: true
```

### 3. Incident Response Procedures

```
Security Incident Response:

1. Detection & Triage (0-15 minutes):
   ├── Automated alert triggers
   ├── Severity assessment
   ├── Initial containment
   └── Team notification

2. Investigation (15-60 minutes):
   ├── Log analysis
   ├── Impact assessment
   ├── Root cause analysis
   └── Evidence collection

3. Containment (immediately):
   ├── Isolate affected systems
   ├── Revoke compromised credentials
   ├── Block malicious IPs
   └── Preserve evidence

4. Eradication (1-4 hours):
   ├── Remove malicious code
   ├── Patch vulnerabilities
   ├── Update security controls
   └── Verify system integrity

5. Recovery (2-8 hours):
   ├── Restore from clean backups
   ├── Gradual service restoration
   ├── Enhanced monitoring
   └── User communication

6. Post-Incident (24-48 hours):
   ├── Detailed analysis report
   ├── Security improvements
   ├── Process updates
   └── Team debriefing
```

## Compliance & Auditing

### 1. Audit Trail Requirements

```sql
-- Comprehensive audit logging
CREATE TABLE security_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    user_id UUID,
    session_id TEXT,
    ip_address INET,
    user_agent TEXT,
    action TEXT NOT NULL,
    resource TEXT,
    resource_id TEXT,
    success BOOLEAN NOT NULL,
    error_code TEXT,
    details JSONB,
    correlation_id UUID
);

-- Immutable log entries
CREATE RULE no_update_audit_log AS ON UPDATE TO security_audit_log DO NOTHING;
CREATE RULE no_delete_audit_log AS ON DELETE TO security_audit_log DO NOTHING;

-- Retention policy (7 years for compliance)
CREATE INDEX idx_audit_log_timestamp ON security_audit_log(timestamp);
```

### 2. Security Metrics Dashboard

```
Security KPIs:

Authentication Security:
├── Failed login attempts: <1%
├── Account lockouts: <0.1%
├── 2FA adoption rate: >80%
└── Session hijack attempts: 0

Authorization Security:
├── Permission violations: 0
├── Privilege escalation attempts: 0
├── Unauthorized data access: 0
└── Admin action audit coverage: 100%

Network Security:
├── DDoS attacks blocked: 100%
├── Malicious IPs blocked: >99%
├── SSL/TLS grade: A+
└── Certificate expiry alerts: >30 days

Application Security:
├── SQL injection attempts: 0 success
├── XSS attempts: 0 success
├── CSRF attacks: 0 success
└── API abuse: <0.1%
```

This comprehensive security architecture ensures the Ainur Protocol maintains high security standards while enabling innovation and user adoption in the decentralized AI agent marketplace.