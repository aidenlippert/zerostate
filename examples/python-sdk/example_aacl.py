#!/usr/bin/env python3
"""
Example: Creating AACL Messages

This demonstrates how to use the AACL Python SDK to create
intent-based agent communication messages.
"""

import sys
sys.path.insert(0, '.')

from aacl import (
    IntentBuilder, MessageBuilder, MessageType,
    create_request, create_response, create_error,
    success_response, error_response,
    ExecutionMetadata, ErrorInfo, ConversationContext,
    new_conversation
)


def main():
    print("=" * 70)
    print(" Creating AACL Messages Example")
    print("=" * 70)
    print()
    
    # Example 1: Simple Request
    print("1. Simple Computation Request")
    print("-" * 70)
    
    intent = IntentBuilder("compute", "Calculate the sum of two numbers") \
        .with_natural_language("Please add 5 and 7") \
        .with_parameter("operation", "add") \
        .with_parameter("a", 5) \
        .with_parameter("b", 7) \
        .requires_capability("math.add") \
        .with_confidence(0.95) \
        .build()
    
    request_msg = create_request(
        from_did="did:ainur:user:alice",
        to_did="did:ainur:agent:math-001",
        intent=intent
    )
    
    print(f"Message ID:   {request_msg.message_id}")
    print(f"Type:         {request_msg.message_type.value}")
    print(f"From:         {request_msg.from_did}")
    print(f"To:           {request_msg.to_did}")
    print(f"Intent:       {intent.action} - {intent.goal}")
    print(f"Parameters:   {intent.parameters}")
    print(f"Confidence:   {intent.confidence:.2%}")
    print()
    print("JSON:")
    print(request_msg.to_json())
    print()
    
    # Example 2: Response with Metadata
    print("\n2. Success Response with Execution Metadata")
    print("-" * 70)
    
    metadata = ExecutionMetadata(
        duration_ms=125,
        gas_used=250,
        cost_uainur=10,
        agent_version="1.0.0",
        agent_trust_score=95.5,
        execution_node_id="did:ainur:agent:math-001"
    )
    
    payload = success_response(
        result={"sum": 12},
        metadata=metadata
    )
    
    response_msg = create_response(
        from_did="did:ainur:agent:math-001",
        to_did="did:ainur:user:alice",
        payload=payload
    )
    
    print(f"Message ID:      {response_msg.message_id}")
    print(f"Status:          {payload.status}")
    print(f"Result:          {payload.result.value}")
    print(f"Duration:        {metadata.duration_ms}ms")
    print(f"Gas Used:        {metadata.gas_used}")
    print(f"Cost:            {metadata.cost_uainur} uAINUR")
    print(f"Trust Score:     {metadata.agent_trust_score}")
    print()
    print("JSON:")
    print(response_msg.to_json())
    print()
    
    # Example 3: Error Response
    print("\n3. Error Response with Recovery")
    print("-" * 70)
    
    error_info = ErrorInfo(
        code="INVALID_INPUT",
        message="Parameter 'b' must be a number",
        details={"param": "b", "received_type": "string", "expected_type": "number"},
        recoverable=True,
        recovery_suggestions=[
            "Provide a numeric value for parameter 'b'",
            "Check input validation rules"
        ]
    )
    
    error_msg = create_error(
        from_did="did:ainur:agent:math-001",
        to_did="did:ainur:user:alice",
        error_info=error_info
    )
    
    print(f"Message ID:   {error_msg.message_id}")
    print(f"Error Code:   {error_info.code}")
    print(f"Message:      {error_info.message}")
    print(f"Recoverable:  {error_info.recoverable}")
    print(f"Suggestions:")
    for suggestion in error_info.recovery_suggestions:
        print(f"  - {suggestion}")
    print()
    print("JSON:")
    print(error_msg.to_json())
    print()
    
    # Example 4: Conversation with Context
    print("\n4. Conversational Request with Context")
    print("-" * 70)
    
    # Start a conversation
    context = new_conversation()
    context.set_state("topic", "mathematics")
    context.set_state("preferred_precision", "high")
    
    conv_intent = IntentBuilder("compute", "Continue calculation") \
        .with_natural_language("Now multiply that result by 3") \
        .with_parameter("operation", "multiply") \
        .with_parameter("use_previous_result", True) \
        .with_parameter("multiplier", 3) \
        .requires_capability("math.multiply") \
        .build()
    
    conv_msg = MessageBuilder(MessageType.REQUEST, 
                              "did:ainur:user:alice",
                              "did:ainur:agent:math-001") \
        .with_intent(conv_intent) \
        .with_conversation_context(context) \
        .build()
    
    print(f"Message ID:        {conv_msg.message_id}")
    print(f"Conversation ID:   {context.conversation_id}")
    print(f"Shared State:")
    for key, value in context.shared_state.items():
        print(f"  {key}: {value}")
    print()
    print("JSON:")
    print(conv_msg.to_json())
    print()
    
    # Example 5: Multi-step Intent
    print("\n5. Complex Multi-Parameter Request")
    print("-" * 70)
    
    complex_intent = IntentBuilder("process", "Transform and analyze data") \
        .with_natural_language("Process the CSV file and calculate statistics") \
        .with_parameter("input_format", "csv") \
        .with_parameter("operations", ["parse", "validate", "aggregate"]) \
        .with_parameter("aggregations", {
            "sum": ["revenue", "profit"],
            "avg": ["rating"],
            "count": ["transactions"]
        }) \
        .with_parameter("output_format", "json") \
        .requires_capability("data.parse") \
        .requires_capability("data.aggregate") \
        .with_confidence(0.87) \
        .build()
    
    complex_msg = create_request(
        from_did="did:ainur:user:bob",
        to_did="did:ainur:agent:data-processor-001",
        intent=complex_intent
    )
    
    print(f"Message ID:       {complex_msg.message_id}")
    print(f"Action:           {complex_intent.action}")
    print(f"Goal:             {complex_intent.goal}")
    print(f"Capabilities:")
    for cap in complex_intent.capabilities_required:
        print(f"  - {cap}")
    print(f"Parameters:       {len(complex_intent.parameters)} total")
    print()
    print("JSON:")
    print(complex_msg.to_json())
    print()
    
    print("=" * 70)
    print(" AACL Messages Created Successfully! ")
    print("=" * 70)
    print()
    print("Key Features Demonstrated:")
    print("  ✓ Intent-based requests with natural language")
    print("  ✓ Rich execution metadata in responses")
    print("  ✓ Recoverable error handling")
    print("  ✓ Conversational context management")
    print("  ✓ Complex multi-parameter intents")
    print()


if __name__ == "__main__":
    main()
