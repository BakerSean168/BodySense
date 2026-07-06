package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeConsultationRepository struct {
	session              *model.ConsultationSession
	createdSession       *model.ConsultationSession
	updatedPhase         string
	updatedHealthFeatures json.RawMessage
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

func (r *fakeConsultationRepository) UpdateHealthFeatures(ctx context.Context, conversationID uuid.UUID, healthFeatures any) error {
	data, _ := healthFeatures.(json.RawMessage)
	if data == nil {
		if bytes, ok := healthFeatures.([]byte); ok {
			data = json.RawMessage(bytes)
		}
	}
	r.updatedHealthFeatures = data
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

func (r *fakeConsultationRepository) CreateRunEnvelope(
	ctx context.Context,
	userID uuid.UUID,
	conversationID *uuid.UUID,
	requestID string,
	userParts datatypes.JSON,
	userMetadata datatypes.JSON,
	modelName string,
) (*model.ConsultationSession, *model.Run, *model.Message, *model.Message, uuid.UUID, bool, error) {
	resolvedConversationID := uuid.New()
	if conversationID != nil {
		resolvedConversationID = *conversationID
	}
	session := r.session
	if session == nil || session.ConversationID != resolvedConversationID {
		session = &model.ConsultationSession{ConversationID: resolvedConversationID, ExtractedInfo: datatypes.JSON("[]"), HealthFeatures: datatypes.JSON(`{}`), Phase: "collecting"}
		r.session = session
	}
	turnID := uuid.New()
	run := &model.Run{ID: uuid.New(), ConversationID: resolvedConversationID, TurnID: turnID, RequestID: requestID, UserID: userID, Status: "running", Model: modelName}
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

func (c *fakeConversationOwnershipChecker) Create(ctx context.Context, conversation *model.Conversation) error {
	c.conversations[conversation.ID] = conversation
	return nil
}

func (c *fakeConversationOwnershipChecker) SoftDelete(ctx context.Context, id, userID uuid.UUID) error {
	delete(c.conversations, id)
	return nil
}

func (c *fakeConversationOwnershipChecker) GetLastEmptyConversation(ctx context.Context, userID uuid.UUID) (*model.Conversation, error) {
	var latest *model.Conversation
	for _, conv := range c.conversations {
		if conv.UserID == userID && conv.Status == "active" && conv.LastMessageAt == nil {
			if latest == nil || conv.CreatedAt.After(latest.CreatedAt) {
				latest = conv
			}
		}
	}
	return latest, nil
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

func TestUpdateHealthFeaturesPersistsData(t *testing.T) {
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

	healthFeatures := map[string]any{
		"posture_findings":     []map[string]any{},
		"discomforts":         []map[string]any{{"label": "酸胀", "body_part": "肩部"}},
		"negative_findings":   []map[string]any{},
		"movement_limitations": []map[string]any{},
		"red_flags":           []map[string]any{},
		"user_answers":        []map[string]any{},
	}
	if err := svc.UpdateHealthFeatures(context.Background(), conversationID, userID, healthFeatures); err != nil {
		t.Fatalf("UpdateHealthFeatures returned error: %v", err)
	}

	if repo.updatedHealthFeatures == nil {
		t.Fatal("expected health features to be persisted")
	}

	var parsed map[string][]map[string]any
	if err := json.Unmarshal(repo.updatedHealthFeatures, &parsed); err != nil {
		t.Fatalf("persisted data is invalid JSON: %v", err)
	}
	if len(parsed["discomforts"]) != 1 || parsed["discomforts"][0]["body_part"] != "肩部" {
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

func TestCreateSessionReusesEmptySession(t *testing.T) {
	userID := uuid.New()
	repo := &fakeConsultationRepository{session: nil}
	ownership := newFakeConversationOwnershipChecker()
	svc := NewConsultationService(repo, ownership)

	// 1. Create a session, this should create a new conversation and session
	session1, err := svc.CreateSession(context.Background(), userID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session1 == nil {
		t.Fatal("expected session1 to be created")
	}

	// 2. Call CreateSession again. Since last_message_at is nil (it's empty), it should reuse session1.
	session2, err := svc.CreateSession(context.Background(), userID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session2 == nil {
		t.Fatal("expected session2 to be created/reused")
	}
	if session2.ConversationID != session1.ConversationID {
		t.Fatalf("expected session to be reused (same ID), got %s and %s", session1.ConversationID, session2.ConversationID)
	}
}
func TestCreateRunEnvelopeReturnsDurableShell(t *testing.T) {
	conversationID := uuid.New()
	userID := uuid.New()
	repo := &fakeConsultationRepository{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			ExtractedInfo:  datatypes.JSON("[]"),
			Phase:          "collecting",
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
