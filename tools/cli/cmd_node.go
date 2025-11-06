package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"zerostate/libs/p2p"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Node operations and information",
	Long:  "Get information about the local node and network",
}

var nodeInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show node information",
	RunE:  runNodeInfo,
}

var nodePeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "List connected peers",
	RunE:  runNodePeers,
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeInfoCmd)
	nodeCmd.AddCommand(nodePeersCmd)
}

func runNodeInfo(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	listenAddr := cmd.Flag("listen").Value.String()
	
	cfg := &p2p.Config{
		ListenAddrs: []string{listenAddr},
		EnableDHT:   true,
	}
	
	node, err := p2p.NewNode(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	defer node.Close()
	
	fmt.Println("🟢 Node Information")
	fmt.Println("==================")
	fmt.Printf("Peer ID: %s\n", node.Host().ID().String())
	fmt.Printf("Addresses:\n")
	for _, addr := range node.Host().Addrs() {
		fmt.Printf("  - %s/p2p/%s\n", addr.String(), node.Host().ID().String())
	}
	fmt.Printf("DHT: %s\n", "enabled")
	
	return nil
}

func runNodePeers(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	listenAddr := cmd.Flag("listen").Value.String()
	bootstrapAddr := cmd.Flag("bootstrap").Value.String()
	
	cfg := &p2p.Config{
		ListenAddrs: []string{listenAddr},
		EnableDHT:   true,
	}
	if bootstrapAddr != "" {
		cfg.BootstrapPeers = []string{bootstrapAddr}
	}
	
	node, err := p2p.NewNode(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	defer node.Close()
	
	if bootstrapAddr != "" {
		fmt.Println("⏳ Bootstrapping...")
		if err := node.Bootstrap(ctx); err != nil {
			return fmt.Errorf("bootstrap failed: %w", err)
		}
		time.Sleep(2 * time.Second) // Give time to connect
	}
	
	peers := node.Host().Network().Peers()
	
	fmt.Printf("🌐 Connected Peers (%d)\n", len(peers))
	fmt.Println("===================")
	
	if len(peers) == 0 {
		fmt.Println("No peers connected. Try using --bootstrap flag.")
		return nil
	}
	
	for i, peerID := range peers {
		peerInfo := node.Host().Peerstore().PeerInfo(peerID)
		fmt.Printf("%d. %s\n", i+1, peerID.String())
		if len(peerInfo.Addrs) > 0 {
			fmt.Printf("   Address: %s\n", peerInfo.Addrs[0].String())
		}
	}
	
	return nil
}
