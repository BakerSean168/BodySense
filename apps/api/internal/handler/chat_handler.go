package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	// maxSSEEvents caps the number of events processed from the AI stream
	// to prevent unbounded memory growth from a compromised AI service.
	maxSSEEvents = 10000
	// sseTimeout is the maximum duration for the SSE streaming loop.
	sseTimeout = 5 * time.Minute
)

// ChatHandler handles the core chat SSE endpoint.
type ChatHandler struct {
	conversationService *service.ConversationService
	messageService      *service.MessageService
	runService          *service.RunService
	consultationService *service.ConsultationService
	aiClient            *service.AIClient
	profileService      *service.ProfileService
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(
	conversationService *service.ConversationService,
	messageService *service.MessageService,
	runService *service.RunService,
	consultationService *service.ConsultationService,
	aiClient *service.AIClient,
	profileService *service.ProfileService,
) *ChatHandler {
	return &ChatHandler{
		conversationService: conversationService,
		messageService:      messageService,
		runService:          runService,
		consultationService: consultationService,
		aiClient:            aiClient,
		profileService:      profileService,
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

	// 2. Idempotency check — propagate DB errors as 500
	existing, found, err := h.runService.CheckIdempotency(c.Request.Context(), uid, req.RequestID)
	if err != nil {
		log.Printf("idempotency check failed for user %s requestID %s: %v", uid, req.RequestID, err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check idempotency")
		return
	}
	if found {
		if existing.Status == "running" {
			respondError(c, http.StatusConflict, "RUN_IN_PROGRESS", "a run with this request ID is already in progress")
			return
		}
		// Idempotent hit: replay completed message via SSE
		h.replayCompletedRun(c, existing)
		return
	}

	// 3. Determine conversation (new or existing)
	var conversationID uuid.UUID
	var isDraft bool

	if req.ConversationID != nil {
		var parseErr error
		conversationID, parseErr = uuid.Parse(*req.ConversationID)
		if parseErr != nil {
			respondError(c, http.StatusBadRequest, "INVALID_CONVERSATION_ID", "invalid conversation id format")
			return
		}
	} else {
		conv, createErr := h.conversationService.CreateConversation(c.Request.Context(), uid, "")
		if createErr != nil {
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create conversation")
			return
		}
		conversationID = conv.ID
		isDraft = true

		// Create consultation_session if entry is 'consultation'
		if req.Context != nil && req.Context.Entry == "consultation" {
			if csErr := h.consultationService.CreateConsultation(c.Request.Context(), conversationID, uid); csErr != nil {
				log.Printf("failed to create consultation session for conversation %s: %v", conversationID, csErr)
			}
		}
	}

	// 4. Get next seq numbers
	userSeq, err := h.messageService.GetNextSeq(c.Request.Context(), conversationID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get message sequence")
		return
	}
	assistantSeq := userSeq + 1

	// 5. Create user message
	turnID := uuid.New()
	partsJSON, err := json.Marshal(req.Message.Parts)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to marshal message parts")
		return
	}

	userMsg, err := h.messageService.CreateMessage(c.Request.Context(), conversationID, turnID, "user", datatypes.JSON(partsJSON), userSeq, "completed")
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create user message")
		return
	}

	// 6. Create assistant placeholder (status = streaming)
	assistantMsg, err := h.messageService.CreateMessage(c.Request.Context(), conversationID, turnID, "assistant", datatypes.JSON("[]"), assistantSeq, "streaming")
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create assistant message")
		return
	}

	// 7. Create run
	run, err := h.runService.CreateRun(c.Request.Context(), conversationID, turnID, req.RequestID, uid, "")
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create run")
		return
	}

	// 8. Set active_run_id on conversation for disconnect recovery
	runIDStr := run.ID.String()
	if updateErr := h.conversationService.UpdateActiveRunID(c.Request.Context(), conversationID, uid, &run.ID, runIDStr); updateErr != nil {
		log.Printf("failed to set active_run_id for conversation %s: %v", conversationID, updateErr)
	}

	// 9. Set SSE headers
	sse := NewSSEWriter(c.Writer)

	// 10. If new conversation, send conversation.created
	if isDraft {
		draftID := ""
		if req.ClientDraftID != "" {
			draftID = req.ClientDraftID
		}
		if err := sse.ConversationCreated(conversationID.String(), draftID); err != nil {
			log.Printf("SSE write error (conversation.created): %v", err)
		}
	}

	// 11. Send message.persisted
	if err := sse.MessagePersisted(req.ClientMessageID, userMsg.ID.String(), "user"); err != nil {
		log.Printf("SSE write error (message.persisted): %v", err)
	}

	// 12. Send message.created
	if err := sse.MessageCreated(assistantMsg.ID.String(), "assistant", turnID.String()); err != nil {
		log.Printf("SSE write error (message.created): %v", err)
	}

	// 13. Load context and call AI
	profile, err := h.profileService.GetProfile(c.Request.Context(), uid)
	profileJSON := json.RawMessage("{}")
	if err == nil && profile != nil {
		if pj, marshalErr := json.Marshal(profile); marshalErr == nil {
			profileJSON = pj
		}
	}

	// Load consultation context (extracted_info, phase) if available
	var extractedInfoJSON json.RawMessage
	phase := ""
	consultSession, err := h.consultationService.GetConsultation(c.Request.Context(), conversationID, uid)
	if err == nil && consultSession != nil {
		extractedInfoJSON = json.RawMessage(consultSession.ExtractedInfo)
		phase = consultSession.Phase
	}
	if extractedInfoJSON == nil {
		extractedInfoJSON = json.RawMessage("[]")
	}

	// Extract text content from message parts
	var contentText string
	for _, part := range req.Message.Parts {
		if part.Type == "text" && part.Text != "" {
			contentText = part.Text
			break
		}
	}

	// Determine use_case from context entry
	useCase := ""
	if req.Context != nil && req.Context.Entry != "" {
		useCase = req.Context.Entry + ".reply"
	}

	chatReq := service.ChatStreamRequest{
		Messages: []service.ChatMessage{
			{Role: "user", Content: contentText},
		},
		Context: service.ChatContext{
			UserID:        uid.String(),
			SessionID:     conversationID.String(),
			Profile:       profileJSON,
			ExtractedInfo: extractedInfoJSON,
			Phase:         phase,
		},
		UseCase: useCase,
		Stream:  true,
	}

	// Apply timeout to the AI service call
	ctx, cancel := context.WithTimeout(c.Request.Context(), sseTimeout)
	defer cancel()

	events, err := h.aiClient.ChatStream(ctx, chatReq)
	if err != nil {
		_ = sse.MessageFailed(assistantMsg.ID.String(), gin.H{"message": "AI service unavailable"})
		_ = sse.Done()
		_ = h.runService.FailRun(c.Request.Context(), run.ID, uid, gin.H{"message": "AI service connection failed"})
		return
	}

	// 14. Stream events to SSE client — collect all parts, detect disconnect
	var assistantParts []map[string]any
	var usage any
	var providerResponseID string
	eventCount := 0
	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			// Client disconnected — mark run as cancelled
			log.Printf("client disconnected during stream for conversation %s", conversationID)
			_ = h.runService.FailRun(context.Background(), run.ID, uid, gin.H{"message": "client disconnected"})
			_ = h.messageService.UpdateMessageStatus(context.Background(), assistantMsg.ID, conversationID, "aborted")
			h.clearActiveRun(context.Background(), conversationID, uid)
			return

		case event, ok := <-events:
			if !ok {
				// Channel closed — stream done
				goto streamDone
			}

			eventCount++
			if eventCount > maxSSEEvents {
				log.Printf("event count exceeded limit for conversation %s", conversationID)
				_ = sse.MessageFailed(assistantMsg.ID.String(), gin.H{"message": "stream exceeded maximum event count"})
				_ = sse.Done()
				_ = h.runService.FailRun(c.Request.Context(), run.ID, uid, gin.H{"message": "event count limit exceeded"})
				h.clearActiveRun(c.Request.Context(), conversationID, uid)
				return
			}

			switch event.Type {
			case "text":
				if err := sse.TextDelta(assistantMsg.ID.String(), event.Delta); err != nil {
					log.Printf("SSE write error (text.delta): %v", err)
				}
				assistantParts = append(assistantParts, map[string]any{"type": "text", "text": event.Delta})

			case "tool_call":
				if err := sse.ToolCall(assistantMsg.ID.String(), event.Tool, event.Args); err != nil {
					log.Printf("SSE write error (tool.call): %v", err)
				}
				assistantParts = append(assistantParts, map[string]any{"type": "tool_call", "tool": event.Tool, "args": json.RawMessage(event.Args)})

			case "tool_result":
				if err := sse.ToolResult(assistantMsg.ID.String(), event.Tool, event.Result); err != nil {
					log.Printf("SSE write error (tool.result): %v", err)
				}
				assistantParts = append(assistantParts, map[string]any{"type": "tool_result", "tool": event.Tool, "result": json.RawMessage(event.Result)})

			case "extracted_info":
				if err := sse.ExtractedInfo(assistantMsg.ID.String(), event.Info); err != nil {
					log.Printf("SSE write error (extracted_info): %v", err)
				}

			case "phase_changed":
				// Pass current phase as 'from', then update local phase
				if err := sse.PhaseChange(assistantMsg.ID.String(), phase, event.Phase, event.Reason); err != nil {
					log.Printf("SSE write error (phase_change): %v", err)
				}
				phase = event.Phase
				if err := h.consultationService.UpdatePhase(c.Request.Context(), conversationID, uid, event.Phase); err != nil {
					log.Printf("failed to update phase for conversation %s: %v", conversationID, err)
				}

			case "red_flag":
				if err := sse.RedFlag(assistantMsg.ID.String(), event.Flag); err != nil {
					log.Printf("SSE write error (red_flag): %v", err)
				}

			case "citation":
				if err := sse.Citation(assistantMsg.ID.String(), event.Citation); err != nil {
					log.Printf("SSE write error (citation): %v", err)
				}

			case "usage":
				usage = event.Usage

			case "response_id":
				providerResponseID = event.ResponseID

			case "done":
				// Stream finished
			}
		}
	}

