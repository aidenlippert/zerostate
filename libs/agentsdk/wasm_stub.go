//go:build !js || !wasm
// +build !js !wasm

package agentsdk

import (
	"context"

	"go.uber.org/zap"
)

// WASMAgent wraps an Agent for WebAssembly execution (stub for non-WASM builds)
type WASMAgent struct {
	agent  Agent
	logger *zap.Logger
}

// NewWASMAgent creates a new WASM agent wrapper
func NewWASMAgent(agent Agent, logger *zap.Logger) *WASMAgent {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &WASMAgent{
		agent:  agent,
		logger: logger,
	}
}

// Run runs the WASM agent (stub - just runs agent normally)
func (w *WASMAgent) Run(ctx context.Context) error {
	w.logger.Info("running agent in native mode (not WASM)",
		zap.String("name", w.agent.GetName()),
	)

	<-ctx.Done()
	return w.agent.Stop(ctx)
}

// Export is a no-op for non-WASM builds
func (w *WASMAgent) Export() {
	w.logger.Warn("WASM export not available in native builds")
}
