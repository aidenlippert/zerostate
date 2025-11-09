package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/agentsdk"
	"go.uber.org/zap"
)

// EchoAgent is a simple agent that echoes back input (for testing)
type EchoAgent struct {
	*agentsdk.BaseAgent
}

// HandleTask processes incoming tasks by echoing the input
func (a *EchoAgent) HandleTask(ctx context.Context, task *agentsdk.Task) (*agentsdk.TaskResult, error) {
	logger := a.GetLogger()

	logger.Info("echo agent received task",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.String("input", string(task.Input)),
	)

	// Parse input
	var input map[string]interface{}
	if err := json.Unmarshal(task.Input, &input); err != nil {
		logger.Error("failed to parse input", zap.Error(err))
		return &agentsdk.TaskResult{
			TaskID: task.ID,
			Status: agentsdk.TaskStatusFailed,
			Error:  fmt.Sprintf("invalid input JSON: %v", err),
		}, err
	}

	// Echo back the input with metadata
	result := map[string]interface{}{
		"echo":          input,
		"message":       "Successfully echoed your input!",
		"timestamp":     time.Now().Unix(),
		"agent_name":    a.GetName(),
		"agent_version": a.GetVersion(),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		logger.Error("failed to marshal result", zap.Error(err))
		return &agentsdk.TaskResult{
			TaskID: task.ID,
			Status: agentsdk.TaskStatusFailed,
			Error:  fmt.Sprintf("failed to serialize result: %v", err),
		}, err
	}

	logger.Info("echo agent completed task successfully",
		zap.String("task_id", task.ID),
	)

	return &agentsdk.TaskResult{
		TaskID: task.ID,
		Status: agentsdk.TaskStatusCompleted,
		Result: resultJSON,
		Cost:   0.10, // Very cheap for echo service
	}, nil
}

func main() {
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	defer logger.Sync()

	logger.Info("starting echo agent")

	// Configure agent
	config := &agentsdk.Config{
		Name:        "EchoAgent",
		Description: "Simple echo agent for testing and development",
		Version:     "1.0.0",
		Capabilities: []agentsdk.Capability{
			{
				Name:        "echo",
				Version:     "1.0.0",
				Description: "Echoes back input data",
				Cost: &agentsdk.Cost{
					Unit:  "task",
					Price: 0.10,
				},
				Limits: &agentsdk.Limits{
					TPS:         100,
					Concurrency: 10,
				},
			},
			{
				Name:        "test",
				Version:     "1.0.0",
				Description: "Test capability for development",
				Cost: &agentsdk.Cost{
					Unit:  "task",
					Price: 0.01,
				},
			},
		},
		DefaultPrice:       0.10,
		MinBudget:          0.05,
		MaxConcurrentTasks: 10,
		TaskTimeout:        30 * time.Second,
		HeartbeatInterval:  15 * time.Second,
		LogLevel:           "debug",
	}

	// Create base agent
	baseAgent := agentsdk.NewBaseAgent(config, logger)

	// Create echo agent
	echoAgent := &EchoAgent{
		BaseAgent: baseAgent,
	}

	// Initialize agent
	ctx := context.Background()
	if err := echoAgent.Initialize(ctx, config); err != nil {
		logger.Fatal("failed to initialize agent", zap.Error(err))
	}

	// Start agent
	if err := echoAgent.Start(ctx); err != nil {
		logger.Fatal("failed to start agent", zap.Error(err))
	}

	logger.Info("echo agent started successfully",
		zap.String("did", echoAgent.GetDID()),
		zap.String("name", echoAgent.GetName()),
		zap.Int("capabilities", len(echoAgent.GetCapabilities())),
	)

	// For WASM deployment, export functions
	wasmAgent := agentsdk.NewWASMAgent(echoAgent, logger)
	if err := wasmAgent.Run(ctx); err != nil {
		logger.Fatal("WASM agent failed", zap.Error(err))
	}
}
