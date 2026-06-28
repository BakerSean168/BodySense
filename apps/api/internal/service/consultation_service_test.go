package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeConsultationRepository struct {
	session              *model.ConsultationSession
	createdSession       *model.ConsultationSession
	updatedPhase         string
	updatedExtractedInfo json.RawMessage
	updatedDiagnosis     json.RawMessage
	updatedTreatmentPlan json.RawMessage
	sessions             []model.ConsultationSession
}

func (r *fakeConsultationRepository) Create(ctx context.Context, session *model.ConsultationSession) error {
	r.createdSession = session
	r.session = session
	return nil
}

func (r *fakeConsultationRepository) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error) {
	if r.session == nil || r.session.ConversationID != conversationID {
		return nil, nil
	}
	return r.session, nil
}

func (r *fakeConsultationRepository) ListByConversationIDs(ctx context.Context, conversationIDs []uuid.UUID) ([]model.ConsultationSession, error) {
	return r.sessions, nil
}

func (r *fakeConsultationRepository) UpdateExtractedInfo(ctx context.Context, conversationID uuid.UUID, extractedInfo any) error {
	data, _ := extractedInfo.(json.RawMessage)
	if data == nil {
		if bytes, ok := extractedInfo.([]byte); ok {
			data = json.RawMessage(bytes)
		}
	}
	r.updatedExtractedInfo = data
	return nil
}

func (r *fakeConsultationRepository) UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase string) error {
	r.updatedPhase = phase
	return nil
}

func (r *fakeConsultationRepository) UpdateDiagnosis(ctx context.Context, conversationID uuid.UUID, diagnosis any) error {
	data, _ := diagnosis.(json.RawMessage)
	if data == nil {
		if bytes, ok := diagnosis.([]byte); ok {
			data = json.RawMessage(bytes)
		}
	}
	r.updatedDiagnosis = data
	return nil
}

func (r *fakeConsultationRepository) UpdateTreatmentPlan(ctx context.Context, conversationID uuid.UUID, treatmentPlan any) error {
	data, _ := treatmentPlan.(json.RawMessage)
	if data == nil {
		if bytes, ok := treatmentPlan.([]byte); ok {
			data = json.RawMessage(bytes)
		}
	}
	r.updatedTreatmentPlan = data
	return nil
}

func (r *fakeConsultationRepository) Delete(ctx context.Context, conversationID uuid.UUID) error {
	if r.session != nil && r.session.ConversationID == conversationID {
		r.session = nil
	}
	return nil
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

func (c *fakeConversationOwnershipChecker) Create(ctx context.Context, conversation *model.Conversation) error {
	c.conversations[conversation.ID] = conversation
	return nil
}

func (c *fakeConversationOwnershipChecker) SoftDelete(ctx context.Context, id, userID uuid.UUID) error {
	delete(c.conversations, id)
	return nil
}

func TestUpdatePhasePersistsWorkflowPhase(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          "collecting",
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	if err := svc.UpdatePhase(context.Background(), conversationID, userID, "ready_for_analysis"); err != nil {
		t.Fatalf("UpdatePhase returned error: %v", err)
	}

	if repo.updatedPhase != "ready_for_analysis" {
		t.Fatalf("expected phase ready_for_analysis, got %q", repo.updatedPhase)
	}
}

func TestUpdatePhaseAllowsForwardTransition(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          "ready_for_analysis",
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	if err := svc.UpdatePhase(context.Background(), conversationID, userID, "plan_ready"); err != nil {
		t.Fatalf("UpdatePhase returned error: %v", err)
	}

	if repo.updatedPhase != "plan_ready" {
		t.Fatalf("expected phase plan_ready, got %q", repo.updatedPhase)
	}
}

func TestUpdatePhaseBlocksBackwardRegression(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          "plan_ready",
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	err := svc.UpdatePhase(context.Background(), conversationID, userID, "collecting")
	if err != nil {
		t.Fatalf("UpdatePhase returned unexpected error: %v", err)
	}

	if repo.updatedPhase != "" {
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

	err := svc.UpdatePhase(context.Background(), conversationID, userID, "ready_for_analysis")
	if err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
	if repo.updatedPhase != "" {
		t.Fatalf("expected no phase update for missing session, got %q", repo.updatedPhase)
	}
}

func TestCreateConsultationCreatesNew(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{session: nil}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	if err := svc.CreateConsultation(context.Background(), conversationID, userID); err != nil {
		t.Fatalf("CreateConsultation returned error: %v", err)
	}

	if repo.createdSession == nil {
		t.Fatal("expected Create to be called")
	}
	if repo.createdSession.ConversationID != conversationID {
		t.Fatalf("expected conversationID %s, got %s", conversationID, repo.createdSession.ConversationID)
	}
	if repo.createdSession.Phase != "collecting" {
		t.Fatalf("expected phase collecting, got %s", repo.createdSession.Phase)
	}
}

func TestCreateConsultationFailsOwnershipCheck(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()
	repo := &fakeConsultationRepository{session: nil}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	err := svc.CreateConsultation(context.Background(), conversationID, otherUserID)
	if err == nil {
		t.Fatal("expected error for ownership check failure, got nil")
	}
}

func TestUpdateExtractedInfoPersistsData(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          "collecting",
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	info := []map[string]any{
		{"body_part": "肩部", "symptom_type": "酸胀"},
	}
	if err := svc.UpdateExtractedInfo(context.Background(), conversationID, userID, info); err != nil {
		t.Fatalf("UpdateExtractedInfo returned error: %v", err)
	}

	if repo.updatedExtractedInfo == nil {
		t.Fatal("expected extracted info to be persisted")
	}

	var parsed []map[string]any
	if err := json.Unmarshal(repo.updatedExtractedInfo, &parsed); err != nil {
		t.Fatalf("persisted data is invalid JSON: %v", err)
	}
	if len(parsed) != 1 || parsed[0]["body_part"] != "肩部" {
		t.Fatalf("unexpected persisted data: %v", parsed)
	}
}

func TestUpdateDiagnosisPersistsData(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          "collecting",
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	diagnosis := map[string]any{
		"diagnoses": []map[string]any{
			{"name": "头前伸倾向", "confidence": "中"},
		},
	}
	if err := svc.UpdateDiagnosis(context.Background(), conversationID, userID, diagnosis); err != nil {
		t.Fatalf("UpdateDiagnosis returned error: %v", err)
	}

	if repo.updatedDiagnosis == nil {
		t.Fatal("expected diagnosis to be persisted")
	}
}

func TestUpdateTreatmentPlanPersistsData(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          "collecting",
		},
	}
	ownership := newFakeConversationOwnershipChecker()
	ownership.addConversation(conversationID, userID)
	svc := NewConsultationService(repo, ownership)

	plan := map[string]any{
		"goal":           "缓解肩颈酸胀",
		"duration_weeks": 4,
	}
	if err := svc.UpdateTreatmentPlan(context.Background(), conversationID, userID, plan); err != nil {
		t.Fatalf("UpdateTreatmentPlan returned error: %v", err)
	}

	if repo.updatedTreatmentPlan == nil {
		t.Fatal("expected treatment plan to be persisted")
	}
}

func TestGetConsultationReturnsSession(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			Phase:          "collecting",
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
			Phase:          "collecting",
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
