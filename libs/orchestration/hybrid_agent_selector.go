package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/substrate"
	"go.uber.org/zap"
)

// HybridAgentSelector implements a transition strategy between database and blockchain
//
// Migration Strategy (4 weeks):
// Week 1: Dual-write (DB primary, chain secondary) - write to both, read from DB
// Week 2: Dual-read (DB primary, chain validation) - read from both, prefer DB, validate against chain
// Week 3: Chain-primary (chain primary, DB fallback) - read from chain first, fallback to DB
// Week 4: Chain-only (DELETE database) - read only from chain
//
// This selector implements Week 2-3 logic.
type HybridAgentSelector struct {
	dbSelector    *DatabaseAgentSelector
	chainSelector *ChainAgentSelector
	mode          HybridMode
	logger        *zap.Logger
	blockchain    *substrate.BlockchainService // For reputation queries
}

// HybridMode defines the migration phase
type HybridMode string

const (
	// ModeDBPrimary: Database is primary, blockchain is validation
	ModeDBPrimary HybridMode = "db_primary"

	// ModeChainPrimary: Blockchain is primary, database is fallback
	ModeChainPrimary HybridMode = "chain_primary"

	// ModeChainOnly: Only use blockchain (database deleted)
	ModeChainOnly HybridMode = "chain_only"
)

// NewHybridAgentSelector creates a new hybrid selector
//
// Example:
//
//	// Week 2: DB primary with chain validation
//	selector := orchestration.NewHybridAgentSelector(dbSelector, chainSelector, ModeDBPrimary, logger)
//
//	// Week 3: Chain primary with DB fallback
//	selector := orchestration.NewHybridAgentSelector(dbSelector, chainSelector, ModeChainPrimary, logger)
func NewHybridAgentSelector(
	dbSelector *DatabaseAgentSelector,
	chainSelector *ChainAgentSelector,
	mode HybridMode,
	logger *zap.Logger,
) *HybridAgentSelector {
	return NewHybridAgentSelectorWithBlockchain(dbSelector, chainSelector, mode, logger, nil)
}

// NewHybridAgentSelectorWithBlockchain creates a hybrid selector with blockchain integration for reputation
func NewHybridAgentSelectorWithBlockchain(
	dbSelector *DatabaseAgentSelector,
	chainSelector *ChainAgentSelector,
	mode HybridMode,
	logger *zap.Logger,
	blockchain *substrate.BlockchainService,
) *HybridAgentSelector {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &HybridAgentSelector{
		dbSelector:    dbSelector,
		chainSelector: chainSelector,
		mode:          mode,
		logger:        logger,
		blockchain:    blockchain,
	}
}

// SelectAgent selects an agent using the hybrid strategy
func (s *HybridAgentSelector) SelectAgent(ctx context.Context, task *Task) (*identity.AgentCard, error) {
	s.logger.Info("hybrid agent selection",
		zap.String("mode", string(s.mode)),
		zap.String("task_id", task.ID),
	)

	switch s.mode {
	case ModeDBPrimary:
		return s.selectDBPrimary(ctx, task)
	case ModeChainPrimary:
		return s.selectChainPrimary(ctx, task)
	case ModeChainOnly:
		return s.selectChainOnly(ctx, task)
	default:
		return nil, fmt.Errorf("unknown hybrid mode: %s", s.mode)
	}
}

// selectDBPrimary: Database is source of truth, blockchain validates
func (s *HybridAgentSelector) selectDBPrimary(ctx context.Context, task *Task) (*identity.AgentCard, error) {
	// Get candidate agents from database (multiple options for reputation comparison)
	candidates, err := s.getDBCandidates(ctx, task)
	if err != nil {
		s.logger.Error("database selection failed",
			zap.Error(err),
		)
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable agents found in database")
	}

	// Apply reputation-based selection if blockchain is available
	selectedAgent := s.selectWithReputation(ctx, candidates, task)

	// Secondary: Validate agent exists on blockchain
	go s.validateAgentOnChain(ctx, selectedAgent.DID)

	return selectedAgent, nil
}

// getDBCandidates gets multiple candidate agents from database for reputation comparison
func (s *HybridAgentSelector) getDBCandidates(ctx context.Context, task *Task) ([]*identity.AgentCard, error) {
	// For now, get the single best agent from database
	// In a more sophisticated implementation, you'd get top N candidates
	agent, err := s.dbSelector.SelectAgent(ctx, task)
	if err != nil {
		return nil, err
	}

	return []*identity.AgentCard{agent}, nil
}

