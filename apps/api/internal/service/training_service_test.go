package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeTrainingRepository struct {
	active   *model.TrainingPlan
	log      *model.TrainingLog
	savedLog *model.TrainingLog
}

func (r *fakeTrainingRepository) GetPlanByID(context.Context, uuid.UUID, uuid.UUID) (*model.TrainingPlan, error) {
	return r.active, nil
}
func (r *fakeTrainingRepository) ListPlansByUserID(context.Context, uuid.UUID) ([]model.TrainingPlan, error) {
	if r.active == nil {
		return nil, nil
	}
	return []model.TrainingPlan{*r.active}, nil
}
func (r *fakeTrainingRepository) GetActivePlanByUserID(context.Context, uuid.UUID) (*model.TrainingPlan, error) {
	return r.active, nil
}
func (r *fakeTrainingRepository) GetOrCreateLog(context.Context, *model.TrainingPlan, time.Time) (*model.TrainingLog, error) {
	if r.log == nil {
		r.log = &model.TrainingLog{ID: uuid.New(), Exercises: json.RawMessage(`[]`)}
	}
	return r.log, nil
}
func (r *fakeTrainingRepository) SaveLog(_ context.Context, log *model.TrainingLog) error {
	copy := *log
	r.savedLog = &copy
	return nil
}
func (r *fakeTrainingRepository) CheckInAndGet(context.Context, *model.TrainingPlan, time.Time) (*model.TrainingLog, error) {
	return r.GetOrCreateLog(context.Background(), nil, time.Time{})
}
func (r *fakeTrainingRepository) MarkOutcomeRecorded(context.Context, uuid.UUID, time.Time) error {
	return nil
}
func (r *fakeTrainingRepository) GetLogsByPlanID(context.Context, uuid.UUID) ([]model.TrainingLog, error) {
	return nil, nil
}
func (r *fakeTrainingRepository) GetConsecutiveCheckInDays(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}

type fakeTrainingOutcomes struct {
	recorded []model.Outcome
}

func (f *fakeTrainingOutcomes) RecordOutcome(_ context.Context, userID uuid.UUID, outcome model.Outcome) (*model.Outcome, bool, error) {
	outcome.ID, outcome.UserID = uuid.New(), userID
	f.recorded = append(f.recorded, outcome)
	return &outcome, true, nil
}

func TestUpdateLogWithFeedbackPreservesExercisesWhenReassessmentOmitsThem(t *testing.T) {
	userID := uuid.New()
	plan := &model.TrainingPlan{ID: uuid.New(), UserID: userID, Status: "active"}
	original := json.RawMessage(`[{"name":"chin tuck","completed":true}]`)
	repo := &fakeTrainingRepository{active: plan, log: &model.TrainingLog{ID: uuid.New(), UserID: userID, PlanID: plan.ID, Exercises: original}}
	svc := NewTrainingService(repo, nil)

	if _, err := svc.UpdateLogWithFeedback(context.Background(), plan.ID, userID, TrainingFeedbackInput{
		Notes: "复评反馈", TrainingFeeling: "更稳定",
	}); err != nil {
		t.Fatalf("UpdateLogWithFeedback returned error: %v", err)
	}
	if repo.savedLog == nil || string(repo.savedLog.Exercises) != string(original) {
		t.Fatalf("reassessment overwrote exercises: got=%s want=%s", repo.savedLog.Exercises, original)
	}
}

func TestTrainingFeedbackRecordsAnUnlinkedOutcomeOnly(t *testing.T) {
	userID := uuid.New()
	plan := &model.TrainingPlan{ID: uuid.New(), UserID: userID, Status: "active"}
	repo := &fakeTrainingRepository{active: plan}
	outcomes := &fakeTrainingOutcomes{}
	svc := NewTrainingService(repo, outcomes)

	result, err := svc.Reassess(context.Background(), plan.ID, userID, TrainingFeedbackInput{
		SymptomChanges: "肩膀不适有所改善", Trend: "improving",
	})
	if err != nil {
		t.Fatalf("Reassess returned error: %v", err)
	}
	if len(outcomes.recorded) != 1 || outcomes.recorded[0].Kind != "symptom_change" {
		t.Fatalf("feedback must record one symptom Outcome: %#v", outcomes.recorded)
	}
	if len(result) != 1 || result["outcome"] == nil {
		t.Fatalf("feedback result must carry only the Outcome, never a plan proposal: %#v", result)
	}
}

func TestTrainingCheckInRecordsAdherenceOutcome(t *testing.T) {
	userID := uuid.New()
	plan := &model.TrainingPlan{ID: uuid.New(), UserID: userID, Status: "active"}
	outcomes := &fakeTrainingOutcomes{}
	svc := NewTrainingService(&fakeTrainingRepository{active: plan}, outcomes)

	if err := svc.CheckIn(context.Background(), plan.ID, userID); err != nil {
		t.Fatalf("CheckIn returned error: %v", err)
	}
	if len(outcomes.recorded) != 1 || outcomes.recorded[0].Kind != "training_adherence" {
		t.Fatalf("check-in must record one adherence Outcome: %#v", outcomes.recorded)
	}
}

func TestTrainingCheckInRejectsInactivePlan(t *testing.T) {
	userID := uuid.New()
	plan := &model.TrainingPlan{ID: uuid.New(), UserID: userID, Status: "paused"}
	outcomes := &fakeTrainingOutcomes{}
	svc := NewTrainingService(&fakeTrainingRepository{active: plan}, outcomes)

	if err := svc.CheckIn(context.Background(), plan.ID, userID); err == nil {
		t.Fatal("a paused plan must not accept a check-in")
	}
	if len(outcomes.recorded) != 0 {
		t.Fatalf("a rejected check-in must not record an Outcome: %#v", outcomes.recorded)
	}
}
