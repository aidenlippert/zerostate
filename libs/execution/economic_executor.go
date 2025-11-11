package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EconomicExecutor wraps WASM execution with economic layer integration
type EconomicExecutor struct {
	wasmRunner    *WASMRunner
	resultStore   ResultStore
	binaryStore   BinaryStore
	escrowService *economic.EscrowService
	metrics       *EconomicTaskMetrics
	logger        *zap.Logger
}

// EconomicExecutionRequest represents a task execution with economic parameters
type EconomicExecutionRequest struct {
	TaskID          uuid.UUID                      `json:"task_id"`
	AgentID         uuid.UUID                      `json:"agent_id"`
	UserID          uuid.UUID                      `json:"user_id"`
	Input           string                         `json:"input"`
	Budget          float64                        `json:"budget"`
	EscrowID        uuid.UUID                      `json:"escrow_id"`
	Timeout         time.Duration                  `json:"timeout"`
}

// EconomicExecutionResult extends execution result with economic metadata
type EconomicExecutionResult struct {
	TaskID          uuid.UUID              `json:"task_id"`
	AgentID         uuid.UUID              `json:"agent_id"`
	Success         bool                   `json:"success"`
	Output          string                 `json:"output,omitempty"`
	Error           string                 `json:"error,omitempty"`
	ExecutionTime   time.Duration          `json:"execution_time"`
	ResourceUsage   *ResourceUsage         `json:"resource_usage"`

	// Economic metadata
	EscrowID        uuid.UUID              `json:"escrow_id"`
	EscrowStatus    string                 `json:"escrow_status"`
	AmountPaid      float64                `json:"amount_paid"`
	PaymentMethod   string                 `json:"payment_method"` // "escrow" or "channel"
	ReputationDelta float64                `json:"reputation_delta"`
	Timestamp       time.Time              `json:"timestamp"`
}

// ResourceUsage captures actual resource consumption
type ResourceUsage struct {
	MemoryUsedMB     uint64        `json:"memory_used_mb"`
	CPUTimeMs        uint64        `json:"cpu_time_ms"`
	ExecutionTimeMs  uint64        `json:"execution_time_ms"`
	StorageUsedKB    uint64        `json:"storage_used_kb"`
}

// NewEconomicExecutor creates a new economic executor instance
func NewEconomicExecutor(
	wasmRunner *WASMRunner,
	resultStore ResultStore,
	binaryStore BinaryStore,
	escrowService *economic.EscrowService,
	metrics *EconomicTaskMetrics,
	logger *zap.Logger,
) *EconomicExecutor {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &EconomicExecutor{
		wasmRunner:    wasmRunner,
		resultStore:   resultStore,
		binaryStore:   binaryStore,
		escrowService: escrowService,
		metrics:       metrics,
		logger:        logger,
	}
}

// ExecuteWithEconomics executes a task with full economic integration
func (e *EconomicExecutor) ExecuteWithEconomics(ctx context.Context, req *EconomicExecutionRequest) (*EconomicExecutionResult, error) {
	startTime := time.Now()

	// Record task started
	if e.metrics != nil {
		e.metrics.RecordTaskStarted()
		defer e.metrics.RecordTaskCompleted()
	}

	e.logger.Info("starting economic task execution",
		zap.String("task_id", req.TaskID.String()),
		zap.String("agent_id", req.AgentID.String()),
		zap.Float64("budget", req.Budget),
		zap.String("escrow_id", req.EscrowID.String()),
	)

	// Step 1: Pre-execution validation
	if err := e.validatePreExecution(ctx, req); err != nil {
		// Record pre-execution error
		if e.metrics != nil {
			errorType := "validation_failed"
			if err.Error() == "escrow not funded" {
				errorType = "escrow_not_funded"
			} else if err.Error() == "insufficient escrow funds" {
				errorType = "insufficient_funds"
			}
			e.metrics.RecordPreExecutionError(errorType)
		}
		return nil, fmt.Errorf("pre-execution validation failed: %w", err)
	}

	// Step 2: Execute WASM task
	wasmStart := time.Now()
	execResult, err := e.executeTask(ctx, req)
	wasmDuration := time.Since(wasmStart).Seconds()

	if err != nil {
		// Record WASM execution failure
		if e.metrics != nil {
			e.metrics.RecordWasmExecution("failure", wasmDuration)
			e.metrics.RecordExecutionError("wasm_failure")
		}

		// Handle execution failure - update escrow and reputation
		e.handleExecutionFailure(ctx, req, err)

		// Record failed task execution
		if e.metrics != nil {
			e.metrics.RecordTaskExecution(false, time.Since(startTime).Seconds())
		}

		return &EconomicExecutionResult{
			TaskID:        req.TaskID,
			AgentID:       req.AgentID,
			Success:       false,
			Error:         err.Error(),
			ExecutionTime: time.Since(startTime),
			EscrowID:      req.EscrowID,
			EscrowStatus:  "refunded",
			Timestamp:     time.Now(),
		}, err
	}

	// Record successful WASM execution
	if e.metrics != nil {
		e.metrics.RecordWasmExecution("success", wasmDuration)
	}

	// Step 3: Post-execution settlement
	result, err := e.handleExecutionSuccess(ctx, req, execResult, startTime)
	if err != nil {
		// Record settlement error
		if e.metrics != nil {
			e.metrics.RecordSettlementError("release_failed")
		}

		e.logger.Error("post-execution settlement failed",
			zap.Error(err),
			zap.String("task_id", req.TaskID.String()),
		)
		return nil, fmt.Errorf("post-execution settlement failed: %w", err)
	}

	// Record successful task execution
	if e.metrics != nil {
		e.metrics.RecordTaskExecution(true, time.Since(startTime).Seconds())
	}

	return result, nil
}

