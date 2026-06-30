package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// mockAIOutputReviewRepo implements OutputReviewRepo interface.
type mockAIOutputReviewRepo struct {
	created *model.AIOutputReview
}

var _ OutputReviewRepo = (*mockAIOutputReviewRepo)(nil)

func (m *mockAIOutputReviewRepo) Create(_ context.Context, review *model.AIOutputReview) error {
	m.created = review
	return nil
}

func TestRecordReview_PersistsReview(t *testing.T) {
	repo := &mockAIOutputReviewRepo{}
	svc := NewOutputReviewService(repo)

	userID := uuid.New()
	runID := uuid.New()
	convID := uuid.New()
	issues := datatypes.JSON(`[{"policy":"test","severity":"warning","message":"test issue"}]`)
	validatedOutput := datatypes.JSON(`"test output"`)
	rawOutput := datatypes.JSON(`{"status":"degraded","issues":[]}`)

	svc.RecordReview(
		context.Background(),
		"consultation_reply", "degraded",
		&userID, &runID, nil, &convID,
		issues, validatedOutput, rawOutput,
	)

	if repo.created == nil {
		t.Fatal("expected review to be created")
	}
	if repo.created.OutputType != "consultation_reply" {
		t.Errorf("expected output_type 'consultation_reply', got %q", repo.created.OutputType)
	}
	if repo.created.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %q", repo.created.Status)
	}
	if repo.created.UserID == nil || *repo.created.UserID != userID {
		t.Error("expected user_id to be set")
	}
	if repo.created.RunID == nil || *repo.created.RunID != runID {
		t.Error("expected run_id to be set")
	}
	if repo.created.ConversationID == nil || *repo.created.ConversationID != convID {
		t.Error("expected conversation_id to be set")
	}
	if string(repo.created.Issues) != string(issues) {
		t.Errorf("issues mismatch: got %s", string(repo.created.Issues))
	}
	if string(repo.created.ValidatedOutput) != string(validatedOutput) {
		t.Errorf("validated_output mismatch: got %s", string(repo.created.ValidatedOutput))
	}
}

func TestRecordReview_NilIDs(t *testing.T) {
	repo := &mockAIOutputReviewRepo{}
	svc := NewOutputReviewService(repo)

	svc.RecordReview(
		context.Background(),
		"test", "accepted",
		nil, nil, nil, nil,
		datatypes.JSON("[]"), nil, nil,
	)

	if repo.created == nil {
		t.Fatal("expected review to be created")
	}
	if repo.created.UserID != nil {
		t.Error("expected user_id to be nil")
	}
	if repo.created.RunID != nil {
		t.Error("expected run_id to be nil")
	}
}
