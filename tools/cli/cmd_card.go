package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"zerostate/libs/identity"
	"zerostate/libs/p2p"
)

var cardCmd = &cobra.Command{
	Use:   "card",
	Short: "Manage Agent Cards",
	Long:  "Create, publish, resolve, and manage Agent Cards in the DHT",
}

var cardCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Agent Card",
	Long:  "Generate a new DID and create a signed Agent Card",
	RunE:  runCardCreate,
}

var cardPublishCmd = &cobra.Command{
	Use:   "publish [card-file]",
	Short: "Publish an Agent Card to the DHT",
	Args:  cobra.ExactArgs(1),
	RunE:  runCardPublish,
}

var cardResolveCmd = &cobra.Command{
	Use:   "resolve [cid]",
	Short: "Resolve an Agent Card from the DHT",
	Args:  cobra.ExactArgs(1),
	RunE:  runCardResolve,
}

var cardVerifyCmd = &cobra.Command{
	Use:   "verify [card-file]",
	Short: "Verify an Agent Card signature",
	Args:  cobra.ExactArgs(1),
	RunE:  runCardVerify,
}

func init() {
	rootCmd.AddCommand(cardCmd)
	cardCmd.AddCommand(cardCreateCmd)
	cardCmd.AddCommand(cardPublishCmd)
	cardCmd.AddCommand(cardResolveCmd)
	cardCmd.AddCommand(cardVerifyCmd)
	
	// Create flags
	cardCreateCmd.Flags().String("output", "agent-card.json", "Output file path")
	cardCreateCmd.Flags().String("name", "", "Agent name (optional)")
	cardCreateCmd.Flags().StringSlice("capabilities", []string{}, "Capabilities (comma-separated)")
	cardCreateCmd.Flags().StringSlice("endpoints", []string{}, "Libp2p endpoints")
}

func runCardCreate(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")
	capabilities, _ := cmd.Flags().GetStringSlice("capabilities")
	endpoints, _ := cmd.Flags().GetStringSlice("endpoints")
	
	// Create signer
	signer, err := identity.NewSigner(nil)
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}
	
	// Build capabilities
	caps := make([]identity.Capability, 0, len(capabilities))
	for _, capName := range capabilities {
		caps = append(caps, identity.Capability{
			Name:    capName,
			Version: "1.0",
		})
	}
	
	// Build endpoints
	endpointsList := endpoints
	if len(endpointsList) == 0 {
		endpointsList = []string{"/ip4/127.0.0.1/tcp/4001/p2p/" + signer.DID()}
	}
	
	// Create card
	card := &identity.AgentCard{
		DID:          signer.DID(),
		Capabilities: caps,
		Endpoints: &identity.Endpoints{
			Libp2p: endpointsList,
		},
	}
	
	// Sign card
	if err := signer.SignCard(card); err != nil {
		return fmt.Errorf("failed to sign card: %w", err)
	}
	
	// Write to file
	cardJSON, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal card: %w", err)
	}
	
	if err := os.WriteFile(output, cardJSON, 0644); err != nil {
		return fmt.Errorf("failed to write card: %w", err)
	}
	
	fmt.Printf("✅ Agent Card created successfully!\n")
	fmt.Printf("   DID: %s\n", signer.DID())
	fmt.Printf("   File: %s\n", output)
	fmt.Printf("   Capabilities: %d\n", len(caps))
	
	return nil
}

func runCardPublish(cmd *cobra.Command, args []string) error {
	cardFile := args[0]
	
	// Read card
	cardJSON, err := os.ReadFile(cardFile)
	if err != nil {
		return fmt.Errorf("failed to read card file: %w", err)
	}
	
	// Parse and verify card
	var card identity.AgentCard
	if err := json.Unmarshal(cardJSON, &card); err != nil {
		return fmt.Errorf("failed to parse card: %w", err)
	}
	
	if err := identity.VerifyCard(&card); err != nil {
		return fmt.Errorf("invalid card signature: %w", err)
	}
	
	// Create P2P node
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
		return fmt.Errorf("failed to create P2P node: %w", err)
	}
	defer node.Close()
	
	// Bootstrap if needed
	if bootstrapAddr != "" {
		if err := node.Bootstrap(ctx); err != nil {
			return fmt.Errorf("bootstrap failed: %w", err)
		}
		fmt.Println("⏳ Bootstrapped to DHT, waiting for peers...")
		
		// Wait for at least 1 peer
		if err := node.WaitForPeers(ctx, 1, 10*time.Second); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}
		time.Sleep(2 * time.Second) // Give DHT time to stabilize
	}
	
	// Publish card
	fmt.Println("📤 Publishing Agent Card to DHT...")
	cid, err := node.PublishAgentCard(ctx, cardJSON)
	if err != nil {
		return fmt.Errorf("failed to publish card: %w", err)
	}
	
	fmt.Printf("✅ Agent Card published successfully!\n")
	fmt.Printf("   CID: %s\n", cid)
	fmt.Printf("   DID: %s\n", card.DID)
	
	return nil
}

func runCardResolve(cmd *cobra.Command, args []string) error {
	cidStr := args[0]
	
	// Create P2P node
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
		return fmt.Errorf("failed to create P2P node: %w", err)
	}
	defer node.Close()
	
	// Bootstrap if needed
	if bootstrapAddr != "" {
		if err := node.Bootstrap(ctx); err != nil {
			return fmt.Errorf("bootstrap failed: %w", err)
		}
		fmt.Println("⏳ Bootstrapped to DHT...")
	}
	
	// Resolve card
	fmt.Printf("🔍 Resolving Agent Card %s...\n", cidStr)
	cardJSON, err := node.ResolveAgentCard(ctx, cidStr)
	if err != nil {
		return fmt.Errorf("failed to resolve card: %w", err)
	}
	
	// Parse and verify
	var card identity.AgentCard
	if err := json.Unmarshal(cardJSON, &card); err != nil {
		return fmt.Errorf("failed to parse card: %w", err)
	}
	
	if err := identity.VerifyCard(&card); err != nil {
		fmt.Println("⚠️  WARNING: Card signature verification failed!")
	} else {
		fmt.Println("✅ Signature verified")
	}
	
	// Pretty print
	prettyJSON, _ := json.MarshalIndent(&card, "", "  ")
	fmt.Println("\n" + string(prettyJSON))
	
	return nil
}

func runCardVerify(cmd *cobra.Command, args []string) error {
	cardFile := args[0]
	
	// Read card
	cardJSON, err := os.ReadFile(cardFile)
	if err != nil {
		return fmt.Errorf("failed to read card file: %w", err)
	}
	
	// Parse card
	var card identity.AgentCard
	if err := json.Unmarshal(cardJSON, &card); err != nil {
		return fmt.Errorf("failed to parse card: %w", err)
	}
	
	// Verify
	if err := identity.VerifyCard(&card); err != nil {
		fmt.Printf("❌ Signature verification FAILED: %v\n", err)
		return err
	}
	
	fmt.Println("✅ Signature verified successfully!")
	fmt.Printf("   DID: %s\n", card.DID)
	if card.Proof != nil {
		fmt.Printf("   Proof Type: %s\n", card.Proof.Type)
		fmt.Printf("   Created: %s\n", card.Proof.Created)
	}
	
	return nil
}
