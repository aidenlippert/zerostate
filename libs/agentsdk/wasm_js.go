//go:build js && wasm
// +build js,wasm

package agentsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"go.uber.org/zap"
)

// WASMAgent wraps an Agent for WebAssembly execution
type WASMAgent struct {
	agent  Agent
	logger *zap.Logger
}

// NewWASMAgent creates a new WASM agent wrapper
func NewWASMAgent(agent Agent, logger *zap.Logger) *WASMAgent {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &WASMAgent{
		agent:  agent,
		logger: logger,
	}
}

// Export exports the agent functions to JavaScript
func (w *WASMAgent) Export() {
	w.logger.Info("exporting WASM functions",
		zap.String("agent", w.agent.GetName()),
		zap.String("did", w.agent.GetDID()),
	)

	// Export getInfo function
	js.Global().Set("getInfo", js.FuncOf(w.jsGetInfo))

	// Export handleTask function
	js.Global().Set("handleTask", js.FuncOf(w.jsHandleTask))

	// Export health function
	js.Global().Set("health", js.FuncOf(w.jsHealth))

	// Export initialize function
	js.Global().Set("initialize", js.FuncOf(w.jsInitialize))

	w.logger.Info("WASM functions exported successfully")
}

// jsGetInfo returns agent information
func (w *WASMAgent) jsGetInfo(this js.Value, args []js.Value) interface{} {
	info := map[string]interface{}{
		"did":          w.agent.GetDID(),
		"name":         w.agent.GetName(),
		"version":      w.agent.GetVersion(),
		"capabilities": w.agent.GetCapabilities(),
	}

	infoJSON, err := json.Marshal(info)
	if err != nil {
		w.logger.Error("failed to marshal agent info", zap.Error(err))
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return string(infoJSON)
}

// jsHandleTask handles task execution from JavaScript
func (w *WASMAgent) jsHandleTask(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return w.errorResponse("task JSON required")
	}

	taskJSON := args[0].String()
	w.logger.Info("received task from WASM", zap.String("task_json", taskJSON))

	// Parse task
	var task Task
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		w.logger.Error("failed to parse task", zap.Error(err))
		return w.errorResponse(fmt.Sprintf("invalid task JSON: %v", err))
	}

	// Execute task
	ctx := context.Background()
	result, err := w.agent.HandleTask(ctx, &task)
	if err != nil {
		w.logger.Error("task execution failed", zap.Error(err))
		return w.errorResponse(fmt.Sprintf("task failed: %v", err))
	}

	// Marshal result
	resultJSON, err := json.Marshal(result)
	if err != nil {
		w.logger.Error("failed to marshal result", zap.Error(err))
		return w.errorResponse(fmt.Sprintf("failed to serialize result: %v", err))
	}

	return string(resultJSON)
}

// jsHealth returns agent health status
func (w *WASMAgent) jsHealth(this js.Value, args []js.Value) interface{} {
	health := w.agent.Health()

	healthJSON, err := json.Marshal(health)
	if err != nil {
		w.logger.Error("failed to marshal health", zap.Error(err))
		return w.errorResponse(err.Error())
	}

	return string(healthJSON)
}

// jsInitialize initializes the agent
func (w *WASMAgent) jsInitialize(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return w.errorResponse("config JSON required")
	}

	configJSON := args[0].String()
	w.logger.Info("initializing agent from WASM", zap.String("config", configJSON))

	// Parse config
	var config Config
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		w.logger.Error("failed to parse config", zap.Error(err))
		return w.errorResponse(fmt.Sprintf("invalid config JSON: %v", err))
	}

	// Initialize agent
	ctx := context.Background()
	if err := w.agent.Initialize(ctx, &config); err != nil {
		w.logger.Error("failed to initialize agent", zap.Error(err))
		return w.errorResponse(fmt.Sprintf("initialization failed: %v", err))
	}

	// Start agent
	if err := w.agent.Start(ctx); err != nil {
		w.logger.Error("failed to start agent", zap.Error(err))
		return w.errorResponse(fmt.Sprintf("start failed: %v", err))
	}

	return w.successResponse("agent initialized successfully")
}

// errorResponse creates an error response
func (w *WASMAgent) errorResponse(message string) string {
	resp := map[string]interface{}{
		"status": "error",
		"error":  message,
	}

	respJSON, _ := json.Marshal(resp)
	return string(respJSON)
}

// successResponse creates a success response
func (w *WASMAgent) successResponse(message string) string {
	resp := map[string]interface{}{
		"status":  "success",
		"message": message,
	}

	respJSON, _ := json.Marshal(resp)
	return string(respJSON)
}

// Run runs the WASM agent (keeps it alive)
func (w *WASMAgent) Run(ctx context.Context) error {
	w.logger.Info("WASM agent running",
		zap.String("name", w.agent.GetName()),
		zap.String("did", w.agent.GetDID()),
	)

	// Export functions
	w.Export()

	// Keep agent running
	select {
	case <-ctx.Done():
		w.logger.Info("WASM agent shutting down")
		return w.agent.Stop(ctx)
	}
}
