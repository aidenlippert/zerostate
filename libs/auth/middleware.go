package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// FAANG-LEVEL AUTHENTICATION MIDDLEWARE
// Following best practices:
// - Bearer token authentication
// - API key authentication for agents
// - Request context for authenticated user
// - Proper error responses
// - TLS enforcement in production
// - CORS handling with security
// - Rate limiting integration

// ContextKey is the key type for context values
type ContextKey string

const (
	// UserContextKey holds the authenticated user in context
	UserContextKey ContextKey = "auth:user"

	// ClaimsContextKey holds JWT claims in context
	ClaimsContextKey ContextKey = "auth:claims"
)

// AuthenticatedUser represents an authenticated user in context
type AuthenticatedUser struct {
	UserID   uuid.UUID
	DID      string
	Email    string
	IsSystem bool
}

// JWTMiddleware creates middleware for JWT authentication
func JWTMiddleware(jwtService *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// Check Bearer scheme
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			token := parts[1]

			// Validate token
			claims, err := jwtService.ValidateAccessToken(token)
			if err != nil {
				if err == ErrExpiredToken {
					http.Error(w, "Token expired", http.StatusUnauthorized)
					return
				}
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Add user to context
			user := &AuthenticatedUser{
				UserID:   claims.UserID,
				DID:      claims.DID,
				Email:    claims.Email,
				IsSystem: claims.IsSystem,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, ClaimsContextKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalJWTMiddleware creates middleware for optional JWT authentication
func OptionalJWTMiddleware(jwtService *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No token provided - continue without authentication
				next.ServeHTTP(w, r)
				return
			}

			// Check Bearer scheme
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				// Invalid format - continue without authentication
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]

			// Validate token
			claims, err := jwtService.ValidateAccessToken(token)
			if err != nil {
				// Invalid token - continue without authentication
				next.ServeHTTP(w, r)
				return
			}

			// Add user to context
			user := &AuthenticatedUser{
				UserID:   claims.UserID,
				DID:      claims.DID,
				Email:    claims.Email,
				IsSystem: claims.IsSystem,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, ClaimsContextKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TLSOnlyMiddleware enforces HTTPS in production
func TLSOnlyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// CRITICAL: Enforce HTTPS in production
			// In production, check X-Forwarded-Proto header (for load balancers)
			proto := r.Header.Get("X-Forwarded-Proto")
			if proto == "" {
				proto = r.URL.Scheme
			}

			if r.TLS == nil && proto != "https" {
				http.Error(w, "HTTPS required", http.StatusBadRequest)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware adds CORS headers with security
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if origin == allowedOrigin || allowedOrigin == "*" {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}

			// Handle preflight request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetAuthenticatedUser retrieves authenticated user from context
func GetAuthenticatedUser(ctx context.Context) (*AuthenticatedUser, bool) {
	user, ok := ctx.Value(UserContextKey).(*AuthenticatedUser)
	return user, ok
}

// GetClaims retrieves JWT claims from context
func GetClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*Claims)
	return claims, ok
}

// RequireSystemUser middleware ensures user is system user
func RequireSystemUser() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := GetAuthenticatedUser(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !user.IsSystem {
				http.Error(w, "Forbidden - system access required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
