package orchestration

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/substrate"
	"go.uber.org/zap"
)

// ChainAgentSelector selects agents from the blockchain (pallet-registry)
// This is the DECENTRALIZED replacement for DatabaseAgentSelector.
type ChainAgentSelector struct {
	client      *substrate.Client
	searchIndex SearchIndex // Still used for semantic ranking
	config      *MetaAgentConfig
	logger      *zap.Logger
}

// NewChainAgentSelector creates a new blockchain-backed agent selector
//
// Example:
//
//	client, _ := substrate.NewClient("ws://127.0.0.1:9944")
//	selector := orchestration.NewChainAgentSelector(client, hnsw, config, logger)
func NewChainAgentSelector(client *substrate.Client, searchIndex SearchIndex, config *MetaAgentConfig, logger *zap.Logger) *ChainAgentSelector {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ChainAgentSelector{
		client:      client,
		searchIndex: searchIndex,
		config:      config,
		logger:      logger,
	}
}

// SelectAgent selects the best agent for a task by querying the blockchain
//
// Algorithm:
// 1. Query pallet-registry for agents with matching capabilities
// 2. Fetch each agent's full AgentCard from chain
// 3. Use HNSW semantic search to rank agents
// 4. Run auction between top candidates
// 5. Return winner
func (s *ChainAgentSelector) SelectAgent(ctx context.Context, task *Task) (*identity.AgentCard, error) {
	s.logger.Info("selecting agent from blockchain",
		zap.String("task_id", task.ID),
		zap.Strings("capabilities", task.Capabilities),
	)

	// Step 1: Find agents with matching capabilities
	candidateDIDs := make(map[substrate.DID]bool)

	for _, capability := range task.Capabilities {
		dids, err := s.client.FindAgentsByCapability(ctx, capability)
		if err != nil {
			s.logger.Warn("failed to query capability index",
				zap.String("capability", capability),
				zap.Error(err),
			)
			continue
		}

		for _, did := range dids {
			candidateDIDs[did] = true
		}
	}

	if len(candidateDIDs) == 0 {
		return nil, fmt.Errorf("no agents found on blockchain with required capabilities: %v", task.Capabilities)
	}

	s.logger.Info("found candidate agents on blockchain",
		zap.Int("count", len(candidateDIDs)),
	)

	// Step 2: Fetch full AgentCards from chain
	var agentCards []*identity.AgentCard
	for did := range candidateDIDs {
		chainCard, err := s.client.GetAgentCard(ctx, did)
		if err != nil {
			s.logger.Warn("failed to fetch agent card from chain",
				zap.String("did", string(did)),
				zap.Error(err),
			)
			continue
		}

		// Convert substrate.AgentCard → identity.AgentCard
		identityCard := s.convertToIdentityAgentCard(chainCard)

		// Verify agent has active DID
		active, err := s.client.IsDIDActive(ctx, did)
		if err != nil || !active {
			s.logger.Debug("skipping inactive agent",
				zap.String("did", string(did)),
			)
			continue
		}

		agentCards = append(agentCards, identityCard)
	}

	if len(agentCards) == 0 {
		return nil, fmt.Errorf("no active agents available on blockchain")
	}

	// Step 3: Use semantic search to rank agents
	// (If search index is available, we can still rank by similarity)
	rankedCards := s.rankAgentsBySemantic(task, agentCards)

	// Step 4: Run auction between top candidates
	// For now, we'll use a simplified selection: best semantic match
	// In the future, this can call meta-agent auction logic
	selectedCard := s.selectBestAgent(rankedCards)

	s.logger.Info("agent selected from blockchain",
		zap.String("task_id", task.ID),
		zap.String("agent_did", selectedCard.DID),
	)

	return selectedCard, nil
}

// convertToIdentityAgentCard converts substrate.AgentCard → identity.AgentCard
func (s *ChainAgentSelector) convertToIdentityAgentCard(chainCard *substrate.AgentCard) *identity.AgentCard {
	// Parse price from chain (Balance is hex string)
	priceFloat := s.parseBalanceToFloat(chainCard.PricePerTask)

	// Build capabilities from on-chain data
	capabilities := make([]identity.Capability, 0, len(chainCard.Capabilities))
	for _, capName := range chainCard.Capabilities {
		capabilities = append(capabilities, identity.Capability{
			Name:    capName,
			Version: "1.0.0", // Version not stored on-chain yet
			Cost: &identity.Cost{
				Unit:  "task",
				Price: priceFloat,
			},
			Metadata: map[string]interface{}{
				"registered_at": chainCard.RegisteredAt,
				"updated_at":    chainCard.UpdatedAt,
				"wasm_hash":     hex.EncodeToString(chainCard.WASMHash[:]),
			},
		})
	}

	// Build endpoints (not stored on-chain yet, use defaults)
	endpoints := &identity.Endpoints{
		Libp2p: []string{}, // Will be populated from off-chain metadata in future
		HTTP:   []string{},
		Region: "global", // Default for now
	}

	// Reputation (not stored on-chain yet, use defaults)
	reputation := &identity.Reputation{
		Score: 0.8, // Default score for blockchain-registered agents
		// ZKAccumulator will be populated when reputation system is live
	}

	// Proof (blockchain-native agents are inherently verified)
	proof := &identity.Proof{
		Type:         "BlockchainRegistry",
		Created:      fmt.Sprintf("block-%d", chainCard.RegisteredAt),
		ProofPurpose: "agentRegistration",
		JWS:          "", // On-chain registration is the proof
	}

	return &identity.AgentCard{
		DID:          string(chainCard.DID),
		Endpoints:    endpoints,
		Capabilities: capabilities,
		Reputation:   reputation,
		Proof:        proof,
	}
}

