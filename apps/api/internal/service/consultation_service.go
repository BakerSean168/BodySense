package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type consultationRepository interface {
	Create(ctx context.Context, session *model.ConsultationSession) error
	GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error)
	GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error)
	ListByConversationIDs(ctx context.Context, conversationIDs []uuid.UUID) ([]model.ConsultationSession, error)
	Delete(ctx context.Context, conversationID uuid.UUID) error
	UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase string) error
	CreateRunEnvelope(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID, requestID string, userParts datatypes.JSON, userMetadata datatypes.JSON, modelName string) (*model.ConsultationSession, *model.Run, *model.Message, *model.Message, uuid.UUID, bool, error)
}

// conversationOwnershipChecker verifies that a conversation belongs to a user.
type conversationOwnershipChecker interface {
	Create(ctx context.Context, conversation *model.Conversation) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error)
	SoftDelete(ctx context.Context, id, userID uuid.UUID) error
	GetLastEmptyConversation(ctx context.Context, userID uuid.UUID) (*model.Conversation, error)
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
		return fmt.Errorf("conversation not found or access denied: %s", conversationID)
	}
	return nil
}

// CreateSession resolves the user's long-lived consultation. Historical
// conversations are retained, but ordinary product flows no longer create a new
// consultation merely because the existing one already contains messages.
func (s *ConsultationService) CreateSession(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
	if latest, err := s.consultationRepo.GetLatestByUserID(ctx, userID); err != nil {
		return nil, fmt.Errorf("get long-lived consultation: %w", err)
	} else if latest != nil {
		return latest, nil
	}

	existingConv, err := s.conversationRepo.GetLastEmptyConversation(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check empty conversation: %w", err)
	}

	if existingConv != nil {
		session, err := s.consultationRepo.GetByConversationID(ctx, existingConv.ID)
		if err == nil && session != nil {
			return session, nil
		}
		// If session is missing for some reason, recreate it
		session = &model.ConsultationSession{
			ConversationID: existingConv.ID,
			ExtractedInfo:  datatypes.JSON("[]"),
			Phase:          "collecting",
		}
		if err := s.consultationRepo.Create(ctx, session); err != nil {
			return nil, fmt.Errorf("recreate consultation: %w", err)
		}
		return session, nil
	}

	conversation := &model.Conversation{
		ID:          uuid.New(),
		UserID:      userID,
		Status:      "active",
		TitleStatus: "pending",
	}
	if err := s.conversationRepo.Create(ctx, conversation); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}

	session := &model.ConsultationSession{
		ConversationID: conversation.ID,
		ExtractedInfo:  datatypes.JSON("[]"),
		Phase:          "collecting",
	}
	if err := s.consultationRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create consultation: %w", err)
	}
	return session, nil
}

// GetLatestForUser returns the canonical long-lived consultation without
// creating one as a side effect. Workspace reads use this method.
func (s *ConsultationService) GetLatestForUser(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
	return s.consultationRepo.GetLatestByUserID(ctx, userID)
}

// CreateSessionWithID creates a conversation with a specific ID and its associated consultation session.
func (s *ConsultationService) CreateSessionWithID(ctx context.Context, conversationID, userID uuid.UUID) (*model.ConsultationSession, error) {
	conversation := &model.Conversation{
		ID:          conversationID,
		UserID:      userID,
		Status:      "active",
		TitleStatus: "pending",
	}
	if err := s.conversationRepo.Create(ctx, conversation); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}

	session := &model.ConsultationSession{
		ConversationID: conversationID,
		ExtractedInfo:  datatypes.JSON("[]"),
		Phase:          "collecting",
	}
	if err := s.consultationRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create consultation: %w", err)
	}
	return session, nil
}

// GetSession retrieves a consultation session by conversation ID with ownership check.
func (s *ConsultationService) GetSession(ctx context.Context, conversationID, userID uuid.UUID) (*model.ConsultationSession, error) {
	if err := s.verifyOwnership(ctx, conversationID, userID); err != nil {
		return nil, err
	}

	session, err := s.consultationRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return session, nil
}

// DeleteSession deletes a consultation session and its parent conversation.
func (s *ConsultationService) DeleteSession(ctx context.Context, conversationID, userID uuid.UUID) error {
	if err := s.verifyOwnership(ctx, conversationID, userID); err != nil {
		return err
	}

	// Delete consultation session first (FK dependency)
	if err := s.consultationRepo.Delete(ctx, conversationID); err != nil {
		return fmt.Errorf("delete consultation: %w", err)
	}

	// Soft-delete the conversation
	if err := s.conversationRepo.SoftDelete(ctx, conversationID, userID); err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

// CreateConsultation creates a new consultation session for a conversation.
func (s *ConsultationService) CreateConsultation(ctx context.Context, conversationID, userID uuid.UUID) error {
	if err := s.verifyOwnership(ctx, conversationID, userID); err != nil {
		return err
	}

	session := &model.ConsultationSession{
		ConversationID: conversationID,
		ExtractedInfo:  datatypes.JSON("[]"),
		Phase:          "collecting",
	}
	if err := s.consultationRepo.Create(ctx, session); err != nil {
		return fmt.Errorf("create consultation: %w", err)
	}
	return nil
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
func (s *ConsultationService) UpdatePhase(ctx context.Context, conversationID, userID uuid.UUID, phase string) error {
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
	resolvedConversationID := conversationID
	conversationCreated := false
	if conversationID == nil || *conversationID == uuid.Nil {
		latest, lookupErr := s.consultationRepo.GetLatestByUserID(ctx, userID)
		if lookupErr != nil {
			return nil, fmt.Errorf("resolve long-lived consultation: %w", lookupErr)
		}
		if latest != nil {
			id := latest.ConversationID
			resolvedConversationID = &id
		} else {
			conversationCreated = true
		}
	}

	session, run, userMsg, assistantMsg, turnID, existed, err := s.consultationRepo.CreateRunEnvelope(
		ctx,
		userID,
		resolvedConversationID,
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
