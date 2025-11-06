// Package main implements the zerostate edge node.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/zerostate/libs/identity"
	"github.com/zerostate/libs/p2p"
	"github.com/zerostate/libs/search"
	"github.com/zerostate/libs/telemetry"
	"go.uber.org/zap"
)

var (
	cfgFile string
	logger  *zap.Logger
)

var rootCmd = &cobra.Command{
	Use:   "edge-node",
	Short: "zerostate edge node",
	Long:  `The zerostate edge node provides P2P networking, identity, and task execution.`,
	RunE:  runNode,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./edge-node.yaml)")
	rootCmd.PersistentFlags().String("listen", "/ip4/0.0.0.0/udp/4001/quic-v1", "listen address")
	rootCmd.PersistentFlags().StringSlice("bootstrap", []string{}, "bootstrap peer addresses")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().Bool("enable-mdns", false, "enable mDNS peer discovery")
	rootCmd.PersistentFlags().Bool("enable-telemetry", false, "enable OpenTelemetry tracing")
	rootCmd.PersistentFlags().String("jaeger-endpoint", "http://localhost:14268/api/traces", "Jaeger collector endpoint")

	viper.BindPFlag("listen", rootCmd.PersistentFlags().Lookup("listen"))
	viper.BindPFlag("bootstrap", rootCmd.PersistentFlags().Lookup("bootstrap"))
	viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("enable_mdns", rootCmd.PersistentFlags().Lookup("enable-mdns"))
	viper.BindPFlag("enable_telemetry", rootCmd.PersistentFlags().Lookup("enable-telemetry"))
	viper.BindPFlag("jaeger_endpoint", rootCmd.PersistentFlags().Lookup("jaeger-endpoint"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("edge-node")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.zerostate")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("ZEROSTATE")

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func initLogger() error {
	var cfg zap.Config
	level := viper.GetString("log_level")

	if level == "debug" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	var err error
	logger, err = cfg.Build()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	return nil
}

func runNode(cmd *cobra.Command, args []string) error {
	if err := initLogger(); err != nil {
		return err
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info("starting zerostate edge node",
		zap.String("version", "v0.1.0"),
	)

	// Initialize telemetry if enabled
	var tp *telemetry.TracerProvider
	if viper.GetBool("enable_telemetry") {
		var err error
		tp, err = telemetry.InitTracer(&telemetry.Config{
			ServiceName:    "zerostate-edge-node",
			ServiceVersion: "v0.1.0",
			JaegerEndpoint: viper.GetString("jaeger_endpoint"),
			Enabled:        true,
			Logger:         logger,
		})
		if err != nil {
			logger.Warn("failed to initialize telemetry", zap.Error(err))
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	// Create or load persistent identity
	keystore := identity.NewKeyStore("", logger)
	signer, err := keystore.LoadOrCreateSigner()
	if err != nil {
		return fmt.Errorf("failed to load/create signer: %w", err)
	}

	logger.Info("identity initialized",
		zap.String("did", signer.DID()),
	)

	// Create P2P node
	listenAddr := viper.GetString("listen")
	bootstrapPeers := viper.GetStringSlice("bootstrap")
	enableMDNS := viper.GetBool("enable_mdns")

	p2pCfg := &p2p.Config{
		ListenAddrs:    []string{listenAddr},
		BootstrapPeers: bootstrapPeers,
		EnableDHT:      true,
		EnableMDNS:     enableMDNS,
		DHTMode:        dht.ModeAuto,
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

	// Initialize search index
	searchIndex := search.NewIndex(logger)
	logger.Info("search index initialized")

	// Bootstrap if peers configured
	if len(bootstrapPeers) > 0 {
		if err := node.Bootstrap(ctx); err != nil {
			logger.Error("bootstrap failed", zap.Error(err))
		}
	}

	// Create and publish agent card
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
				Name:    "node.edge",
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

	logger.Info("agent card created and signed",
		zap.String("did", card.DID),
	)

	// Publish agent card to DHT
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("failed to marshal agent card: %w", err)
	}

	cid, err := node.PublishAgentCard(ctx, cardJSON)
	if err != nil {
		logger.Error("failed to publish agent card to DHT", zap.Error(err))
	} else {
		logger.Info("agent card published to DHT",
			zap.String("cid", cid),
			zap.String("did", card.DID),
		)
	}

	// Index the card for semantic search
	if err := searchIndex.IndexCard(ctx, cardJSON); err != nil {
		logger.Error("failed to index agent card", zap.Error(err))
	} else {
		logger.Info("agent card indexed for semantic search",
			zap.String("did", card.DID),
		)
	}

	// Start HTTP server for health and metrics
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Check if DHT is ready
		if node.DHT() != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
		}
	})

	// Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// Search endpoint
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			http.Error(w, "missing query parameter 'q'", http.StatusBadRequest)
			return
		}
		
		limitStr := r.URL.Query().Get("limit")
		limit := 10
		if limitStr != "" {
			if n, err := fmt.Sscanf(limitStr, "%d", &limit); n != 1 || err != nil {
				http.Error(w, "invalid limit parameter", http.StatusBadRequest)
				return
			}
		}
		
		results, err := searchIndex.Search(r.Context(), query, limit)
		if err != nil {
			http.Error(w, fmt.Sprintf("search failed: %v", err), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query":   query,
			"limit":   limit,
			"results": results,
		})
	})

	go func() {
		logger.Info("HTTP server listening on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
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

	logger.Info("node running, waiting for shutdown signal")
	<-sigCh

	logger.Info("shutting down...")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
