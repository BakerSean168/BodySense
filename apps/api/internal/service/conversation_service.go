package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const defaultTitle = "新对话"

type conversationRepo interface {
	GetOrCreateForUser(ctx context.Context, userID uuid.UUID) (*model.Conversation, bool, error)
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error)
	UpdateTitleGeneration(ctx context.Context, id, userID uuid.UUID, title, status, configurationID string, configuration, provenance, decisionTrace []byte) error
	UpdateTitleStatus(ctx context.Context, id, userID uuid.UUID, status string) error
	UpdateLastMessageAt(ctx context.Context, id, userID uuid.UUID) error
	UpdateActiveRunID(ctx context.Context, id, userID uuid.UUID, runID *uuid.UUID, streamID string) error
}

type messageRepo interface {
	Create(ctx context.Context, message *model.Message) error
	GetByID(ctx context.Context, id, conversationID uuid.UUID) (*model.Message, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Message, error)
	UpdateStatus(ctx context.Context, id, conversationID uuid.UUID, status string) error
	FailStreamingAssistantByRunID(ctx context.Context, runID, conversationID uuid.UUID, errorPayload any) (*model.Message, error)
	UpdateParts(ctx context.Context, id, conversationID uuid.UUID, parts any) error
	UpdateCompleted(ctx context.Context, id, conversationID uuid.UUID, parts any, usage map[string]any, providerInfo map[string]any) error
	UpdateCompletedWithStatus(ctx context.Context, id, conversationID uuid.UUID, parts any, status string) error
	GetNextSeq(ctx context.Context, conversationID uuid.UUID) (int, error)
}

type runRepo interface {
	Create(ctx context.Context, run *model.Run) error
	CreateWithIdempotency(ctx context.Context, run *model.Run) (*model.Run, bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Run, error)
	GetByRequestID(ctx context.Context, userID uuid.UUID, requestID string) (*model.Run, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Run, error)
	MarkWaitingUser(ctx context.Context, id uuid.UUID) (bool, error)
	ResumeRunning(ctx context.Context, id uuid.UUID, owner string, expiresAt time.Time) (bool, error)
	FailWaitingUser(ctx context.Context, id uuid.UUID, errJSON any) (bool, error)
	CompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) error
	TryCompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) (bool, error)
	CancelRun(ctx context.Context, id, userID uuid.UUID, reason any) (bool, error)
	FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) (bool, error)
	UpdateAgentConfiguration(ctx context.Context, id uuid.UUID, configurationID string, configuration datatypes.JSON, provenance datatypes.JSON) error
	RenewLease(ctx context.Context, id, userID uuid.UUID, owner string, expiresAt, heartbeatAt time.Time) (bool, error)
	ListExpiredRuns(ctx context.Context, now time.Time, limit int) ([]model.Run, error)
	FailExpiredRun(ctx context.Context, id uuid.UUID, now time.Time, errJSON any) (bool, error)
}

// ConversationService handles the user's single Conversation (PC-D4).
type ConversationService struct {
	conversationRepo conversationRepo
	messageRepo      messageRepo
	runRepo          runRepo
	aiClient         *AIClient
	txManager        *database.TransactionManager
	deployment       *AgentDeploymentPolicy
}

// NewConversationService creates a new ConversationService.
func NewConversationService(
	conversationRepo conversationRepo,
	messageRepo messageRepo,
	runRepo runRepo,
	aiClient *AIClient,
	txManager *database.TransactionManager,
) *ConversationService {
	return &ConversationService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		runRepo:          runRepo,
		aiClient:         aiClient,
		txManager:        txManager,
	}
}

// WithAgentDeployment attaches the Go-owned immutable Agent configuration pointer.
func (s *ConversationService) WithAgentDeployment(deployment *AgentDeploymentPolicy) *ConversationService {
	s.deployment = deployment
	return s
}

func (s *ConversationService) titleConfigurationID() string {
	if s.deployment != nil {
		return s.deployment.TitleConfigurationID()
	}
	return defaultTitleConfigurationID
}

// GetOrCreateConversation returns the user's single Conversation and its
// ordered messages, lazily creating the Conversation on first use. Concurrent
// first calls from several devices converge on the same identity.
func (s *ConversationService) GetOrCreateConversation(ctx context.Context, userID uuid.UUID) (*model.Conversation, []model.Message, error) {
	conversation, _, err := s.conversationRepo.GetOrCreateForUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get or create conversation: %w", err)
	}
	messages, err := s.messageRepo.ListByConversationID(ctx, conversation.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list messages: %w", err)
	}
	return conversation, messages, nil
}

