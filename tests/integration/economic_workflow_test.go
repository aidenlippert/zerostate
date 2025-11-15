package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/api"
	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/aidenlippert/zerostate/libs/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestCompleteEconomicWorkflow tests the entire economic flow:
// User register → Create auction → Agent bids → Open payment channel →
// Create escrow → Fund escrow → Execute task → Release escrow →
// Settle payment → Update reputation → Analytics
func TestCompleteEconomicWorkflow(t *testing.T) {
	// Skip if DATABASE_URL not set
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	// Initialize context
	ctx := context.Background()

	// Initialize database
	logger.Info("connecting to PostgreSQL database")
	sqlDB, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.NewDatabase(sqlDB)
	require.NoError(t, err)
	defer db.Close()

	// Initialize p2p host
	p2pHost, err := libp2p.New()
	require.NoError(t, err)
	defer p2pHost.Close()

	// Initialize identity signer
	signer, err := identity.NewSigner(logger)
	require.NoError(t, err)

	// Initialize HNSW index
	hnsw := search.NewHNSWIndex(16, 200)

	// Initialize task queue
	taskQueue := orchestration.NewTaskQueue(ctx, 100, logger)
	defer taskQueue.Close()

	// Initialize WASM runner and result store
	wasmRunner := execution.NewWASMRunner(logger, 5*time.Minute)
	resultStore := execution.NewPostgresResultStore(sqlDB, logger)

	// Initialize orchestrator
	selector := orchestration.NewDatabaseAgentSelector(db, orchestration.DefaultMetaAgentConfig(), logger)
	executor := orchestration.NewMockTaskExecutor(logger)
	orchConfig := orchestration.DefaultOrchestratorConfig()
	orchConfig.NumWorkers = 1

	orch := orchestration.NewOrchestrator(ctx, taskQueue, selector, executor, orchConfig, logger)
	require.NoError(t, orch.Start())
	defer orch.Stop()

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(ctx, logger)
	wsHub.Start()
	defer wsHub.Stop()

	// Initialize API handlers
	handlers := api.NewHandlers(
		ctx,
		logger,
		p2pHost,
		signer,
		hnsw,
		taskQueue,
		orch,
		db,
		nil,
		wsHub,
		wasmRunner,
		resultStore,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	// Create test router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Setup economic routes
	v1 := router.Group("/api/v1")
	protected := v1.Group("")
	protected.Use(func(c *gin.Context) {
		// Mock auth middleware for testing - inject test user
		c.Set("user_id", "test-user-123")
		c.Set("user_did", "did:zerostate:test-user")
		c.Next()
	})

	economic := protected.Group("/economic")
	{
		economic.POST("/auctions", handlers.CreateAuction)
		economic.POST("/auctions/:id/bids", handlers.SubmitBid)
		economic.POST("/payment-channels", handlers.OpenPaymentChannel)
		economic.POST("/payment-channels/:id/settle", handlers.SettlePaymentChannel)
		economic.GET("/reputation/:agent_id", handlers.GetAgentReputation)
		economic.POST("/reputation", handlers.UpdateAgentReputation)
		economic.POST("/escrows", handlers.CreateEscrow)
		economic.GET("/escrows/:id", handlers.GetEscrow)
		economic.POST("/escrows/:id/fund", handlers.FundEscrow)
		economic.POST("/escrows/:id/release", handlers.ReleaseEscrow)
		economic.POST("/escrows/:id/refund", handlers.RefundEscrow)
		economic.POST("/escrows/:id/dispute", handlers.OpenDispute)
		economic.GET("/disputes/:id", handlers.GetDispute)
		economic.POST("/disputes/:id/evidence", handlers.SubmitEvidence)
		economic.POST("/disputes/:id/resolve", handlers.ResolveDispute)
		economic.POST("/meta-orchestrator/delegate", handlers.DelegateToMetaOrchestrator)
		economic.GET("/meta-orchestrator/status/:task_id", handlers.GetOrchestrationStatus)
	}

	analytics := protected.Group("/analytics")
	{
		analytics.GET("/escrow", handlers.GetEscrowMetrics)
		analytics.GET("/auctions", handlers.GetAuctionMetrics)
		analytics.GET("/payment-channels", handlers.GetPaymentChannelMetrics)
		analytics.GET("/reputation", handlers.GetReputationMetrics)
		analytics.GET("/dashboard", handlers.GetAnalyticsDashboard)
	}

	// Test 1: Create Auction
	t.Run("CreateAuction", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"task_id":        "task-" + uuid.New().String(),
			"auction_type":   "first_price",
			"duration_sec":   300,
			"reserve_price":  0.05,
			"max_price":      1.00,
			"min_reputation": 50.0,
			"capabilities":   []string{"compute", "storage"},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/economic/auctions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		auctionData := resp["auction"].(map[string]interface{})
		auctionID := auctionData["id"].(string)
		assert.NotEmpty(t, auctionID)
		assert.Equal(t, "open", auctionData["status"])

		t.Logf("Created auction: %s", auctionID)

		// Test 2: Submit Bid
		t.Run("SubmitBid", func(t *testing.T) {
			bidBody := map[string]interface{}{
				"agent_did":          "did:zerostate:agent-1",
				"price":              0.10,
				"estimated_time_sec": 120,
			}

			body, _ := json.Marshal(bidBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/economic/auctions/"+auctionID+"/bids", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			bidData := resp["bid"].(map[string]interface{})
			assert.Equal(t, 0.10, bidData["price"])

			t.Logf("Submitted bid with price: %.2f", bidData["price"].(float64))
		})

		// Test 3: Open Payment Channel
		t.Run("OpenPaymentChannel", func(t *testing.T) {
			channelBody := map[string]interface{}{
				"payer_did":       "did:zerostate:test-user",
				"payee_did":       "did:zerostate:agent-1",
				"initial_deposit": 1.00,
				"auction_id":      auctionID,
			}

			body, _ := json.Marshal(channelBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/economic/payment-channels", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			channelData := resp["channel"].(map[string]interface{})
			channelID := channelData["id"].(string)
			assert.NotEmpty(t, channelID)
			assert.Equal(t, 1.00, channelData["total_deposit"])

			t.Logf("Opened payment channel: %s", channelID)

			// Test 4: Create Escrow
			t.Run("CreateEscrow", func(t *testing.T) {
				escrowBody := map[string]interface{}{
					"task_id":              "task-" + uuid.New().String(),
					"payer_id":             "did:zerostate:test-user",
					"payee_id":             "did:zerostate:agent-1",
					"amount":               0.10,
					"expiration_minutes":   60,
					"auto_release_minutes": 30,
					"conditions":           "Task must complete successfully within 30 minutes",
				}

				body, _ := json.Marshal(escrowBody)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/economic/escrows", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusCreated, w.Code)

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)

				escrowData := resp["escrow"].(map[string]interface{})
				escrowID := escrowData["id"].(string)
				assert.NotEmpty(t, escrowID)
				assert.Equal(t, "created", escrowData["status"])
				assert.Equal(t, 0.10, escrowData["amount"])

				t.Logf("Created escrow: %s", escrowID)

				// Test 5: Fund Escrow
				t.Run("FundEscrow", func(t *testing.T) {
					fundBody := map[string]interface{}{
						"signature": "mock-signature-123",
					}

					body, _ := json.Marshal(fundBody)
					req := httptest.NewRequest(http.MethodPost, "/api/v1/economic/escrows/"+escrowID+"/fund", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")

					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)

					assert.Equal(t, http.StatusOK, w.Code)

					var resp map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &resp)
					require.NoError(t, err)

					assert.Contains(t, resp["message"], "funded successfully")

					t.Log("Funded escrow successfully")

					// Test 6: Release Escrow
					t.Run("ReleaseEscrow", func(t *testing.T) {
						req := httptest.NewRequest(http.MethodPost, "/api/v1/economic/escrows/"+escrowID+"/release", nil)
						w := httptest.NewRecorder()
						router.ServeHTTP(w, req)

						assert.Equal(t, http.StatusOK, w.Code)

						var resp map[string]interface{}
						err := json.Unmarshal(w.Body.Bytes(), &resp)
						require.NoError(t, err)

						assert.Contains(t, resp["message"], "released successfully")

						t.Log("Released escrow successfully")

						// Test 7: Settle Payment Channel
						t.Run("SettlePaymentChannel", func(t *testing.T) {
							settleBody := map[string]interface{}{
								"final_amount": 0.10,
							}

							body, _ := json.Marshal(settleBody)
							req := httptest.NewRequest(http.MethodPost, "/api/v1/economic/payment-channels/"+channelID+"/settle", bytes.NewReader(body))
							req.Header.Set("Content-Type", "application/json")

							w := httptest.NewRecorder()
							router.ServeHTTP(w, req)

							assert.Equal(t, http.StatusOK, w.Code)

							var resp map[string]interface{}
							err := json.Unmarshal(w.Body.Bytes(), &resp)
							require.NoError(t, err)

							assert.Contains(t, resp["message"], "settled successfully")

							t.Log("Settled payment channel successfully")

							// Test 8: Update Reputation
							t.Run("UpdateReputation", func(t *testing.T) {
								repBody := map[string]interface{}{
									"agent_did":     "did:zerostate:agent-1",
									"task_id":       "task-" + uuid.New().String(),
									"success":       true,
									"rating":        4.5,
									"response_time": 120,
								}

								body, _ := json.Marshal(repBody)
								req := httptest.NewRequest(http.MethodPost, "/api/v1/economic/reputation", bytes.NewReader(body))
								req.Header.Set("Content-Type", "application/json")

								w := httptest.NewRecorder()
								router.ServeHTTP(w, req)

								assert.Equal(t, http.StatusOK, w.Code)

								var resp map[string]interface{}
								err := json.Unmarshal(w.Body.Bytes(), &resp)
								require.NoError(t, err)

								assert.Contains(t, resp["message"], "updated successfully")

								t.Log("Updated agent reputation successfully")

								// Test 9: Get Analytics Dashboard
								t.Run("GetAnalyticsDashboard", func(t *testing.T) {
									req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/dashboard", nil)
									w := httptest.NewRecorder()
									router.ServeHTTP(w, req)

									assert.Equal(t, http.StatusOK, w.Code)

									var resp map[string]interface{}
									err := json.Unmarshal(w.Body.Bytes(), &resp)
									require.NoError(t, err)

									// Verify all metric categories are present
									assert.Contains(t, resp, "escrow_metrics")
									assert.Contains(t, resp, "auction_metrics")
									assert.Contains(t, resp, "payment_channel_metrics")
									assert.Contains(t, resp, "reputation_metrics")

									escrowMetrics := resp["escrow_metrics"].(map[string]interface{})
									assert.NotNil(t, escrowMetrics)

									t.Log("Analytics dashboard successfully returned comprehensive metrics")
								})
							})
						})
					})
				})
			})
		})
	})
}

