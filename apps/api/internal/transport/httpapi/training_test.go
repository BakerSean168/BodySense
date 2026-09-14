package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestTrainingLogPublicProjectionRemovesPersistenceUserIdentity(t *testing.T) {
	now := time.Now().UTC()
	logEntry := &model.TrainingLog{
		ID: uuid.New(), UserID: uuid.New(), PlanID: uuid.New(), Date: now,
		Exercises: json.RawMessage(`[{
			"intervention_id":"11111111-1111-4111-8111-111111111111",
			"name":"Scapular control","completed":true
		}]`),
		IsCheckedIn: true, CreatedAt: now,
	}
	projected, err := strictTrainingLog(logEntry)
	if err != nil {
		t.Fatalf("strict TrainingLog projection: %v", err)
	}
	encoded, err := json.Marshal(projected)
	if err != nil {
		t.Fatalf("marshal public TrainingLog: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("decode public TrainingLog: %v", err)
	}
	if _, leaked := body["user_id"]; leaked {
		t.Fatalf("TrainingLog leaked user_id: %#v", body)
	}
	if body["id"] != logEntry.ID.String() || body["plan_id"] != logEntry.PlanID.String() {
		t.Fatalf("public TrainingLog lost required identity: %#v", body)
	}
}

func TestTrainingFeedbackResultUsesPublicNestedProjections(t *testing.T) {
	result := map[string]any{
		"has_proposal":           false,
		"review_recommended":     true,
		"requires_new_diagnosis": false,
	}
	projected, err := strictTrainingFeedbackResult(result)
	if err != nil {
		t.Fatalf("strict feedback result: %v", err)
	}
	if projected.HasProposal == nil || *projected.HasProposal {
		t.Fatalf("unexpected has_proposal: %+v", projected.HasProposal)
	}
	if projected.ReviewRecommended == nil || !*projected.ReviewRecommended {
		t.Fatalf("unexpected review_recommended: %+v", projected.ReviewRecommended)
	}
}
