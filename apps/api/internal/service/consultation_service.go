package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ErrConsultationNotFound deliberately does not disclose another owner's conversation.
var ErrConsultationNotFound = errors.New("consultation not found")

type consultationRepository interface {
	GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error)
	GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error)
	UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase model.ConsultationPhase) error
	CreateRunEnvelope(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID, requestID string, userParts datatypes.JSON, userMetadata datatypes.JSON, modelName string) (*model.ConsultationSession, *model.Run, *model.Message, *model.Message, uuid.UUID, bool, error)
}

// conversationOwnershipChecker verifies that a conversation belongs to a user.
type conversationOwnershipChecker interface {
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error)
}

// RunEnvelope is the transactionally-created durable shell for one AI run.
type RunEnvelope struct {
	Session             *model.ConsultationSession
	TurnID              uuid.UUID
	Run                 *model.Run
	UserMessage         *model.Message
	AssistantMessage    *model.Message
	Existed             bool
	ConversationCreated bool
}

// ConsultationService handles consultation business logic.
type ConsultationService struct {
	consultationRepo consultationRepository
	conversationRepo conversationOwnershipChecker
}

// NewConsultationService creates a new ConsultationService.
func NewConsultationService(
	consultationRepo consultationRepository,
	conversationRepo conversationOwnershipChecker,
) *ConsultationService {
	return &ConsultationService{
		consultationRepo: consultationRepo,
		conversationRepo: conversationRepo,
	}
}

// verifyOwnership checks that the conversation belongs to the user.
func (s *ConsultationService) verifyOwnership(ctx context.Context, conversationID, userID uuid.UUID) error {
	conversation, err := s.conversationRepo.GetByID(ctx, conversationID, userID)
	if err != nil {
		return fmt.Errorf("verify ownership: %w", err)
	}
	if conversation == nil {
		return ErrConsultationNotFound
	}
	return nil
}

// GetLatestForUser returns the consultation of the user's single Conversation
// without creating one as a side effect. Workspace reads use this method.
func (s *ConsultationService) GetLatestForUser(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
	return s.consultationRepo.GetLatestByUserID(ctx, userID)
}

// GetConsultation retrieves a consultation session by conversation ID with ownership check.
func (s *ConsultationService) GetConsultation(ctx context.Context, conversationID, userID uuid.UUID) (*model.ConsultationSession, error) {
	if err := s.verifyOwnership(ctx, conversationID, userID); err != nil {
		return nil, err
	}

	session, err := s.consultationRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get consultation: %w", err)
	}
	return session, nil
}

// UpdatePhase updates the workflow phase for a session, enforcing forward-only
// phase transitions. If the requested phase would regress the session, the
// update is silently skipped (idempotent).
func (s *ConsultationService) UpdatePhase(ctx context.Context, conversationID, userID uuid.UUID, phase model.ConsultationPhase) error {
	if err := s.verifyOwnership(ctx, conversationID, userID); err != nil {
		return err
	}

	session, err := s.consultationRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("get session for phase update: %w", err)
	}
	if session == nil {
		return fmt.Errorf("session not found: %s", conversationID)
	}

	currentPhase := session.Phase
	if !ShouldAdvancePhase(currentPhase, phase) {
		return nil
	}

	return s.consultationRepo.UpdatePhase(ctx, conversationID, phase)
}

// CreateRunEnvelope atomically creates the durable shell for a consultation run.
func (s *ConsultationService) CreateRunEnvelope(
	ctx context.Context,
	userID uuid.UUID,
	conversationID *uuid.UUID,
	requestID string,
	userParts datatypes.JSON,
	userMetadata datatypes.JSON,
	modelName string,
) (*RunEnvelope, error) {
	// The repository lazily creates the user's single Conversation inside the
	// envelope transaction. ConversationCreated only drives the one-time title
	// hint, so a pre-read is sufficient; identity itself is race-free in the DB.
	conversationCreated := false
	if conversationID == nil || *conversationID == uuid.Nil {
		existing, lookupErr := s.consultationRepo.GetLatestByUserID(ctx, userID)
		if lookupErr != nil {
			return nil, fmt.Errorf("resolve singleton consultation: %w", lookupErr)
		}
		conversationCreated = existing == nil
	}

	session, run, userMsg, assistantMsg, turnID, existed, err := s.consultationRepo.CreateRunEnvelope(
		ctx,
		userID,
		conversationID,
		requestID,
		userParts,
		userMetadata,
		modelName,
	)
	if err != nil {
		return nil, fmt.Errorf("create run envelope: %w", err)
	}
	return &RunEnvelope{
		Session:             session,
		TurnID:              turnID,
		Run:                 run,
		UserMessage:         userMsg,
		AssistantMessage:    assistantMsg,
		Existed:             existed,
		ConversationCreated: conversationCreated,
	}, nil
}
