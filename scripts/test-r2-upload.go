package main
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aidenlippert/zerostate/libs/storage"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	fmt.Println("🧪 Testing R2 Upload/Download")
	fmt.Println("================================")
	fmt.Println("")

	// Create R2 client
	r2, err := storage.NewR2StorageFromEnv()
	if err != nil {
		log.Fatalf("❌ Failed to create R2 client: %v", err)
	}

	fmt.Println("✅ R2 client created successfully")
	fmt.Printf("   Endpoint: %s\n", os.Getenv("R2_ENDPOINT"))
	fmt.Printf("   Bucket: %s\n", os.Getenv("R2_BUCKET_NAME"))
	fmt.Println("")

	ctx := context.Background()

	// Test 1: Upload a test file
	fmt.Println("📤 Test 1: Uploading test file...")
	testData := []byte("Hello from ZeroState! This is a test upload.")
	testKey := "test/hello.txt"

	err = r2.UploadWASM(ctx, testKey, testData)
	if err != nil {
		log.Fatalf("❌ Upload failed: %v", err)
	}
	fmt.Println("✅ Upload successful!")
	fmt.Println("")

	// Test 2: Check if file exists
	fmt.Println("🔍 Test 2: Checking if file exists...")
	exists, err := r2.Exists(ctx, testKey)
	if err != nil {
		log.Fatalf("❌ Exists check failed: %v", err)
	}
	if !exists {
		log.Fatalf("❌ File not found after upload!")
	}
	fmt.Println("✅ File exists!")
	fmt.Println("")

	// Test 3: Download the file
	fmt.Println("📥 Test 3: Downloading file...")
	downloaded, err := r2.DownloadWASM(ctx, testKey)
	if err != nil {
		log.Fatalf("❌ Download failed: %v", err)
	}
	fmt.Printf("✅ Downloaded %d bytes\n", len(downloaded))
	fmt.Printf("   Content: %s\n", string(downloaded))
	fmt.Println("")

	// Test 4: List files
	fmt.Println("📋 Test 4: Listing files...")
	files, err := r2.ListWASM(ctx, "test/")
	if err != nil {
		log.Fatalf("❌ List failed: %v", err)
	}
	fmt.Printf("✅ Found %d file(s):\n", len(files))
	for _, file := range files {
		fmt.Printf("   - %s\n", file)
	}
	fmt.Println("")

	// Test 5: Get metadata
	fmt.Println("📊 Test 5: Getting metadata...")
	metadata, err := r2.GetMetadata(ctx, testKey)
	if err != nil {
		log.Printf("⚠️  Metadata retrieval failed: %v", err)
	} else {
		fmt.Println("✅ Metadata retrieved:")
		for k, v := range metadata {
			fmt.Printf("   %s: %s\n", k, v)
		}
	}
	fmt.Println("")

	// Test 6: Delete the test file
	fmt.Println("🗑️  Test 6: Cleaning up...")
	err = r2.DeleteWASM(ctx, testKey)
	if err != nil {
		log.Fatalf("❌ Delete failed: %v", err)
	}
	fmt.Println("✅ Test file deleted")
	fmt.Println("")

	// Test 7: Verify deletion
	fmt.Println("🔍 Test 7: Verifying deletion...")
	exists, err = r2.Exists(ctx, testKey)
	if err != nil {
		log.Fatalf("❌ Exists check failed: %v", err)
	}
	if exists {
		log.Fatalf("❌ File still exists after deletion!")
	}
	fmt.Println("✅ File successfully deleted")
	fmt.Println("")

	fmt.Println("🎉 All R2 tests passed!")
	fmt.Println("")
	fmt.Println("📚 Next Steps:")
	fmt.Println("   1. Build WASM agents: cd agents/math-agent-rust && cargo build --target wasm32-wasi")
	fmt.Println("   2. Upload agents: go run scripts/upload-wasm-agents.go")
	fmt.Println("   3. Test execution: go run scripts/test-wasm-execution.go")
}