// TestEscrowDisputeWorkflow tests the dispute resolution flow
func TestEscrowDisputeWorkflow(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	ctx := context.Background()

	sqlDB, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.NewDatabase(sqlDB)
	defer db.Close()

	// Initialize economic service
	economicService := economic.NewEconomicService(db)
	escrowService := economic.NewEscrowService(sqlDB, logger)

	// Create escrow
	escrow, err := escrowService.CreateEscrow(
		ctx,
		"dispute-task-"+uuid.New().String(),
		"did:zerostate:payer",
		"did:zerostate:payee",
		0.50,
		60,
		nil,
		"Task completion required",
	)
	require.NoError(t, err)
	require.NotNil(t, escrow)

	// Fund escrow
	err = escrowService.FundEscrow(ctx, escrow.ID, "mock-signature")
	require.NoError(t, err)

	// Open dispute
	dispute, err := escrowService.OpenDispute(
		ctx,
		escrow.ID,
		"did:zerostate:payer",
		"Task not completed as specified",
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, dispute)
	assert.Equal(t, "open", dispute.Status)

	t.Logf("Opened dispute: %s", dispute.ID)

	// Submit evidence
	err = escrowService.SubmitEvidence(
		ctx,
		dispute.ID,
		"did:zerostate:payer",
		"Screenshot showing incomplete work",
		fmt.Sprintf(`{"screenshot": "https://example.com/evidence.png", "timestamp": "%s"}`, time.Now().Format(time.RFC3339)),
	)
	require.NoError(t, err)

	t.Log("Submitted evidence successfully")

	// Resolve dispute
	err = escrowService.ResolveDispute(
		ctx,
		dispute.ID,
		"did:zerostate:arbitrator",
		"requester_favor",
		"Evidence supports requester's claim. Refund approved.",
	)
	require.NoError(t, err)

	t.Log("Resolved dispute successfully")

	// Verify escrow status
	updatedEscrow, err := escrowService.GetEscrow(ctx, escrow.ID)
	require.NoError(t, err)
	assert.Equal(t, economic.EscrowStatusDisputed, updatedEscrow.Status)
}

