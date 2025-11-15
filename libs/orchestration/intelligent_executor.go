package orchestration

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/llm"
	"github.com/aidenlippert/zerostate/libs/storage"
	"go.uber.org/zap"
)

// IntelligentTaskExecutor uses LLM to decompose tasks and executes them with WASM agents
type IntelligentTaskExecutor struct {
	llmClient  llm.LLMProvider
	wasmRunner *execution.WASMRunnerV2
	r2Storage  *storage.R2Storage
	logger     *zap.Logger
}

// NewIntelligentTaskExecutor creates a new intelligent executor
func NewIntelligentTaskExecutor(
	llmClient llm.LLMProvider,
	wasmRunner *execution.WASMRunnerV2,
	r2Storage *storage.R2Storage,
	logger *zap.Logger,
) *IntelligentTaskExecutor {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &IntelligentTaskExecutor{
		llmClient:  llmClient,
		wasmRunner: wasmRunner,
		r2Storage:  r2Storage,
		logger:     logger,
	}
}

// ExecuteTask executes a task by decomposing it with LLM and running steps
func (e *IntelligentTaskExecutor) ExecuteTask(
	ctx context.Context,
	task *Task,
	agent *identity.AgentCard,
) (*TaskResult, error) {
	startTime := time.Now()

	// Agent parameter is optional for intelligent executor (uses WASM from R2)
	agentDID := "intelligent-executor"
	if agent != nil {
		agentDID = agent.DID
	}

	e.logger.Info("executing task with intelligent decomposition",
		zap.String("task_id", task.ID),
		zap.String("query", task.Description),
		zap.Strings("capabilities", task.Capabilities),
	)

	// Step 1: Determine available agent types from task capabilities or use defaults
	availableAgents := task.Capabilities
	if len(availableAgents) == 0 {
		// Default to common WASM agents available in R2
		availableAgents = []string{"math", "string", "json", "validation"}
	}

	// Step 2: Use LLM to decompose task into steps
	taskDescription := task.Description
	if taskDescription == "" {
		// Use input query as description if Description is empty
		if query, ok := task.Input["query"].(string); ok {
			taskDescription = query
		}
	}

	plan, err := e.llmClient.DecomposeTask(ctx, taskDescription, availableAgents)
	if err != nil {
		e.logger.Error("failed to decompose task",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
		return &TaskResult{
			TaskID:      task.ID,
			Status:      TaskStatusFailed,
			Error:       fmt.Sprintf("Task decomposition failed: %v", err),
			ExecutionMS: time.Since(startTime).Milliseconds(),
			AgentDID:    agentDID,
			Timestamp:   time.Now(),
		}, err
	}

	e.logger.Info("task decomposed into steps",
		zap.String("task_id", task.ID),
		zap.Int("num_steps", len(plan.Steps)),
	)

	// Step 3: Execute plan step-by-step
	execContext := llm.NewExecutionContext(taskDescription)
	var finalResult interface{}

	for _, step := range plan.Steps {
		e.logger.Info("executing step",
			zap.String("task_id", task.ID),
			zap.Int("step", step.Step),
			zap.String("agent", step.Agent),
			zap.String("function", step.Function),
		)

		// Handle LLM-only steps (for text generation)
		if step.Agent == "llm" {
			result, err := e.executeLLMStep(ctx, step, execContext)
			if err != nil {
				return e.failedResult(task, agentDID, err, startTime), err
			}
			execContext.AddResult(step.Step, result)
			finalResult = result
			continue
		}

		// Handle WASM agent steps
		result, err := e.executeWASMStep(ctx, step, execContext, agentDID)
		if err != nil {
			return e.failedResult(task, agentDID, err, startTime), err
		}

		execContext.AddResult(step.Step, result)
		finalResult = result

		e.logger.Info("step completed",
			zap.String("task_id", task.ID),
			zap.Int("step", step.Step),
			zap.Any("result", result),
		)
	}

	// Step 4: Return final result
	executionTime := time.Since(startTime)

	e.logger.Info("task completed successfully",
		zap.String("task_id", task.ID),
		zap.Duration("execution_time", executionTime),
		zap.Any("final_result", finalResult),
	)

	return &TaskResult{
		TaskID: task.ID,
		Status: TaskStatusCompleted,
		Result: map[string]interface{}{
			"final_result":      finalResult,
			"steps_executed":    len(plan.Steps),
			"execution_plan":    plan,
			"execution_context": execContext,
		},
		ExecutionMS: executionTime.Milliseconds(),
		AgentDID:    agentDID,
		Timestamp:   time.Now(),
	}, nil
}

// executeWASMStep executes a single WASM function
func (e *IntelligentTaskExecutor) executeWASMStep(
	ctx context.Context,
	step llm.TaskStep,
	execContext *llm.ExecutionContext,
	agentDID string,
) (interface{}, error) {
	// Convert args, replacing placeholders with previous step results
	convertedArgs, err := e.convertArgs(step.Args, execContext)
	if err != nil {
		return nil, fmt.Errorf("failed to convert args: %w", err)
	}

	// Determine R2 key for agent
	r2Key := fmt.Sprintf("agents/%s.wasm", step.Agent)

	// If agent has endpoints with HTTP URL, use that as the binary URL
	// (For now, we assume the agent binary is at agents/{agent-name}.wasm in R2)

	// Execute WASM function
	result, err := e.wasmRunner.Execute(ctx, &execution.WASMExecutionRequest{
		R2Key:    r2Key,
		Function: step.Function,
		Args:     convertedArgs,
		Timeout:  30 * time.Second,
	})

	if err != nil {
		return nil, fmt.Errorf("WASM execution failed: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("WASM execution failed: %s", result.Error)
	}

	return result.Result, nil
}

// executeLLMStep executes a text generation step
func (e *IntelligentTaskExecutor) executeLLMStep(
	ctx context.Context,
	step llm.TaskStep,
	execContext *llm.ExecutionContext,
) (interface{}, error) {
	// Build prompt with context from previous steps
	prompt := step.Description
	if len(execContext.StepResults) > 0 {
		prompt += fmt.Sprintf("\n\nPrevious results: %v", execContext.StepResults)
	}

	response, err := e.llmClient.Execute(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM execution failed: %w", err)
	}

	return response, nil
}

// convertArgs converts args to appropriate types, replacing placeholders
func (e *IntelligentTaskExecutor) convertArgs(
	args []interface{},
	execContext *llm.ExecutionContext,
) ([]interface{}, error) {
	converted := make([]interface{}, len(args))

	for i, arg := range args {
		// If already a number or bool, keep as is
		switch v := arg.(type) {
		case int, int64, float64, bool:
			converted[i] = v
			continue
		case string:
			// String arg - check for placeholders
			arg = v
		default:
			// Unknown type, convert to string
			arg = fmt.Sprintf("%v", v)
		}

		argStr, ok := arg.(string)
		if !ok {
			converted[i] = arg
			continue
		}

		// Check for step result placeholders like "{{step_1_result}}" or "$step_1_result$"
		if len(argStr) > 2 && (argStr[:2] == "{{" || argStr[0] == '$') {
			// Extract step number
			var stepNum int
			if argStr[:2] == "{{" {
				// Format: {{step_1_result}}
				_, err := fmt.Sscanf(argStr, "{{step_%d_result}}", &stepNum)
				if err != nil {
					return nil, fmt.Errorf("invalid placeholder format: %s", argStr)
				}
			} else {
				// Format: $step_1_result$
				_, err := fmt.Sscanf(argStr, "$step_%d_result$", &stepNum)
				if err != nil {
					return nil, fmt.Errorf("invalid placeholder format: %s", argStr)
				}
			}

			// Get result from previous step
			result, exists := execContext.GetResult(stepNum)
			if !exists {
				return nil, fmt.Errorf("step %d result not found", stepNum)
			}

			converted[i] = result
			continue
		}

		// Try to parse as number
		if intVal, err := strconv.ParseInt(argStr, 10, 64); err == nil {
			converted[i] = intVal
			continue
		}

		if floatVal, err := strconv.ParseFloat(argStr, 64); err == nil {
			converted[i] = floatVal
			continue
		}

		// Keep as string
		converted[i] = argStr
	}

	return converted, nil
}

// failedResult creates a failed task result
func (e *IntelligentTaskExecutor) failedResult(
	task *Task,
	agentDID string,
	err error,
	startTime time.Time,
) *TaskResult {
	return &TaskResult{
		TaskID:      task.ID,
		Status:      TaskStatusFailed,
		Error:       err.Error(),
		ExecutionMS: time.Since(startTime).Milliseconds(),
		AgentDID:    agentDID,
		Timestamp:   time.Now(),
	}
}
