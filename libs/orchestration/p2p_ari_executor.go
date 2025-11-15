package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/metrics"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"go.uber.org/zap"
)

// P2PARIExecutor executes tasks on ARI-v1 runtimes discovered via P2P
type P2PARIExecutor struct {
	registry *RuntimeRegistry
	logger   *zap.Logger
}

// NewP2PARIExecutor creates a new P2P-enabled ARI executor
func NewP2PARIExecutor(presenceTopic string, logger *zap.Logger, promMetrics *metrics.PrometheusMetrics) (*P2PARIExecutor, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create runtime registry
	registry, err := NewRuntimeRegistry(context.Background(), logger, promMetrics)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime registry: %w", err)
	}

	// Setup mDNS discovery for local peer discovery
	mdnsService := discovery.NewMdnsService(registry.host, "ainur-runtime", &discoveryNotifee{host: registry.host, logger: logger})
	if err := mdnsService.Start(); err != nil {
		logger.Warn("Failed to start mDNS discovery", zap.Error(err))
	} else {
		logger.Info("mDNS peer discovery enabled")
	}

	// Subscribe to presence topic
	if err := registry.SubscribeToPresence(presenceTopic); err != nil {
		registry.Close()
		return nil, fmt.Errorf("failed to subscribe to presence: %w", err)
	}

	executor := &P2PARIExecutor{
		registry: registry,
		logger:   logger,
	}

	logger.Info("P2P ARI executor created",
		zap.String("presence_topic", presenceTopic),
	)

	return executor, nil
}

// ExecuteTask executes a task by discovering and selecting an appropriate runtime
func (e *P2PARIExecutor) ExecuteTask(ctx context.Context, task *Task, agent *identity.AgentCard) (*TaskResult, error) {
	startTime := time.Now()

	e.logger.Info("P2P task execution started",
		zap.String("task_id", task.ID),
		zap.Strings("required_capabilities", task.Capabilities),
	)

	// Wait a moment for runtime discovery (in case we just started)
	time.Sleep(2 * time.Second)

	// Find runtimes with required capabilities
	runtimes := e.registry.GetRuntimeByCapabilities(task.Capabilities)
	if len(runtimes) == 0 {
		// List all available runtimes for debugging
		allRuntimes := e.registry.GetAllRuntimes()
		e.logger.Error("no runtimes found with required capabilities",
			zap.String("task_id", task.ID),
			zap.Strings("required_capabilities", task.Capabilities),
			zap.Int("total_runtimes", len(allRuntimes)),
		)

		for _, rt := range allRuntimes {
			e.logger.Info("available runtime",
				zap.String("did", rt.DID),
				zap.String("name", rt.Name),
				zap.Strings("capabilities", rt.Capabilities),
				zap.String("grpc_address", rt.GRPCAddress),
			)
		}

		return &TaskResult{
			TaskID:      task.ID,
			Status:      TaskStatusFailed,
			Error:       fmt.Sprintf("no runtimes available with capabilities: %v", task.Capabilities),
			ExecutionMS: time.Since(startTime).Milliseconds(),
			Timestamp:   time.Now(),
		}, nil
	}

	// Select first matching runtime (TODO: implement proper selection strategy)
	selectedRuntime := runtimes[0]
	e.logger.Info("runtime selected for task",
		zap.String("task_id", task.ID),
		zap.String("runtime_did", selectedRuntime.DID),
		zap.String("runtime_name", selectedRuntime.Name),
		zap.String("grpc_address", selectedRuntime.GRPCAddress),
	)

	// Create direct ARI executor for this runtime
	ariExecutor, err := NewARIExecutor(selectedRuntime.GRPCAddress, e.logger)
	if err != nil {
		return &TaskResult{
			TaskID:      task.ID,
			Status:      TaskStatusFailed,
			Error:       fmt.Sprintf("failed to create ARI executor for runtime %s: %v", selectedRuntime.DID, err),
			ExecutionMS: time.Since(startTime).Milliseconds(),
			Timestamp:   time.Now(),
		}, nil
	}
	defer ariExecutor.Close()

	// Execute task on selected runtime (pass nil for agent since runtime doesn't use it)
	ariResult, err := ariExecutor.ExecuteTask(ctx, task, nil)
	if err != nil {
		return &TaskResult{
			TaskID:      task.ID,
			Status:      TaskStatusFailed,
			Error:       fmt.Sprintf("task execution failed on runtime %s: %v", selectedRuntime.DID, err),
			ExecutionMS: time.Since(startTime).Milliseconds(),
			Timestamp:   time.Now(),
		}, nil
	}

	e.logger.Info("P2P task execution completed",
		zap.String("task_id", task.ID),
		zap.String("runtime_did", selectedRuntime.DID),
		zap.Int64("execution_ms", time.Since(startTime).Milliseconds()),
	)

	// Convert ARI result to TaskResult format
	var resultData map[string]interface{}
	if ariResult != nil {
		resultData = ariResult.Result
	}

	actualCost := extractActualCost(task.Budget, resultData)

	return &TaskResult{
		TaskID:      task.ID,
		Status:      ariResult.Status,
		Result:      resultData,
		Error:       ariResult.Error,
		ExecutionMS: time.Since(startTime).Milliseconds(),
		AgentDID:    selectedRuntime.DID,
		Timestamp:   time.Now(),
		Cost:        actualCost,
	}, nil
}

// Close closes the P2P executor and cleanup resources
func (e *P2PARIExecutor) Close() error {
	return e.registry.Close()
}

// GetDiscoveredRuntimes returns all discovered runtimes (for debugging/monitoring)
func (e *P2PARIExecutor) GetDiscoveredRuntimes() []*RuntimeInfo {
	return e.registry.GetAllRuntimes()
}

// RuntimeRegistry returns the underlying runtime registry
func (e *P2PARIExecutor) RuntimeRegistry() *RuntimeRegistry {
	return e.registry
}

// discoveryNotifee handles mDNS peer discoveries for the orchestrator
type discoveryNotifee struct {
	host   host.Host
	logger *zap.Logger
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.logger.Info("discovered peer via mDNS",
		zap.String("peer_id", pi.ID.String()),
		zap.Int("addrs", len(pi.Addrs)),
	)
	// Connect to the discovered peer
	if err := n.host.Connect(context.Background(), pi); err != nil {
		n.logger.Debug("failed to connect to discovered peer", zap.Error(err))
	} else {
		n.logger.Info("connected to discovered peer", zap.String("peer_id", pi.ID.String()))
	}
}
