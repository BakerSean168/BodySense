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
		Exercises:   json.RawMessage(`[{"name":"Scapular control","completed":true}]`),
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

func TestTrainingFeedbackResultCarriesOnlyTheOutcome(t *testing.T) {
	now := time.Now().UTC()
	outcome := &model.Outcome{
		ID: uuid.New(), UserID: uuid.New(), SourceType: "training_feedback", SourceKey: "k",
		Kind: "symptom_change", ConcernKey: "general", Value: []byte(`{}`), Provenance: []byte(`{}`),
		CausalityLevel: "association_only", OccurredAt: now, CreatedAt: now,
	}
	projected, err := strictTrainingFeedbackResult(map[string]any{"outcome": outcome})
	if err != nil {
		t.Fatalf("strict feedback result: %v", err)
	}
	if projected.Outcome == nil || projected.Outcome.Id != outcome.ID {
		t.Fatalf("feedback result lost its Outcome: %+v", projected)
	}
	if _, err := strictTrainingFeedbackResult(map[string]any{"has_proposal": false}); err == nil {
		t.Fatal("the retired Treatment proposal fields must be rejected by the contract")
	}
}
