package dto

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
