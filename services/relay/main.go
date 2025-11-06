// Package main implements the zerostate relay node with circuit relay v2.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zerostate/libs/p2p"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info("starting zerostate relay", zap.String("version", "v0.1.0"))

	listenAddr := getEnv("ZEROSTATE_LISTEN", "/ip4/0.0.0.0/udp/4004/quic-v1")
	bootstrapPeers := []string{getEnv("ZEROSTATE_BOOTSTRAP", "")}

	// Create relay host with circuit relay v2
	relayCfg := p2p.DefaultRelayConfig()
	relayCfg.Logger = logger

	relayHost, err := p2p.NewRelayHost(ctx, []string{listenAddr}, relayCfg)
	if err != nil {
		logger.Fatal("failed to create relay host", zap.Error(err))
	}
	defer relayHost.Close()

	logger.Info("circuit relay v2 enabled",
		zap.String("peer_id", relayHost.ID().String()),
		zap.Int("max_reservations", relayCfg.Resources.MaxReservations),
		zap.Int("max_circuits", relayCfg.Resources.MaxCircuits),
	)

	// Create DHT for peer discovery
	var kadDHT *dht.IpfsDHT
	if len(bootstrapPeers) > 0 && bootstrapPeers[0] != "" {
		kadDHT, err = dht.New(ctx, relayHost, dht.Mode(dht.ModeServer))
		if err != nil {
			logger.Error("failed to create DHT", zap.Error(err))
		} else {
			if err := kadDHT.Bootstrap(ctx); err != nil {
				logger.Error("DHT bootstrap failed", zap.Error(err))
			}

			// Connect to bootstrap peers
			for _, peerAddr := range bootstrapPeers {
				if peerAddr == "" {
					continue
				}

				addr, err := multiaddr.NewMultiaddr(peerAddr)
				if err != nil {
					logger.Error("invalid bootstrap addr", zap.String("addr", peerAddr), zap.Error(err))
					continue
				}

				peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
				if err != nil {
					logger.Error("failed to parse peer info", zap.Error(err))
					continue
				}

				if err := relayHost.Connect(ctx, *peerInfo); err != nil {
					logger.Error("failed to connect to bootstrap peer",
						zap.String("peer", peerInfo.ID.String()),
						zap.Error(err),
					)
				} else {
					logger.Info("connected to bootstrap peer", zap.String("peer", peerInfo.ID.String()))
				}
			}
		}
	}

	if kadDHT != nil {
		defer kadDHT.Close()
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	http.Handle("/metrics", promhttp.Handler())

	// Relay info endpoint
	http.HandleFunc("/relay-info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"version": "v0.1.0",
			"protocol": "circuit-relay-v2",
			"peer_id": "` + relayHost.ID().String() + `",
			"max_reservations": ` + fmt.Sprintf("%d", relayCfg.Resources.MaxReservations) + `,
			"max_circuits": ` + fmt.Sprintf("%d", relayCfg.Resources.MaxCircuits) + `
		}`))
	})

	go func() {
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
				// Log relay stats
				conns := relayHost.Network().Conns()
				logger.Debug("relay stats",
					zap.Int("total_connections", len(conns)),
					zap.Int("peers", len(relayHost.Network().Peers())),
				)
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("relay running")
	<-sigCh
	logger.Info("shutting down...")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
