package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"time"

	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	"github.com/aidenlippert/zerostate/reference-runtime-v1/internal/agent"
	"github.com/aidenlippert/zerostate/reference-runtime-v1/internal/health"
	"github.com/aidenlippert/zerostate/reference-runtime-v1/internal/presence"
	"github.com/aidenlippert/zerostate/reference-runtime-v1/internal/server"
	"github.com/aidenlippert/zerostate/reference-runtime-v1/internal/task"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// RuntimeConfig represents the full runtime configuration
type RuntimeConfig struct {
	Agent struct {
		DID     string `yaml:"did"`
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
		Runtime struct {
			Type string `yaml:"type"`
			Path string `yaml:"path"`
		} `yaml:"runtime"`
		Capabilities []string `yaml:"capabilities"`
		Limits       struct {
			MaxMemoryMB        int32 `yaml:"max_memory_mb"`
			MaxExecutionTimeMS int32 `yaml:"max_execution_time_ms"`
			MaxConcurrentTasks int32 `yaml:"max_concurrent_tasks"`
		} `yaml:"limits"`
	} `yaml:"agent"`

	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`

	P2P struct {
		Enabled           bool     `yaml:"enabled"`
		Bootstrap         []string `yaml:"bootstrap"`
		PresenceTopic     string   `yaml:"presence_topic"`
		HeartbeatInterval int      `yaml:"heartbeat_interval"`
	} `yaml:"p2p"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
}

