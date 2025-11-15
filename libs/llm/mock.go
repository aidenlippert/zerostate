package llm

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// MockLLMClient is a mock LLM for testing without API access
type MockLLMClient struct {
	logger *zap.Logger
}

// NewMockLLMClient creates a new mock LLM client
func NewMockLLMClient(logger *zap.Logger) *MockLLMClient {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &MockLLMClient{
		logger: logger,
	}
}

// Execute returns a simple mock response
func (m *MockLLMClient) Execute(ctx context.Context, prompt string) (string, error) {
	m.logger.Info("Mock LLM Execute",
		zap.String("prompt", prompt),
	)
	return fmt.Sprintf("Mock response to: %s", prompt), nil
}

// ExecuteWithSystem returns a mock response with system instruction
func (m *MockLLMClient) ExecuteWithSystem(ctx context.Context, prompt, systemInstruction string) (string, error) {
	m.logger.Info("Mock LLM ExecuteWithSystem",
		zap.String("prompt", prompt),
		zap.String("system", systemInstruction),
	)
	return fmt.Sprintf("Mock response (system: %s): %s", systemInstruction, prompt), nil
}

// DecomposeTask returns a hardcoded plan for common prompts
func (m *MockLLMClient) DecomposeTask(ctx context.Context, userPrompt string, availableAgents []string) (*TaskPlan, error) {
	m.logger.Info("Mock LLM DecomposeTask",
		zap.String("prompt", userPrompt),
		zap.Strings("agents", availableAgents),
	)

	// Detect intent from prompt
	lower := strings.ToLower(userPrompt)

	var steps []TaskStep

	// Pattern 1: "factorial of X and multiply by Y"
	if strings.Contains(lower, "factorial") && strings.Contains(lower, "multiply") {
		// Extract numbers (simplified - looks for common patterns)
		factorialNum := "5" // default
		multiplyNum := "7"  // default

		if strings.Contains(lower, "factorial of 5") || strings.Contains(lower, "5!") {
			factorialNum = "5"
		}
		if strings.Contains(lower, "multiply by 7") || strings.Contains(lower, "* 7") {
			multiplyNum = "7"
		}

		steps = []TaskStep{
			{
				Step:        1,
				Description: fmt.Sprintf("Calculate factorial of %s", factorialNum),
				Agent:       "math-agent-v1.0",
				Function:    "factorial",
				Args:        []interface{}{factorialNum},
			},
			{
				Step:        2,
				Description: fmt.Sprintf("Multiply result by %s", multiplyNum),
				Agent:       "math-agent-v1.0",
				Function:    "multiply",
				Args:        []interface{}{"{{step_1_result}}", multiplyNum},
			},
		}
	} else if strings.Contains(lower, "add") || strings.Contains(lower, "+") {
		// Pattern 2: Simple addition
		steps = []TaskStep{
			{
				Step:        1,
				Description: "Add two numbers",
				Agent:       "math-agent-v1.0",
				Function:    "add",
				Args:        []interface{}{"2", "2"},
			},
		}
	} else if strings.Contains(lower, "multiply") || strings.Contains(lower, "*") {
		// Pattern 3: Simple multiplication
		steps = []TaskStep{
			{
				Step:        1,
				Description: "Multiply two numbers",
				Agent:       "math-agent-v1.0",
				Function:    "multiply",
				Args:        []interface{}{"6", "7"},
			},
		}
	} else {
		// Default: Return a simple plan
		steps = []TaskStep{
			{
				Step:        1,
				Description: "Process user request",
				Agent:       "math-agent-v1.0",
				Function:    "add",
				Args:        []interface{}{"2", "2"},
			},
		}
	}

	return &TaskPlan{
		Steps: steps,
	}, nil
}

// GetModel returns the mock model name
func (m *MockLLMClient) GetModel() string {
	return "mock-llm-v1.0"
}
