"""
AACL-v1 Python SDK

Simple Python implementation for creating and working with AACL messages
(Ainur Agent Communication Language).
"""

import json
from datetime import datetime
from typing import Dict, List, Optional, Any
from dataclasses import dataclass, field
from enum import Enum
import uuid


class MessageType(Enum):
    """AACL message types"""
    REQUEST = "Request"
    RESPONSE = "Response"
    QUERY = "Query"
    NOTIFICATION = "Notification"
    NEGOTIATION = "Negotiation"
    ERROR = "Error"
    ACKNOWLEDGMENT = "Acknowledgment"
    WORKFLOW_REQUEST = "WorkflowRequest"
    WORKFLOW_RESPONSE = "WorkflowResponse"
    WORKFLOW_STATUS = "WorkflowStatus"
    STREAMING = "Streaming"


@dataclass
class Intent:
    """User intent description"""
    action: str
    goal: str
    natural_language: Optional[str] = None
    parsed: Optional[Dict[str, Any]] = None
    confidence: Optional[float] = None
    capabilities_required: List[str] = field(default_factory=list)
    parameters: Dict[str, Any] = field(default_factory=dict)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        data = {
            "action": self.action,
            "goal": self.goal,
            "parameters": self.parameters
        }
        if self.natural_language:
            data["natural_language"] = self.natural_language
        if self.parsed:
            data["parsed"] = self.parsed
        if self.confidence is not None:
            data["confidence"] = self.confidence
        if self.capabilities_required:
            data["capabilities_required"] = self.capabilities_required
        return data


@dataclass
class ExecutionMetadata:
    """Execution metadata"""
    duration_ms: int
    gas_used: int
    cost_uainur: int
    agent_version: str
    agent_trust_score: float
    execution_node_id: str
    retry_count: int = 0
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        return {
            "duration_ms": self.duration_ms,
            "gas_used": self.gas_used,
            "cost_uainur": self.cost_uainur,
            "agent_version": self.agent_version,
            "agent_trust_score": self.agent_trust_score,
            "execution_node_id": self.execution_node_id,
            "retry_count": self.retry_count
        }


@dataclass
class ResponseResult:
    """Response result"""
    value: Any
    result_type: Optional[str] = None
    confidence: Optional[float] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        data = {"value": self.value}
        if self.result_type:
            data["type"] = self.result_type
        if self.confidence is not None:
            data["confidence"] = self.confidence
        return data


@dataclass
class ResponsePayload:
    """Response message payload"""
    status: str
    result: Optional[ResponseResult] = None
    error: Optional['ErrorInfo'] = None
    execution_metadata: Optional[ExecutionMetadata] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        data = {"status": self.status}
        if self.result:
            data["result"] = self.result.to_dict()
        if self.error:
            data["error"] = self.error.to_dict()
        if self.execution_metadata:
            data["execution_metadata"] = self.execution_metadata.to_dict()
        return data


@dataclass
class ErrorInfo:
    """Error information"""
    code: str
    message: str
    details: Optional[Dict[str, Any]] = None
    recoverable: bool = False
    recovery_suggestions: List[str] = field(default_factory=list)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        data = {
            "code": self.code,
            "message": self.message,
            "recoverable": self.recoverable
        }
        if self.details:
            data["details"] = self.details
        if self.recovery_suggestions:
            data["recovery_suggestions"] = self.recovery_suggestions
        return data


@dataclass
class ConversationContext:
    """Conversation context"""
    conversation_id: str
    previous_messages: List[str] = field(default_factory=list)
    shared_state: Dict[str, Any] = field(default_factory=dict)
    
    def add_message(self, message_id: str):
        """Add a message to history"""
        self.previous_messages.append(message_id)
    
    def set_state(self, key: str, value: Any):
        """Set shared state"""
        self.shared_state[key] = value
    
    def get_state(self, key: str) -> Optional[Any]:
        """Get shared state"""
        return self.shared_state.get(key)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        return {
            "conversation_id": self.conversation_id,
            "previous_messages": self.previous_messages,
            "shared_state": self.shared_state
        }


@dataclass
class WorkflowStep:
    """Workflow step"""
    step_id: str
    agent_did: str
    intent: Intent
    depends_on: List[str] = field(default_factory=list)
    timeout: Optional[int] = None
    retry_policy: Optional[Dict[str, Any]] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        data = {
            "step_id": self.step_id,
            "agent_did": self.agent_did,
            "intent": self.intent.to_dict()
        }
        if self.depends_on:
            data["depends_on"] = self.depends_on
        if self.timeout:
            data["timeout"] = self.timeout
        if self.retry_policy:
            data["retry_policy"] = self.retry_policy
        return data


@dataclass
class Workflow:
    """Multi-agent workflow"""
    workflow_id: str
    goal: str
    steps: List[WorkflowStep]
    dependencies: Dict[str, List[str]] = field(default_factory=dict)
    metadata: Optional[Dict[str, Any]] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        data = {
            "workflow_id": self.workflow_id,
            "goal": self.goal,
            "steps": [step.to_dict() for step in self.steps],
            "dependencies": self.dependencies
        }
        if self.metadata:
            data["metadata"] = self.metadata
        return data


