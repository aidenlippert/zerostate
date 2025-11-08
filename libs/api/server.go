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

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware(logger))

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
			}
		}

		// Protected routes - require authentication
		protected := v1.Group("")
		protected.Use(authMiddleware())
		{
			// Agent management
			agents := protected.Group("/agents")
			{
				agents.POST("/register", s.handlers.RegisterAgent)
				agents.GET("/:id", s.handlers.GetAgent)
				agents.GET("", s.handlers.ListAgents)
				agents.PUT("/:id", s.handlers.UpdateAgent)
				agents.DELETE("/:id", s.handlers.DeleteAgent)
				agents.GET("/search", s.handlers.SearchAgents)
			}

			// Task management
			tasks := protected.Group("/tasks")
			{
				tasks.POST("/submit", s.handlers.SubmitTask)
				tasks.GET("/:id", s.handlers.GetTask)
				tasks.GET("", s.handlers.ListTasks)
				tasks.DELETE("/:id", s.handlers.CancelTask)
				tasks.GET("/:id/status", s.handlers.GetTaskStatus)
				tasks.GET("/:id/result", s.handlers.GetTaskResult)
			}

			// Orchestrator monitoring
			orchestrator := protected.Group("/orchestrator")
			{
				orchestrator.GET("/metrics", s.handlers.GetOrchestratorMetrics)
				orchestrator.GET("/health", s.handlers.GetOrchestratorHealth)
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

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "zerostate-api",
		"version": "0.1.0",
		"time":    time.Now().UTC(),
	})
}

// handleReady handles readiness check requests
func (s *Server) handleReady(c *gin.Context) {
	// Check if handlers are initialized
	if s.handlers == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "handlers not initialized",
		})
		return
	}

	// Additional readiness checks can be added here
	// (database connection, external services, etc.)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ready",
		"service": "zerostate-api",
		"time":    time.Now().UTC(),
	})
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
