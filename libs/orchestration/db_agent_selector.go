package orchestration

import (
	"context"
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
func NewDatabaseAgentSelector(db *database.DB, config *MetaAgentConfig, logger *zap.Logger) *DatabaseAgentSelector {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &DatabaseAgentSelector{
		metaAgent: NewMetaAgent(db, config, logger),
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
	// Parse capabilities from JSON string
	// For now, create a simple AgentCard matching the identity.AgentCard structure

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

	return &identity.AgentCard{
		DID: agentDID,
		Endpoints: &identity.Endpoints{
			Libp2p: []string{}, // TODO: Get from agent metadata
		},
		Capabilities: []identity.Capability{
			{
				Name:    dbAgent.Name,
				Version: "1.0.0",
				Cost: &identity.Cost{
					Unit:  "task",
					Price: 0.10, // TODO: Parse from pricing_model JSON
				},
				Metadata: map[string]interface{}{
					"description": description,
					"status":      string(dbAgent.Status),
				},
			},
		},
		Reputation: &identity.Reputation{
			Score: 0.8, // TODO: Get from reputation system
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
