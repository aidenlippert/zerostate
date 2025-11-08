package api

import (
	"context"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/aidenlippert/zerostate/libs/storage"
	"github.com/aidenlippert/zerostate/libs/websocket"
	"github.com/gin-gonic/gin"
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
	db           *database.DB
	s3Storage    *storage.S3Storage
	wsHub        *websocket.Hub

	// WASM Execution components (Sprint 9)
	wasmRunner     *execution.WASMRunner
	resultStore    *execution.PostgresResultStore
	binaryStore    execution.BinaryStore
	execHandlers   *ExecutionHandlers

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
	db *database.DB,
	s3Storage *storage.S3Storage,
	wsHub *websocket.Hub,
	wasmRunner *execution.WASMRunner,
	resultStore *execution.PostgresResultStore,
	binaryStore execution.BinaryStore,
) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create execution handlers
	execHandlers := NewExecutionHandlers(logger, db, wasmRunner, resultStore, binaryStore)

	return &Handlers{
		logger:       logger,
		host:         host,
		signer:       signer,
		hnsw:         hnsw,
		taskQueue:    taskQueue,
		orchestrator: orchestrator,
		db:           db,
		s3Storage:    s3Storage,
		wsHub:        wsHub,
		wasmRunner:   wasmRunner,
		resultStore:  resultStore,
		binaryStore:  binaryStore,
		execHandlers: execHandlers,
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

// Execution handler delegation methods (Sprint 9)

// ExecuteTaskDirect delegates to execution handlers
func (h *Handlers) ExecuteTaskDirect(c *gin.Context) {
	h.execHandlers.ExecuteTaskDirect(c)
}

// ListTaskResults delegates to execution handlers
func (h *Handlers) ListTaskResults(c *gin.Context) {
	h.execHandlers.ListTaskResults(c)
}
