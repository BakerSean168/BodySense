package dto

// RegisterRequest is the request body for user registration.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginRequest is the request body for user login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest is the request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// AuthResponse is the response body for authentication endpoints.
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"-"`
	ExpiresIn    int64  `json:"expires_in"` // Access token expiry in seconds
}

// UserResponse is the response body for user data.
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// ErrorResponse is the response body for errors.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
