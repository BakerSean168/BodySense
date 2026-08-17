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
	// 定义的变量，用于存储后续的业务请求数据
	var req dto.LoginRequest
	// 绑定请求体中的 JSON 数据到 req 结构体中，如果绑定失败则返回错误响应
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果请求体中的 JSON 数据无法绑定到 req 结构体中，返回 400 Bad Request 错误响应，提示验证错误
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// 调用对应的业务服务方法进行登录操作，并获取响应数据和错误信息。这个本身只是一个 handle 处理层，它就是要把请求进来的数据（也就是对应的 HTTP 请求中的数据）提取出来，然后调用相关的业务方法进行业务上的处理。比如说，这个 authService.Login 就是进行登录的业务处理。这本质上体现了 Go 的一个分层思想，把 handle 和对应的业务分开了，然后通过函数来调用，进行一个解耦。后续如果 handle 的内容需要修改，只需要修改这里，业务的修改也和它分离开了。
	// c.Request.Context() 是获取当前请求的上下文信息，传入 context 能够让 go 实现对流程的一个控制，后续可以用于超时控制等，不传下去go就不能进行控制，也会失去横切的一些必要数据。req 是前面绑定的请求数据结构体。这个方法会返回一个响应数据结构体和一个错误信息。
	resp, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		// 如果登录失败，返回 401 Unauthorized 错误响应，提示用户邮箱或密码无效
		// 避免给攻击者提供过多信息，通常不建议在错误消息中透露具体的失败原因（例如“邮箱不存在”或“密码错误”），以防止潜在的暴力破解攻击。
		respondError(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "invalid email or password")
		return
	}

	// 返回成功的 JSON 响应，包含登录成功后的数据
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

// Logout handles user logout (invalidates refresh token + session cache).
func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		respondError(c, http.StatusInternalServerError, "LOGOUT_FAILED", "failed to logout")
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

	uid, ok := userID.(string)
	if !ok {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "invalid user id type")
		return
	}

	email, _ := c.Get("email")
	emailStr, _ := email.(string)

	c.JSON(http.StatusOK, dto.UserResponse{
		ID:    uid,
		Email: emailStr,
	})
}
