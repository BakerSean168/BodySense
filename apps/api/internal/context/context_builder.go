package context

import (
	"context"
	"encoding/json"
	"log"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

// BuildChatContextInput holds all parameters needed to assemble a ChatStreamRequest.
type BuildChatContextInput struct {
	ConversationID uuid.UUID
	TurnID         uuid.UUID
	UserID         uuid.UUID
	ContextDTO     *dto.ContextDTO
	MessageParts   []dto.PartDTO
	IsDraft        bool
}

// ContextTrace records what was included/excluded during context assembly.
type ContextTrace struct {
	IncludedMessageIDs   []uuid.UUID
	ExcludedCurrentTurn  bool
	ProfileIncluded      bool
	ConsultationIncluded bool
	UseCase              string
}

// Builder assembles a ChatStreamRequest from conversation state.
type Builder interface {
	BuildChatContext(ctx context.Context, input BuildChatContextInput) (*service.ChatStreamRequest, *ContextTrace, error)
}

// ContextBuilder implements Builder using existing service dependencies.
type ContextBuilder struct {
	profileService      *service.ProfileService
	consultationService *service.ConsultationService
	messageService      *service.MessageService
}

// NewContextBuilder creates a ContextBuilder.
func NewContextBuilder(
	profileService *service.ProfileService,
	consultationService *service.ConsultationService,
	messageService *service.MessageService,
) *ContextBuilder {
	return &ContextBuilder{
		profileService:      profileService,
		consultationService: consultationService,
		messageService:      messageService,
	}
}

// BuildChatContext assembles a ChatStreamRequest matching the current inline behavior.
func (b *ContextBuilder) BuildChatContext(ctx context.Context, input BuildChatContextInput) (*service.ChatStreamRequest, *ContextTrace, error) {
	trace := &ContextTrace{
		ExcludedCurrentTurn: true,
	}

	// 1. Extract text content from message parts
	contentText := ExtractTextFromParts(input.MessageParts)

	// 2. Determine use_case from context entry
	useCase := ""
	if input.ContextDTO != nil && input.ContextDTO.Entry != "" {
		useCase = input.ContextDTO.Entry + ".reply"
	}
	trace.UseCase = useCase

	// 3. Load profile
	profileJSON, profileIncluded := b.loadProfile(ctx, input.UserID)
	trace.ProfileIncluded = profileIncluded

	// 4. Load consultation context
	extractedInfoJSON, phase, consultationIncluded := b.loadConsultationContext(ctx, input.ConversationID, input.UserID)
	trace.ConsultationIncluded = consultationIncluded

	// 5. Load conversation history (excluding current turn, completed only)
	chatHistory, includedIDs := b.loadHistory(ctx, input.ConversationID, input.TurnID, input.IsDraft)
	trace.IncludedMessageIDs = includedIDs

	return &service.ChatStreamRequest{
		SessionID:     input.ConversationID.String(),
		UserID:        input.UserID.String(),
		Content:       contentText,
		Messages:      chatHistory,
		Profile:       profileJSON,
		ExtractedInfo: extractedInfoJSON,
		Phase:         phase,
		UseCase:       useCase,
	}, trace, nil
}

// ExtractTextFromParts returns the first text content from message parts.
func ExtractTextFromParts(parts []dto.PartDTO) string {
	for _, part := range parts {
		if part.Type == "text" && part.Text != "" {
			return part.Text
		}
	}
	return ""
}

// loadProfile fetches the user profile and returns JSON. Returns {} on error.
func (b *ContextBuilder) loadProfile(ctx context.Context, userID uuid.UUID) (json.RawMessage, bool) {
	profileJSON := json.RawMessage("{}")
	profile, err := b.profileService.GetProfile(ctx, userID)
	if err == nil && profile != nil {
		if pj, marshalErr := json.Marshal(profile); marshalErr == nil {
			return pj, true
		}
	}
	return profileJSON, false
}

// loadConsultationContext fetches extracted_info and phase from the consultation session.
func (b *ContextBuilder) loadConsultationContext(ctx context.Context, conversationID, userID uuid.UUID) (json.RawMessage, string, bool) {
	var extractedInfoJSON json.RawMessage
	phase := ""

	consultSession, err := b.consultationService.GetConsultation(ctx, conversationID, userID)
	if err == nil && consultSession != nil {
		extractedInfoJSON = json.RawMessage(consultSession.ExtractedInfo)
		phase = consultSession.Phase
		return extractedInfoJSON, phase, true
	}

	if extractedInfoJSON == nil {
		extractedInfoJSON = json.RawMessage("[]")
	}
	return extractedInfoJSON, phase, false
}

// loadHistory loads previous-turn completed messages for context.
// Returns chat messages and the IDs of included messages.
func (b *ContextBuilder) loadHistory(ctx context.Context, conversationID, currentTurnID uuid.UUID, isDraft bool) ([]service.ChatMessage, []uuid.UUID) {
	if isDraft {
		return nil, nil
	}

	historyMsgs, err := b.messageService.GetMessages(ctx, conversationID)
	if err != nil {
		log.Printf("failed to load history for conversation %s: %v", conversationID, err)
		return nil, nil
	}

	var chatHistory []service.ChatMessage
	var includedIDs []uuid.UUID
	for _, m := range historyMsgs {
		// Only include completed messages from previous turns
		if m.TurnID != currentTurnID && m.Status == "completed" {
			text := getMessageTextContent(m)
			if text != "" {
				chatHistory = append(chatHistory, service.ChatMessage{
					Role:    m.Role,
					Content: text,
				})
				includedIDs = append(includedIDs, m.ID)
			}
		}
	}
	return chatHistory, includedIDs
}

// getMessageTextContent extracts text from a message, preferring ContentText.
func getMessageTextContent(msg model.Message) string {
	if msg.ContentText != "" {
		return msg.ContentText
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Parts, &parts); err == nil {
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				return p.Text
			}
		}
	}
	return ""
}