streamDone:

	// 15. Persist: merge all collected parts
	finalPartsJSON, _ := json.Marshal(assistantParts)

	if err := h.messageService.UpdateMessageCompleted(
		c.Request.Context(), assistantMsg.ID, conversationID,
		datatypes.JSON(finalPartsJSON), nil, nil,
	); err != nil {
		log.Printf("failed to update assistant message %s: %v", assistantMsg.ID, err)
	}

	// Update conversation last_message_at
	if err := h.conversationService.UpdateLastMessageAt(c.Request.Context(), conversationID, uid); err != nil {
		log.Printf("failed to update last_message_at for conversation %s: %v", conversationID, err)
	}

	// Complete run
	if err := h.runService.CompleteRun(c.Request.Context(), run.ID, uid, usage, providerResponseID); err != nil {
		log.Printf("failed to complete run %s: %v", run.ID, err)
	}

	// Clear active_run_id
	h.clearActiveRun(c.Request.Context(), conversationID, uid)

	// Send message.completed and done
	_ = sse.MessageCompleted(assistantMsg.ID.String(), usage)
	_ = sse.Done()

	// 16. Auto-trigger title generation for first message
	h.maybeGenerateTitle(c.Request.Context(), conversationID, uid)
}

// replayCompletedRun replays a completed run's message as an SSE stream
// when an idempotent request is detected (requestId already exists and completed).
func (h *ChatHandler) replayCompletedRun(c *gin.Context, run *model.Run) {
	sse := NewSSEWriter(c.Writer)

	// Send SSE events so the client can reconcile with existing data
	if err := sse.sendEvent("idempotent", map[string]any{
		"runId":   run.ID.String(),
		"status":  run.Status,
		"message": "this request has already been processed",
	}); err != nil {
		log.Printf("SSE write error (idempotent): %v", err)
	}
	_ = sse.Done()
}

// clearActiveRun clears the active_run_id and active_stream_id on a conversation.
func (h *ChatHandler) clearActiveRun(ctx context.Context, conversationID, userID uuid.UUID) {
	if err := h.conversationService.UpdateActiveRunID(ctx, conversationID, userID, nil, ""); err != nil {
		log.Printf("failed to clear active_run_id for conversation %s: %v", conversationID, err)
	}
}

// maybeGenerateTitle triggers async title generation if this is the first message.
func (h *ChatHandler) maybeGenerateTitle(ctx context.Context, conversationID, userID uuid.UUID) {
	conv, err := h.conversationService.GetConversationByID(ctx, conversationID, userID)
	if err != nil || conv == nil {
		return
	}
	if conv.TitleStatus == "pending" && conv.Title == "" {
		if err := h.conversationService.GenerateTitle(ctx, conversationID, userID); err != nil {
			log.Printf("failed to trigger title generation for conversation %s: %v", conversationID, err)
		}
	}
}
