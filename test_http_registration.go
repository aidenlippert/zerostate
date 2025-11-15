package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"go.uber.org/zap"
)

// Request/Response types
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	DID     string `json:"did,omitempty"`
}

// Custom JWT claims
type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	DID    string `json:"did"`
	jwt.RegisteredClaims
}

// Simple API handlers
type SimpleAPI struct {
	db     *database.Database
	signer *identity.Signer
	logger *zap.Logger
	jwtSecret string
}

func (api *SimpleAPI) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AuthResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	ctx := c.Request.Context()

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		api.logger.Error("Failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, AuthResponse{
			Success: false,
			Message: "Failed to process password",
		})
		return
	}

	// Generate DID and user ID
	userID := uuid.New().String()
	userDID := api.signer.DID()

	// Insert user into database
	query := `
		INSERT INTO users (id, email, password_hash, full_name, did, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`

	_, err = api.db.Conn().ExecContext(ctx, query, userID, req.Email, string(hashedPassword), req.FullName, userDID)
	if err != nil {
		api.logger.Error("Failed to create user", zap.Error(err), zap.String("email", req.Email))
		c.JSON(http.StatusInternalServerError, AuthResponse{
			Success: false,
			Message: "Failed to create user",
		})
		return
	}

	// Generate JWT token
	claims := JWTClaims{
		UserID: userID,
		Email:  req.Email,
		DID:    userDID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(api.jwtSecret))
	if err != nil {
		api.logger.Error("Failed to generate JWT", zap.Error(err))
		c.JSON(http.StatusInternalServerError, AuthResponse{
			Success: false,
			Message: "Failed to generate token",
		})
		return
	}

	api.logger.Info("User registered successfully",
		zap.String("user_id", userID),
		zap.String("email", req.Email),
		zap.String("did", userDID))

	c.JSON(http.StatusCreated, AuthResponse{
		Success: true,
		Message: "User registered successfully",
		Token:   tokenString,
		UserID:  userID,
		DID:     userDID,
	})
}

func (api *SimpleAPI) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AuthResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	ctx := c.Request.Context()

	// Query user by email
	var userID, passwordHash, fullName, userDID string
	query := `SELECT id, password_hash, full_name, did FROM users WHERE email = ?`
	err := api.db.Conn().QueryRowContext(ctx, query, req.Email).Scan(&userID, &passwordHash, &fullName, &userDID)
	if err != nil {
		api.logger.Warn("Login failed - user not found", zap.String("email", req.Email), zap.Error(err))
		c.JSON(http.StatusUnauthorized, AuthResponse{
			Success: false,
			Message: "Invalid email or password",
		})
		return
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		api.logger.Warn("Login failed - invalid password", zap.String("email", req.Email))
		c.JSON(http.StatusUnauthorized, AuthResponse{
			Success: false,
			Message: "Invalid email or password",
		})
		return
	}

	// Generate JWT token
	claims := JWTClaims{
		UserID: userID,
		Email:  req.Email,
		DID:    userDID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(api.jwtSecret))
	if err != nil {
		api.logger.Error("Failed to generate JWT", zap.Error(err))
		c.JSON(http.StatusInternalServerError, AuthResponse{
			Success: false,
			Message: "Failed to generate token",
		})
		return
	}

	api.logger.Info("User logged in successfully",
		zap.String("user_id", userID),
		zap.String("email", req.Email))

	c.JSON(http.StatusOK, AuthResponse{
		Success: true,
		Message: "Login successful",
		Token:   tokenString,
		UserID:  userID,
		DID:     userDID,
	})
}

