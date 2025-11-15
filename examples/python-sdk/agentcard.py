"""
AgentCard-VC-v1 Python SDK

Simple Python implementation for creating and working with AgentCards
following the W3C Verifiable Credentials standard.
"""

import json
import hashlib
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any
from dataclasses import dataclass, field, asdict
from enum import Enum


class DIDType(Enum):
    AGENT = "agent"
    USER = "user"
    NETWORK = "network"


@dataclass
class DID:
    """Decentralized Identifier"""
    identifier: str
    did_type: DIDType
    
    def __str__(self) -> str:
        return f"did:ainur:{self.did_type.value}:{self.identifier}"
    
    @classmethod
    def agent(cls, identifier: str) -> 'DID':
        return cls(identifier, DIDType.AGENT)
    
    @classmethod
    def user(cls, identifier: str) -> 'DID':
        return cls(identifier, DIDType.USER)
    
    @classmethod
    def network(cls, identifier: str) -> 'DID':
        return cls(identifier, DIDType.NETWORK)


@dataclass
class Operation:
    """Agent capability operation"""
    name: str
    category: str
    gas_estimate: int = 100
    input_schema: Optional[Dict[str, Any]] = None
    output_schema: Optional[Dict[str, Any]] = None
    complexity: Optional[str] = None


@dataclass
class CapabilityConstraints:
    """Resource constraints for capabilities"""
    max_input_size: int = 1048576  # 1MB
    max_execution_time_ms: int = 30000  # 30 seconds
    concurrent_tasks: Optional[int] = None


@dataclass
class Capabilities:
    """Agent capabilities"""
    domains: List[str]
    operations: List[Operation]
    constraints: CapabilityConstraints
    interfaces: List[str] = field(default_factory=lambda: ["http", "grpc"])


@dataclass
class Badge:
    """Reputation badge"""
    badge_type: str
    threshold: Optional[str] = None
    issued_by: Optional[str] = None
    issued_at: Optional[str] = None


@dataclass
class Reputation:
    """Agent reputation metrics"""
    trust_score: float = 50.0
    total_tasks: int = 0
    successful_tasks: int = 0
    failed_tasks: int = 0
    success_rate: float = 0.0
    average_execution_time_ms: int = 0
    uptime_percentage: float = 100.0
    peer_endorsements: int = 0
    violations: int = 0
    created_at: str = field(default_factory=lambda: datetime.utcnow().isoformat() + "Z")
    last_active: str = field(default_factory=lambda: datetime.utcnow().isoformat() + "Z")
    badges: List[Badge] = field(default_factory=list)
    slashing_history: List[Dict[str, Any]] = field(default_factory=list)


@dataclass
class Discount:
    """Pricing discount"""
    discount_type: str
    discount_percentage: float
    min_tasks: Optional[int] = None
    min_trust_score: Optional[float] = None


@dataclass
class SurgePricing:
    """Dynamic surge pricing"""
    enabled: bool = False
    multiplier_max: float = 2.0
    demand_threshold: float = 0.8


@dataclass
class Economic:
    """Economic parameters"""
    pricing_model: str = "per_operation"
    base_price_uainur: int = 100
    surge_pricing: Optional[SurgePricing] = None
    discounts: List[Discount] = field(default_factory=list)
    payment_methods: List[str] = field(default_factory=lambda: ["ainur"])
    escrow_required: bool = False
    refund_policy: str = "full_refund_on_failure"


@dataclass
class ExecutionEnvironment:
    """Runtime execution environment"""
    memory_limit_mb: int = 128
    cpu_quota_ms: int = 1000
    network_enabled: bool = True
    filesystem_enabled: bool = False


@dataclass
class Endpoint:
    """Network endpoint"""
    protocol: str
    address: str
    tls: Optional[bool] = None


@dataclass
class RuntimeInfo:
    """Agent runtime information"""
    protocol: str
    implementation: str
    version: str
    wasm_engine: str
    wasm_version: str
    module_hash: str
    module_url: Optional[str] = None
    execution_environment: ExecutionEnvironment = field(default_factory=ExecutionEnvironment)
    endpoints: List[Endpoint] = field(default_factory=list)


