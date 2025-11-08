package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Message represents a WebSocket message
type Message struct {
	Type      string                 `json:"type"`       // "task_update", "agent_update", "system"
	Timestamp time.Time              `json:"timestamp"`  // Message timestamp
	Data      map[string]interface{} `json:"data"`       // Message payload
	UserID    string                 `json:"user_id,omitempty"` // Target user (for private messages)
}

// Client represents a WebSocket client connection
type Client struct {
	ID       string              // Unique client ID
	UserID   string              // Authenticated user ID
	Conn     *websocket.Conn     // WebSocket connection
	Send     chan *Message       // Outbound message channel
	Hub      *Hub                // Reference to hub
	Logger   *zap.Logger         // Client logger
	ctx      context.Context     // Client context
	cancel   context.CancelFunc  // Cancel function
}

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	// Client management
	clients    map[*Client]bool     // Active clients
	register   chan *Client         // Client registration channel
	unregister chan *Client         // Client unregistration channel
	clientsMu  sync.RWMutex         // Protect clients map

	// Message broadcasting
	broadcast  chan *Message        // Broadcast to all clients
	userMsg    chan *Message        // Send to specific user

	// Configuration
	logger     *zap.Logger          // Hub logger
	ctx        context.Context      // Hub context
	cancel     context.CancelFunc   // Cancel function

	// Metrics
	totalConnections   int64         // Total connections since start
	currentConnections int           // Current active connections
	messagesSent       int64         // Total messages sent
}

// NewHub creates a new WebSocket hub
func NewHub(ctx context.Context, logger *zap.Logger) *Hub {
	if logger == nil {
		logger = zap.NewNop()
	}

	hubCtx, cancel := context.WithCancel(ctx)

	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client, 10),
		unregister: make(chan *Client, 10),
		broadcast:  make(chan *Message, 100),
		userMsg:    make(chan *Message, 100),
		logger:     logger,
		ctx:        hubCtx,
		cancel:     cancel,
	}
}

// Start starts the hub's message processing loop
func (h *Hub) Start() {
	h.logger.Info("WebSocket hub starting")

	go func() {
		for {
			select {
			case client := <-h.register:
				h.clientsMu.Lock()
				h.clients[client] = true
				h.currentConnections = len(h.clients)
				h.totalConnections++
				h.clientsMu.Unlock()

				h.logger.Info("client registered",
					zap.String("client_id", client.ID),
					zap.String("user_id", client.UserID),
					zap.Int("active_connections", h.currentConnections),
				)

				// Send welcome message
				welcome := &Message{
					Type:      "system",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"message":      "connected to ZeroState WebSocket",
						"client_id":    client.ID,
						"server_time":  time.Now().Format(time.RFC3339),
					},
				}
				select {
				case client.Send <- welcome:
				default:
					h.logger.Warn("failed to send welcome message, client send channel full")
				}

			case client := <-h.unregister:
				h.clientsMu.Lock()
				if _, ok := h.clients[client]; ok {
					delete(h.clients, client)
					close(client.Send)
					h.currentConnections = len(h.clients)
					h.clientsMu.Unlock()

					h.logger.Info("client unregistered",
						zap.String("client_id", client.ID),
						zap.String("user_id", client.UserID),
						zap.Int("active_connections", h.currentConnections),
					)
				} else {
					h.clientsMu.Unlock()
				}

			case message := <-h.broadcast:
				h.clientsMu.RLock()
				for client := range h.clients {
					select {
					case client.Send <- message:
						h.messagesSent++
					default:
						// Client's send buffer is full, remove client
						h.logger.Warn("client send buffer full, disconnecting",
							zap.String("client_id", client.ID),
						)
						go h.UnregisterClient(client)
					}
				}
				h.clientsMu.RUnlock()

			case message := <-h.userMsg:
				// Send message to specific user
				h.clientsMu.RLock()
				sent := false
				for client := range h.clients {
					if client.UserID == message.UserID {
						select {
						case client.Send <- message:
							sent = true
							h.messagesSent++
						default:
							h.logger.Warn("failed to send user message, buffer full",
								zap.String("client_id", client.ID),
								zap.String("user_id", message.UserID),
							)
						}
					}
				}
				h.clientsMu.RUnlock()

				if !sent {
					h.logger.Debug("no active connection for user",
						zap.String("user_id", message.UserID),
					)
				}

			case <-h.ctx.Done():
				h.logger.Info("WebSocket hub shutting down")
				return
			}
		}
	}()

	h.logger.Info("WebSocket hub started")
}

// Stop stops the hub and closes all client connections
func (h *Hub) Stop() {
	h.logger.Info("stopping WebSocket hub")

	h.cancel()

	// Close all client connections
	h.clientsMu.Lock()
	for client := range h.clients {
		close(client.Send)
		client.Conn.Close()
		delete(h.clients, client)
	}
	h.clientsMu.Unlock()

	h.logger.Info("WebSocket hub stopped",
		zap.Int64("total_connections", h.totalConnections),
		zap.Int64("messages_sent", h.messagesSent),
	)
}

// RegisterClient registers a new client
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient unregisters a client
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(msgType string, data map[string]interface{}) {
	message := &Message{
		Type:      msgType,
		Timestamp: time.Now(),
		Data:      data,
	}

	select {
	case h.broadcast <- message:
	default:
		h.logger.Warn("broadcast channel full, dropping message")
	}
}

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID string, msgType string, data map[string]interface{}) {
	message := &Message{
		Type:      msgType,
		Timestamp: time.Now(),
		Data:      data,
		UserID:    userID,
	}

	select {
	case h.userMsg <- message:
	default:
		h.logger.Warn("user message channel full, dropping message",
			zap.String("user_id", userID),
		)
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	return map[string]interface{}{
		"current_connections": h.currentConnections,
		"total_connections":   h.totalConnections,
		"messages_sent":       h.messagesSent,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.UnregisterClient(c)
		c.Conn.Close()
		c.cancel()
	}()

	// Configure connection
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.Logger.Error("unexpected websocket close", zap.Error(err))
				}
				return
			}

			// Log received message
			c.Logger.Debug("received websocket message",
				zap.String("client_id", c.ID),
				zap.ByteString("message", message),
			)

			// For now, just echo back (can add custom handlers later)
			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				c.Logger.Warn("invalid JSON message", zap.Error(err))
				continue
			}

			// Handle message types (placeholder for future expansion)
			switch msg.Type {
			case "ping":
				c.Send <- &Message{
					Type:      "pong",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"received": msg.Timestamp,
					},
				}
			default:
				c.Logger.Debug("unhandled message type",
					zap.String("type", msg.Type),
				)
			}
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
		c.cancel()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Marshal message to JSON
			data, err := json.Marshal(message)
			if err != nil {
				c.Logger.Error("failed to marshal message", zap.Error(err))
				continue
			}

			// Send message
			if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.Logger.Error("failed to write message", zap.Error(err))
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// NewClient creates a new WebSocket client
func NewClient(conn *websocket.Conn, userID string, hub *Hub, logger *zap.Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		ID:     generateClientID(),
		UserID: userID,
		Conn:   conn,
		Send:   make(chan *Message, 10),
		Hub:    hub,
		Logger: logger.With(zap.String("client_type", "websocket")),
		ctx:    ctx,
		cancel: cancel,
	}
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of length n
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