// validatePreExecution performs pre-execution validation
func (e *EconomicExecutor) validatePreExecution(ctx context.Context, req *EconomicExecutionRequest) error {
	// 1. Verify escrow exists and is funded
	escrow, err := e.escrowService.GetEscrow(ctx, req.EscrowID)
	if err != nil {
		return fmt.Errorf("failed to get escrow: %w", err)
	}

	if escrow.Status != "funded" {
		return fmt.Errorf("escrow not funded: status=%s", escrow.Status)
	}

	if escrow.Amount < req.Budget {
		return fmt.Errorf("insufficient escrow funds: have=%.4f, need=%.4f", escrow.Amount, req.Budget)
	}

	// Note: Reputation verification disabled until reputation service is implemented
	// TODO: Add reputation check back when reputation service is complete

	e.logger.Info("pre-execution validation passed",
		zap.String("task_id", req.TaskID.String()),
		zap.String("escrow_status", string(escrow.Status)),
		zap.Float64("escrow_amount", escrow.Amount),
	)

	return nil
}

// executeTask performs the actual WASM execution
func (e *EconomicExecutor) executeTask(ctx context.Context, req *EconomicExecutionRequest) (*WASMResult, error) {
	// Get agent binary
	if e.binaryStore == nil {
		return nil, fmt.Errorf("binary store not configured")
	}

	wasmBinary, err := e.binaryStore.GetBinary(ctx, req.AgentID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get agent binary: %w", err)
	}

	// Prepare execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Execute WASM
	result, err := e.wasmRunner.Execute(execCtx, wasmBinary, []byte(req.Input))
	if err != nil {
		return nil, fmt.Errorf("WASM execution failed: %w", err)
	}

	// Store result in database
	if e.resultStore != nil {
		taskResult := &TaskResult{
			TaskID:   req.TaskID.String(),
			AgentID:  req.AgentID.String(),
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
			Duration: result.Duration,
			Error:    "",
			CreatedAt: time.Now(),
		}
		if result.Error != nil {
			taskResult.Error = result.Error.Error()
		}

		if err := e.resultStore.StoreResult(ctx, taskResult); err != nil {
			e.logger.Error("failed to store execution result",
				zap.Error(err),
				zap.String("task_id", req.TaskID.String()),
			)
		}
	}

	return result, nil
}

// handleExecutionSuccess handles successful task execution
func (e *EconomicExecutor) handleExecutionSuccess(
	ctx context.Context,
	req *EconomicExecutionRequest,
	execResult *WASMResult,
	startTime time.Time,
) (*EconomicExecutionResult, error) {
	// 1. Calculate resource usage and cost
	resourceUsage := e.calculateResourceUsage(execResult)
	actualCost := e.calculateActualCost(req.Budget, resourceUsage)

	// Record resource usage metrics
	if e.metrics != nil {
		e.metrics.RecordResourceUsage(
			float64(resourceUsage.CPUTimeMs)/1000.0, // Convert ms to seconds
			resourceUsage.MemoryUsedMB*1024*1024,    // Convert MB to bytes
			float64(resourceUsage.ExecutionTimeMs)/1000.0,
		)
		e.metrics.RecordCost(req.Budget, actualCost)
	}

	// 2. Release escrow payment
	var paymentMethod string
	var amountPaid float64

	// For now, always use escrow (payment channel integration pending database implementation)
	paymentMethod = "escrow"
	amountPaid = actualCost

	settlementStart := time.Now()
	if err := e.escrowService.ReleaseEscrow(ctx, req.EscrowID, req.UserID.String()); err != nil {
		// Record failed settlement
		if e.metrics != nil {
			e.metrics.RecordSettlementError("release_failed")
		}
		return nil, fmt.Errorf("failed to release escrow: %w", err)
	}
	settlementDuration := time.Since(settlementStart).Seconds()

	// Record successful escrow settlement
	if e.metrics != nil {
		e.metrics.RecordEscrowSettlement("release", settlementDuration, amountPaid, true)
		e.metrics.RecordPayment(paymentMethod, amountPaid, true)
	}

	// Note: Reputation update disabled until reputation service is implemented
	// TODO: Add reputation update back when reputation service is complete
	reputationDelta := 0.0 // Placeholder for when reputation is implemented

	// 3. Get updated escrow status
	escrow, err := e.escrowService.GetEscrow(ctx, req.EscrowID)
	escrowStatus := "released"
	if err != nil {
		e.logger.Error("failed to get escrow status", zap.Error(err))
	} else {
		escrowStatus = string(escrow.Status)
	}

	return &EconomicExecutionResult{
		TaskID:          req.TaskID,
		AgentID:         req.AgentID,
		Success:         true,
		Output:          string(execResult.Stdout),
		ExecutionTime:   time.Since(startTime),
		ResourceUsage:   resourceUsage,
		EscrowID:        req.EscrowID,
		EscrowStatus:    escrowStatus,
		AmountPaid:      amountPaid,
		PaymentMethod:   paymentMethod,
		ReputationDelta: reputationDelta,
		Timestamp:       time.Now(),
	}, nil
}

