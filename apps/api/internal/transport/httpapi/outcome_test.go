package httpapi

import (
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestOutcomeAndTrainingPlanProjectionsRemoveUserIdentity(t *testing.T) {
	userID := uuid.New()
	planMap, err := publicTrainingPlanMap(&model.TrainingPlan{ID: uuid.New(), UserID: userID})
	if err != nil {
		t.Fatalf("project training plan: %v", err)
	}
	if _, leaked := planMap["user_id"]; leaked {
		t.Fatalf("training plan leaked user_id: %#v", planMap)
	}
	outcomeMap, err := publicOutcomeMap(&model.Outcome{ID: uuid.New(), UserID: userID})
	if err != nil {
		t.Fatalf("project outcome: %v", err)
	}
	if _, leaked := outcomeMap["user_id"]; leaked {
		t.Fatalf("outcome leaked user_id: %#v", outcomeMap)
	}
}
