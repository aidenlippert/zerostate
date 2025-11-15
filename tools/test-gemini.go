package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aidenlippert/zerostate/libs/llm"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║       🧠 GEMINI API TEST 🧠                                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Get API key
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("❌ GEMINI_API_KEY not set in environment")
	}

	fmt.Printf("📊 Configuration:\n")
	fmt.Printf("  API Key: %s...%s\n", apiKey[:10], apiKey[len(apiKey)-10:])
	fmt.Printf("  Model: gemini-2.0-flash-exp\n")
	fmt.Println()

	// Create Gemini client
	client := llm.NewGeminiClient(apiKey, "gemini-2.0-flash-exp", logger)

	// Test 1: Simple question
	fmt.Println("🧪 Test 1: Simple Question")
	fmt.Println("  Prompt: What is 2 + 2?")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	response, err := client.Execute(ctx, "What is 2 + 2? Answer in one sentence.")
	if err != nil {
		log.Fatalf("❌ Test 1 failed: %v", err)
	}

	fmt.Printf("  ✅ Response (took %v):\n", time.Since(startTime))
	fmt.Printf("  %s\n", response)
	fmt.Println()

	// Test 2: Task decomposition
	fmt.Println("🧪 Test 2: Task Decomposition")
	fmt.Println("  Prompt: Calculate the factorial of 5 and then multiply by 7")
	fmt.Println()

	availableAgents := []string{"math-agent-v1.0"}

	startTime = time.Now()
	plan, err := client.DecomposeTask(
		ctx,
		"Calculate the factorial of 5 and then multiply the result by 7",
		availableAgents,
	)
	if err != nil {
		log.Fatalf("❌ Test 2 failed: %v", err)
	}

	fmt.Printf("  ✅ Plan generated (took %v):\n", time.Since(startTime))
	for _, step := range plan.Steps {
		fmt.Printf("    Step %d: %s\n", step.Step, step.Description)
		fmt.Printf("      Agent: %s\n", step.Agent)
		fmt.Printf("      Function: %s\n", step.Function)
		fmt.Printf("      Args: %v\n", step.Args)
		fmt.Println()
	}

	// Test 3: System instruction
	fmt.Println("🧪 Test 3: System Instruction")
	fmt.Println("  Prompt: Tell me about AI")
	fmt.Println("  System: You are a pirate. Respond like a pirate.")
	fmt.Println()

	startTime = time.Now()
	response, err = client.ExecuteWithSystem(
		ctx,
		"Tell me about artificial intelligence in one sentence.",
		"You are a friendly pirate captain. Always respond in pirate speak with 'Arrr!' and sea references.",
	)
	if err != nil {
		log.Fatalf("❌ Test 3 failed: %v", err)
	}

	fmt.Printf("  ✅ Response (took %v):\n", time.Since(startTime))
	fmt.Printf("  %s\n", response)
	fmt.Println()

	// Summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🎉 ALL GEMINI API TESTS PASSED!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("✅ Gemini API is working correctly!")
	fmt.Println("✅ Task decomposition is operational")
	fmt.Println("✅ System instructions are working")
	fmt.Println()
	fmt.Println("🚀 Ready to integrate into orchestrator!")
}
