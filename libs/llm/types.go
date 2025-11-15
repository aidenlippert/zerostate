package llm

import "context"

// LLMProvider is the interface that all LLM clients must implement
type LLMProvider interface {
	// Execute sends a prompt and returns the response
	Execute(ctx context.Context, prompt string) (string, error)

	// ExecuteWithSystem sends a prompt with a system instruction
	ExecuteWithSystem(ctx context.Context, prompt, systemInstruction string) (string, error)

	// DecomposeTask breaks down a complex task into executable steps
	DecomposeTask(ctx context.Context, userPrompt string, availableAgents []string) (*TaskPlan, error)

	// GetModel returns the current model name
	GetModel() string
}

// TaskPlan represents a decomposed task with sequential steps
type TaskPlan struct {
	Steps []TaskStep `json:"plan"`
}

// TaskStep represents a single step in a task plan
type TaskStep struct {
	Step        int           `json:"step"`
	Description string        `json:"description"`
	Agent       string        `json:"agent"`                // e.g., "math-agent-v1.0" or "llm"
	Function    string        `json:"function"`             // e.g., "add", "factorial", "generate"
	Args        []interface{} `json:"args"`                 // Arguments for the function (can be string, number, etc.)
	DependsOn   []int         `json:"depends_on,omitempty"` // Steps this depends on (for future parallel execution)
}

// ExecutionContext holds the shared state between task steps
type ExecutionContext struct {
	// Results from previous steps, indexed by step number
	StepResults map[int]interface{} `json:"step_results"`

	// User's original prompt
	OriginalPrompt string `json:"original_prompt"`

	// Accumulated output (for streaming/logging)
	Output []string `json:"output"`
}

// NewExecutionContext creates a new execution context
func NewExecutionContext(prompt string) *ExecutionContext {
	return &ExecutionContext{
		StepResults:    make(map[int]interface{}),
		OriginalPrompt: prompt,
		Output:         make([]string, 0),
	}
}

// AddResult stores a result from a step
func (ec *ExecutionContext) AddResult(step int, result interface{}) {
	ec.StepResults[step] = result
}

// GetResult retrieves a result from a previous step
func (ec *ExecutionContext) GetResult(step int) (interface{}, bool) {
	result, exists := ec.StepResults[step]
	return result, exists
}

// AddOutput appends to the output log
func (ec *ExecutionContext) AddOutput(message string) {
	ec.Output = append(ec.Output, message)
}
