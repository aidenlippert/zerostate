package server

import (
	"fmt"
	"net"

	"github.com/aidenlippert/zerostate/reference-runtime-v1/internal/agent"
	"github.com/aidenlippert/zerostate/reference-runtime-v1/internal/health"
	"github.com/aidenlippert/zerostate/reference-runtime-v1/internal/task"
	ariv1 "github.com/aidenlippert/zerostate/reference-runtime-v1/pkg/ari/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Server represents the gRPC server
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	logger     *zap.Logger

	// Services
	agentService  *agent.Service
	taskService   *task.Service
	healthService *health.Service
}

// Config contains server configuration
type Config struct {
	Host string
	Port int
}

// NewServer creates a new gRPC server
func NewServer(
	config *Config,
	agentService *agent.Service,
	taskService *task.Service,
	healthService *health.Service,
	logger *zap.Logger,
) (*Server, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create listener
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(10*1024*1024), // 10 MB
		grpc.MaxSendMsgSize(10*1024*1024), // 10 MB
	)

	// Register services
	ariv1.RegisterAgentServer(grpcServer, agentService)
	ariv1.RegisterTaskServer(grpcServer, taskService)
	ariv1.RegisterHealthServer(grpcServer, healthService)

	logger.Info("gRPC server created",
		zap.String("address", addr),
	)

	return &Server{
		grpcServer:    grpcServer,
		listener:      listener,
		logger:        logger,
		agentService:  agentService,
		taskService:   taskService,
		healthService: healthService,
	}, nil
}

// Start starts the gRPC server
func (s *Server) Start() error {
	s.logger.Info("Starting gRPC server",
		zap.String("address", s.listener.Addr().String()),
	)

	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the server
func (s *Server) Stop() {
	s.logger.Info("Stopping gRPC server")
	s.grpcServer.GracefulStop()
}

// Address returns the server address
func (s *Server) Address() string {
	return s.listener.Addr().String()
}