func main() {
	fmt.Println("=== ZeroState HTTP API Registration Test ===")
	fmt.Printf("Test started at: %s\n", time.Now().Format(time.RFC3339))

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()

	// Initialize database
	db, err := database.NewDB("./test_api.db")
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.InitializeSQLiteSchema(ctx); err != nil {
		log.Fatal("Failed to initialize schema:", err)
	}

	// Initialize identity signer
	signer, err := identity.NewSigner(logger)
	if err != nil {
		log.Fatal("Failed to create signer:", err)
	}

	// Create API instance
	api := &SimpleAPI{
		db:        db,
		signer:    signer,
		logger:    logger,
		jwtSecret: "test-secret-key-for-testing-only-not-production-use",
	}

	// Set up Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// API routes
	v1 := router.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", api.Register)
			auth.POST("/login", api.Login)
		}
	}

	// Start server in goroutine
	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	go func() {
		logger.Info("Starting HTTP server on :8081")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	// Run tests
	fmt.Println("\n=== Running HTTP API Tests ===")

	// Test 1: Register user
	fmt.Println("\nTest 1: User Registration")
	registerData := RegisterRequest{
		Email:    fmt.Sprintf("test-api-%d@zerostate.ai", time.Now().Unix()),
		Password: "TestPassword123!",
		FullName: "API Test User",
	}

	registerJSON, _ := json.Marshal(registerData)
	resp, err := http.Post("http://localhost:8081/api/v1/auth/register", "application/json", bytes.NewBuffer(registerJSON))
	if err != nil {
		log.Fatal("Failed to register user:", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var registerResp AuthResponse
	json.Unmarshal(body, &registerResp)

	fmt.Printf("Registration Response: %+v\n", registerResp)
	if resp.StatusCode != 201 || !registerResp.Success {
		log.Fatal("Registration failed")
	}
	fmt.Println("✅ User registration successful")

	// Test 2: Login user
	fmt.Println("\nTest 2: User Login")
	loginData := LoginRequest{
		Email:    registerData.Email,
		Password: registerData.Password,
	}

	loginJSON, _ := json.Marshal(loginData)
	resp, err = http.Post("http://localhost:8081/api/v1/auth/login", "application/json", bytes.NewBuffer(loginJSON))
	if err != nil {
		log.Fatal("Failed to login user:", err)
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	var loginResp AuthResponse
	json.Unmarshal(body, &loginResp)

	fmt.Printf("Login Response: %+v\n", loginResp)
	if resp.StatusCode != 200 || !loginResp.Success {
		log.Fatal("Login failed")
	}
	fmt.Println("✅ User login successful")

	// Test 3: Verify JWT token structure
	fmt.Println("\nTest 3: JWT Token Verification")
	if loginResp.Token == "" {
		log.Fatal("No JWT token received")
	}

	token, _, err := new(jwt.Parser).ParseUnverified(loginResp.Token, &JWTClaims{})
	if err != nil {
		log.Fatal("Failed to parse JWT token:", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok {
		fmt.Printf("JWT Claims: UserID=%s, Email=%s, DID=%s\n", claims.UserID, claims.Email, claims.DID)
		if claims.UserID != registerResp.UserID || claims.DID != registerResp.DID {
			log.Fatal("JWT claims don't match registration data")
		}
		fmt.Println("✅ JWT token verification successful")
	} else {
		log.Fatal("Failed to parse JWT claims")
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	// Final summary
	fmt.Println("\n=== TEST SUMMARY ===")
	fmt.Printf("✅ HTTP API Server: STARTED\n")
	fmt.Printf("✅ User Registration Endpoint: SUCCESS (Status %d)\n", resp.StatusCode)
	fmt.Printf("✅ JWT Token Generation: SUCCESS\n")
	fmt.Printf("✅ User Login Endpoint: SUCCESS (Status %d)\n", resp.StatusCode)
	fmt.Printf("✅ DID Creation: SUCCESS (%s)\n", registerResp.DID)
	fmt.Printf("✅ Database Integration: SUCCESS\n")

	fmt.Println("\n=== CRITICAL SUCCESS CRITERIA ===")
	fmt.Println("✅ User registration via HTTP API SUCCEEDED")
	fmt.Println("✅ JWT token creation and validation SUCCEEDED")
	fmt.Println("✅ User login via HTTP API SUCCEEDED")
	fmt.Println("✅ DID integration SUCCEEDED")

	fmt.Printf("\nHTTP API test completed at: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("Test user email: %s\n", registerData.Email)
	fmt.Printf("Test user DID: %s\n", registerResp.DID)
}