// parseBalanceToFloat converts substrate.Balance (hex string) to float64 AINU
func (s *ChainAgentSelector) parseBalanceToFloat(balance substrate.Balance) float64 {
	// Balance is stored as u128 (smallest unit, like satoshis)
	// 1 AINU = 10^12 units (like ETH wei)

	// Remove "0x" prefix
	balanceStr := string(balance)
	if len(balanceStr) > 2 && balanceStr[:2] == "0x" {
		balanceStr = balanceStr[2:]
	}

	// Parse as big int
	balanceInt := new(big.Int)
	balanceInt.SetString(balanceStr, 16)

	// Convert to float: divide by 10^12
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(12), nil)
	balanceFloat := new(big.Float).SetInt(balanceInt)
	divisorFloat := new(big.Float).SetInt(divisor)
	result := new(big.Float).Quo(balanceFloat, divisorFloat)

	price, _ := result.Float64()
	return price
}

// rankAgentsBySemantic uses HNSW search to rank agents by semantic similarity
func (s *ChainAgentSelector) rankAgentsBySemantic(task *Task, agents []*identity.AgentCard) []*identity.AgentCard {
	if s.searchIndex == nil {
		// No ranking, return as-is
		return agents
	}

	// TODO: Implement semantic ranking using HNSW
	// For now, return agents sorted by price (ascending)
	// This matches the DB selector behavior when HNSW is not available

	sortedAgents := make([]*identity.AgentCard, len(agents))
	copy(sortedAgents, agents)

	// Simple price-based sorting (cheaper first)
	for i := 0; i < len(sortedAgents); i++ {
		for j := i + 1; j < len(sortedAgents); j++ {
			priceI := sortedAgents[i].Capabilities[0].Cost.Price
			priceJ := sortedAgents[j].Capabilities[0].Cost.Price
			if priceJ < priceI {
				sortedAgents[i], sortedAgents[j] = sortedAgents[j], sortedAgents[i]
			}
		}
	}

	return sortedAgents
}

// selectBestAgent picks the best agent from ranked candidates
func (s *ChainAgentSelector) selectBestAgent(rankedAgents []*identity.AgentCard) *identity.AgentCard {
	// For now, just return the top-ranked agent
	// In the future, this can run a meta-agent auction

	if len(rankedAgents) == 0 {
		return nil
	}

	// Take top N candidates and run auction
	topN := s.config.MaxAgentsForAuction
	if topN > len(rankedAgents) {
		topN = len(rankedAgents)
	}

	candidates := rankedAgents[:topN]

	// Run auction (simplified for now: pick best price/reputation ratio)
	bestAgent := candidates[0]
	bestScore := s.calculateAgentScore(bestAgent)

	for _, agent := range candidates[1:] {
		score := s.calculateAgentScore(agent)
		if score > bestScore {
			bestScore = score
			bestAgent = agent
		}
	}

	return bestAgent
}

// calculateAgentScore computes a score for agent selection
func (s *ChainAgentSelector) calculateAgentScore(agent *identity.AgentCard) float64 {
	// Score = (reputation / price) * 100
	// Higher score = better value

	price := agent.Capabilities[0].Cost.Price
	if price <= 0 {
		price = 0.01 // Avoid division by zero
	}

	reputation := agent.Reputation.Score
	if reputation <= 0 {
		reputation = 0.5 // Default
	}

	return (reputation / price) * 100.0
}

// SelectAgentWithFailover selects a failover agent when the primary fails
func (s *ChainAgentSelector) SelectAgentWithFailover(ctx context.Context, task *Task, failedAgentDID string) (*identity.AgentCard, error) {
	s.logger.Info("selecting failover agent from blockchain",
		zap.String("task_id", task.ID),
		zap.String("failed_agent_did", failedAgentDID),
	)

	// Find all candidates (same as SelectAgent)
	candidateDIDs := make(map[substrate.DID]bool)

	for _, capability := range task.Capabilities {
		dids, err := s.client.FindAgentsByCapability(ctx, capability)
		if err != nil {
			continue
		}

		for _, did := range dids {
			// Exclude the failed agent
			if string(did) != failedAgentDID {
				candidateDIDs[did] = true
			}
		}
	}

	if len(candidateDIDs) == 0 {
		return nil, fmt.Errorf("no failover agents available on blockchain")
	}

	// Fetch agent cards and select best
	var agentCards []*identity.AgentCard
	for did := range candidateDIDs {
		chainCard, err := s.client.GetAgentCard(ctx, did)
		if err != nil {
			continue
		}

		identityCard := s.convertToIdentityAgentCard(chainCard)
		agentCards = append(agentCards, identityCard)
	}

	rankedCards := s.rankAgentsBySemantic(task, agentCards)
	if len(rankedCards) == 0 {
		return nil, fmt.Errorf("no valid failover agents found")
	}

	return s.selectBestAgent(rankedCards), nil
}
