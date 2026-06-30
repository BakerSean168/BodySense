package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/chat"
	ctxbuilder "github.com/bodysense/api/internal/context"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChatHandler handles the core chat SSE endpoint.
type ChatHandler struct {
	runtime *chat.Runtime
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(
	conversationService *service.ConversationService,
	messageService *service.MessageService,
	runService *service.RunService,
	consultationService *service.ConsultationService,
	aiClient *service.AIClient,
	profileService *service.ProfileService,
	agentToolService *service.AgentToolService,
	interactionService *service.AgentInteractionService,
	outputReviewService *service.OutputReviewService,
) *ChatHandler {
	cb := ctxbuilder.NewContextBuilder(profileService, consultationService, messageService)
	return &ChatHandler{
		runtime: chat.NewRuntime(
			conversationService,
			messageService,
			runService,
			consultationService,
			aiClient,
			cb,
			agentToolService,
			interactionService,
			outputReviewService,
		),
	}
}

// SendMessage handles POST /api/v1/chat — the core SSE streaming endpoint.
func (h *ChatHandler) SendMessage(c *gin.Context) {
	// 1. Parse request
	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	if err := h.runtime.SendMessage(c.Request.Context(), c.Writer, uid, req); err != nil {
		respondError(c, err.Status, err.Code, err.Message)
		return
	}
}