// handleExecutionFailure handles failed task execution
func (e *EconomicExecutor) handleExecutionFailure(ctx context.Context, req *EconomicExecutionRequest, execErr error) {
	// 1. Refund escrow
	settlementStart := time.Now()
	if err := e.escrowService.RefundEscrow(ctx, req.EscrowID, req.UserID.String()); err != nil {
		// Record failed refund settlement
		if e.metrics != nil {
			e.metrics.RecordSettlementError("refund_failed")
		}

		e.logger.Error("failed to refund escrow",
			zap.Error(err),
			zap.String("escrow_id", req.EscrowID.String()),
		)
		return
	}
	settlementDuration := time.Since(settlementStart).Seconds()

	// Record successful escrow refund
	if e.metrics != nil {
		e.metrics.RecordEscrowSettlement("refund", settlementDuration, req.Budget, true)
		e.metrics.RecordPayment("escrow", 0, true) // No payment on refund
	}

	// Note: Reputation update disabled until reputation service is implemented
	// TODO: Add negative reputation update back when reputation service is complete

	e.logger.Warn("execution failed, escrow refunded",
		zap.String("task_id", req.TaskID.String()),
		zap.String("agent_id", req.AgentID.String()),
		zap.Error(execErr),
	)
}

// calculateResourceUsage extracts resource usage from execution result
func (e *EconomicExecutor) calculateResourceUsage(result *WASMResult) *ResourceUsage {
	return &ResourceUsage{
		MemoryUsedMB:    0, // TODO: Extract from WASM runtime metrics
		CPUTimeMs:       uint64(result.Duration.Milliseconds()),
		ExecutionTimeMs: uint64(result.Duration.Milliseconds()),
		StorageUsedKB:   0, // TODO: Calculate storage usage
	}
}

// calculateActualCost calculates actual cost based on resource usage
func (e *EconomicExecutor) calculateActualCost(budgetedCost float64, usage *ResourceUsage) float64 {
	// For now, use budgeted cost
	// TODO: Implement dynamic pricing based on actual resource consumption
	// Formula: base_cost + (memory_cost * MB) + (cpu_cost * ms) + (storage_cost * KB)
	return budgetedCost
}

// calculateReputationDelta calculates reputation change
func (e *EconomicExecutor) calculateReputationDelta(success bool, execTime time.Duration, usage *ResourceUsage) float64 {
	if !success {
		return -5.0 // Failure penalty
	}

	// Success bonus: base 2.0 + efficiency bonus
	delta := 2.0

	// Fast execution bonus (< 1 second)
	if execTime < time.Second {
		delta += 0.5
	}

	return delta
}

// calculateEfficiency calculates resource efficiency score
func (e *EconomicExecutor) calculateEfficiency(usage *ResourceUsage) float64 {
	// TODO: Implement efficiency calculation based on resource usage
	// For now, return a default good score
	return 0.85
}

// GetExecutionReceipt generates an execution receipt with economic metadata
func (e *EconomicExecutor) GetExecutionReceipt(ctx context.Context, taskID uuid.UUID) (map[string]interface{}, error) {
	// Get stored result
	if e.resultStore == nil {
		return nil, fmt.Errorf("result store not configured")
	}

	result, err := e.resultStore.GetResult(ctx, taskID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get execution result: %w", err)
	}

	// Build receipt
	receipt := map[string]interface{}{
		"task_id":        taskID.String(),
		"exit_code":      result.ExitCode,
		"execution_time": result.Duration.String(),
		"created_at":     result.CreatedAt,
	}

	// Add output or error
	if result.ExitCode == 0 {
		receipt["stdout"] = string(result.Stdout)
	} else {
		receipt["error"] = result.Error
		receipt["stderr"] = string(result.Stderr)
	}

	return receipt, nil
}

// HealthCheck verifies all economic services are available
func (e *EconomicExecutor) HealthCheck(ctx context.Context) error {
	if e.wasmRunner == nil {
		return fmt.Errorf("WASM runner not initialized")
	}

	if e.escrowService == nil {
		return fmt.Errorf("escrow service not initialized")
	}

	// Optional services
	if e.resultStore == nil {
		e.logger.Warn("result store not configured")
	}

	if e.binaryStore == nil {
		e.logger.Warn("binary store not configured")
	}

	return nil
}
