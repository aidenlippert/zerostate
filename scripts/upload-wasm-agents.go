package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/aidenlippert/zerostate/libs/storage"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	fmt.Println("📤 Uploading WASM Agents to R2")
	fmt.Println("================================")
	fmt.Println("")

	// Create R2 client
	r2, err := storage.NewR2StorageFromEnv()
	if err != nil {
		log.Fatalf("❌ Failed to create R2 client: %v\n   Make sure R2 environment variables are set", err)
	}

	fmt.Println("✅ R2 client created")
	fmt.Printf("   Endpoint: %s\n", os.Getenv("R2_ENDPOINT"))
	fmt.Printf("   Bucket: %s\n", os.Getenv("R2_BUCKET_NAME"))
	fmt.Println("")

	ctx := context.Background()

	// Define agents to upload
	agents := []struct {
		localPath string
		r2Key     string
		name      string
	}{
		{
			localPath: "agents/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent_rust.wasm",
			r2Key:     "agents/math-agent-v1.0.wasm",
			name:      "Math Agent v1.0",
		},
		// Add more agents here as you build them
	}

	successCount := 0
	totalSize := int64(0)

	for i, agent := range agents {
		fmt.Printf("Agent %d/%d: %s\n", i+1, len(agents), agent.name)
		fmt.Printf("  Local: %s\n", agent.localPath)
		fmt.Printf("  R2 Key: %s\n", agent.r2Key)

		// Check if file exists locally
		if _, err := os.Stat(agent.localPath); os.IsNotExist(err) {
			fmt.Printf("  ⚠️  File not found locally, skipping...\n")
			fmt.Println("")
			continue
		}

		// Read WASM binary
		wasmData, err := ioutil.ReadFile(agent.localPath)
		if err != nil {
			fmt.Printf("  ❌ Failed to read file: %v\n", err)
			fmt.Println("")
			continue
		}

		fmt.Printf("  📦 Size: %.2f KB (%d bytes)\n", float64(len(wasmData))/1024.0, len(wasmData))

		// Upload to R2
		err = r2.UploadWASM(ctx, agent.r2Key, wasmData)
		if err != nil {
			fmt.Printf("  ❌ Upload failed: %v\n", err)
			fmt.Println("")
			continue
		}

		fmt.Println("  ✅ Uploaded successfully!")

		// Verify upload
		exists, err := r2.Exists(ctx, agent.r2Key)
		if err != nil || !exists {
			fmt.Printf("  ⚠️  Verification failed: exists=%v, err=%v\n", exists, err)
		} else {
			fmt.Println("  ✅ Verified in R2")
		}

		successCount++
		totalSize += int64(len(wasmData))
		fmt.Println("")
	}

	// Summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 Upload Summary:")
	fmt.Printf("   Uploaded: %d/%d agents\n", successCount, len(agents))
	fmt.Printf("   Total Size: %.2f KB\n", float64(totalSize)/1024.0)
	fmt.Println("")

	if successCount > 0 {
		fmt.Println("✅ Upload complete!")
		fmt.Println("")
		fmt.Println("📚 Next Steps:")
		fmt.Println("   1. Test execution: go run scripts/test-wasm-execution.go")
		fmt.Println("   2. List uploaded agents: go run scripts/list-r2-agents.go")
		fmt.Println("   3. Test with orchestrator: go run scripts/test-e2e-workflow.go")
	} else {
		fmt.Println("⚠️  No agents uploaded")
		fmt.Println("")
		fmt.Println("💡 Build the math agent first:")
		fmt.Println("   cd agents/math-agent-rust")
		fmt.Println("   cargo build --target wasm32-unknown-unknown --release")
	}
}