// GetConversationByID retrieves a conversation by ID with ownership check (without messages).
func (s *ConversationService) GetConversationByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return conversation, nil
}

// FinalizeExecutionLostProjection atomically converts the durable conversation
// projection for a reclaimed run from "active/streaming" to "failed/inactive".
// ReclaimExpiredRuns decides the terminal winner first; this method makes the
// persisted assistant message and conversation pointer agree before the public
// run.failed event becomes observable to recovering browsers.
func (s *ConversationService) FinalizeExecutionLostProjection(
	ctx context.Context,
	run *model.Run,
) (*model.Message, error) {
	if run == nil {
		return nil, nil
	}
	if s.txManager == nil {
		return nil, errors.New("finalize execution lost: transaction manager is not configured")
	}
	if s.messageRepo == nil || s.conversationRepo == nil {
		return nil, errors.New("finalize execution lost: conversation/message repository is not configured")
	}

	errorPayload := map[string]any{
		"code":    "execution_lost",
		"message": "run execution lost; lease expired",
	}
	var failedMessage *model.Message
	err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		message, err := s.messageRepo.FailStreamingAssistantByRunID(
			txCtx, run.ID, run.ConversationID, errorPayload,
		)
		if err != nil {
			return fmt.Errorf("fail reclaimed assistant message: %w", err)
		}
		failedMessage = message
		if err := s.conversationRepo.UpdateActiveRunID(
			txCtx, run.ConversationID, run.UserID, nil, "",
		); err != nil {
			return fmt.Errorf("clear reclaimed active run: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return failedMessage, nil
}

// GenerateTitle asynchronously generates a title for the conversation.
// Sets title_status to 'generating' and launches a goroutine that calls the AI service.
// On completion, updates title and title_status to 'generated' (or 'failed' on error).
func (s *ConversationService) GenerateTitle(ctx context.Context, id, userID uuid.UUID) error {
	// Verify ownership
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("get conversation for title generation: %w", err)
	}
	if conversation == nil {
		return fmt.Errorf("conversation not found: %s", id)
	}

	// Set status to generating
	if err := s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "generating"); err != nil {
		return fmt.Errorf("update title status: %w", err)
	}

	// Async title generation (placeholder — will be replaced with queue)
	go s.generateTitleAsync(id, userID)

	return nil
}

// GenerateTitleSync generates a title synchronously and returns it.
// Used when the caller needs the title to emit an SSE event before the stream closes.
// Returns ("", nil) if the title has already been generated or is not needed.
func (s *ConversationService) GenerateTitleSync(ctx context.Context, id, userID uuid.UUID) (string, error) {
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return "", fmt.Errorf("get conversation for title generation: %w", err)
	}
	if conversation == nil {
		return "", fmt.Errorf("conversation not found: %s", id)
	}

	// Skip if title is already generated or in progress
	if conversation.TitleStatus != "pending" || conversation.Title != "" {
		return "", nil
	}

	if err := s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "generating"); err != nil {
		return "", fmt.Errorf("update title status: %w", err)
	}

	messages, err := s.messageRepo.ListByConversationID(ctx, id)
	if err != nil {
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return "", fmt.Errorf("list messages for title: %w", err)
	}

	msgPayload := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := map[string]any{"role": msg.Role}
		if msg.Parts != nil {
			m["parts"] = msg.Parts
		}
		msgPayload = append(msgPayload, m)
	}

	configurationID := s.titleConfigurationID()
	generated, err := s.aiClient.GenerateTitle(ctx, msgPayload, configurationID)
	if err != nil {
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return "", fmt.Errorf("generate title: %w", err)
	}
	title, configurationJSON, provenanceJSON, traceJSON, err := validateTitleAgentResponse(generated, configurationID)
	if err != nil {
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return "", err
	}
	if title == "" {
		title = defaultTitle
	}
	if err := s.conversationRepo.UpdateTitleGeneration(ctx, id, userID, title, "generated", configurationID, configurationJSON, provenanceJSON, traceJSON); err != nil {
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return "", fmt.Errorf("persist title generation: %w", err)
	}

	return title, nil
}

// UpdateLastMessageAt updates the last_message_at timestamp for a conversation.
func (s *ConversationService) UpdateLastMessageAt(ctx context.Context, id, userID uuid.UUID) error {
	if err := s.conversationRepo.UpdateLastMessageAt(ctx, id, userID); err != nil {
		return fmt.Errorf("update last_message_at: %w", err)
	}
	return nil
}