// selectWithReputation selects the best agent from candidates using reputation weighting
// Weight: 60% capability match, 40% reputation score
func (s *HybridAgentSelector) selectWithReputation(ctx context.Context, candidates []*identity.AgentCard, task *Task) *identity.AgentCard {
	if len(candidates) == 1 {
		return candidates[0] // No choice to make
	}

	if s.blockchain == nil || !s.blockchain.IsEnabled() {
		s.logger.Debug("blockchain not available, using first candidate",
			zap.String("task_id", task.ID),
		)
		return candidates[0] // Fall back to first candidate
	}

	type agentScore struct {
		agent           *identity.AgentCard
		capabilityMatch float64
		reputation      uint32
		finalScore      float64
	}

	scores := make([]agentScore, 0, len(candidates))

	for _, agent := range candidates {
		// Calculate capability match (simplified - in production use semantic similarity)
		capabilityMatch := s.calculateCapabilityMatch(agent, task)

		// Get reputation score from blockchain
		reputation := s.getAgentReputationSafe(ctx, agent.DID)

		// Normalize reputation to 0-1 scale (assume max reputation is 1000)
		normalizedReputation := float64(reputation) / 1000.0
		if normalizedReputation > 1.0 {
			normalizedReputation = 1.0
		}

		// Calculate final score: 60% capability + 40% reputation
		finalScore := (0.6 * capabilityMatch) + (0.4 * normalizedReputation)

		scores = append(scores, agentScore{
			agent:           agent,
			capabilityMatch: capabilityMatch,
			reputation:      reputation,
			finalScore:      finalScore,
		})

		s.logger.Debug("agent scoring",
			zap.String("agent_did", agent.DID),
			zap.Float64("capability_match", capabilityMatch),
			zap.Uint32("reputation", reputation),
			zap.Float64("final_score", finalScore),
		)
	}

	// Select agent with highest final score
	bestAgent := scores[0]
	for _, score := range scores[1:] {
		if score.finalScore > bestAgent.finalScore {
			bestAgent = score
		}
	}

	s.logger.Info("reputation-weighted agent selected",
		zap.String("task_id", task.ID),
		zap.String("selected_agent", bestAgent.agent.DID),
		zap.Float64("capability_match", bestAgent.capabilityMatch),
		zap.Uint32("reputation", bestAgent.reputation),
		zap.Float64("final_score", bestAgent.finalScore),
	)

	return bestAgent.agent
}

// calculateCapabilityMatch calculates how well an agent matches the task capabilities
func (s *HybridAgentSelector) calculateCapabilityMatch(agent *identity.AgentCard, task *Task) float64 {
	if len(task.Capabilities) == 0 {
		return 1.0 // No requirements means perfect match
	}

	matches := 0
	for _, taskCap := range task.Capabilities {
		for _, agentCap := range agent.Capabilities {
			if agentCap.Name == taskCap {
				matches++
				break
			}
		}
	}

	return float64(matches) / float64(len(task.Capabilities))
}

// getAgentReputationSafe gets agent reputation from blockchain with error handling
func (s *HybridAgentSelector) getAgentReputationSafe(ctx context.Context, agentDID string) uint32 {
	if s.blockchain == nil || !s.blockchain.IsEnabled() {
		return 500 // Default reputation when blockchain unavailable
	}

	// Convert DID to AccountID
	agentAccount, err := s.convertDIDToAccountID(agentDID)
	if err != nil {
		s.logger.Debug("failed to convert agent DID to account ID for reputation lookup",
			zap.String("agent_did", agentDID),
			zap.Error(err),
		)
		return 500 // Default reputation
	}

	// Create timeout context for reputation query
	repCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Use the method from reputation_integration.go which handles the orchestrator interface
	// We need to create a temporary orchestrator struct to use this method, or move the logic here
	repClient := s.blockchain.Reputation()
	if repClient == nil {
		return 500
	}

	reputation, err := repClient.GetReputationScore(repCtx, agentAccount)
	if err != nil {
		s.logger.Debug("failed to get agent reputation from blockchain",
			zap.String("agent_did", agentDID),
			zap.Error(err),
		)
		return 500 // Default reputation on failure
	}

	return reputation
}

