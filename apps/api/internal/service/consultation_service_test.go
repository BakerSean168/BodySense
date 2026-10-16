package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeConsultationRepository struct {
	session        *model.ConsultationSession
	createdSession *model.ConsultationSession
	updatedPhase   model.ConsultationPhase
	sessions       []model.ConsultationSession
}

func (r *fakeConsultationRepository) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error) {
	if r.session == nil || r.session.ConversationID != conversationID {
		return nil, nil
	}
	return r.session, nil
}

func (r *fakeConsultationRepository) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
	return r.session, nil
}

func (r *fakeConsultationRepository) UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase model.ConsultationPhase) error {
	r.updatedPhase = phase
	return nil
}

func (r *fakeConsultationRepository) CreateRunEnvelope(
	ctx context.Context,
	userID uuid.UUID,
	conversationID *uuid.UUID,
	requestID string,
	userParts datatypes.JSON,
	userMetadata datatypes.JSON,
	modelName string,
) (*model.ConsultationSession, *model.Run, *model.Message, *model.Message, uuid.UUID, bool, error) {
	// Mirrors the repository: nil resolves the user's singleton.
	resolvedConversationID := uuid.New()
	if r.session != nil {
		resolvedConversationID = r.session.ConversationID
	}
	if conversationID != nil {
		resolvedConversationID = *conversationID
	}
	session := r.session
	if session == nil || session.ConversationID != resolvedConversationID {
		session = &model.ConsultationSession{ConversationID: resolvedConversationID, ExtractedInfo: datatypes.JSON("[]"), Phase: model.ConsultationPhaseCollecting}
		r.session = session
	}
	turnID := uuid.New()
	run := &model.Run{ID: uuid.New(), ConversationID: resolvedConversationID, TurnID: turnID, RequestID: requestID, UserID: userID, Status: model.RunStatusRunning, Model: modelName}
	userMsg := &model.Message{ID: uuid.New(), ConversationID: resolvedConversationID, TurnID: turnID, Role: "user", Status: "completed", Seq: 1, Parts: userParts, Metadata: userMetadata}
	assistantMsg := &model.Message{ID: uuid.New(), ConversationID: resolvedConversationID, TurnID: turnID, RunID: &run.ID, Role: "assistant", Status: "streaming", Seq: 2, Parts: datatypes.JSON("[]"), Metadata: datatypes.JSON("{}")}
	return session, run, userMsg, assistantMsg, turnID, false, nil
}

type fakeConversationOwnershipChecker struct {
	conversations map[uuid.UUID]*model.Conversation
}

func newFakeConversationOwnershipChecker() *fakeConversationOwnershipChecker {
	return &fakeConversationOwnershipChecker{
		conversations: make(map[uuid.UUID]*model.Conversation),
	}
}

func (c *fakeConversationOwnershipChecker) addConversation(id, userID uuid.UUID) {
	c.conversations[id] = &model.Conversation{
		ID:     id,
		UserID: userID,
		Status: "active",
	}
}

func (c *fakeConversationOwnershipChecker) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	conv, ok := c.conversations[id]
	if !ok {
		return nil, nil
	}
	if conv.UserID != userID {
		return nil, nil
	}
	return conv, nil
}

func TestUpdatePhasePersistsWorkflowPhase(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          model.ConsultationPhaseCollecting,
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	if err := svc.UpdatePhase(context.Background(), conversationID, userID, model.ConsultationPhaseReadyForAnalysis); err != nil {
		t.Fatalf("UpdatePhase returned error: %v", err)
	}

	if repo.updatedPhase != model.ConsultationPhaseReadyForAnalysis {
		t.Fatalf("expected phase ready_for_analysis, got %q", repo.updatedPhase)
	}
}

func TestUpdatePhaseBlocksBackwardRegression(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          model.ConsultationPhaseReadyForAnalysis,
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	err := svc.UpdatePhase(context.Background(), conversationID, userID, model.ConsultationPhaseCollecting)
	if err != nil {
		t.Fatalf("UpdatePhase returned unexpected error: %v", err)
	}

	if repo.updatedPhase != model.ConsultationPhase("") {
		t.Fatalf("expected phase update to be skipped (empty), got %q", repo.updatedPhase)
	}
}