// UpdateActiveRunID sets or clears the active_run_id and active_stream_id on a conversation.
func (s *ConversationService) UpdateActiveRunID(ctx context.Context, id, userID uuid.UUID, runID *uuid.UUID, streamID string) error {
	if err := s.conversationRepo.UpdateActiveRunID(ctx, id, userID, runID, streamID); err != nil {
		return fmt.Errorf("update active_run_id: %w", err)
	}
	return nil
}

// ListRuns retrieves all runs for a conversation with ownership check.
func (s *ConversationService) ListRuns(ctx context.Context, conversationID, userID uuid.UUID) ([]model.Run, error) {
	conversation, err := s.conversationRepo.GetByID(ctx, conversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversation for runs: %w", err)
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found: %s", conversationID)
	}

	runs, err := s.runRepo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return runs, nil
}

func validateTitleAgentResponse(response *TitleGenerateResponse, expectedConfigurationID string) (string, []byte, []byte, []byte, error) {
	if response == nil {
		return "", nil, nil, nil, fmt.Errorf("title Agent returned no response")
	}
	registration, ok := knownTitleConfigurations[strings.TrimSpace(expectedConfigurationID)]
	if !ok {
		return "", nil, nil, nil, fmt.Errorf("unknown Title Agent configuration id %q", expectedConfigurationID)
	}
	configuration := response.AgentConfiguration
	if id, _ := configuration["id"].(string); id != expectedConfigurationID {
		return "", nil, nil, nil, fmt.Errorf("title Agent configuration mismatch: got %q want %q", id, expectedConfigurationID)
	}
	if role, _ := configuration["role"].(string); role != "title" {
		return "", nil, nil, nil, fmt.Errorf("title Agent role mismatch: %q", role)
	}
	if policy, _ := configuration["decision_policy_revision"].(string); policy != registration.DecisionPolicyRevision {
		return "", nil, nil, nil, fmt.Errorf("title Agent decision policy mismatch: %q", policy)
	}
	if model, _ := configuration["logical_model"].(string); model != registration.LogicalModel {
		return "", nil, nil, nil, fmt.Errorf("title Agent logical model mismatch: %q", model)
	}
	provenance := response.ExecutionProvenance
	if status, _ := provenance["status"].(string); status != "executed" {
		return "", nil, nil, nil, fmt.Errorf("title Agent execution status is %q", status)
	}
	if model, _ := provenance["logical_model"].(string); model != registration.LogicalModel {
		return "", nil, nil, nil, fmt.Errorf("title Agent execution logical model mismatch: %q", model)
	}
	configurationJSON, err := json.Marshal(configuration)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("marshal title Agent configuration: %w", err)
	}
	provenanceJSON, err := json.Marshal(provenance)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("marshal title execution provenance: %w", err)
	}
	traceJSON, err := json.Marshal(map[string]any{
		"decision":                 "persist",
		"authority":                "go",
		"agent_configuration_id":   expectedConfigurationID,
		"decision_policy_revision": registration.DecisionPolicyRevision,
		"logical_model":            registration.LogicalModel,
	})
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("marshal title decision trace: %w", err)
	}
	return response.Title, configurationJSON, provenanceJSON, traceJSON, nil
}

// generateTitleAsync calls the AI service to generate a title for the conversation.
func (s *ConversationService) generateTitleAsync(id, userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil || conversation == nil {
		log.Printf("generateTitleAsync: conversation not found %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return
	}

	// Fetch messages to send to AI service
	messages, err := s.messageRepo.ListByConversationID(ctx, id)
	if err != nil {
		log.Printf("generateTitleAsync: failed to list messages for %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return
	}

	// Build message payload for the AI service
	msgPayload := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := map[string]any{
			"role": msg.Role,
		}
		if msg.Parts != nil {
			m["parts"] = msg.Parts
		}
		msgPayload = append(msgPayload, m)
	}

	// Call AI service with the exact Go-selected immutable Title configuration.
	configurationID := s.titleConfigurationID()
	generated, err := s.aiClient.GenerateTitle(ctx, msgPayload, configurationID)
	if err != nil {
		log.Printf("generateTitleAsync: AI service call failed for %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return
	}
	title, configurationJSON, provenanceJSON, traceJSON, err := validateTitleAgentResponse(generated, configurationID)
	if err != nil {
		log.Printf("generateTitleAsync: title Agent identity validation failed for %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return
	}
	if title == "" {
		title = defaultTitle
	}
	if err := s.conversationRepo.UpdateTitleGeneration(ctx, id, userID, title, "generated", configurationID, configurationJSON, provenanceJSON, traceJSON); err != nil {
		log.Printf("generateTitleAsync: failed to persist generated title for %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
	}
}
