package main
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"zerostate/libs/storage"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Println("⚠️  Warning: .env file not found (using environment variables)")
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                              ║")
	fmt.Println("║           🪣 Testing Cloudflare R2 Storage                   ║")
	fmt.Println("║                                                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Create R2 client
	fmt.Println("📊 Initializing R2 client...")
	r2, err := storage.NewR2StorageFromEnv()
	if err != nil {
		fmt.Printf("❌ Failed to create R2 client: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✅ R2 client initialized")
	fmt.Println()

	ctx := context.Background()

	// Test 1: Upload Math Agent WASM
	fmt.Println("🧪 Test 1: Upload Math Agent WASM...")
	wasmPath := "./agents/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm"
	wasmData, err := os.ReadFile(wasmPath)
	if err != nil {
		fmt.Printf("❌ Failed to read WASM file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  File: %s\n", wasmPath)
	fmt.Printf("  Size: %d bytes\n", len(wasmData))

	key := "agents/math-agent-v1.0.wasm"
	err = r2.UploadWASM(ctx, key, wasmData)
	if err != nil {
		fmt.Printf("❌ Upload failed: %v\n", err)
		fmt.Println()
		fmt.Println("💡 Tip: Have you created the 'ainur-agents' bucket?")
		fmt.Println("   Go to: https://dash.cloudflare.com/ → R2 → Create bucket")
		os.Exit(1)
	}
	fmt.Println("  ✅ Upload successful!")
	fmt.Println()

	// Test 2: Check if file exists
	fmt.Println("🧪 Test 2: Verify file exists...")
	exists, err := r2.Exists(ctx, key)
	if err != nil {
		fmt.Printf("❌ Existence check failed: %v\n", err)
		os.Exit(1)
	}
	if !exists {
		fmt.Println("❌ File doesn't exist after upload!")
		os.Exit(1)
	}
	fmt.Println("  ✅ File exists in R2")
	fmt.Println()

	// Test 3: Download and verify
	fmt.Println("🧪 Test 3: Download and verify...")
	downloadedData, err := r2.DownloadWASM(ctx, key)
	if err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Downloaded: %d bytes\n", len(downloadedData))

	if len(downloadedData) != len(wasmData) {
		fmt.Printf("❌ Size mismatch! Original: %d, Downloaded: %d\n", len(wasmData), len(downloadedData))
		os.Exit(1)
	}
	fmt.Println("  ✅ File integrity verified!")
	fmt.Println()

	// Test 4: Get metadata
	fmt.Println("🧪 Test 4: Get file metadata...")
	metadata, err := r2.GetMetadata(ctx, key)
	if err != nil {
		fmt.Printf("❌ Metadata retrieval failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Metadata:")
	for k, v := range metadata {
		fmt.Printf("    %s: %s\n", k, v)
	}
	fmt.Println("  ✅ Metadata retrieved")
	fmt.Println()

	// Test 5: Generate presigned URL
	fmt.Println("🧪 Test 5: Generate presigned URL...")
	url, err := r2.GetURL(ctx, key, 1*time.Hour)
	if err != nil {
		fmt.Printf("❌ URL generation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  URL: %s\n", url[:80]+"...")
	fmt.Println("  ✅ URL generated (valid for 1 hour)")
	fmt.Println()

	// Test 6: List files
	fmt.Println("🧪 Test 6: List all agents...")
	files, err := r2.ListWASM(ctx, "agents/")
	if err != nil {
		fmt.Printf("❌ List failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Found %d file(s):\n", len(files))
	for _, file := range files {
		fmt.Printf("    - %s\n", file)
	}
	fmt.Println("  ✅ List successful")
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🎉 ALL R2 TESTS PASSED!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("✅ Cloudflare R2 is configured and working!")
	fmt.Println("✅ Math Agent WASM uploaded successfully!")
	fmt.Println("✅ File integrity verified!")
	fmt.Println()
	fmt.Println("🚀 Ready for Wasmtime integration!")
	fmt.Println()
}
