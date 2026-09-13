package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type diagnosisApplicationConsultationRepo struct {
	session *model.ConsultationSession
}

func (r *diagnosisApplicationConsultationRepo) Create(context.Context, *model.ConsultationSession) error {
	return nil
}
func (r *diagnosisApplicationConsultationRepo) GetByConversationID(_ context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error) {
	if r.session == nil || r.session.ConversationID != conversationID {
		return nil, nil
	}
	return r.session, nil
}
func (r *diagnosisApplicationConsultationRepo) GetLatestByUserID(context.Context, uuid.UUID) (*model.ConsultationSession, error) {
	return r.session, nil
}
func (r *diagnosisApplicationConsultationRepo) ListByConversationIDs(context.Context, []uuid.UUID) ([]model.ConsultationSession, error) {
	return nil, nil
}
func (r *diagnosisApplicationConsultationRepo) Delete(context.Context, uuid.UUID) error { return nil }
func (r *diagnosisApplicationConsultationRepo) UpdatePhase(context.Context, uuid.UUID, string) error {
	return nil
}
func (r *diagnosisApplicationConsultationRepo) UpdateDiagnosis(context.Context, uuid.UUID, any) error {
	return nil
}
func (r *diagnosisApplicationConsultationRepo) CreateRunEnvelope(context.Context, uuid.UUID, *uuid.UUID, string, datatypes.JSON, datatypes.JSON, string) (*model.ConsultationSession, *model.Run, *model.Message, *model.Message, uuid.UUID, bool, error) {
	return nil, nil, nil, nil, uuid.Nil, false, nil
}

type diagnosisApplicationConversationRepo struct {
	conversation *model.Conversation
}

func (r *diagnosisApplicationConversationRepo) Create(context.Context, *model.Conversation) error {
	return nil
}
func (r *diagnosisApplicationConversationRepo) GetByID(_ context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	if r.conversation == nil || r.conversation.ID != id || r.conversation.UserID != userID {
		return nil, nil
	}
	return r.conversation, nil
}
func (*diagnosisApplicationConversationRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (*diagnosisApplicationConversationRepo) GetLastEmptyConversation(context.Context, uuid.UUID) (*model.Conversation, error) {
	return nil, nil
}

func newDiagnosisApplicationConsultationService(
	userID uuid.UUID,
	conversationID uuid.UUID,
	session *model.ConsultationSession,
) *ConsultationService {
	return NewConsultationService(
		&diagnosisApplicationConsultationRepo{session: session},
		&diagnosisApplicationConversationRepo{conversation: &model.Conversation{
			ID: conversationID, UserID: userID, Status: "active",
		}},
	)
}

func TestDiagnosisApplicationConfigurationMatchesRequiresSelectedIDAndRole(t *testing.T) {
	if diagnosisApplicationConfigurationMatches(map[string]any{
		"agent_configuration": map[string]any{"id": "diag-config-wrong", "role": "diagnosis"},
	}, "diag-config-selected") {
		t.Fatal("mismatched Agent configuration must be rejected")
	}
	if diagnosisApplicationConfigurationMatches(map[string]any{
		"agent_configuration": map[string]any{"id": "diag-config-selected", "role": "treatment"},
	}, "diag-config-selected") {
		t.Fatal("wrong Agent role must be rejected")
	}
	if !diagnosisApplicationConfigurationMatches(map[string]any{
		"agent_configuration": map[string]any{"id": "diag-config-selected", "role": "diagnosis"},
	}, "diag-config-selected") {
		t.Fatal("selected Diagnosis Agent configuration should be accepted")
	}
}

func TestDiagnosisApplicationReturnsNotFoundWhenConsultationSessionDoesNotExist(t *testing.T) {
	userID := uuid.New()
	conversationID := uuid.New()
	application := NewDiagnosisApplicationService(
		newDiagnosisApplicationConsultationService(userID, conversationID, nil),
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	payload, appErr := application.Analyze(context.Background(), userID, conversationID)
	if payload != nil {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if appErr == nil || appErr.Code != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %+v", appErr)
	}
}

func TestDiagnosisApplicationRequiresBodyStateDomainServices(t *testing.T) {
	userID := uuid.New()
	conversationID := uuid.New()
	session := &model.ConsultationSession{
		ConversationID: conversationID,
		Phase:          "ready_for_analysis",
		ExtractedInfo:  datatypes.JSON(`[]`),
	}
	application := NewDiagnosisApplicationService(
		newDiagnosisApplicationConsultationService(userID, conversationID, session),
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	payload, appErr := application.Analyze(context.Background(), userID, conversationID)
	if payload != nil {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if appErr == nil || appErr.Code != "DIAGNOSIS_DOMAIN_UNAVAILABLE" {
		t.Fatalf("expected DIAGNOSIS_DOMAIN_UNAVAILABLE, got %+v", appErr)
	}
}
