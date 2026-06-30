package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	ctxbuilder "github.com/bodysense/api/internal/context"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
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
	conversationService   *service.ConversationService
	messageService        *service.MessageService
	runService            *service.RunService
	consultationService   *service.ConsultationService
	aiClient              *service.AIClient
	profileService        *service.ProfileService
	agentToolService      *service.AgentToolService
	interactionService    *service.AgentInteractionService
	outputReviewService   *service.OutputReviewService
	contextBuilder        ctxbuilder.Builder
	streamRuntime         *stream.Runtime
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
		conversationService:   conversationService,
		messageService:        messageService,
		runService:            runService,
		consultationService:   consultationService,
		aiClient:              aiClient,
		profileService:        profileService,
		agentToolService:      agentToolService,
		interactionService:    interactionService,
		outputReviewService:   outputReviewService,
		contextBuilder:        cb,
		streamRuntime:         stream.NewRuntime(),
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

	// 9. Create stream writer with base IDs
	baseIDs := dto.StreamEventIDs{
		ConversationID: conversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         turnID.String(),
	}
	sw := h.streamRuntime.NewWriter(c.Writer, baseIDs)

	// 10. If new conversation, send conversation.created
	if isDraft {
		draftID := ""
		if req.ClientDraftID != "" {
			draftID = req.ClientDraftID
		}
		if err := sw.SendNew(c.Request.Context(), "conversation", "conversation.created", baseIDs, gin.H{"replaces_draft_id": draftID}, ""); err != nil {
			log.Printf("SSE write error (conversation.created): %v", err)
		}
	}

	// 11. Send message.persisted
	if err := sw.SendNew(c.Request.Context(), "message", "message.persisted", dto.StreamEventIDs{
		ConversationID: conversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         turnID.String(),
		MessageID:      userMsg.ID.String(),
	}, gin.H{"client_message_id": req.ClientMessageID, "role": "user"}, userMsg.ID.String()); err != nil {
		log.Printf("SSE write error (message.persisted): %v", err)
	}

	// 12. Send message.created
	if err := sw.SendNew(c.Request.Context(), "message", "message.created", dto.StreamEventIDs{
		ConversationID: conversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         turnID.String(),
		MessageID:      assistantMsg.ID.String(),
	}, gin.H{"role": "assistant", "status": "streaming"}, assistantMsg.ID.String()); err != nil {
		log.Printf("SSE write error (message.created): %v", err)
	}

	// 13. Build context and call AI
	chatReq, _, err := h.contextBuilder.BuildChatContext(c.Request.Context(), ctxbuilder.BuildChatContextInput{
		ConversationID: conversationID,
		TurnID:         turnID,
		UserID:         uid,
		ContextDTO:     req.Context,
		MessageParts:   req.Message.Parts,
		IsDraft:        isDraft,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to build chat context")
		return
	}

	// Extract phase for state.phase.changed event handling below
	phase := chatReq.Phase

	// Apply timeout to the AI service call
	ctx, cancel := context.WithTimeout(c.Request.Context(), sseTimeout)
	defer cancel()

	events, err := h.aiClient.ChatStream(ctx, *chatReq)
	if err != nil {
		_ = sw.SendNew(c.Request.Context(), "message", "message.failed", dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         turnID.String(),
			MessageID:      assistantMsg.ID.String(),
		}, gin.H{"status": "failed", "error": gin.H{"message": "AI service unavailable"}}, assistantMsg.ID.String())
		_ = sw.SendNew(c.Request.Context(), "stream", "stream.done", baseIDs, gin.H{}, "")
		_ = h.runService.FailRun(c.Request.Context(), run.ID, uid, gin.H{"message": "AI service connection failed"})
		return
	}

	// 14. Stream events to SSE client — collect all parts, detect disconnect
	var assistantParts []map[string]any
	var usage any
	var providerResponseID string
	var governanceResult datatypes.JSON
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
				_ = sw.SendNew(c.Request.Context(), "message", "message.failed", dto.StreamEventIDs{
					ConversationID: conversationID.String(),
					RunID:          run.ID.String(),
					TurnID:         turnID.String(),
					MessageID:      assistantMsg.ID.String(),
				}, gin.H{"status": "failed", "error": gin.H{"message": "stream exceeded maximum event count"}}, assistantMsg.ID.String())
				_ = sw.SendNew(c.Request.Context(), "stream", "stream.done", baseIDs, gin.H{}, "")
				_ = h.runService.FailRun(c.Request.Context(), run.ID, uid, gin.H{"message": "event count limit exceeded"})
				h.clearActiveRun(c.Request.Context(), conversationID, uid)
				return
			}

			switch event.Type {
			case "message.text.delta":
				var payload struct {
					Delta string `json:"delta"`
				}
				_ = event.PayloadAs(&payload)
				if err := sw.Send(c.Request.Context(), event, assistantMsg.ID.String()); err != nil {
					log.Printf("SSE write error (text.delta): %v", err)
				}
				assistantParts = append(assistantParts, map[string]any{"type": "text", "text": payload.Delta})

			case "tool.call":
				var payload struct {
					Tool string          `json:"tool"`
					Args json.RawMessage `json:"args"`
				}
				_ = event.PayloadAs(&payload)
				if err := sw.Send(c.Request.Context(), event, assistantMsg.ID.String()); err != nil {
					log.Printf("SSE write error (tool.call): %v", err)
				}
				assistantParts = append(assistantParts, map[string]any{"type": "tool_call", "tool": payload.Tool, "args": payload.Args})
				// Audit: persist tool call as running
				msgID := assistantMsg.ID
				h.agentToolService.RecordToolCall(
					c.Request.Context(), run.ID, conversationID, &msgID,
					event.IDs.ToolCallID, payload.Tool, datatypes.JSON(payload.Args),
				)

			case "tool.result":
				var payload struct {
					Tool   string          `json:"tool"`
					Result json.RawMessage `json:"result"`
				}
				_ = event.PayloadAs(&payload)
				if err := sw.Send(c.Request.Context(), event, assistantMsg.ID.String()); err != nil {
					log.Printf("SSE write error (tool.result): %v", err)
				}
				assistantParts = append(assistantParts, map[string]any{"type": "tool_result", "tool": payload.Tool, "result": payload.Result})
				// Audit: persist tool result
				h.agentToolService.RecordToolResult(
					c.Request.Context(), run.ID, event.IDs.ToolCallID,
					datatypes.JSON(payload.Result), false,
				)

			case "state.extracted_info.upsert", "state.diagnosis.ready", "state.treatment.ready", "source.citation.added", "safety.red_flag.detected":
				if err := sw.Send(c.Request.Context(), event, assistantMsg.ID.String()); err != nil {
					log.Printf("SSE write error (extracted_info): %v", err)
				}

			case "state.interaction.required":
				// Persist the interaction and mark run as waiting_user
				var iPayload struct {
					InteractionID string          `json:"interaction_id"`
					Question      json.RawMessage `json:"question"`
				}
				_ = event.PayloadAs(&iPayload)

				// Generate interaction_id if Python didn't provide one
				interactionID := iPayload.InteractionID
				if interactionID == "" {
					interactionID = uuid.New().String()
				}

				// Persist interaction
				if createErr := h.interactionService.CreatePendingInteraction(
					c.Request.Context(), run.ID, conversationID,
					event.IDs.ToolCallID, datatypes.JSON(iPayload.Question),
				); createErr != nil {
					log.Printf("failed to create pending interaction for conversation %s: %v", conversationID, createErr)
				}

				// Forward event to frontend with interaction_id in IDs
				event.IDs.InteractionID = interactionID
				if err := sw.Send(c.Request.Context(), event, assistantMsg.ID.String()); err != nil {
					log.Printf("SSE write error (interaction.required): %v", err)
				}

				// Mark run as waiting_user
				if err := h.runService.MarkWaitingUser(c.Request.Context(), run.ID); err != nil {
					log.Printf("failed to mark run %s as waiting_user: %v", run.ID, err)
				}

				// Mark assistant message as aborted (will be completed on resume)
				_ = h.messageService.UpdateMessageStatus(c.Request.Context(), assistantMsg.ID, conversationID, "aborted")

				// End the stream — the run is paused until user answers
				_ = sw.SendNew(c.Request.Context(), "stream", "stream.done", baseIDs, gin.H{}, "")
				h.clearActiveRun(c.Request.Context(), conversationID, uid)
				return

			case "state.phase.changed":
				var payload struct {
					From   string `json:"from,omitempty"`
					To     string `json:"to"`
					Reason string `json:"reason"`
				}
				_ = event.PayloadAs(&payload)
				if payload.From == "" {
					payload.From = phase
				}
				if patched, marshalErr := json.Marshal(payload); marshalErr == nil {
					event.Payload = patched
				}
				if err := sw.Send(c.Request.Context(), event, assistantMsg.ID.String()); err != nil {
					log.Printf("SSE write error (phase_change): %v", err)
				}
				phase = payload.To
				if err := h.consultationService.UpdatePhase(c.Request.Context(), conversationID, uid, payload.To); err != nil {
					log.Printf("failed to update phase for conversation %s: %v", conversationID, err)
				}

			case "usage.reported":
				var payload struct {
					Usage json.RawMessage `json:"usage"`
				}
				_ = event.PayloadAs(&payload)
				usage = payload.Usage
				if err := sw.Send(c.Request.Context(), event, assistantMsg.ID.String()); err != nil {
					log.Printf("SSE write error (usage.reported): %v", err)
				}

			case "stream.done":
				var payload struct {
					ResponseID string          `json:"response_id"`
					Usage      json.RawMessage `json:"usage"`
					Governance json.RawMessage `json:"governance"`
				}
				_ = event.PayloadAs(&payload)
				if payload.ResponseID != "" {
					providerResponseID = payload.ResponseID
				}
				if len(payload.Usage) > 0 {
					usage = payload.Usage
				}
				if len(payload.Governance) > 0 {
					governanceResult = datatypes.JSON(payload.Governance)
				}
			case "stream.error":
				if err := sw.Send(c.Request.Context(), event, assistantMsg.ID.String()); err != nil {
					log.Printf("SSE write error (stream.error): %v", err)
				}
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

	// Observe-only governance persistence (non-blocking)
	if len(governanceResult) > 0 {
		var govStatus string
		var govIssues datatypes.JSON
		// Extract status from governance result for the review record
		var govPayload struct {
			Status string          `json:"status"`
			Issues json.RawMessage `json:"issues"`
		}
		if jsonErr := json.Unmarshal(governanceResult, &govPayload); jsonErr == nil {
			govStatus = govPayload.Status
			govIssues = datatypes.JSON(govPayload.Issues)
		} else {
			govStatus = "unknown"
			govIssues = datatypes.JSON("[]")
		}
		h.outputReviewService.RecordReview(
			c.Request.Context(),
			"consultation_reply", govStatus,
			&uid, &run.ID, nil, &conversationID,
			govIssues, nil, governanceResult,
		)
	}

	// Clear active_run_id
	h.clearActiveRun(c.Request.Context(), conversationID, uid)

	// Send message.completed and done
	_ = sw.SendNew(c.Request.Context(), "message", "message.completed", dto.StreamEventIDs{
		ConversationID: conversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         turnID.String(),
		MessageID:      assistantMsg.ID.String(),
	}, gin.H{"status": "completed", "finish_reason": "stop", "usage": usage}, assistantMsg.ID.String())
	_ = sw.SendNew(c.Request.Context(), "stream", "stream.done", baseIDs, gin.H{}, "")

	// 16. Auto-trigger title generation for first message
	h.maybeGenerateTitle(c.Request.Context(), conversationID, uid)
}

// replayCompletedRun replays a completed run's message as an SSE stream
// when an idempotent request is detected (requestId already exists and completed).
func (h *ChatHandler) replayCompletedRun(c *gin.Context, run *model.Run) {
	baseIDs := dto.StreamEventIDs{
		ConversationID: run.ConversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         run.TurnID.String(),
	}
	sw := h.streamRuntime.NewWriter(c.Writer, baseIDs)

	_ = sw.SendNew(c.Request.Context(), "stream", "stream.error", baseIDs, gin.H{"message": "this request has already been processed", "status": run.Status}, "")
	_ = sw.SendNew(c.Request.Context(), "stream", "stream.done", baseIDs, gin.H{}, "")
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
