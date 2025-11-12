package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aidenlippert/zerostate/libs/api"
	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/aidenlippert/zerostate/libs/storage"
	"github.com/aidenlippert/zerostate/libs/websocket"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/libp2p/go-libp2p"
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
		logger.Info("SQLite database initialized (migrations not supported for SQLite)")
	}
	defer db.Close()
	logger.Info("database connection established")

	// Initialize HNSW index for agent discovery
	logger.Info("initializing HNSW index")
	hnsw := search.NewHNSWIndex(16, 200)
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

	// Initialize orchestrator components
	logger.Info("initializing orchestrator components")

	// Use database-backed agent selector with meta-agent auction
	selector := orchestration.NewDatabaseAgentSelector(db, orchestration.DefaultMetaAgentConfig(), logger)

	// Use real WASM executor if S3 is configured, otherwise use mock
	var executor orchestration.TaskExecutor
	if binaryStore != nil {
		// Use real WASM task executor with production components
		executor = orchestration.NewWASMTaskExecutor(wasmRunner, binaryStore, logger)
		logger.Info("using real WASM task executor with S3 backend")
	} else {
		executor = orchestration.NewMockTaskExecutor(logger)
		logger.Info("using mock task executor (S3 not configured)")
	}

	orchConfig := orchestration.DefaultOrchestratorConfig()
	orchConfig.NumWorkers = *workers

	orch := orchestration.NewOrchestrator(ctx, taskQueue, selector, executor, orchConfig, logger)
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

	// Initialize API handlers
	logger.Info("initializing API handlers")
	handlers := api.NewHandlers(ctx, logger, p2pHost, signer, hnsw, taskQueue, orch, db, s3Storage, wsHub, wasmRunner, resultStore, binaryStore)

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
