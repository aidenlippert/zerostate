package api

import (
	"context"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/zap"
)

// Handlers holds all API request handlers and their dependencies
type Handlers struct {
	// Core dependencies
	logger       *zap.Logger
	host         host.Host
	signer       *identity.Signer
	hnsw         *search.HNSWIndex
	taskQueue    *orchestration.TaskQueue
	orchestrator *orchestration.Orchestrator

	// Services (to be added)
	// userManager    *auth.UserManager
	// paymentService *payment.Service

	ctx context.Context
}

// NewHandlers creates a new Handlers instance
func NewHandlers(
	ctx context.Context,
	logger *zap.Logger,
	host host.Host,
	signer *identity.Signer,
	hnsw *search.HNSWIndex,
	taskQueue *orchestration.TaskQueue,
	orchestrator *orchestration.Orchestrator,
) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Handlers{
		logger:       logger,
		host:         host,
		signer:       signer,
		hnsw:         hnsw,
		taskQueue:    taskQueue,
		orchestrator: orchestrator,
		ctx:          ctx,
	}
}

// Context returns the handlers' context
func (h *Handlers) Context() context.Context {
	if h.ctx == nil {
		return context.Background()
	}
	return h.ctx
}

// Logger returns the handlers' logger
func (h *Handlers) Logger() *zap.Logger {
	return h.logger
}