func TestUpdatePhaseReturnsErrorWhenSessionNotFound(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{session: nil}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	err := svc.UpdatePhase(context.Background(), conversationID, userID, model.ConsultationPhaseReadyForAnalysis)
	if err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
	if repo.updatedPhase != model.ConsultationPhase("") {
		t.Fatalf("expected no phase update for missing session, got %q", repo.updatedPhase)
	}
}

func TestGetConsultationReturnsSession(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          model.ConsultationPhaseCollecting,
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	session, err := svc.GetConsultation(context.Background(), conversationID, userID)
	if err != nil {
		t.Fatalf("GetConsultation returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.ConversationID != conversationID {
		t.Fatalf("expected conversationID %s, got %s", conversationID, session.ConversationID)
	}
}

func TestGetConsultationFailsOwnershipCheck(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          model.ConsultationPhaseCollecting,
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	session, err := svc.GetConsultation(context.Background(), conversationID, otherUserID)
	if err == nil {
		t.Fatal("expected error for ownership check failure, got nil")
	}
	if session != nil {
		t.Fatal("expected nil session for ownership check failure")
	}
}

func TestCreateRunEnvelopeReusesLongLivedConversationWhenIDIsOmitted(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{session: &model.ConsultationSession{
		ConversationID: conversationID,
		ExtractedInfo:  datatypes.JSON("[]"),
		Phase:          model.ConsultationPhaseCollecting,
	}}
	svc := NewConsultationService(repo, newFakeConversationOwnershipChecker())

	envelope, err := svc.CreateRunEnvelope(
		context.Background(), userID, nil, "req-long-lived",
		datatypes.JSON(`[{"type":"text","text":"continue"}]`), datatypes.JSON("{}"), "consultation-thread",
	)
	if err != nil {
		t.Fatalf("CreateRunEnvelope returned error: %v", err)
	}
	if envelope.Session.ConversationID != conversationID || envelope.Run.ConversationID != conversationID {
		t.Fatalf("expected long-lived conversation %s, got session=%s run=%s", conversationID, envelope.Session.ConversationID, envelope.Run.ConversationID)
	}
	if envelope.ConversationCreated {
		t.Fatal("reusing the long-lived conversation must not emit conversation.created semantics")
	}
}

func TestCreateRunEnvelopeMarksOnlyFirstConversationAsCreated(t *testing.T) {
	userID := uuid.New()
	repo := &fakeConsultationRepository{}
	svc := NewConsultationService(repo, newFakeConversationOwnershipChecker())

	envelope, err := svc.CreateRunEnvelope(
		context.Background(), userID, nil, "req-first",
		datatypes.JSON(`[{"type":"text","text":"hello"}]`), datatypes.JSON("{}"), "consultation-thread",
	)
	if err != nil {
		t.Fatalf("CreateRunEnvelope returned error: %v", err)
	}
	if !envelope.ConversationCreated {
		t.Fatal("the user's first consultation should be marked as newly created")
	}
}

func TestCreateRunEnvelopeReturnsDurableShell(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			ExtractedInfo:  datatypes.JSON("[]"),
			Phase:          model.ConsultationPhaseCollecting,
		},
	}
	svc := NewConsultationService(repo, newFakeConversationOwnershipChecker())

	envelope, err := svc.CreateRunEnvelope(
		context.Background(),
		userID,
		&conversationID,
		"req-1",
		datatypes.JSON(`[{"type":"text","text":"hello"}]`),
		datatypes.JSON("{}"),
		"consultation-thread",
	)
	if err != nil {
		t.Fatalf("CreateRunEnvelope returned error: %v", err)
	}
	if envelope.Session.ConversationID != conversationID {
		t.Fatalf("expected session conversation %s, got %s", conversationID, envelope.Session.ConversationID)
	}
	if envelope.Run.ConversationID != conversationID || envelope.Run.RequestID != "req-1" {
		t.Fatalf("unexpected run envelope: %+v", envelope.Run)
	}
	if envelope.UserMessage.Role != "user" || envelope.UserMessage.Status != "completed" {
		t.Fatalf("unexpected user message: %+v", envelope.UserMessage)
	}
	if envelope.AssistantMessage.Role != "assistant" || envelope.AssistantMessage.Status != "streaming" {
		t.Fatalf("unexpected assistant message: %+v", envelope.AssistantMessage)
	}
	if envelope.AssistantMessage.RunID == nil || *envelope.AssistantMessage.RunID != envelope.Run.ID {
		t.Fatalf("assistant message must reference run id")
	}
}
