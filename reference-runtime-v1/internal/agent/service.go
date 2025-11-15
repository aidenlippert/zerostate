package agent

import (
	"context"
	"sync"

	ariv1 "github.com/aidenlippert/zerostate/reference-runtime-v1/pkg/ari/v1"
	"go.uber.org/zap"
)

// Service implements the ARI v1 Agent service
type Service struct {
	ariv1.UnimplementedAgentServer

	config *Config
	logger *zap.Logger
	mu     sync.RWMutex
}

// Config contains agent configuration
type Config struct {
	DID          string
	Name         string
	Version      string
	Capabilities []string
	RuntimeInfo  *RuntimeInfo
	Limits       *ResourceLimits
}

// RuntimeInfo describes the runtime environment
type RuntimeInfo struct {
	Type     string
	Version  string
	Metadata map[string]string
}

// ResourceLimits defines resource constraints
type ResourceLimits struct {
	MaxMemoryMB        int32
	MaxExecutionTimeMS int32
	MaxConcurrentTasks int32
}

// NewService creates a new Agent service
func NewService(config *Config, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		config: config,
		logger: logger,
	}
}

// GetInfo returns the agent's information
func (s *Service) GetInfo(ctx context.Context, req *ariv1.GetInfoRequest) (*ariv1.GetInfoResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.logger.Info("GetInfo called",
		zap.String("did", s.config.DID),
	)

	resp := &ariv1.GetInfoResponse{
		Did:          s.config.DID,
		Name:         s.config.Name,
		Version:      s.config.Version,
		Capabilities: s.config.Capabilities,
		RuntimeInfo: &ariv1.RuntimeInfo{
			Type:     s.config.RuntimeInfo.Type,
			Version:  s.config.RuntimeInfo.Version,
			Metadata: s.config.RuntimeInfo.Metadata,
		},
		Limits: &ariv1.ResourceLimits{
			MaxMemoryMb:        s.config.Limits.MaxMemoryMB,
			MaxExecutionTimeMs: s.config.Limits.MaxExecutionTimeMS,
			MaxConcurrentTasks: s.config.Limits.MaxConcurrentTasks,
		},
	}

	return resp, nil
}

// UpdateConfig updates the agent configuration
func (s *Service) UpdateConfig(config *Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
	s.logger.Info("Agent config updated",
		zap.String("did", config.DID),
		zap.String("name", config.Name),
	)
}