@dataclass
class P2PConfig:
    """P2P network configuration"""
    peer_id: str
    listen_addresses: List[str]
    announce_addresses: List[str] = field(default_factory=list)
    protocols: List[str] = field(default_factory=lambda: ["/ainur/gossipsub/1.0.0"])


@dataclass
class Discovery:
    """Discovery configuration"""
    methods: List[str] = field(default_factory=lambda: ["mdns", "dht"])
    bootstrap_nodes: List[str] = field(default_factory=list)


@dataclass
class LatencyTargets:
    """Network latency targets"""
    p50_ms: int = 50
    p95_ms: int = 100
    p99_ms: int = 200


@dataclass
class Availability:
    """Availability configuration"""
    regions: List[str] = field(default_factory=lambda: ["global"])
    latency_targets: LatencyTargets = field(default_factory=LatencyTargets)


@dataclass
class Network:
    """Network configuration"""
    p2p: P2PConfig
    discovery: Discovery
    availability: Availability = field(default_factory=Availability)


@dataclass
class CredentialSubject:
    """The core agent data"""
    id: str
    subject_type: str
    name: str
    description: str
    version: str
    capabilities: Capabilities
    runtime: RuntimeInfo
    reputation: Reputation
    economic: Economic
    network: Network


@dataclass
class Proof:
    """Cryptographic proof"""
    proof_type: str
    created: str
    verification_method: str
    proof_purpose: str
    proof_value: str