@dataclass
class AACLMessage:
    """AACL message"""
    message_type: MessageType
    from_did: str
    to_did: str
    message_id: str
    timestamp: str
    intent: Optional[Intent] = None
    payload: Optional[Any] = None
    conversation_context: Optional[ConversationContext] = None
    signature: Optional[str] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary with JSON-LD structure"""
        data = {
            "@context": "https://ainur.network/contexts/aacl/v1",
            "@type": self.message_type.value,
            "id": self.message_id,
            "from": self.from_did,
            "to": self.to_did,
            "timestamp": self.timestamp
        }
        
        if self.intent:
            data["intent"] = self.intent.to_dict()
        
        if self.payload:
            if hasattr(self.payload, 'to_dict'):
                data["payload"] = self.payload.to_dict()
            else:
                data["payload"] = self.payload
        
        if self.conversation_context:
            data["conversation_context"] = self.conversation_context.to_dict()
        
        if self.signature:
            data["signature"] = self.signature
        
        return data
    
    def to_json(self, indent: int = 2) -> str:
        """Convert to JSON string"""
        return json.dumps(self.to_dict(), indent=indent)
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'AACLMessage':
        """Create from dictionary"""
        message_type = MessageType(data["@type"])
        
        intent = None
        if "intent" in data:
            intent_data = data["intent"]
            intent = Intent(
                action=intent_data["action"],
                goal=intent_data["goal"],
                natural_language=intent_data.get("natural_language"),
                parsed=intent_data.get("parsed"),
                confidence=intent_data.get("confidence"),
                capabilities_required=intent_data.get("capabilities_required", []),
                parameters=intent_data.get("parameters", {})
            )
        
        context = None
        if "conversation_context" in data:
            ctx_data = data["conversation_context"]
            context = ConversationContext(
                conversation_id=ctx_data["conversation_id"],
                previous_messages=ctx_data.get("previous_messages", []),
                shared_state=ctx_data.get("shared_state", {})
            )
        
        return cls(
            message_type=message_type,
            from_did=data["from"],
            to_did=data["to"],
            message_id=data["id"],
            timestamp=data["timestamp"],
            intent=intent,
            payload=data.get("payload"),
            conversation_context=context,
            signature=data.get("signature")
        )


class IntentBuilder:
    """Builder for creating Intents"""
    
    def __init__(self, action: str, goal: str):
        self.action = action
        self.goal = goal
        self.natural_language: Optional[str] = None
        self.parsed: Optional[Dict[str, Any]] = None
        self.confidence: Optional[float] = None
        self.capabilities_required: List[str] = []
        self.parameters: Dict[str, Any] = {}
    
    def with_natural_language(self, nl: str) -> 'IntentBuilder':
        """Set natural language description"""
        self.natural_language = nl
        return self
    
    def with_parsed(self, parsed: Dict[str, Any]) -> 'IntentBuilder':
        """Set parsed data"""
        self.parsed = parsed
        return self
    
    def with_confidence(self, confidence: float) -> 'IntentBuilder':
        """Set confidence score"""
        self.confidence = confidence
        return self
    
    def requires_capability(self, capability: str) -> 'IntentBuilder':
        """Add required capability"""
        self.capabilities_required.append(capability)
        return self
    
    def with_parameter(self, key: str, value: Any) -> 'IntentBuilder':
        """Add parameter"""
        self.parameters[key] = value
        return self
    
    def build(self) -> Intent:
        """Build the intent"""
        return Intent(
            action=self.action,
            goal=self.goal,
            natural_language=self.natural_language,
            parsed=self.parsed,
            confidence=self.confidence,
            capabilities_required=self.capabilities_required,
            parameters=self.parameters
        )


class MessageBuilder:
    """Builder for creating AACL messages"""
    
    def __init__(self, message_type: MessageType, from_did: str, to_did: str):
        self.message_type = message_type
        self.from_did = from_did
        self.to_did = to_did
        self.intent: Optional[Intent] = None
        self.payload: Optional[Any] = None
        self.conversation_context: Optional[ConversationContext] = None
    
    def with_intent(self, intent: Intent) -> 'MessageBuilder':
        """Set intent"""
        self.intent = intent
        return self
    
    def with_payload(self, payload: Any) -> 'MessageBuilder':
        """Set payload"""
        self.payload = payload
        return self
    
    def with_conversation_context(self, context: ConversationContext) -> 'MessageBuilder':
        """Set conversation context"""
        self.conversation_context = context
        return self
    
    def build(self) -> AACLMessage:
        """Build the message"""
        return AACLMessage(
            message_type=self.message_type,
            from_did=self.from_did,
            to_did=self.to_did,
            message_id=f"urn:uuid:{uuid.uuid4()}",
            timestamp=datetime.utcnow().isoformat() + "Z",
            intent=self.intent,
            payload=self.payload,
            conversation_context=self.conversation_context
        )


def create_request(from_did: str, to_did: str, intent: Intent) -> AACLMessage:
    """Create a Request message"""
    return MessageBuilder(MessageType.REQUEST, from_did, to_did).with_intent(intent).build()


def create_response(from_did: str, to_did: str, payload: ResponsePayload) -> AACLMessage:
    """Create a Response message"""
    return MessageBuilder(MessageType.RESPONSE, from_did, to_did).with_payload(payload).build()


def create_error(from_did: str, to_did: str, error_info: ErrorInfo) -> AACLMessage:
    """Create an Error message"""
    return MessageBuilder(MessageType.ERROR, from_did, to_did).with_payload(error_info).build()


def create_workflow_request(from_did: str, to_did: str, workflow: Workflow) -> AACLMessage:
    """Create a WorkflowRequest message"""
    return MessageBuilder(MessageType.WORKFLOW_REQUEST, from_did, to_did).with_payload(workflow).build()


def success_response(result: Any, metadata: Optional[ExecutionMetadata] = None) -> ResponsePayload:
    """Create a success response payload"""
    return ResponsePayload(
        status="success",
        result=ResponseResult(value=result),
        execution_metadata=metadata
    )


def error_response(error_info: ErrorInfo) -> ResponsePayload:
    """Create an error response payload"""
    return ResponsePayload(status="error", error=error_info)


def new_conversation() -> ConversationContext:
    """Create a new conversation context"""
    return ConversationContext(conversation_id=str(uuid.uuid4()))
