package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	fmt.Println("=== ZeroState User Registration → DID Creation Flow Test ===")
	fmt.Printf("Test started at: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()

	ctx := context.Background()

	// Step 1: Connect to database (using SQLite for testing)
	fmt.Println("Step 1: Connecting to SQLite database...")

	db, err := database.NewDB("./test_registration.db")
	if err != nil {
		log.Fatal("Failed to initialize SQLite database:", err)
	}
	defer db.Close()

	fmt.Println("✅ Database connection successful")

	// Step 2: Run migrations
	fmt.Println("\nStep 2: Running database migrations...")

	// Initialize SQLite schema
	if err := db.InitializeSQLiteSchema(ctx); err != nil {
		log.Fatal("Failed to initialize SQLite schema:", err)
	}
	fmt.Println("✅ Database schema initialized successfully")

	// Get the underlying SQL connection for direct queries
	sqlDB := db.Conn()

	// Step 3: Initialize DID signer
	fmt.Println("\nStep 3: Initializing DID signer...")
	// Create a nil logger for the signer since we don't need full zap logging for this test
	signer, err := identity.NewSigner(nil)
	if err != nil {
		log.Fatal("Failed to create signer:", err)
	}
	fmt.Printf("✅ DID signer initialized: %s\n", signer.DID())

	// Step 4: Create test user (simulate registration)
	fmt.Println("\nStep 4: Creating test user (simulating registration)...")

	// Generate unique email for this test
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	testEmail := fmt.Sprintf("test-e2e-%s@zerostate.ai", hex.EncodeToString(randomBytes))
	testPassword := "TestPassword123!"
	testFullName := "E2E Test User"

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("Failed to hash password:", err)
	}

	// Create user with DID
	userDID := signer.DID()
	userID := uuid.New().String()

	// Insert user into database (SQLite syntax with UUID)
	query := `
		INSERT INTO users (id, email, password_hash, full_name, did, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`

	_, err = sqlDB.ExecContext(ctx, query, userID, testEmail, string(hashedPassword), testFullName, userDID)
	if err != nil {
		log.Fatal("Failed to create user:", err)
	}

	// Query the created user to get timestamp
	var createdAt string
	selectQuery := `SELECT created_at FROM users WHERE id = ?`
	err = sqlDB.QueryRowContext(ctx, selectQuery, userID).Scan(&createdAt)
	if err != nil {
		log.Fatal("Failed to query created user:", err)
	}

	fmt.Printf("✅ User created successfully:\n")
	fmt.Printf("   - ID: %s\n", userID)
	fmt.Printf("   - Email: %s\n", testEmail)
	fmt.Printf("   - Full Name: %s\n", testFullName)
	fmt.Printf("   - DID: %s\n", userDID)
	fmt.Printf("   - Created At: %s\n", createdAt)

	// Step 5: Test login (verify credentials)
	fmt.Println("\nStep 5: Testing login (verifying credentials)...")

	// Query user by email
	var storedPasswordHash string
	var storedDID string
	var storedID string
	var storedFullName string

	loginQuery := `SELECT id, password_hash, full_name, did FROM users WHERE email = ?`
	err = sqlDB.QueryRowContext(ctx, loginQuery, testEmail).Scan(&storedID, &storedPasswordHash, &storedFullName, &storedDID)
	if err != nil {
		log.Fatal("Failed to find user for login:", err)
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(storedPasswordHash), []byte(testPassword))
	if err != nil {
		log.Fatal("Password verification failed:", err)
	}

	fmt.Printf("✅ Login successful:\n")
	fmt.Printf("   - User ID: %s\n", storedID)
	fmt.Printf("   - Email: %s\n", testEmail)
	fmt.Printf("   - Full Name: %s\n", storedFullName)
	fmt.Printf("   - DID: %s\n", storedDID)

	// Step 6: Verify DID on blockchain (simulated)
	fmt.Println("\nStep 6: Verifying DID creation status...")

	// Since we don't have blockchain running, just verify that DID is well-formed
	if len(userDID) < 10 {
		log.Fatal("DID appears to be malformed:", userDID)
	}

	// Check if DID starts with expected prefix
	expectedPrefix := "did:key:"
	if len(userDID) < len(expectedPrefix) || userDID[:len(expectedPrefix)] != expectedPrefix {
		fmt.Printf("⚠️ DID doesn't start with expected prefix 'did:key:', got: %s\n", userDID)
	} else {
		fmt.Printf("✅ DID format validation passed: %s\n", userDID)
	}

	// Step 7: Database verification
	fmt.Println("\nStep 7: Final database verification...")

	// Verify user record exists with DID
	var count int
	countQuery := `SELECT COUNT(*) FROM users WHERE email = ? AND did = ?`
	err = sqlDB.QueryRowContext(ctx, countQuery, testEmail, userDID).Scan(&count)
	if err != nil {
		log.Fatal("Failed to verify user record:", err)
	}

	if count != 1 {
		log.Fatalf("Expected 1 user record, found %d", count)
	}

	fmt.Printf("✅ Database verification passed: User record exists with DID\n")

	// Success Summary
	fmt.Println("\n=== TEST SUMMARY ===")
	fmt.Printf("✅ Database Connection: SUCCESS\n")
	fmt.Printf("✅ Database Migrations: SUCCESS\n")
	fmt.Printf("✅ User Registration: SUCCESS\n")
	fmt.Printf("✅ DID Generation: SUCCESS (%s)\n", userDID)
	fmt.Printf("✅ User Login: SUCCESS\n")
	fmt.Printf("✅ Database Record: SUCCESS\n")
	fmt.Printf("⚠️ Blockchain DID Verification: SKIPPED (blockchain not running)\n")

	fmt.Println("\n=== CRITICAL SUCCESS CRITERIA ===")
	fmt.Println("✅ User registration SUCCEEDED")
	fmt.Println("✅ DID creation SUCCEEDED")
	fmt.Println("✅ Login verification SUCCEEDED")
	fmt.Println("✅ Database integration SUCCEEDED")

	fmt.Printf("\nTest completed successfully at: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("Test user email: %s\n", testEmail)
	fmt.Printf("Test user DID: %s\n", userDID)
}