@dataclass
class AgentCard:
    """W3C Verifiable Credential for agent identity"""
    context: List[str]
    id: str
    card_type: List[str]
    issuer: str
    issuance_date: str
    expiration_date: str
    credential_subject: CredentialSubject
    proof: Optional[Proof] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary with proper JSON-LD keys"""
        data = {
            "@context": self.context,
            "id": self.id,
            "type": self.card_type,
            "issuer": self.issuer,
            "issuanceDate": self.issuance_date,
            "expirationDate": self.expiration_date,
            "credentialSubject": self._serialize_subject(self.credential_subject)
        }
        if self.proof:
            data["proof"] = asdict(self.proof)
        return data
    
    def _serialize_subject(self, subject: CredentialSubject) -> Dict[str, Any]:
        """Serialize credential subject with proper structure"""
        return {
            "id": subject.id,
            "type": subject.subject_type,
            "name": subject.name,
            "description": subject.description,
            "version": subject.version,
            "capabilities": {
                "domains": subject.capabilities.domains,
                "operations": [asdict(op) for op in subject.capabilities.operations],
                "constraints": asdict(subject.capabilities.constraints),
                "interfaces": subject.capabilities.interfaces
            },
            "runtime": self._serialize_runtime(subject.runtime),
            "reputation": asdict(subject.reputation),
            "economic": self._serialize_economic(subject.economic),
            "network": self._serialize_network(subject.network)
        }
    
    def _serialize_runtime(self, runtime: RuntimeInfo) -> Dict[str, Any]:
        """Serialize runtime info"""
        data = {
            "protocol": runtime.protocol,
            "implementation": runtime.implementation,
            "version": runtime.version,
            "wasm_engine": runtime.wasm_engine,
            "wasm_version": runtime.wasm_version,
            "module_hash": runtime.module_hash,
            "execution_environment": asdict(runtime.execution_environment),
            "endpoints": [asdict(ep) for ep in runtime.endpoints]
        }
        if runtime.module_url:
            data["module_url"] = runtime.module_url
        return data
    
    def _serialize_economic(self, economic: Economic) -> Dict[str, Any]:
        """Serialize economic parameters"""
        data = {
            "pricing_model": economic.pricing_model,
            "base_price_uainur": economic.base_price_uainur,
            "discounts": [asdict(d) for d in economic.discounts],
            "payment_methods": economic.payment_methods,
            "escrow_required": economic.escrow_required,
            "refund_policy": economic.refund_policy
        }
        if economic.surge_pricing:
            data["surge_pricing"] = asdict(economic.surge_pricing)
        return data
    
    def _serialize_network(self, network: Network) -> Dict[str, Any]:
        """Serialize network configuration"""
        return {
            "p2p": asdict(network.p2p),
            "discovery": asdict(network.discovery),
            "availability": {
                "regions": network.availability.regions,
                "latency_targets": asdict(network.availability.latency_targets)
            }
        }
    
    def to_json(self, indent: int = 2) -> str:
        """Convert to JSON string"""
        return json.dumps(self.to_dict(), indent=indent)
    
    def hash(self) -> str:
        """Calculate SHA-256 hash of the credential subject"""
        subject_json = json.dumps(self._serialize_subject(self.credential_subject), sort_keys=True)
        hash_bytes = hashlib.sha256(subject_json.encode()).hexdigest()
        return f"sha256:{hash_bytes}"


class AgentCardBuilder:
    """Builder for creating AgentCards"""
    
    def __init__(self):
        self.agent_did: Optional[DID] = None
        self.name: Optional[str] = None
        self.description: Optional[str] = None
        self.version: str = "1.0.0"
        self.capabilities: Optional[Capabilities] = None
        self.runtime: Optional[RuntimeInfo] = None
        self.reputation: Optional[Reputation] = None
        self.economic: Optional[Economic] = None
        self.network: Optional[Network] = None
        self.issuer: Optional[DID] = None
        self.expiration_days: int = 365
    
    def set_agent_did(self, did: DID) -> 'AgentCardBuilder':
        """Set the agent DID"""
        self.agent_did = did
        return self
    
    def set_name(self, name: str) -> 'AgentCardBuilder':
        """Set the agent name"""
        self.name = name
        return self
    
    def set_description(self, description: str) -> 'AgentCardBuilder':
        """Set the agent description"""
        self.description = description
        return self
    
    def set_version(self, version: str) -> 'AgentCardBuilder':
        """Set the agent version"""
        self.version = version
        return self
    
    def set_capabilities(self, capabilities: Capabilities) -> 'AgentCardBuilder':
        """Set the agent capabilities"""
        self.capabilities = capabilities
        return self
    
    def set_runtime(self, runtime: RuntimeInfo) -> 'AgentCardBuilder':
        """Set the runtime info"""
        self.runtime = runtime
        return self
    
    def set_reputation(self, reputation: Reputation) -> 'AgentCardBuilder':
        """Set the reputation"""
        self.reputation = reputation
        return self
    
    def set_economic(self, economic: Economic) -> 'AgentCardBuilder':
        """Set economic parameters"""
        self.economic = economic
        return self
    
    def set_network(self, network: Network) -> 'AgentCardBuilder':
        """Set network configuration"""
        self.network = network
        return self
    
    def set_issuer(self, issuer: DID) -> 'AgentCardBuilder':
        """Set the issuer DID"""
        self.issuer = issuer
        return self
    
    def set_expiration_days(self, days: int) -> 'AgentCardBuilder':
        """Set expiration in days"""
        self.expiration_days = days
        return self
    
    def build(self) -> AgentCard:
        """Build the AgentCard"""
        if not self.agent_did:
            raise ValueError("agent_did is required")
        if not self.name:
            raise ValueError("name is required")
        if not self.description:
            raise ValueError("description is required")
        if not self.capabilities:
            raise ValueError("capabilities are required")
        if not self.runtime:
            raise ValueError("runtime is required")
        if not self.network:
            raise ValueError("network is required")
        
        # Use defaults
        reputation = self.reputation or Reputation()
        economic = self.economic or Economic()
        issuer = self.issuer or self.agent_did
        
        # Generate timestamps
        now = datetime.utcnow()
        issuance_date = now.isoformat() + "Z"
        expiration_date = (now + timedelta(days=self.expiration_days)).isoformat() + "Z"
        
        # Generate card ID
        import uuid
        card_id = f"did:ainur:agentcard:{uuid.uuid4()}"
        
        # Build credential subject
        subject = CredentialSubject(
            id=str(self.agent_did),
            subject_type="AutonomousAgent",
            name=self.name,
            description=self.description,
            version=self.version,
            capabilities=self.capabilities,
            runtime=self.runtime,
            reputation=reputation,
            economic=economic,
            network=self.network
        )
        
        # Create AgentCard
        return AgentCard(
            context=[
                "https://www.w3.org/2018/credentials/v1",
                "https://ainur.network/contexts/agentcard/v1"
            ],
            id=card_id,
            card_type=["VerifiableCredential", "AgentCard"],
            issuer=str(issuer),
            issuance_date=issuance_date,
            expiration_date=expiration_date,
            credential_subject=subject,
            proof=None
        )
