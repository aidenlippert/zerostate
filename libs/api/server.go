package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Server represents the ZeroState API server
type Server struct {
	config   *Config
	router   *gin.Engine
	server   *http.Server
	logger   *zap.Logger
	tracer   trace.Tracer
	handlers *Handlers
	ctx      context.Context
	cancel   context.CancelFunc
}

// Config holds the API server configuration
type Config struct {
	// Server settings
	Host string
	Port int

	// TLS settings (optional)
	TLSCertFile string
	TLSKeyFile  string

	// Request limits
	MaxUploadSize    int64 // Maximum WASM binary size (default: 50MB)
	RequestTimeout   time.Duration
	ShutdownTimeout  time.Duration

	// Rate limiting
	EnableRateLimit bool
	RateLimit       int // Requests per minute per IP

	// CORS
	EnableCORS      bool
	AllowedOrigins  []string

	// Observability
	EnableMetrics   bool
	EnableTracing   bool
	MetricsPath     string
}

// DefaultConfig returns a default server configuration
func DefaultConfig() *Config {
	return &Config{
		Host:             "0.0.0.0",
		Port:             8080,
		MaxUploadSize:    50 * 1024 * 1024, // 50MB
		RequestTimeout:   30 * time.Second,
		ShutdownTimeout:  10 * time.Second,
		EnableRateLimit:  true,
		RateLimit:        100, // 100 requests per minute
		EnableCORS:       true,
		AllowedOrigins:   []string{"*"}, // Configure properly in production
		EnableMetrics:    true,
		EnableTracing:    true,
		MetricsPath:      "/metrics",
	}
}