// TestMetaOrchestratorDelegation tests meta-orchestrator workflow
func TestMetaOrchestratorDelegation(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	ctx := context.Background()

	sqlDB, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.NewDatabase(sqlDB)
	defer db.Close()

	economicService := economic.NewEconomicService(db)

	// Create complex task requiring delegation
	taskID := "complex-task-" + uuid.New().String()
	subtasks := []map[string]interface{}{
		{
			"description":  "Data preprocessing",
			"capabilities": []string{"data_processing"},
			"budget":       0.20,
		},
		{
			"description":  "Model training",
			"capabilities": []string{"ml_training"},
			"budget":       0.50,
		},
		{
			"description":  "Results analysis",
			"capabilities": []string{"analytics"},
			"budget":       0.30,
		},
	}

	delegation, err := economicService.DelegateToMetaOrchestrator(
		ctx,
		taskID,
		"did:zerostate:requester",
		"Train ML model on dataset",
		subtasks,
		1.00,
	)
	require.NoError(t, err)
	require.NotNil(t, delegation)

	assert.Equal(t, taskID, delegation.TaskID)
	assert.Equal(t, "pending", delegation.Status)
	assert.Equal(t, 1.00, delegation.TotalBudget)

	t.Logf("Created delegation: %s with %d subtasks", delegation.ID, len(subtasks))

	// Get orchestration status
	status, err := economicService.GetOrchestrationStatus(ctx, taskID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "pending", status.Status)
	t.Log("Meta-orchestrator delegation workflow completed successfully")
}
