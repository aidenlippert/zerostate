package orchestration

import (
	"context"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"go.uber.org/zap"
)

// RegisterAgentWithRouter registers agent with CQ-Router for intelligent routing
// This should be called when agents announce presence via L3 Aether
func (o *Orchestrator) RegisterAgentWithRouter(agentDID string, capabilities []string) {
	if o.cqRouter != nil {
		o.cqRouter.RegisterPeer(agentDID, capabilities)

		o.logger.Info("registered agent with CQ-Router",
			zap.String("agent", agentDID),
			zap.Strings("capabilities", capabilities),
		)
	}
}

// UnregisterAgentFromRouter removes agent from CQ-Router
// This should be called when agents go offline
func (o *Orchestrator) UnregisterAgentFromRouter(agentDID string) {
	if o.cqRouter != nil {
		o.cqRouter.UnregisterPeer(agentDID)

		o.logger.Info("unregistered agent from CQ-Router",
			zap.String("agent", agentDID),
		)
	}
}

// RouteTaskWithCQ uses CQ-Routing to find best agent for task
// Returns agent DID, expected latency, and error
func (o *Orchestrator) RouteTaskWithCQ(ctx context.Context, task *Task) (string, float64, error) {
	if o.cqRouter == nil {
		return "", 0, ErrNoSuitableAgent
	}

	// Use first capability for routing
	// TODO: In future, support multi-capability routing
	if len(task.Capabilities) == 0 {
		return "", 0, ErrNoSuitableAgent
	}

	capability := task.Capabilities[0]

	startTime := time.Now()

	// Route via CQ-Router
	agentDID, expectedLatency, err := o.cqRouter.RouteCFP(capability)
	if err != nil {
		o.logger.Warn("CQ-Routing failed, falling back to selector",
			zap.String("task_id", task.ID),
			zap.String("capability", capability),
			zap.Error(err),
		)
		return "", 0, err
	}

	routingTime := time.Since(startTime)

	o.logger.Info("task routed via CQ-Routing",
		zap.String("task_id", task.ID),
		zap.String("capability", capability),
		zap.String("agent", agentDID),
		zap.Float64("expected_latency_ms", expectedLatency),
		zap.Duration("routing_time", routingTime),
	)

	return agentDID, expectedLatency, nil
}

// ReportRoutingOutcome reports task execution outcome to CQ-Router for learning
// This is called after task completion/failure to update Q-values
func (o *Orchestrator) ReportRoutingOutcome(task *Task, agentDID string, latency time.Duration, success bool) {
	if o.cqRouter == nil {
		return
	}

	// Use first capability
	if len(task.Capabilities) == 0 {
		return
	}

	capability := task.Capabilities[0]

	outcome := RouteOutcome{
		Capability: capability,
		PeerDID:    agentDID,
		Latency:    latency,
		Success:    success,
		Timestamp:  time.Now(),
	}

	// CQ-Router learns from outcome
	o.cqRouter.Learn(outcome)

	o.logger.Debug("reported routing outcome to CQ-Router",
		zap.String("task_id", task.ID),
		zap.String("agent", agentDID),
		zap.String("capability", capability),
		zap.Duration("latency", latency),
		zap.Bool("success", success),
	)
}

// GetCQRouterStats returns CQ-Router statistics for monitoring
func (o *Orchestrator) GetCQRouterStats() map[string]interface{} {
	if o.cqRouter == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := o.cqRouter.GetRoutingStats()
	stats["enabled"] = true

	return stats
}

// ImportAgentCardsToRouter imports existing agent cards into CQ-Router
// This should be called at orchestrator startup to bootstrap routing table
func (o *Orchestrator) ImportAgentCardsToRouter(agents []*identity.AgentCard) {
	if o.cqRouter == nil {
		return
	}

	for _, agent := range agents {
		// Parse capabilities from agent card
		var capabilities []string
		// Assuming AgentCard has Capabilities field (adjust if needed)
		// For now, use empty slice - this should be populated from actual agent data
		capabilities = []string{} // TODO: Extract from agent.Capabilities

		o.cqRouter.RegisterPeer(agent.DID, capabilities)
	}

	o.logger.Info("imported agent cards to CQ-Router",
		zap.Int("count", len(agents)),
	)
}
