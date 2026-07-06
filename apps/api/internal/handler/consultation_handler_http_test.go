package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	consultationruntime "github.com/bodysense/api/internal/consultation"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type consultationHandlerRepo struct {
	session              *model.ConsultationSession
	updatedHealthFeature json.RawMessage
}

func (r *consultationHandlerRepo) Create(ctx context.Context, session *model.ConsultationSession) error {
	r.session = session
	return nil
}

func (r *consultationHandlerRepo) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error) {
	if r.session == nil || r.session.ConversationID != conversationID {
		return nil, nil
	}
	return r.session, nil
}

func (r *consultationHandlerRepo) ListByConversationIDs(ctx context.Context, conversationIDs []uuid.UUID) ([]model.ConsultationSession, error) {
	return nil, nil
}

func (r *consultationHandlerRepo) Delete(ctx context.Context, conversationID uuid.UUID) error {
	return nil
}

func (r *consultationHandlerRepo) UpdateHealthFeatures(ctx context.Context, conversationID uuid.UUID, healthFeatures any) error {
	switch typed := healthFeatures.(type) {
	case json.RawMessage:
		r.updatedHealthFeature = typed
	case []byte:
		r.updatedHealthFeature = json.RawMessage(typed)
	}
	return nil
}

func (r *consultationHandlerRepo) UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase string) error {
	return nil
}

func (r *consultationHandlerRepo) UpdateDiagnosis(ctx context.Context, conversationID uuid.UUID, diagnosis any) error {
	return nil
}

func (r *consultationHandlerRepo) UpdateTreatmentPlan(ctx context.Context, conversationID uuid.UUID, treatmentPlan any) error {
	return nil
}

func (r *consultationHandlerRepo) CreateRunEnvelope(
	ctx context.Context,
	userID uuid.UUID,
	conversationID *uuid.UUID,
	requestID string,
	userParts datatypes.JSON,
	userMetadata datatypes.JSON,
	modelName string,
) (*model.ConsultationSession, *model.Run, *model.Message, *model.Message, uuid.UUID, bool, error) {
	return nil, nil, nil, nil, uuid.Nil, false, nil
}

type consultationHandlerConversationRepo struct {
	conversation *model.Conversation
}

func (r *consultationHandlerConversationRepo) Create(ctx context.Context, conversation *model.Conversation) error {
	r.conversation = conversation
	return nil
}

func (r *consultationHandlerConversationRepo) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	if r.conversation == nil || r.conversation.ID != id || r.conversation.UserID != userID {
		return nil, nil
	}
	return r.conversation, nil
}

func (r *consultationHandlerConversationRepo) SoftDelete(ctx context.Context, id, userID uuid.UUID) error {
	return nil
}

func (r *consultationHandlerConversationRepo) GetLastEmptyConversation(ctx context.Context, userID uuid.UUID) (*model.Conversation, error) {
	return nil, nil
}

func TestUpdateHealthFeaturesHandlerPersistsStructuredPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	conversationID := uuid.New()
	userID := uuid.New()
	repo := &consultationHandlerRepo{
		session: &model.ConsultationSession{
			ConversationID: conversationID,
			HealthFeatures: datatypes.JSON(`{}`),
			Phase:          "collecting",
		},
	}
	conversationRepo := &consultationHandlerConversationRepo{
		conversation: &model.Conversation{
			ID:     conversationID,
			UserID: userID,
			Status: "active",
		},
	}

	handler := NewConsultationHandler(
		service.NewConsultationService(repo, conversationRepo),
		nil,
		&consultationruntime.Runtime{},
	)

	body := []byte(`{"health_features":{"posture_findings":[{"label":"头前移"}],"discomforts":[],"negative_findings":[],"movement_limitations":[],"red_flags":[],"user_answers":[]}}`)
	req := httptest.NewRequest(http.MethodPut, "/consultations/"+conversationID.String()+"/health-features", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: conversationID.String()}}
	ctx.Set("user_id", userID.String())

	handler.UpdateHealthFeatures(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	if repo.updatedHealthFeature == nil {
		t.Fatal("expected health features payload to be persisted")
	}

	var persisted map[string]any
	if err := json.Unmarshal(repo.updatedHealthFeature, &persisted); err != nil {
		t.Fatalf("persisted payload is invalid JSON: %v", err)
	}
	postureFindings, _ := persisted["posture_findings"].([]any)
	if len(postureFindings) != 1 {
		t.Fatalf("expected 1 posture finding, got %#v", persisted)
	}
}
