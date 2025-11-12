package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/aidenlippert/zerostate/libs/auth"
	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
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

	// Check if user already exists (distinguish not found vs other errors)
	existingUser, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// OK - user does not exist
			h.logger.Debug("email available", zap.String("email", req.Email))
		} else {
			h.logger.Error("user lookup failed", zap.String("email", req.Email), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
	} else if existingUser != nil {
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
	// Generate a DID for the user (in production, this would be more sophisticated)
	userDID := "did:zerostate:user:" + uuid.New().String()

	user := &database.User{
		DID:          userDID,
		Email:        sql.NullString{String: req.Email, Valid: true},
		PasswordHash: sql.NullString{String: passwordHash, Valid: true},
		IsActive:     true,
		Metadata:     json.RawMessage(`{}`), // Initialize to empty JSON object
	}

	// Use UserRepository to create user
	userRepo := database.NewUserRepository(h.db)
	if err := userRepo.Create(c.Request.Context(), user); err != nil {
		// Surface pq error details when possible
		var pqErr *pq.Error
		if errors.Is(err, database.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "user with this email already exists"})
			return
		} else if errors.As(err, &pqErr) {
			h.logger.Error(
				"failed to create user (pq)",
				zap.String("code", string(pqErr.Code)),
				zap.String("constraint", pqErr.Constraint),
				zap.String("detail", pqErr.Detail),
				zap.String("email", req.Email),
			)
		} else {
			h.logger.Error("failed to create user", zap.Error(err), zap.String("email", req.Email))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	h.logger.Info("user registered", zap.String("user_id", user.ID.String()), zap.String("did", user.DID), zap.String("email", req.Email))
	// Generate JWT token
	jwtService := auth.NewJWTService(auth.DefaultJWTConfig())
	tokenPair, err := jwtService.GenerateTokenPair(user.ID, user.DID, user.Email.String, false)
	if err != nil {
		h.logger.Error("failed to generate token: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, LoginResponse{
		Token:        tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User: UserResponse{
			ID:        user.ID.String(),
			Email:     user.Email.String,
			FullName:  req.FullName, // Store in response but not in DB for now
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

	// Get user by email - using GetUserByEmail which returns ErrNotFound
	user, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		if err == database.ErrNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		h.logger.Error("failed to get user: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Check password
	if !user.PasswordHash.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	if err := auth.VerifyPassword(req.Password, user.PasswordHash.String); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	// Generate JWT token
	jwtService := auth.NewJWTService(auth.DefaultJWTConfig())
	tokenPair, err := jwtService.GenerateTokenPair(user.ID, user.DID, user.Email.String, false)
	if err != nil {
		h.logger.Error("failed to generate token: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token:        tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User: UserResponse{
			ID:        user.ID.String(),
			Email:     user.Email.String,
			FullName:  "", // Not stored in DB currently
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
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	userRepo := database.NewUserRepository(h.db)
	user, err := userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		if err == database.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		h.logger.Error("failed to get user: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, UserResponse{
		ID:        user.ID.String(),
		Email:     user.Email.String,
		FullName:  "", // Not stored in DB currently
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
	userIDStr := ""
	if uid, ok := userID.(uuid.UUID); ok {
		userIDStr = uid.String()
	}
	avatarURL := "https://ui-avatars.com/api/?name=" + userIDStr + "&size=200&background=4A90E2&color=fff"

	c.JSON(http.StatusOK, gin.H{
		"avatar_url": avatarURL,
		"message":    "avatar uploaded successfully",
	})
}
