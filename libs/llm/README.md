# LLM Abstraction Layer

This package provides a model-agnostic interface for interacting with Large Language Models (LLMs).

## Features

- **Model Agnostic**: Unified interface for multiple LLM providers
- **Task Decomposition**: Intelligent breakdown of complex tasks into executable steps
- **Execution Context**: Shared state management across multi-step workflows
- **Gemini API Support**: Built-in support for Google Gemini (gemini-2.0-flash-exp)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Orchestrator                           │
│  (Receives complex user prompts, executes plans)            │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                   LLM Service (llm.go)                      │
│  - Task Decomposition                                        │
│  - Text Generation                                           │
│  - Context Management                                        │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                 LLMProvider Interface                        │
│  - Execute(prompt) → response                                │
│  - DecomposeTask(prompt, agents) → TaskPlan                  │
└──────────────────────┬──────────────────────────────────────┘
                       │
         ┌─────────────┴─────────────┐
         ▼                           ▼
┌──────────────────┐        ┌──────────────────┐
│  GeminiClient    │        │  Future: Claude  │
│  (gemini.go)     │        │  OpenAI, etc.    │
└──────────────────┘        └──────────────────┘
```

## Usage

### Basic Text Generation

```go
import (
    "context"
    "github.com/aidenlippert/zerostate/libs/llm"
    "go.uber.org/zap"
)

// Create Gemini client
logger, _ := zap.NewProduction()
client := llm.NewGeminiClient("your-api-key", "gemini-2.0-flash-exp", logger)

// Generate text
ctx := context.Background()
response, err := client.Execute(ctx, "Explain quantum computing in simple terms")
if err != nil {
    log.Fatal(err)
}
fmt.Println(response)
```

### Task Decomposition

```go
// Available agents in your system
agents := []string{"math-agent-v1.0", "data-agent-v1.0"}

// Decompose complex task
plan, err := client.DecomposeTask(
    ctx,
    "Calculate the factorial of 5 and then multiply the result by 7",
    agents,
)

// Execute plan step by step
execContext := llm.NewExecutionContext("Calculate 5! * 7")
for _, step := range plan.Steps {
    // Execute each step using WASMRunnerV2 or LLM
    // Store results in execContext
}
```

### Example Task Plan

Input: "Calculate the factorial of 5 and then multiply the result by 7"

Output:
```json
{
  "plan": [
    {
      "step": 1,
      "description": "Calculate factorial of 5",
      "agent": "math-agent-v1.0",
      "function": "factorial",
      "args": ["5"]
    },
    {
      "step": 2,
      "description": "Multiply result by 7",
      "agent": "math-agent-v1.0",
      "function": "multiply",
      "args": ["{{step_1_result}}", "7"]
    }
  ]
}
```

## Environment Variables

```bash
# Gemini API Key
GEMINI_API_KEY=your_api_key_here

# Optional: Override default model
GEMINI_MODEL=gemini-2.0-flash-exp
```

## Gemini API Details

**Model**: `gemini-2.0-flash-exp` (default)
- Fast inference (~1-2s per request)
- Cost-effective: $0.075/1M input tokens, $0.30/1M output tokens
- Free tier: 15 RPM, 1M tokens/min

**Endpoint**: `https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent`

**Features Used**:
- System instructions for role/behavior definition
- Temperature control for deterministic vs creative outputs
- Token usage tracking for cost monitoring

## Testing

```bash
# Test Gemini API connection
go run tools/test-gemini.go

# Test task decomposition
go run tools/test-task-decomposition.go
```

## Future Extensions

- **Claude Support**: Add Anthropic Claude 3.5 Sonnet
- **OpenAI Support**: Add GPT-4o/GPT-4 Turbo
- **Local LLM Support**: Add Ollama integration (Llama 3, Mistral)
- **Streaming**: Add streaming response support
- **Function Calling**: Add native function calling support
- **Caching**: Add response caching for repeated prompts

## Cost Optimization

- Use Gemini Flash (not Pro) for most tasks: 4x cheaper
- Cache common prompts (task decomposition patterns)
- Set `maxOutputTokens` to limit response size
- Monitor token usage via logs

## Security

- API keys stored in environment variables (not committed to git)
- Rate limiting on client side (15 RPM for free tier)
- No sensitive data logged (only token counts and metadata)
