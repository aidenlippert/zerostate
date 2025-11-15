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
	"github.com/aidenlippert/zerostate/libs/identity"
	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"
)

func main() {
	// Parse command-line flags
	var (
		host  = flag.String("host", "0.0.0.0", "Server host")
		port  = flag.Int("port", 8080, "Server port")
		debug = flag.Bool("debug", false, "Enable debug logging")
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

	logger.Info("starting ZeroState Registration API Test Server",
		zap.String("host", *host),
		zap.Int("port", *port),
		zap.Bool("debug", *debug),
		zap.String("log_level_env", logLevel),
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// Initialize minimal handlers (auth only)
	logger.Info("initializing API handlers")
	handlers := &api.Handlers{
		DB:     db,
		Logger: logger,
		Signer: signer,
		// NOTE: Orchestrator and other services are nil for this test
	}

	// Create server configuration
	config := api.DefaultConfig()
	config.Host = *host
	config.Port = *port

	// Create and start server
	logger.Info("creating API server")
	server := api.NewServer(config, handlers, logger)

	// Start server in a goroutine
	go func() {
		logger.Info("starting HTTP server",
			zap.String("address", fmt.Sprintf("%s:%d", *host, *port)))
		if err := server.Start(ctx); err != nil {
			logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown server gracefully
	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}