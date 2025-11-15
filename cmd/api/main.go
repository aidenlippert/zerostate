package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aidenlippert/zerostate/libs/api"
	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/llm"
	"github.com/aidenlippert/zerostate/libs/metrics"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/aidenlippert/zerostate/libs/storage"
	"github.com/aidenlippert/zerostate/libs/substrate"
	"github.com/aidenlippert/zerostate/libs/websocket"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/libp2p/go-libp2p"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func main() {
	// Parse command-line flags
	var (
		host    = flag.String("host", "0.0.0.0", "Server host")
		port    = flag.Int("port", 8080, "Server port")
		workers = flag.Int("workers", 5, "Number of orchestrator workers")
		debug   = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	// Initialize logger (allow LOG_LEVEL env override)
	var logger *zap.Logger
	var err error
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "debug" {
		*debug = true
	}
	if *debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting ZeroState API server",
		zap.String("host", *host),
		zap.Int("port", *port),
		zap.Int("workers", *workers),
		zap.Bool("debug", *debug),
		zap.String("log_level_env", logLevel),
	)

	// Initialize Prometheus metrics registry shared across components
	promRegistry := prometheus.NewRegistry()
	promMetrics := metrics.NewPrometheusMetrics(promRegistry)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize p2p host
	logger.Info("initializing p2p host")
	p2pHost, err := libp2p.New()
	if err != nil {
		logger.Fatal("failed to create p2p host", zap.Error(err))
	}
	defer p2pHost.Close()
	logger.Info("p2p host initialized", zap.String("peer_id", p2pHost.ID().String()))

	// Initialize GossipService for market (L4 Concordat)
	logger.Info("initializing gossip service for market messaging")
	gossip, err := p2p.NewGossipService(ctx, p2pHost, logger)
	if err != nil {
		logger.Fatal("failed to create gossip service", zap.Error(err))
	}
	logger.Info("gossip service initialized for market auction protocol")

	// Initialize identity signer
	logger.Info("initializing identity signer")
	signer, err := identity.NewSigner(logger)
	if err != nil {
		logger.Fatal("failed to create signer", zap.Error(err))
	}
	logger.Info("identity signer initialized", zap.String("did", signer.DID()))

	// Initialize database
	logger.Info("initializing database")

	// Check for DATABASE_URL environment variable (Postgres in production)
	var db *database.Database
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		logger.Info("connecting to PostgreSQL database")
		sqlDB, err := sql.Open("postgres", databaseURL)
		if err != nil {
			logger.Fatal("failed to connect to PostgreSQL", zap.Error(err))
		}
		db = database.NewDatabase(sqlDB)

		// Run Postgres migrations
		logger.Info("running database migrations")
		if err := database.Migrate(ctx, db.Conn()); err != nil {
			logger.Fatal("failed to run database migrations", zap.Error(err))
		}
		logger.Info("database migrations completed successfully")

		// Run additional schema fixes
		logger.Info("running schema fix migrations")
		if err := db.RunMigrations(ctx); err != nil {
			logger.Warn("schema fix migrations failed (may already be applied)", zap.Error(err))
		} else {
			logger.Info("schema fix migrations completed successfully")
			// Verify the schema
			if err := db.VerifySchema(ctx); err != nil {
				logger.Warn("schema verification failed", zap.Error(err))
			}
		}
	} else {
		// Fallback to SQLite for local development
		logger.Info("using SQLite database for local development")
		var err error
		db, err = database.NewDB("./zerostate.db")
		if err != nil {
			logger.Fatal("failed to initialize SQLite database", zap.Error(err))
		}

		// Initialize SQLite schema automatically
		logger.Info("initializing SQLite schema")
		if err := db.InitializeSQLiteSchema(ctx); err != nil {
			logger.Fatal("failed to initialize SQLite schema", zap.Error(err))
		}
		logger.Info("SQLite schema initialized successfully")
	}
	defer db.Close()
	logger.Info("database connection established")

	// Initialize HNSW index for agent discovery
	logger.Info("initializing HNSW index")
	hnsw := search.NewIndex(logger)
	logger.Info("HNSW index initialized")

	// Initialize task queue
	logger.Info("initializing task queue")
	taskQueue := orchestration.NewTaskQueue(ctx, 1000, logger)
	defer taskQueue.Close()
	logger.Info("task queue initialized")

	// Initialize WASM execution components (partial - binaryStore needs S3 which is initialized later)
	logger.Info("initializing WASM execution components")

	// Create WASM runner with 5-minute timeout
	wasmRunner := execution.NewWASMRunner(logger, 5*time.Minute)

	// Create result store with database connection
	resultStore := execution.NewPostgresResultStore(db.Conn(), logger)

	logger.Info("WASM runner and result store initialized")

	// Initialize S3 storage (optional) - must be before orchestrator
	var s3Storage *storage.S3Storage
	if bucket := os.Getenv("S3_BUCKET"); bucket != "" {
		logger.Info("initializing S3 storage")
		s3Config := &storage.S3Config{
			Bucket:          bucket,
			Region:          getEnv("S3_REGION", "us-east-1"),
			AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			Endpoint:        os.Getenv("S3_ENDPOINT"), // For LocalStack/MinIO
		}
		var err error
		s3Storage, err = storage.NewS3Storage(ctx, s3Config, logger)
		if err != nil {
			logger.Warn("failed to initialize S3 storage, uploads will use placeholder URLs",
				zap.Error(err),
			)
			s3Storage = nil
		} else {
			logger.Info("S3 storage initialized successfully",
				zap.String("bucket", s3Config.Bucket),
				zap.String("region", s3Config.Region),
			)
		}
	} else {
		logger.Info("S3 storage not configured (set S3_BUCKET env var to enable)")
	}

	// Create binary store adapter (depends on S3 storage)
	var binaryStore execution.BinaryStore
	if s3Storage != nil {
		// Create adapter function that converts database.Agent to execution.Agent
		getAgentFunc := func(id string) (*execution.Agent, error) {
			dbAgent, err := db.GetAgentByID(id)
			if err != nil {
				return nil, err
			}
			if dbAgent == nil {
				return nil, nil
			}
			return &execution.Agent{
				BinaryURL:  dbAgent.BinaryURL,
				BinaryHash: dbAgent.BinaryHash,
			}, nil
		}
		dbAdapter := execution.NewDatabaseAdapter(getAgentFunc)
		binaryStore = execution.NewS3BinaryStore(s3Storage, dbAdapter)
		logger.Info("binary store initialized with S3 backend")
	} else {
		logger.Info("binary store not available (S3 storage not configured)")
	}

	// Initialize Groq LLM client
	logger.Info("initializing Groq LLM client")
	groqAPIKey := os.Getenv("GROQ_API_KEY")
	if groqAPIKey == "" {
		logger.Warn("GROQ_API_KEY not set, intelligent task decomposition will fail")
	}
	groqClient := llm.NewGroqClient(groqAPIKey, "meta-llama/llama-4-scout-17b-16e-instruct", logger)
	logger.Info("Groq LLM client initialized", zap.String("model", groqClient.GetModel()))

	// Initialize R2 storage for WASM binaries
	var r2Storage *storage.R2Storage
	if accountID := os.Getenv("R2_ACCOUNT_ID"); accountID != "" {
		logger.Info("initializing R2 storage")
		r2Config := storage.Config{
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			Endpoint:        os.Getenv("R2_ENDPOINT"),
			BucketName:      getEnv("R2_BUCKET_NAME", "zerostate-agents"),
			Region:          "auto",
		}
		var err error
		r2Storage, err = storage.NewR2Storage(r2Config)
		if err != nil {
			logger.Warn("failed to initialize R2 storage, will fallback to mock executor",
				zap.Error(err),
			)
			r2Storage = nil
		} else {
			logger.Info("R2 storage initialized successfully",
				zap.String("bucket", r2Config.BucketName),
			)
		}
	} else {
		logger.Info("R2 storage not configured (set R2_ACCOUNT_ID env var to enable)")
	}

	// Initialize WASM runner V2 for intelligent execution
	var wasmRunnerV2 *execution.WASMRunnerV2
	if r2Storage != nil {
		wasmRunnerV2 = execution.NewWASMRunnerV2(logger, r2Storage, 30*time.Second, 128)
		logger.Info("WASM runner V2 initialized with R2 backend")
	}

	// Initialize orchestrator components
	logger.Info("initializing orchestrator components")

	// ============================================================================
	// AGENT SELECTOR: Choose between database, blockchain, or hybrid
	// ============================================================================
	// Environment variable controls which selector to use:
	//   AGENT_SELECTOR=database  (default, legacy)
	//   AGENT_SELECTOR=chain     (blockchain only, Sprint 5 Phase 4)
	//   AGENT_SELECTOR=hybrid    (transition mode with HYBRID_MODE env var)
	//
	// HYBRID_MODE (when AGENT_SELECTOR=hybrid):
	//   db_primary     (database primary, chain validation)
	//   chain_primary  (chain primary, database fallback)
	//   chain_only     (chain only, database deleted)
	// ============================================================================

	var selector orchestration.AgentSelector

	selectorType := os.Getenv("AGENT_SELECTOR")
	if selectorType == "" {
		selectorType = "database" // Default to database for backwards compatibility
	}

	switch selectorType {
	case "chain":
		// Sprint 5 Phase 4: Blockchain-only agent discovery
		logger.Info("🔗 Using BLOCKCHAIN agent selector (Sprint 5 Phase 4)")

		substrateEndpoint := os.Getenv("SUBSTRATE_ENDPOINT")
		if substrateEndpoint == "" {
			substrateEndpoint = "ws://127.0.0.1:9944" // Default local node
		}

		substrateClient, err := substrate.NewClient(substrateEndpoint)
		if err != nil {
			logger.Fatal("failed to connect to substrate blockchain",
				zap.String("endpoint", substrateEndpoint),
				zap.Error(err),
			)
		}
		defer substrateClient.Close()

		selector = orchestration.NewChainAgentSelector(substrateClient, hnsw, orchestration.DefaultMetaAgentConfig(), logger)
		logger.Info("✅ ChainAgentSelector initialized",
			zap.String("substrate_endpoint", substrateEndpoint),
		)

	case "hybrid":
		// Transition mode: Database + Blockchain
		logger.Info("🔄 Using HYBRID agent selector (migration mode)")

		// Initialize both selectors
		dbSelector := orchestration.NewDatabaseAgentSelector(db, hnsw, orchestration.DefaultMetaAgentConfig(), logger)

		substrateEndpoint := os.Getenv("SUBSTRATE_ENDPOINT")
		if substrateEndpoint == "" {
			substrateEndpoint = "ws://127.0.0.1:9944"
		}

		substrateClient, err := substrate.NewClient(substrateEndpoint)
		if err != nil {
			logger.Warn("failed to connect to substrate, using database only",
				zap.Error(err),
			)
			selector = dbSelector
			break
		}
		defer substrateClient.Close()

		chainSelector := orchestration.NewChainAgentSelector(substrateClient, hnsw, orchestration.DefaultMetaAgentConfig(), logger)

		// Determine hybrid mode
		hybridMode := orchestration.HybridMode(os.Getenv("HYBRID_MODE"))
		if hybridMode == "" {
			hybridMode = orchestration.ModeDBPrimary // Default: database primary
		}

		selector = orchestration.NewHybridAgentSelector(dbSelector, chainSelector, hybridMode, logger)
		logger.Info("✅ HybridAgentSelector initialized",
			zap.String("mode", string(hybridMode)),
			zap.String("substrate_endpoint", substrateEndpoint),
		)

	case "database":
		fallthrough
	default:
		// Legacy: Database-backed agent selector
		logger.Info("💾 Using DATABASE agent selector (legacy)")
		selector = orchestration.NewDatabaseAgentSelector(db, hnsw, orchestration.DefaultMetaAgentConfig(), logger)
	}

	logger.Info("agent selector initialized", zap.String("type", selectorType))

	// Choose executor based on configuration
	var executor orchestration.TaskExecutor
	var runtimeRegistry *orchestration.RuntimeRegistry

	// Prefer decentralized runtime discovery unless explicitly disabled
	presenceTopic := getEnv("P2P_PRESENCE_TOPIC", "ainur/v1/presence/global")
	disableP2P := strings.EqualFold(strings.TrimSpace(os.Getenv("DISABLE_P2P_ARI")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("DISABLE_P2P_ARI")), "true")

	if !disableP2P {
		p2pExecutor, err := orchestration.NewP2PARIExecutor(presenceTopic, logger, promMetrics)
		if err != nil {
			logger.Warn("unable to initialize P2P ARI executor",
				zap.String("presence_topic", presenceTopic),
				zap.Error(err),
			)
		} else {
			executor = p2pExecutor
			defer p2pExecutor.Close()
			runtimeRegistry = p2pExecutor.RuntimeRegistry()

			logger.Info("using P2P ARI executor",
				zap.String("presence_topic", presenceTopic),
			)
		}
	} else {
		logger.Info("P2P ARI executor disabled by configuration",
			zap.String("presence_topic", presenceTopic),
		)
	}

	if executor == nil && os.Getenv("ARI_RUNTIME_ADDR") != "" {
		// Check if ARI runtime is configured (Sprint 1 Phase 3)
		ariRuntimeAddr := os.Getenv("ARI_RUNTIME_ADDR")
		logger.Info("ARI runtime configured, using ARI-v1 protocol",
			zap.String("runtime_address", ariRuntimeAddr),
		)
		ariExecutor, err := orchestration.NewARIExecutor(ariRuntimeAddr, logger)
		if err != nil {
			logger.Fatal("failed to create ARI executor", zap.Error(err))
		}

		// Query runtime info on startup
		runtimeInfo, err := ariExecutor.GetRuntimeInfo(ctx)
		if err != nil {
			logger.Warn("failed to get runtime info, but continuing",
				zap.Error(err),
			)
		} else {
			logger.Info("✅ ARI runtime connected successfully",
				zap.String("runtime_did", runtimeInfo.Did),
				zap.String("runtime_name", runtimeInfo.Name),
				zap.Strings("capabilities", runtimeInfo.Capabilities),
			)
		}

		executor = ariExecutor
		defer ariExecutor.Close()
	} else if groqAPIKey != "" && r2Storage != nil && wasmRunnerV2 != nil {
		// Use intelligent task executor with Groq LLM + WASM agents
		executor = orchestration.NewIntelligentTaskExecutor(groqClient, wasmRunnerV2, r2Storage, logger)
		logger.Info("🧠 using intelligent task executor with Groq LLM + R2 WASM agents")
	} else if binaryStore != nil {
		// Fallback to basic WASM executor with S3
		executor = orchestration.NewWASMTaskExecutor(wasmRunner, binaryStore, logger)
		logger.Info("using basic WASM task executor with S3 backend")
	} else {
		// Fallback to mock executor
		executor = orchestration.NewMockTaskExecutor(logger)
		logger.Info("using mock task executor (no storage configured)")
	}

	orchConfig := orchestration.DefaultOrchestratorConfig()
	orchConfig.NumWorkers = *workers

	orch := orchestration.NewOrchestrator(ctx, taskQueue, selector, executor, orchConfig, logger)

	// Wire Auctioneer into orchestrator for market-based selection (L4 Concordat)
	if gossip != nil {
		auctioneer := orchestration.NewAuctioneer(gossip, logger.With(zap.String("component", "auctioneer")))
		orch.SetAuctioneer(auctioneer)
		logger.Info("auctioneer wired into orchestrator - market-based task selection enabled")
	}

	logger.Info("orchestrator components initialized with meta-agent")

	// Start orchestrator
	logger.Info("starting orchestrator")
	if err := orch.Start(); err != nil {
		logger.Fatal("failed to start orchestrator", zap.Error(err))
	}
	defer orch.Stop()
	logger.Info("orchestrator started successfully")

	// Initialize WebSocket hub
	logger.Info("initializing WebSocket hub")
	wsHub := websocket.NewHub(ctx, logger)
	wsHub.Start()
	defer wsHub.Stop()
	logger.Info("WebSocket hub started")

	// Initialize blockchain service (Sprint 2)
	logger.Info("initializing blockchain service")
	blockchainEndpoint := os.Getenv("BLOCKCHAIN_ENDPOINT")
	if blockchainEndpoint == "" {
		blockchainEndpoint = "ws://127.0.0.1:35651" // Default to local chain-v2
	}
	keystoreSecret := os.Getenv("BLOCKCHAIN_SECRET")
	if keystoreSecret == "" {
		keystoreSecret = "//Alice" // Default dev account
	}
	blockchain, err := substrate.NewBlockchainService(blockchainEndpoint, keystoreSecret, logger)
	if err != nil {
		logger.Warn("blockchain service failed to initialize - continuing without blockchain",
			zap.Error(err),
		)
	} else if blockchain.IsEnabled() {
		logger.Info("blockchain service initialized successfully")
		defer blockchain.Close()
	} else {
		logger.Info("blockchain service running in disabled mode")
	}

	// Initialize API handlers
	logger.Info("initializing API handlers")
	handlers := api.NewHandlers(
		ctx,
		logger,
		p2pHost,
		signer,
		hnsw,
		taskQueue,
		orch,
		db,
		s3Storage,
		wsHub,
		wasmRunner,
		resultStore,
		binaryStore,
		blockchain,
		runtimeRegistry,
		promMetrics,
		promRegistry,
	)

	// Create API server
	logger.Info("creating API server")
	config := api.DefaultConfig()
	config.Host = *host
	config.Port = *port

	server := api.NewServer(config, handlers, logger)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting API server",
			zap.String("address", fmt.Sprintf("http://%s:%d", *host, *port)),
		)
		serverErr <- server.Start()
	}()

	// Print startup message
	fmt.Printf("\n")
	fmt.Printf("╔══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                                                              ║\n")
	fmt.Printf("║              🚀 ZeroState API Server Running 🚀              ║\n")
	fmt.Printf("║                                                              ║\n")
	fmt.Printf("╠══════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║                                                              ║\n")
	fmt.Printf("║  Web UI:          http://localhost:%d                     ║\n", *port)
	fmt.Printf("║  API Endpoints:   http://localhost:%d/api/v1              ║\n", *port)
	fmt.Printf("║  Health Check:    http://localhost:%d/health              ║\n", *port)
	fmt.Printf("║  Metrics:         http://localhost:%d/metrics             ║\n", *port)
	fmt.Printf("║                                                              ║\n")
	fmt.Printf("╠══════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║                                                              ║\n")
	fmt.Printf("║  Orchestrator:    %d workers active                         ║\n", *workers)
	fmt.Printf("║  P2P Node:        %s...          ║\n", p2pHost.ID().String()[:20])
	fmt.Printf("║  DID:             %s...                     ║\n", signer.DID()[:20])
	fmt.Printf("║                                                              ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")
	fmt.Printf("📝 Press Ctrl+C to shutdown gracefully\n\n")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Fatal("server error", zap.Error(err))
	case sig := <-sigCh:
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))
	}

	// Graceful shutdown
	logger.Info("shutting down gracefully...")
	fmt.Printf("\n🛑 Shutting down gracefully...\n")

	// Stop orchestrator
	fmt.Printf("   ⏸  Stopping orchestrator...\n")
	if err := orch.Stop(); err != nil {
		logger.Error("error stopping orchestrator", zap.Error(err))
	} else {
		fmt.Printf("   ✅ Orchestrator stopped\n")
	}

	// Stop server
	fmt.Printf("   ⏸  Stopping API server...\n")
	if err := server.Stop(); err != nil {
		logger.Error("error stopping server", zap.Error(err))
	} else {
		fmt.Printf("   ✅ API server stopped\n")
	}

	// Close task queue
	fmt.Printf("   ⏸  Closing task queue...\n")
	taskQueue.Close()
	fmt.Printf("   ✅ Task queue closed\n")

	// Close p2p host
	fmt.Printf("   ⏸  Closing p2p host...\n")
	if err := p2pHost.Close(); err != nil {
		logger.Error("error closing p2p host", zap.Error(err))
	} else {
		fmt.Printf("   ✅ P2P host closed\n")
	}

	// Give time for cleanup
	time.Sleep(500 * time.Millisecond)

	fmt.Printf("\n✨ Shutdown complete. Goodbye!\n\n")
	logger.Info("shutdown complete")
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
