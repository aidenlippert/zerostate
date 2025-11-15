package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

func main() {
	fmt.Println("🚀 Ainur Genesis Bootstrap Node - Starting...")

	// Create libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/4001",      // TCP
			"/ip4/0.0.0.0/udp/4001/quic", // QUIC
		),
	)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Print our multiaddresses (this is what we'll publish!)
	fmt.Println("\n✅ Bootstrap Node Online!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Peer ID: %s\n", h.ID())
	fmt.Println("\n📡 Multiaddresses (add these to runtime configs):")
	for _, addr := range h.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr, h.ID())
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Setup mDNS for local discovery
	mdnsSvc := discovery.NewMdnsService(h, "ainur-runtime", &notifee{h: h})
	if err := mdnsSvc.Start(); err != nil {
		fmt.Printf("⚠️  mDNS warning: %v\n", err)
	} else {
		fmt.Println("✅ mDNS local discovery enabled")
	}

	// Create gossipsub (for L3 Aether message relay)
	ctx := context.Background()
	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
		pubsub.WithFloodPublish(true),
		pubsub.WithPeerExchange(true),
	)
	if err != nil {
		panic(err)
	}

	// Join the presence topic (but don't publish - we're just a relay)
	topic, err := ps.Join("ainur/v1/global/l3_aether/presence")
	if err != nil {
		panic(err)
	}

	// Subscribe to relay messages
	_, err = topic.Subscribe()
	if err != nil {
		panic(err)
	}

	fmt.Println("✅ Subscribed to L3 Aether presence topic")
	fmt.Println("✅ Ready to relay messages between runtimes\n")
	fmt.Println("Press Ctrl+C to stop...")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n👋 Shutting down gracefully...")
}

// notifee handles mDNS peer discoveries
type notifee struct {
	h host.Host
}

func (n *notifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("🔍 Discovered peer: %s\n", pi.ID)
	if err := n.h.Connect(context.Background(), pi); err != nil {
		fmt.Printf("   Failed to connect: %v\n", err)
	} else {
		fmt.Printf("   ✅ Connected!\n")
	}
}
