package api

import (
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/auth"
	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegisterUserRequest represents a user registration request
type RegisterUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required"`
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
	FullName  string `json:"full_name"`
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
	var req RegisterUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	existingUser, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		h.logger.Error("failed to check existing user: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user with this email already exists"})
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Create user
	user := &database.User{
		ID:           uuid.New().String(),
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.CreateUser(user); err != nil {
		h.logger.Error("failed to create user: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.ID, user.Email)
	if err != nil {
		h.logger.Error("failed to generate token: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, LoginResponse{
		Token: token,
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			FullName:  user.FullName,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		},
		ExpiresIn: 86400, // 24 hours
	})
}

// LoginUser handles user login
func (h *Handlers) LoginUser(c *gin.Context) {
	var req LoginUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user by email
	user, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		h.logger.Error("failed to get user: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	// Check password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.ID, user.Email)
	if err != nil {
		h.logger.Error("failed to generate token: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			FullName:  user.FullName,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		},
		ExpiresIn: 86400, // 24 hours
	})
}

// LogoutUser handles user logout
func (h *Handlers) LogoutUser(c *gin.Context) {
	// For JWT, logout is handled client-side by removing the token
	// We could implement token blacklisting here if needed
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// GetCurrentUser retrieves the currently authenticated user
func (h *Handlers) GetCurrentUser(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.db.GetUserByID(userID.(string))
	if err != nil {
		h.logger.Error("failed to get user: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		FullName:  user.FullName,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	})
}

// UploadAvatar handles user avatar upload
// For simplicity, this returns a placeholder URL
// In production, this would upload to cloud storage (S3, GCS, etc.)
func (h *Handlers) UploadAvatar(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Parse multipart form (10MB max)
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "failed to parse form data",
		})
		return
	}

	// Get file from request
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "no file uploaded",
		})
		return
	}
	defer file.Close()

	// Validate file type (only images)
	contentType := header.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" && contentType != "image/webp" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid file type",
			"message": "only image files (JPEG, PNG, GIF, WebP) are allowed",
		})
		return
	}

	// Validate file size (max 5MB)
	if header.Size > 5<<20 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "file too large",
			"message": "maximum file size is 5MB",
		})
		return
	}

	// In a real implementation, you would:
	// 1. Generate a unique filename
	// 2. Upload to cloud storage (S3, GCS, Cloudinary, etc.)
	// 3. Store the URL in the database
	// 4. Return the URL

	// For now, return a placeholder URL with user ID
	avatarURL := "https://ui-avatars.com/api/?name=" + userID.(string) + "&size=200&background=4A90E2&color=fff"

	c.JSON(http.StatusOK, gin.H{
		"avatar_url": avatarURL,
		"message":    "avatar uploaded successfully",
	})
}
