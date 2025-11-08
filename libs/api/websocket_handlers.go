package api

import (
	"net/http"

	"github.com/aidenlippert/zerostate/libs/websocket"
	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: In production, implement proper origin checking
		return true
	},
}

// HandleWebSocket handles WebSocket connection upgrades
func (h *Handlers) HandleWebSocket(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "HandleWebSocket"))

	// Get user ID from context (if authenticated)
	userID := "anonymous"
	if uid, exists := c.Get("user_id"); exists {
		userID = uid.(string)
	}

	logger.Info("WebSocket connection request",
		zap.String("user_id", userID),
		zap.String("remote_addr", c.Request.RemoteAddr),
	)

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("failed to upgrade to WebSocket", zap.Error(err))
		return
	}

	if h.wsHub == nil {
		logger.Error("WebSocket hub not initialized")
		conn.Close()
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "WebSocket service not available",
		})
		return
	}

	// Create new client
	client := websocket.NewClient(conn, userID, h.wsHub, logger)

	// Register client with hub
	h.wsHub.RegisterClient(client)

	// Start client read/write pumps
	go client.WritePump()
	go client.ReadPump()

	logger.Info("WebSocket client connected",
		zap.String("client_id", client.ID),
		zap.String("user_id", userID),
	)
}

// GetWebSocketStats returns WebSocket hub statistics
func (h *Handlers) GetWebSocketStats(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetWebSocketStats"))

	if h.wsHub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "WebSocket service not available",
		})
		return
	}

	stats := h.wsHub.GetStats()

	logger.Info("WebSocket stats requested", zap.Any("stats", stats))

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// BroadcastMessage broadcasts a message to all WebSocket clients
func (h *Handlers) BroadcastMessage(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "BroadcastMessage"))

	if h.wsHub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "WebSocket service not available",
		})
		return
	}

	var request struct {
		Type string                 `json:"type" binding:"required"`
		Data map[string]interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("invalid broadcast request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	h.wsHub.Broadcast(request.Type, request.Data)

	logger.Info("broadcast message sent",
		zap.String("type", request.Type),
		zap.Any("data", request.Data),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "broadcast sent successfully",
	})
}

// SendUserMessage sends a message to a specific user
func (h *Handlers) SendUserMessage(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "SendUserMessage"))

	if h.wsHub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "WebSocket service not available",
		})
		return
	}

	var request struct {
		UserID string                 `json:"user_id" binding:"required"`
		Type   string                 `json:"type" binding:"required"`
		Data   map[string]interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("invalid user message request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": err.Error(),
		})
		return
	}

	h.wsHub.SendToUser(request.UserID, request.Type, request.Data)

	logger.Info("user message sent",
		zap.String("user_id", request.UserID),
		zap.String("type", request.Type),
		zap.Any("data", request.Data),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "user message sent successfully",
	})
}
