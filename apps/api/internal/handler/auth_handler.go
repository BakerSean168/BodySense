package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register handles user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "REGISTRATION_FAILED"

		if err.Error() == "registration failed" {
			status = http.StatusConflict
		}

		respondError(c, status, code, "registration failed")
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// Login handles user login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "invalid email or password")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// RefreshToken handles token refresh.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	resp, err := h.authService.RefreshToken(c.Request.Context(), req)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "REFRESH_FAILED", "invalid or expired refresh token")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Logout handles user logout (invalidates refresh token).
func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.authService.InvalidateRefreshToken(c.Request.Context(), req.RefreshToken); err != nil {
		respondError(c, http.StatusInternalServerError, "LOGOUT_FAILED", "failed to invalidate refresh token")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// Me returns the current authenticated user info.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	email, _ := c.Get("email")

	c.JSON(http.StatusOK, dto.UserResponse{
		ID:    userID.(string),
		Email: email.(string),
	})
}