// convertDIDToAccountID converts a DID string to substrate AccountID
// This is a simplified conversion - matches the one in orchestrator.go
func (s *HybridAgentSelector) convertDIDToAccountID(did string) (substrate.AccountID, error) {
	if len(did) < 48 { // Basic validation
		return substrate.AccountID{}, fmt.Errorf("invalid DID format: %s", did)
	}

	// Extract the last part which should be the SS58 address
	parts := []string{}
	if len(did) > 13 && did[:13] == "did:substrate" {
		parts = []string{did[14:]} // Skip "did:substrate:"
	} else {
		parts = []string{did} // Use as-is
	}

	if len(parts) == 0 {
		return substrate.AccountID{}, fmt.Errorf("could not extract account from DID: %s", did)
	}

	// For now, create a dummy AccountID from the DID hash
	// In production, you'd properly decode the SS58 address
	var accountID substrate.AccountID
	copy(accountID[:], []byte(parts[0])[:32])

	return accountID, nil
}

// selectChainPrimary: Blockchain is source of truth, database is fallback
func (s *HybridAgentSelector) selectChainPrimary(ctx context.Context, task *Task) (*identity.AgentCard, error) {
	// Primary: Try blockchain first
	chainAgent, err := s.chainSelector.SelectAgent(ctx, task)
	if err != nil {
		s.logger.Warn("blockchain selection failed, falling back to database",
			zap.Error(err),
		)

		// Fallback: Use database
		dbAgent, dbErr := s.dbSelector.SelectAgent(ctx, task)
		if dbErr != nil {
			return nil, fmt.Errorf("both chain and db failed: chain=%w, db=%v", err, dbErr)
		}

		s.logger.Info("fallback to database successful",
			zap.String("agent_did", dbAgent.DID),
		)

		return dbAgent, nil
	}

	// Log discrepancies (async)
	go s.compareAgents(ctx, task, chainAgent)

	return chainAgent, nil
}

// selectChainOnly: Only use blockchain (database deleted)
func (s *HybridAgentSelector) selectChainOnly(ctx context.Context, task *Task) (*identity.AgentCard, error) {
	return s.chainSelector.SelectAgent(ctx, task)
}

// validateAgentOnChain checks if a database agent exists on-chain (async)
func (s *HybridAgentSelector) validateAgentOnChain(ctx context.Context, did string) {
	active, err := s.chainSelector.client.IsDIDActive(ctx, substrate.DID(did))
	if err != nil {
		s.logger.Warn("failed to validate agent on chain",
			zap.String("did", did),
			zap.Error(err),
		)
		return
	}

	if !active {
		s.logger.Error("agent in database but NOT on blockchain!",
			zap.String("did", did),
		)
	} else {
		s.logger.Debug("agent validated on blockchain",
			zap.String("did", did),
		)
	}
}

// compareAgents compares database and blockchain agents (async logging)
func (s *HybridAgentSelector) compareAgents(ctx context.Context, task *Task, chainAgent *identity.AgentCard) {
	dbAgent, err := s.dbSelector.SelectAgent(ctx, task)
	if err != nil {
		s.logger.Debug("database agent not found for comparison",
			zap.String("chain_agent", chainAgent.DID),
		)
		return
	}

	// Compare DIDs
	if dbAgent.DID != chainAgent.DID {
		s.logger.Warn("agent mismatch: database and chain selected different agents",
			zap.String("db_agent", dbAgent.DID),
			zap.String("chain_agent", chainAgent.DID),
		)
	} else {
		s.logger.Debug("agent match: database and chain agree",
			zap.String("agent", dbAgent.DID),
		)
	}

	// Compare prices
	dbPrice := dbAgent.Capabilities[0].Cost.Price
	chainPrice := chainAgent.Capabilities[0].Cost.Price

	if dbPrice != chainPrice {
		s.logger.Warn("price discrepancy between database and chain",
			zap.String("agent", dbAgent.DID),
			zap.Float64("db_price", dbPrice),
			zap.Float64("chain_price", chainPrice),
		)
	}
}

// SelectAgentWithFailover implements failover logic
func (s *HybridAgentSelector) SelectAgentWithFailover(ctx context.Context, task *Task, failedAgentDID string) (*identity.AgentCard, error) {
	switch s.mode {
	case ModeDBPrimary:
		return s.dbSelector.SelectAgentWithFailover(ctx, task, failedAgentDID)
	case ModeChainPrimary:
		// Try chain first, fallback to DB
		agent, err := s.chainSelector.SelectAgentWithFailover(ctx, task, failedAgentDID)
		if err != nil && s.dbSelector != nil {
			return s.dbSelector.SelectAgentWithFailover(ctx, task, failedAgentDID)
		}
		return agent, err
	case ModeChainOnly:
		return s.chainSelector.SelectAgentWithFailover(ctx, task, failedAgentDID)
	default:
		return nil, fmt.Errorf("unknown hybrid mode: %s", s.mode)
	}
}
