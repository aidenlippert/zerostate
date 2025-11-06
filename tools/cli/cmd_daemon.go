package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/zerostate/libs/identity"
	"github.com/zerostate/libs/p2p"
	"github.com/zerostate/libs/telemetry"
	"go.uber.org/zap"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run a persistent zerostate node",
	Long:  `Runs a persistent zerostate node with HTTP API for management and monitoring.`,
	RunE:  runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	
	daemonCmd.Flags().String("http-addr", ":9090", "HTTP API listen address")
	daemonCmd.Flags().Bool("enable-dht", true, "enable DHT")
	daemonCmd.Flags().String("dht-mode", "auto", "DHT mode: client, server, auto")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("starting zerostate daemon",
		zap.String("version", "v0.1.0"),
	)

	// Get flags
	httpAddr, _ := cmd.Flags().GetString("http-addr")
	bootstrap, _ := cmd.Flags().GetString("bootstrap")
	listen, _ := cmd.Flags().GetString("listen")
	enableDHT, _ := cmd.Flags().GetBool("enable-dht")
	dhtModeStr, _ := cmd.Flags().GetString("dht-mode")

	var dhtMode dht.ModeOpt
	switch dhtModeStr {
	case "client":
		dhtMode = dht.ModeClient
	case "server":
		dhtMode = dht.ModeServer
	default:
		dhtMode = dht.ModeAuto
	}

	// Initialize telemetry if configured
	var tp *telemetry.TracerProvider
	jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT")
	if jaegerEndpoint != "" {
		tp, err = telemetry.InitTracer(&telemetry.Config{
			ServiceName:    "zerostate-daemon",
			ServiceVersion: "v0.1.0",
			JaegerEndpoint: jaegerEndpoint,
			Enabled:        true,
			Logger:         logger,
		})
		if err != nil {
			logger.Warn("failed to initialize telemetry", zap.Error(err))
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	// Create signer
	signer, err := identity.NewSigner(logger)
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}

	logger.Info("identity initialized",
		zap.String("did", signer.DID()),
	)

	// Create P2P node
	bootstrapPeers := []string{}
	if bootstrap != "" {
		bootstrapPeers = []string{bootstrap}
	}

	p2pCfg := &p2p.Config{
		ListenAddrs:    []string{listen},
		BootstrapPeers: bootstrapPeers,
		EnableDHT:      enableDHT,
		DHTMode:        dhtMode,
		EnableMDNS:     true,
		Logger:         logger,
	}

	node, err := p2p.NewNode(ctx, p2pCfg)
	if err != nil {
		return fmt.Errorf("failed to create P2P node: %w", err)
	}
	defer node.Close()

	logger.Info("P2P node started",
		zap.String("peer_id", node.ID().String()),
		zap.Int("addrs_count", len(node.Addrs())),
	)

	// Bootstrap if peers configured
	if len(bootstrapPeers) > 0 {
		if err := node.Bootstrap(ctx); err != nil {
			logger.Error("bootstrap failed", zap.Error(err))
		} else {
			logger.Info("bootstrap complete")
		}
	}

	// Create and sign agent card
	card := &identity.AgentCard{
		Context: "https://www.w3.org/2018/credentials/v1",
		Type:    "zs:AgentCard",
		DID:     signer.DID(),
		Endpoints: &identity.Endpoints{
			Libp2p: []string{node.Addrs()[0].String()},
			Region: "local",
		},
		Capabilities: []identity.Capability{
			{
				Name:    "node.daemon",
				Version: "0.1.0",
			},
		},
		Policy: &identity.Policy{
			SLAClass: "best-effort",
			Privacy:  "public",
		},
	}

	if err := signer.SignCard(card); err != nil {
		return fmt.Errorf("failed to sign agent card: %w", err)
	}

	logger.Info("agent card created and signed")

	// Start HTTP API server
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if node.DHT() != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
		}
	})

	// Node info endpoint
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"peer_id":"%s","addrs":[`, node.ID().String())
		for i, addr := range node.Addrs() {
			if i > 0 {
				fmt.Fprintf(w, ",")
			}
			fmt.Fprintf(w, `"%s"`, addr.String())
		}
		fmt.Fprintf(w, `],"did":"%s"}`, signer.DID())
	})

	// Peers endpoint
	mux.HandleFunc("/api/peers", func(w http.ResponseWriter, r *http.Request) {
		peers := node.Host().Network().Peers()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":%d,"peers":[`, len(peers))
		for i, peerID := range peers {
			if i > 0 {
				fmt.Fprintf(w, ",")
			}
			fmt.Fprintf(w, `"%s"`, peerID.String())
		}
		fmt.Fprintf(w, `]}`)
	})

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	go func() {
		logger.Info("HTTP API listening", zap.String("addr", httpAddr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", zap.Error(err))
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

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("daemon running, waiting for shutdown signal")
	<-sigCh

	logger.Info("shutting down...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	return nil
}
