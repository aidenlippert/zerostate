package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	ariv1 "github.com/aidenlippert/zerostate/reference-runtime-v1/pkg/ari/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ARIExecutor executes tasks using ARI-v1 protocol via gRPC
type ARIExecutor struct {
	runtimeAddr string
	conn        *grpc.ClientConn
	agentClient ariv1.AgentClient
	taskClient  ariv1.TaskClient
	logger      *zap.Logger
}

// NewARIExecutor creates a new ARI-v1 executor
func NewARIExecutor(runtimeAddr string, logger *zap.Logger) (*ARIExecutor, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create gRPC connection
	conn, err := grpc.Dial(
		runtimeAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to runtime at %s: %w", runtimeAddr, err)
	}

	logger.Info("Connected to ARI-v1 runtime",
		zap.String("address", runtimeAddr),
	)

	return &ARIExecutor{
		runtimeAddr: runtimeAddr,
		conn:        conn,
		agentClient: ariv1.NewAgentClient(conn),
		taskClient:  ariv1.NewTaskClient(conn),
		logger:      logger,
	}, nil
}

// GetRuntimeInfo queries the runtime's capabilities via GetInfo
func (e *ARIExecutor) GetRuntimeInfo(ctx context.Context) (*ariv1.GetInfoResponse, error) {
	resp, err := e.agentClient.GetInfo(ctx, &ariv1.GetInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime info: %w", err)
	}

	e.logger.Info("Runtime info retrieved",
		zap.String("did", resp.Did),
		zap.String("name", resp.Name),
		zap.Strings("capabilities", resp.Capabilities),
	)

	return resp, nil
}

// ExecuteTask executes a task via ARI-v1 Task/Execute
func (e *ARIExecutor) ExecuteTask(ctx context.Context, task *Task, agent *identity.AgentCard) (*TaskResult, error) {
	startTime := time.Now()

	// Prepare task input - check if function and args are already in task.Input
	var taskInput map[string]interface{}

	if function, ok := task.Input["function"].(string); ok && function != "" {
		// Task already has function/args format - use it directly
		taskInput = map[string]interface{}{
			"function": function,
			"args":     task.Input["args"],
		}
		e.logger.Info("Task has function and args, using directly",
			zap.String("task_id", task.ID),
			zap.String("function", function),
		)
	} else if query, ok := task.Input["query"].(string); ok && query != "" {
		// Task has a query that needs decomposition
		e.logger.Info("Task has query, needs LLM decomposition (not yet implemented)",
			zap.String("task_id", task.ID),
			zap.String("query", query),
		)
		// For now, pass the whole input - runtime will need to handle it
		taskInput = task.Input
	} else {
		// Fallback: use task type as function name
		taskInput = map[string]interface{}{
			"function": task.Type,
			"args":     task.Input["args"],
		}
	}

	inputJSON, err := json.Marshal(taskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task input: %w", err)
	}

	e.logger.Info("Executing task via ARI-v1",
		zap.String("task_id", task.ID),
		zap.String("input", string(inputJSON)),
	)

	// Create streaming request
	stream, err := e.taskClient.Execute(ctx, &ariv1.TaskExecuteRequest{
		TaskId:    task.ID,
		Input:     string(inputJSON),
		TimeoutMs: int32(task.Timeout.Milliseconds()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}

	// Collect streaming responses
	var finalResponse *ariv1.TaskExecuteResponse
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("stream error: %w", err)
		}

		e.logger.Debug("Received task response",
			zap.String("task_id", resp.TaskId),
			zap.String("status", resp.Status.String()),
			zap.Float32("progress", resp.Progress),
		)

		finalResponse = resp

		// Break if completed or failed
		if resp.Status == ariv1.TaskStatus_TASK_STATUS_COMPLETED ||
			resp.Status == ariv1.TaskStatus_TASK_STATUS_FAILED {
			break
		}
	}

	if finalResponse == nil {
		return nil, fmt.Errorf("no response received from runtime")
	}

	// Convert ARI status to our TaskStatus
	var status TaskStatus
	switch finalResponse.Status {
	case ariv1.TaskStatus_TASK_STATUS_COMPLETED:
		status = TaskStatusCompleted
	case ariv1.TaskStatus_TASK_STATUS_FAILED:
		status = TaskStatusFailed
	default:
		status = TaskStatusRunning
	}

	// Parse result
	var result map[string]interface{}
	if finalResponse.Result != "" {
		if err := json.Unmarshal([]byte(finalResponse.Result), &result); err != nil {
			// If not JSON, wrap raw string in a map
			result = map[string]interface{}{
				"value": finalResponse.Result,
			}
		}
	} else {
		result = make(map[string]interface{})
	}

	agentDID := "ari-runtime"
	if agent != nil {
		agentDID = agent.DID
	}

	executionTime := time.Since(startTime)
	e.logger.Info("Task execution completed via ARI-v1",
		zap.String("task_id", task.ID),
		zap.String("status", string(status)),
		zap.Duration("execution_time", executionTime),
	)

	actualCost := extractActualCost(task.Budget, result)

	return &TaskResult{
		TaskID:      task.ID,
		Status:      status,
		Result:      result,
		Error:       finalResponse.Error,
		ExecutionMS: finalResponse.ExecutionMs,
		AgentDID:    agentDID,
		Timestamp:   time.Now(),
		Cost:        actualCost,
	}, nil
}

// Close closes the gRPC connection
func (e *ARIExecutor) Close() error {
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}
