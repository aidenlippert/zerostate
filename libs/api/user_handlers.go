package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterUserRequest represents a user registration request
type RegisterUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required"`
}

// LoginUserRequest represents a login request
type LoginUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserResponse represents a user in API responses
type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	User         UserResponse `json:"user"`
	ExpiresIn    int          `json:"expires_in"` // seconds
}

// RegisterUser handles user registration
func (h *Handlers) RegisterUser(c *gin.Context) {
	// TODO: Implement user registration
	// 1. Validate email uniqueness
	// 2. Hash password (bcrypt)
	// 3. Store user in database
	// 4. Send verification email (optional)

	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "RegisterUser endpoint not yet implemented - Sprint 7 Week 2",
	})
}

// LoginUser handles user login
func (h *Handlers) LoginUser(c *gin.Context) {
	// TODO: Implement user login
	// 1. Validate credentials
	// 2. Generate JWT token
	// 3. Return token and user info

	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "LoginUser endpoint not yet implemented - Sprint 7 Week 2",
	})
}

// LogoutUser handles user logout
func (h *Handlers) LogoutUser(c *gin.Context) {
	// TODO: Implement user logout
	// 1. Invalidate JWT token
	// 2. Clear session

	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "LogoutUser endpoint not yet implemented",
	})
}

// GetCurrentUser retrieves the currently authenticated user
func (h *Handlers) GetCurrentUser(c *gin.Context) {
	// TODO: Implement get current user
	// 1. Extract user from JWT token
	// 2. Return user info

	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not implemented",
		"message": "GetCurrentUser endpoint not yet implemented",
	})
}