// NewServer creates a new API server instance
func NewServer(config *Config, handlers *Handlers, logger *zap.Logger) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize tracer if enabled
	var tracer trace.Tracer
	// TODO: Properly initialize tracer from telemetry package
	// For now, tracing is disabled
	_ = tracer

	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode) // Use gin.DebugMode for development

	router := gin.New()

	// Global middleware (order matters!)
	router.Use(gin.Recovery())
	router.Use(correlationIDMiddleware()) // Add correlation IDs first
	router.Use(loggingMiddleware(logger)) // Then logging with correlation IDs

	if config.EnableTracing {
		router.Use(tracingMiddleware(tracer))
	}

	if config.EnableCORS {
		router.Use(corsMiddleware(config.AllowedOrigins))
	}

	if config.EnableRateLimit {
		router.Use(rateLimitMiddleware(config.RateLimit))
	}

	server := &Server{
		config:   config,
		router:   router,
		logger:   logger,
		tracer:   tracer,
		handlers: handlers,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Setup routes
	server.setupRoutes()

	// Create HTTP server
	server.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      router,
		ReadTimeout:  config.RequestTimeout,
		WriteTimeout: config.RequestTimeout,
		IdleTimeout:  60 * time.Second,
	}

	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check endpoints
	s.router.GET("/health", s.handleHealth)
	s.router.GET("/ready", s.handleReady)

	// Metrics endpoint
	if s.config.EnableMetrics {
		s.router.GET(s.config.MetricsPath, gin.WrapH(promhttp.Handler()))
	}

	// Serve static files (Web UI)
	s.router.Static("/static", "./web/static")
	s.router.StaticFile("/", "./web/static/index.html")
	s.router.StaticFile("/submit-task", "./web/static/index.html")
	s.router.StaticFile("/tasks", "./web/static/index.html")
	s.router.StaticFile("/agents", "./web/static/index.html")
	s.router.StaticFile("/dashboard", "./web/static/index.html")

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// User management (public routes)
		users := v1.Group("/users")
		{
			users.POST("/register", s.handlers.RegisterUser)
			users.POST("/login", s.handlers.LoginUser)

			// Protected user routes
			protected := users.Group("")
			protected.Use(authMiddleware())
			{
				protected.POST("/logout", s.handlers.LogoutUser)
				protected.GET("/me", s.handlers.GetCurrentUser)
				protected.POST("/me/avatar", s.handlers.UploadAvatar)
			}
		}

		// Protected routes - require authentication
		protected := v1.Group("")
		protected.Use(authMiddleware())
		{
			// Agent registration and task management now require auth
			protected.POST("/agents/register", s.handlers.RegisterAgent)
			protected.POST("/tasks/submit", s.handlers.SubmitTask)
			protected.GET("/tasks/:id", s.handlers.GetTask)
			protected.GET("/tasks/:id/status", s.handlers.GetTaskStatus)
			protected.GET("/tasks/:id/result", s.handlers.GetTaskResult)
			// Agent management
			agents := protected.Group("/agents")
			{
				agents.GET("/:id", s.handlers.GetAgent)
				agents.GET("", s.handlers.ListAgents)
				agents.PUT("/:id", s.handlers.UpdateAgent)
				agents.DELETE("/:id", s.handlers.DeleteAgent)
				agents.GET("/search", s.handlers.SearchAgents)

				// Agent WASM binary upload and management
				agents.POST("/upload", s.handlers.UploadAgentSimple)  // Simplified upload endpoint (auto-generates ID)
				agents.POST("/:id/binary", s.handlers.UploadAgent)
				agents.GET("/:id/binary", s.handlers.GetAgentBinary)
				agents.DELETE("/:id/binary", s.handlers.DeleteAgentBinary)
				agents.GET("/:id/versions", s.handlers.ListAgentVersions)
				agents.PUT("/:id/binary", s.handlers.UpdateAgentBinary)
			}

			// Task management
			tasks := protected.Group("/tasks")
			{
				tasks.GET("", s.handlers.ListTasks)
				tasks.DELETE("/:id", s.handlers.CancelTask)

				// Direct task execution (Sprint 9)
				tasks.POST("/execute", s.handlers.ExecuteTaskDirect)
				tasks.GET("/:id/results", s.handlers.GetTaskResult)
				tasks.GET("/results", s.handlers.ListTaskResults)
			}

			// Auction management
			auctions := protected.Group("/auctions")
			{
				auctions.POST("/create", s.handlers.CreateAuction)
				auctions.POST("/:id/bid", s.handlers.SubmitBid)
			}

			// Payment channel management
			payments := protected.Group("/payments")
			{
				payments.POST("/channels/open", s.handlers.OpenPaymentChannel)
				payments.POST("/channels/:id/settle", s.handlers.SettlePaymentChannel)
			}

			// Reputation management
			reputation := protected.Group("/reputation")
			{
				reputation.GET("/:agent_id", s.handlers.GetAgentReputation)
				reputation.POST("/update", s.handlers.UpdateAgentReputation)
			}

			// Orchestrator monitoring
			orchestrator := protected.Group("/orchestrator")
			{
				orchestrator.GET("/metrics", s.handlers.GetOrchestratorMetrics)
				orchestrator.GET("/health", s.handlers.GetOrchestratorHealth)
				orchestrator.POST("/delegate", s.handlers.DelegateToMetaOrchestrator)
				orchestrator.GET("/status/:task_id", s.handlers.GetOrchestrationStatus)
			}

			// WebSocket real-time updates
			ws := protected.Group("/ws")
			{
				ws.GET("/connect", s.handlers.HandleWebSocket)
				ws.GET("/stats", s.handlers.GetWebSocketStats)
				ws.POST("/broadcast", s.handlers.BroadcastMessage)
				ws.POST("/send", s.handlers.SendUserMessage)
			}

			// Deployment management
			deployments := protected.Group("/deployments")
			{
				deployments.POST("", s.handlers.DeployAgent)
				deployments.GET("/:id", s.handlers.GetDeployment)
				deployments.GET("", s.handlers.ListUserDeployments)
				deployments.POST("/:id/stop", s.handlers.StopDeployment)
			}

			// Economic features - auctions, payment channels, reputation, meta-orchestrator, escrow, disputes
			economic := protected.Group("/economic")
			{
				// Auction management
				economic.POST("/auctions", s.handlers.CreateAuction)
				economic.POST("/auctions/:id/bids", s.handlers.SubmitBid)

				// Payment channel management
				economic.POST("/payment-channels", s.handlers.OpenPaymentChannel)
				economic.POST("/payment-channels/:id/settle", s.handlers.SettlePaymentChannel)

				// Reputation management
				economic.GET("/reputation/:agent_id", s.handlers.GetAgentReputation)
				economic.POST("/reputation", s.handlers.UpdateAgentReputation)

				// Meta-orchestrator
				economic.POST("/meta-orchestrator/delegate", s.handlers.DelegateToMetaOrchestrator)
				economic.GET("/meta-orchestrator/status/:task_id", s.handlers.GetOrchestrationStatus)

				// Escrow management
				economic.POST("/escrows", s.handlers.CreateEscrow)
				economic.GET("/escrows/:id", s.handlers.GetEscrow)
				economic.POST("/escrows/:id/fund", s.handlers.FundEscrow)
				economic.POST("/escrows/:id/release", s.handlers.ReleaseEscrow)
				economic.POST("/escrows/:id/refund", s.handlers.RefundEscrow)

				// Dispute resolution
				economic.POST("/escrows/:id/dispute", s.handlers.OpenDispute)
				economic.GET("/disputes/:id", s.handlers.GetDispute)
				economic.POST("/disputes/:id/evidence", s.handlers.SubmitEvidence)
				economic.POST("/disputes/:id/resolve", s.handlers.ResolveDispute)

				// Economic task execution (Sprint 9)
				economic.POST("/tasks/execute", s.handlers.ExecuteEconomicTask)
				economic.GET("/tasks/:id/result", s.handlers.GetEconomicTaskResult)
				economic.GET("/health", s.handlers.EconomicHealthCheck)
			}

		// Analytics and monitoring
		analytics := protected.Group("/analytics")
		{
			analytics.GET("/escrow", s.handlers.GetEscrowMetrics)
			analytics.GET("/auctions", s.handlers.GetAuctionMetrics)
			analytics.GET("/payment-channels", s.handlers.GetPaymentChannelMetrics)
			analytics.GET("/reputation", s.handlers.GetReputationMetrics)
			analytics.GET("/delegations", s.handlers.GetDelegationMetrics)
			analytics.GET("/disputes", s.handlers.GetDisputeMetrics)
			analytics.GET("/economic-health", s.handlers.GetEconomicHealthMetrics)
			analytics.GET("/time-series", s.handlers.GetTimeSeriesData)
			analytics.GET("/anomalies", s.handlers.DetectAnomalies)
			analytics.GET("/dashboard", s.handlers.GetAnalyticsDashboard)
		}
		}
	}
}

