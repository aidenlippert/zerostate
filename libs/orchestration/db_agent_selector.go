package orchestration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/identity"
	"go.uber.org/zap"
)

// DatabaseAgentSelector selects agents from the database using the MetaAgent
type DatabaseAgentSelector struct {
	metaAgent *MetaAgent
	logger    *zap.Logger
}

// NewDatabaseAgentSelector creates a new database-backed agent selector
func NewDatabaseAgentSelector(db *database.DB, searchIndex SearchIndex, config *MetaAgentConfig, logger *zap.Logger) *DatabaseAgentSelector {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &DatabaseAgentSelector{
		metaAgent: NewMetaAgent(db, searchIndex, config, logger),
		logger:    logger,
	}
}

// SelectAgent selects the best agent for a task using meta-agent auction
func (s *DatabaseAgentSelector) SelectAgent(ctx context.Context, task *Task) (*identity.AgentCard, error) {
	s.logger.Info("selecting agent from database",
		zap.String("task_id", task.ID),
		zap.Strings("capabilities", task.Capabilities),
	)

	// Use meta-agent to select best agent
	dbAgent, err := s.metaAgent.SelectAgent(ctx, task)
	if err != nil {
		s.logger.Error("meta-agent failed to select agent",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
		return nil, err
	}

	// Convert database.Agent to identity.AgentCard
	agentCard := s.convertToAgentCard(dbAgent)

	s.logger.Info("agent selected from database",
		zap.String("task_id", task.ID),
		zap.String("agent_id", agentCard.DID),
	)

	return agentCard, nil
}

// convertToAgentCard converts database.Agent to identity.AgentCard
func (s *DatabaseAgentSelector) convertToAgentCard(dbAgent *database.Agent) *identity.AgentCard {
	// Extract description from sql.NullString
	description := ""
	if dbAgent.Description.Valid {
		description = dbAgent.Description.String
	}

	// Use DID field (string) from new Agent struct
	agentDID := dbAgent.DID
	if agentDID == "" {
		agentDID = dbAgent.ID.String() // Fallback to ID if DID is empty
	}

	// ---------------------------------------------------------------------
	// Endpoints
	// ---------------------------------------------------------------------
	endpoints := &identity.Endpoints{
		Libp2p: []string{},
	}

	// Try to extract network-related information from Metadata JSON if present.
	// We keep this intentionally permissive: any "libp2p" or "http" keys become endpoints.
	if len(dbAgent.Metadata) > 0 {
		var meta map[string]interface{}
		if err := json.Unmarshal(dbAgent.Metadata, &meta); err == nil {
			if v, ok := meta["libp2p"].([]interface{}); ok {
				for _, raw := range v {
					if s, ok := raw.(string); ok && s != "" {
						endpoints.Libp2p = append(endpoints.Libp2p, s)
					}
				}
			}
			if v, ok := meta["http"].([]interface{}); ok {
				for _, raw := range v {
					if s, ok := raw.(string); ok && s != "" {
						endpoints.HTTP = append(endpoints.HTTP, s)
					}
				}
			}
			if region, ok := meta["region"].(string); ok && region != "" {
				endpoints.Region = region
			}
		}
	}

	// ---------------------------------------------------------------------
	// Capabilities
	// ---------------------------------------------------------------------
	var capabilities []identity.Capability

	// First, try to parse structured capabilities from the JSON column.
	if len(dbAgent.Capabilities) > 0 {
		// We accept either a direct []Capability JSON, or a wrapper with a "capabilities" field.
		if err := json.Unmarshal(dbAgent.Capabilities, &capabilities); err != nil {
			// Fallback: try to unwrap {"capabilities": [...]}
			var wrapper struct {
				Capabilities []identity.Capability `json:"capabilities"`
			}
			if err2 := json.Unmarshal(dbAgent.Capabilities, &wrapper); err2 == nil && len(wrapper.Capabilities) > 0 {
				capabilities = wrapper.Capabilities
			} else {
				// If both attempts fail, we log and fall back to a single synthetic capability
				s.logger.Warn("failed to unmarshal agent capabilities, using fallback",
					zap.String("agent_id", dbAgent.ID.String()),
					zap.Error(err),
				)
			}
		}
	}

	// If there are still no capabilities, synthesize a minimal one so the agent remains usable.
	if len(capabilities) == 0 {
		capabilities = []identity.Capability{
			{
				Name:    dbAgent.Name,
				Version: "1.0.0",
				Cost: &identity.Cost{
					Unit:  "task",
					Price: 0.10,
				},
				Metadata: map[string]interface{}{
					"description": description,
					"status":      string(dbAgent.Status),
				},
			},
		}
	}

	// ---------------------------------------------------------------------
	// Pricing / reputation
	// ---------------------------------------------------------------------
	cost := capabilities[0].Cost
	// If we have a pricing model, prefer it to any baked-in cost on the first capability.
	if dbAgent.PricingModel.Valid && dbAgent.PricingModel.String != "" {
		var pricing struct {
			Unit  string  `json:"unit"`
			Price float64 `json:"price"`
		}
		if err := json.Unmarshal([]byte(dbAgent.PricingModel.String), &pricing); err == nil {
			cost = &identity.Cost{
				Unit:  pricing.Unit,
				Price: pricing.Price,
			}
		} else {
			s.logger.Warn("failed to unmarshal pricing model, using existing capability cost",
				zap.String("agent_id", dbAgent.ID.String()),
				zap.Error(err),
			)
		}
	}
	// Ensure the first capability always has a cost for downstream gas/cost estimation.
	capabilities[0].Cost = cost

	// Reputation: use the precomputed Rating if available, otherwise a neutral default.
	reputationScore := dbAgent.Rating
	if reputationScore == 0 {
		reputationScore = 0.8
	}

	return &identity.AgentCard{
		DID:          agentDID,
		Endpoints:    endpoints,
		Capabilities: capabilities,
		Reputation: &identity.Reputation{
			Score: reputationScore,
		},
		Proof: &identity.Proof{
			Type:         "SystemGenerated",
			Created:      dbAgent.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ProofPurpose: "authentication",
			JWS:          "", // System-generated agent, no signature
		},
	}
}

// SelectAgentWithFailover selects an agent and provides failover if needed
func (s *DatabaseAgentSelector) SelectAgentWithFailover(ctx context.Context, task *Task, failedAgentID string) (*identity.AgentCard, error) {
	s.logger.Info("selecting failover agent",
		zap.String("task_id", task.ID),
		zap.String("failed_agent_id", failedAgentID),
	)

	// Use meta-agent failover mechanism
	dbAgent, err := s.metaAgent.GetFailoverAgent(ctx, task, failedAgentID)
	if err != nil {
		return nil, fmt.Errorf("failover failed: %w", err)
	}

	return s.convertToAgentCard(dbAgent), nil
}
