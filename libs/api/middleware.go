package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/auth"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Context keys for correlation ID
const (
	correlationIDKey  = "correlation_id"
	requestIDKey      = "request_id"
)

// generateCorrelationID generates a unique correlation ID
func generateCorrelationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random fails
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000")))
	}
	return hex.EncodeToString(b)
}

// correlationIDMiddleware adds correlation ID to request context and response headers
func correlationIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get correlation ID from request header
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			// Generate new correlation ID if not provided
			correlationID = generateCorrelationID()
		}

		// Generate request ID (unique for each request)
		requestID := generateCorrelationID()

		// Store in context
		c.Set(correlationIDKey, correlationID)
		c.Set(requestIDKey, requestID)

		// Add to response headers
		c.Writer.Header().Set("X-Correlation-ID", correlationID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		// Store in Go context for propagation to other services
		ctx := context.WithValue(c.Request.Context(), correlationIDKey, correlationID)
		ctx = context.WithValue(ctx, requestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// loggingMiddleware logs HTTP requests with structured logging and correlation IDs
func loggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Get correlation IDs (should be set by correlationIDMiddleware)
		correlationID, _ := c.Get(correlationIDKey)
		requestID, _ := c.Get(requestIDKey)

		// Log request start
		logger.Info("http request started",
			zap.String("correlation_id", toString(correlationID)),
			zap.String("request_id", toString(requestID)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)

		// Process request
		c.Next()

		// Log after request is processed
		duration := time.Since(start)

		// Get user ID if authenticated
		var userID string
		if uid, exists := c.Get("user_id"); exists {
			userID = toString(uid)
		}

		// Build log fields
		fields := []zap.Field{
			zap.String("correlation_id", toString(correlationID)),
			zap.String("request_id", toString(requestID)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", duration),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("response_size", c.Writer.Size()),
		}

		// Add user ID if available
		if userID != "" {
			fields = append(fields, zap.String("user_id", userID))
		}

		// Log at appropriate level based on status code
		statusCode := c.Writer.Status()
		if statusCode >= 500 {
			logger.Error("http request completed", fields...)
		} else if statusCode >= 400 {
			logger.Warn("http request completed", fields...)
		} else {
			logger.Info("http request completed", fields...)
		}
	}
}

// toString safely converts interface{} to string
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// tracingMiddleware adds OpenTelemetry tracing to requests
func tracingMiddleware(tracer trace.Tracer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tracer == nil {
			c.Next()
			return
		}

		// Start span
		ctx, span := tracer.Start(c.Request.Context(), c.Request.URL.Path,
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.path", c.Request.URL.Path),
				attribute.String("http.user_agent", c.Request.UserAgent()),
			),
		)
		defer span.End()

		// Store context in gin context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Add response status to span
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int("http.response_size", c.Writer.Size()),
		)
	}
}

// corsMiddleware handles Cross-Origin Resource Sharing
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// rateLimiter holds rate limiters per IP address
type rateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     int // requests per minute
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(ratePerMinute int) *rateLimiter {
	return &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     ratePerMinute,
	}
}

// getLimiter gets or creates a rate limiter for an IP
func (rl *rateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Check again in case another goroutine created it
	if limiter, exists := rl.limiters[ip]; exists {
		return limiter
	}

	// Create limiter: ratePerMinute requests per minute
	limiter = rate.NewLimiter(rate.Limit(rl.rate)/60.0, rl.rate)
	rl.limiters[ip] = limiter

	// TODO: Add cleanup for old limiters (after 1 hour of inactivity)

	return limiter
}

// rateLimitMiddleware implements rate limiting per IP
func rateLimitMiddleware(ratePerMinute int) gin.HandlerFunc {
	limiter := newRateLimiter(ratePerMinute)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		ipLimiter := limiter.getLimiter(ip)

		if !ipLimiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"message": "too many requests from your IP address",
				"retry_after": 60, // seconds
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// authMiddleware validates JWT authentication
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing authorization header",
			})
			c.Abort()
			return
		}

		// Extract Bearer token
		parts := splitAuthHeader(authHeader)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid authorization header format, expected: Bearer <token>",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Validate token using auth library
		jwtService := auth.NewJWTService(auth.DefaultJWTConfig())
		claims, err := jwtService.ValidateAccessToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// Store user info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)

		c.Next()
	}
}

// Helper functions for auth middleware
func splitAuthHeader(header string) []string {
	parts := make([]string, 0, 2)
	spaceIdx := -1
	for i, c := range header {
		if c == ' ' {
			spaceIdx = i
			break
		}
	}
	if spaceIdx > 0 {
		parts = append(parts, header[:spaceIdx])
		parts = append(parts, header[spaceIdx+1:])
	}
	return parts
}

// timeoutMiddleware adds a timeout to request processing
func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		// Channel to signal completion
		done := make(chan struct{})

		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			// Request completed
			return
		case <-ctx.Done():
			// Timeout reached
			c.JSON(http.StatusRequestTimeout, gin.H{
				"error":   "request timeout",
				"message": "request took too long to process",
			})
			c.Abort()
		}
	}
}