// Start starts the API server
func (s *Server) Start() error {
	addr := s.server.Addr

	s.logger.Info("starting API server",
		zap.String("address", addr),
		zap.Bool("tls", s.config.TLSCertFile != ""),
		zap.Bool("metrics", s.config.EnableMetrics),
		zap.Bool("tracing", s.config.EnableTracing),
	)

	// Start server
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		return s.server.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile)
	}

	return s.server.ListenAndServe()
}

// Stop gracefully stops the API server
func (s *Server) Stop() error {
	s.logger.Info("stopping API server")

	// Cancel context
	s.cancel()

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// handleHealth handles health check requests (liveness probe)
// Returns 200 OK if the service is alive, even if degraded
func (s *Server) handleHealth(c *gin.Context) {
	checks := make(map[string]interface{})
	overallStatus := "healthy"

	// Check if handlers are initialized
	if s.handlers == nil {
		overallStatus = "unhealthy"
		checks["handlers"] = map[string]interface{}{
			"status":  "unhealthy",
			"message": "handlers not initialized",
		}
	} else {
		checks["handlers"] = map[string]interface{}{
			"status":  "healthy",
			"message": "handlers initialized",
		}
	}

	// Check database connection if available
	if s.handlers != nil && s.handlers.db != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := s.handlers.db.Conn().PingContext(ctx); err != nil {
			overallStatus = "degraded"
			checks["database"] = map[string]interface{}{
				"status":  "degraded",
				"message": fmt.Sprintf("database ping failed: %v", err),
			}
		} else {
			checks["database"] = map[string]interface{}{
				"status":  "healthy",
				"message": "database connection OK",
			}
		}
	}

	// Check orchestrator if available
	if s.handlers != nil && s.handlers.orchestrator != nil {
		metrics := s.handlers.orchestrator.GetMetrics()
		checks["orchestrator"] = map[string]interface{}{
			"status":         "healthy",
			"workers_active": metrics.ActiveWorkers,
			"tasks_total":    metrics.TasksProcessed,
			"tasks_succeeded": metrics.TasksSucceeded,
			"tasks_failed":   metrics.TasksFailed,
		}
	}

	// Return appropriate status code
	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"status":  overallStatus,
		"service": "zerostate-api",
		"version": "0.1.0",
		"time":    time.Now().UTC(),
		"checks":  checks,
	})
}

// handleReady handles readiness check requests (readiness probe)
// Returns 200 OK only if all critical dependencies are ready
func (s *Server) handleReady(c *gin.Context) {
	checks := make(map[string]interface{})
	ready := true

	// Check if handlers are initialized
	if s.handlers == nil {
		ready = false
		checks["handlers"] = map[string]interface{}{
			"status":  "not_ready",
			"message": "handlers not initialized",
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "handlers not initialized",
			"checks": checks,
		})
		return
	}

	checks["handlers"] = map[string]interface{}{
		"status":  "ready",
		"message": "handlers initialized",
	}

	// Check database connection (critical dependency)
	if s.handlers.db != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := s.handlers.db.Conn().PingContext(ctx); err != nil {
			ready = false
			checks["database"] = map[string]interface{}{
				"status":  "not_ready",
				"message": fmt.Sprintf("database connection failed: %v", err),
			}
		} else {
			checks["database"] = map[string]interface{}{
				"status":  "ready",
				"message": "database connection OK",
			}
		}
	}

	// Check orchestrator (critical dependency)
	if s.handlers.orchestrator != nil {
		metrics := s.handlers.orchestrator.GetMetrics()
		if metrics.ActiveWorkers == 0 {
			ready = false
			checks["orchestrator"] = map[string]interface{}{
				"status":  "not_ready",
				"message": "no workers active",
			}
		} else {
			checks["orchestrator"] = map[string]interface{}{
				"status":         "ready",
				"workers_active": metrics.ActiveWorkers,
				"tasks_processed": metrics.TasksProcessed,
			}
		}
	}

	// Check WebSocket hub (non-critical)
	if s.handlers.wsHub != nil {
		checks["websocket"] = map[string]interface{}{
			"status":  "ready",
			"message": "WebSocket hub active",
		}
	}

	// Return appropriate response
	if ready {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"service": "zerostate-api",
			"time":    time.Now().UTC(),
			"checks":  checks,
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not ready",
			"service": "zerostate-api",
			"time":    time.Now().UTC(),
			"checks":  checks,
		})
	}
}

// Address returns the server's listening address
func (s *Server) Address() string {
	return s.server.Addr
}

// Context returns the server's context
func (s *Server) Context() context.Context {
	return s.ctx
}

// Router returns the server's Gin router (for testing)
func (s *Server) Router() *gin.Engine {
	return s.router
}