func main() {
	// Parse flags
	configPath := flag.String("agent-config", "testdata/math-agent.yaml", "Path to agent configuration file")
	flag.Parse()

	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting reference-runtime-v1",
		zap.String("config", *configPath),
	)

	// Load configuration
	config, err := loadConfig(*configPath)
	if err != nil {
		logger.Fatal("Failed to load configuration",
			zap.String("path", *configPath),
			zap.Error(err),
		)
	}

	logger.Info("Configuration loaded",
		zap.String("agent_did", config.Agent.DID),
		zap.String("agent_name", config.Agent.Name),
		zap.Int("server_port", config.Server.Port),
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize services
	agentService := agent.NewService(&agent.Config{
		DID:          config.Agent.DID,
		Name:         config.Agent.Name,
		Version:      config.Agent.Version,
		Capabilities: config.Agent.Capabilities,
		RuntimeInfo: &agent.RuntimeInfo{
			Type:    config.Agent.Runtime.Type,
			Version: "1.0.0",
			Metadata: map[string]string{
				"wasm_path": config.Agent.Runtime.Path,
			},
		},
		Limits: &agent.ResourceLimits{
			MaxMemoryMB:        config.Agent.Limits.MaxMemoryMB,
			MaxExecutionTimeMS: config.Agent.Limits.MaxExecutionTimeMS,
			MaxConcurrentTasks: config.Agent.Limits.MaxConcurrentTasks,
		},
	}, logger)

	healthService := health.NewService(logger)

	taskService, err := task.NewService(config.Agent.Runtime.Path, logger)
	if err != nil {
		logger.Fatal("Failed to create task service",
			zap.Error(err),
		)
	}

	// Create gRPC server
	grpcServer, err := server.NewServer(
		&server.Config{
			Host: config.Server.Host,
			Port: config.Server.Port,
		},
		agentService,
		taskService,
		healthService,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to create gRPC server",
			zap.Error(err),
		)
	}

	// Start server in goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		if err := grpcServer.Start(); err != nil {
			serverErrCh <- err
		}
	}()

	logger.Info("Runtime is now serving",
		zap.String("address", grpcServer.Address()),
		zap.String("did", config.Agent.DID),
	)

	// Generate AgentCard for this runtime
	var agentCard *agentcard.AgentCard
	var signingKey ed25519.PrivateKey
	if config.P2P.Enabled {
		logger.Info("Generating AgentCard-VC-v1...")

		// Generate keypair for signing
		publicKey, privateKey, err := agentcard.GenerateKeyPair()
		if err != nil {
			logger.Fatal("Failed to generate keypair", zap.Error(err))
		}
		signingKey = privateKey

		// Build capabilities from config
		operations := make([]agentcard.Operation, 0, len(config.Agent.Capabilities))
		for _, cap := range config.Agent.Capabilities {
			operations = append(operations, agentcard.Operation{
				Name:        cap,
				Category:    "computation",
				GasEstimate: 100,
			})
		}

		capabilities := agentcard.Capabilities{
			Domains:    []string{"mathematics", "computation"},
			Operations: operations,
			Constraints: agentcard.CapabilityConstraints{
				MaxInputSize:       uint64(config.Agent.Limits.MaxMemoryMB) * 1024 * 1024,
				MaxExecutionTimeMs: uint64(config.Agent.Limits.MaxExecutionTimeMS),
			},
			Interfaces: []string{"grpc", "p2p"},
		}

		runtime := agentcard.RuntimeInfo{
			Protocol:       "ari-v1",
			Implementation: "reference-runtime-v1",
			Version:        config.Agent.Version,
			WasmEngine:     "wasmtime",
			WasmVersion:    "14.0.0",
			ModuleHash:     fmt.Sprintf("file://%s", config.Agent.Runtime.Path),
			ExecutionEnvironment: agentcard.ExecutionEnvironment{
				MemoryLimitMB:     uint32(config.Agent.Limits.MaxMemoryMB),
				CPUQuotaMs:        1000,
				NetworkEnabled:    true,
				FilesystemEnabled: false,
			},
			Endpoints: []agentcard.Endpoint{
				{
					Protocol: "grpc",
					Address:  grpcServer.Address(),
				},
			},
		}

		// Will be populated after p2p host is created
		network := agentcard.Network{
			P2P: agentcard.P2PConfig{
				PeerID:          "",
				ListenAddresses: []string{},
				Protocols:       []string{"/ainur/gossipsub/1.0.0"},
			},
			Discovery: agentcard.Discovery{
				Methods:        []string{"mdns", "bootstrap"},
				BootstrapNodes: config.P2P.Bootstrap,
			},
			Availability: agentcard.Availability{
				Regions: []string{"local"},
				LatencyTargets: agentcard.LatencyTargets{
					P50Ms: 50,
					P95Ms: 100,
					P99Ms: 200,
				},
			},
		}

		agentDID := agentcard.DID(config.Agent.DID)
		agentCard, err = agentcard.NewAgentCardBuilder().
			SetAgentDID(agentDID).
			SetName(config.Agent.Name).
			SetDescription(fmt.Sprintf("Runtime agent with capabilities: %v", config.Agent.Capabilities)).
			SetVersion(config.Agent.Version).
			SetCapabilities(capabilities).
			SetRuntime(runtime).
			SetNetwork(network).
			SetExpirationDays(365).
			Build()

		if err != nil {
			logger.Fatal("Failed to build AgentCard", zap.Error(err))
		}

		// Sign the AgentCard
		if err := agentCard.Sign(signingKey); err != nil {
			logger.Fatal("Failed to sign AgentCard", zap.Error(err))
		}

		// Verify the signature
		valid, err := agentCard.Verify(publicKey)
		if err != nil {
			logger.Fatal("Failed to verify AgentCard signature", zap.Error(err))
		}
		if !valid {
			logger.Fatal("AgentCard signature verification failed")
		}

		cardHash, _ := agentCard.Hash()
		logger.Info("AgentCard generated and signed",
			zap.String("card_id", agentCard.ID),
			zap.String("hash", cardHash),
			zap.String("issuer", agentCard.Issuer.String()),
			zap.Int("capabilities", len(capabilities.Operations)),
			zap.Bool("signature_valid", valid),
		)
	}

	// Start P2P presence announcements
	var presenceService *presence.Service
	if config.P2P.Enabled {
		// Create libp2p host
		p2pHost, err := libp2p.New(
			libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		)
		if err != nil {
			logger.Error("Failed to create libp2p host", zap.Error(err))
		} else {
			// Setup mDNS discovery for local peer discovery
			mdnsService := discovery.NewMdnsService(p2pHost, "ainur-runtime", &discoveryNotifee{host: p2pHost, logger: logger})
			if err := mdnsService.Start(); err != nil {
				logger.Warn("Failed to start mDNS discovery", zap.Error(err))
			} else {
				logger.Info("mDNS peer discovery enabled")
			}

			// Update AgentCard with P2P network information
			if agentCard != nil {
				agentCard.CredentialSubject.Network.P2P.PeerID = p2pHost.ID().String()
				p2pAddrs := make([]string, 0, len(p2pHost.Addrs()))
				for _, addr := range p2pHost.Addrs() {
					fullAddr := fmt.Sprintf("%s/p2p/%s", addr.String(), p2pHost.ID().String())
					p2pAddrs = append(p2pAddrs, fullAddr)
				}
				agentCard.CredentialSubject.Network.P2P.ListenAddresses = p2pAddrs
				agentCard.CredentialSubject.Network.P2P.AnnounceAddresses = p2pAddrs

				logger.Info("AgentCard updated with P2P network info",
					zap.String("peer_id", p2pHost.ID().String()),
					zap.Int("addresses", len(p2pAddrs)),
				)
			}

			// Create presence service
			presenceConfig := &presence.Config{
				AgentDID:          config.Agent.DID,
				AgentName:         config.Agent.Name,
				Capabilities:      config.Agent.Capabilities,
				GRPCAddress:       grpcServer.Address(),
				HeartbeatInterval: time.Duration(config.P2P.HeartbeatInterval) * time.Second,
				PresenceTopic:     config.P2P.PresenceTopic,
				AgentCard:         agentCard, // Include AgentCard in presence messages
			}

			presenceService, err = presence.NewService(ctx, p2pHost, presenceConfig, logger)
			if err != nil {
				logger.Error("Failed to create presence service", zap.Error(err))
			} else {
				if err := presenceService.Start(); err != nil {
					logger.Error("Failed to start presence service", zap.Error(err))
				} else {
					logger.Info("P2P presence service started",
						zap.String("topic", config.P2P.PresenceTopic),
						zap.Int("heartbeat_seconds", config.P2P.HeartbeatInterval),
						zap.String("peer_id", p2pHost.ID().String()),
					)
				}
			}
		}
	}

	// Wait for interrupt signal or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		logger.Info("Received interrupt signal, shutting down gracefully")
	case err := <-serverErrCh:
		logger.Error("Server error", zap.Error(err))
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down")
	}

	// Graceful shutdown
	if presenceService != nil {
		if err := presenceService.Stop(); err != nil {
			logger.Error("Failed to stop presence service", zap.Error(err))
		}
	}
	grpcServer.Stop()
	logger.Info("Runtime stopped successfully")
}

// loadConfig loads the runtime configuration from a YAML file
func loadConfig(path string) (*RuntimeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config RuntimeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// initLogger initializes the zap logger
func initLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)

	return config.Build()
}

// discoveryNotifee handles mDNS peer discoveries
type discoveryNotifee struct {
	host   host.Host
	logger *zap.Logger
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.logger.Info("discovered peer via mDNS",
		zap.String("peer_id", pi.ID.String()),
		zap.Int("addrs", len(pi.Addrs)),
	)
	// Connect to the discovered peer
	if err := n.host.Connect(context.Background(), pi); err != nil {
		n.logger.Debug("failed to connect to discovered peer", zap.Error(err))
	} else {
		n.logger.Info("connected to discovered peer", zap.String("peer_id", pi.ID.String()))
	}
}
