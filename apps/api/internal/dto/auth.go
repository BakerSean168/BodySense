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

// ErrorDetail is the canonical public error payload.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the canonical public error envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{Error: ErrorDetail{Code: code, Message: message}}
}
