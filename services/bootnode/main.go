// Package main implements the zerostate bootnode.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zerostate/libs/p2p"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info("starting zerostate bootnode", zap.String("version", "v0.1.0"))

	listenAddr := getEnv("ZEROSTATE_LISTEN", "/ip4/0.0.0.0/udp/4001/quic-v1")

	cfg := &p2p.Config{
		ListenAddrs:    []string{listenAddr},
		BootstrapPeers: []string{},
		EnableDHT:      true,
		DHTMode:        dht.ModeServer,
		Logger:         logger,
	}

	node, err := p2p.NewNode(ctx, cfg)
	if err != nil {
		logger.Fatal("failed to create node", zap.Error(err))
	}
	defer node.Close()

	logger.Info("bootnode started",
		zap.String("peer_id", node.ID().String()),
		zap.Int("addrs_count", len(node.Addrs())),
	)

	// Health and metrics endpoints
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	http.HandleFunc("/peer-id", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(node.ID().String()))
	})

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		logger.Info("health endpoint listening on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			logger.Error("health endpoint failed", zap.Error(err))
		}
	}()

	// Metrics update loop
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				node.UpdateMetrics(ctx)
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("bootnode running")
	<-sigCh

	logger.Info("shutting down...")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
