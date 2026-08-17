package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeTrainingRepository struct {
	active          *model.TrainingPlan
	byRevision      *model.TrainingPlan
	created         *model.TrainingPlan
	createErr       error
	supersedeErr    error
	supersedeCalled bool
}

func (r *fakeTrainingRepository) CreatePlan(_ context.Context, plan *model.TrainingPlan) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = plan
	r.active = plan
	r.byRevision = plan
	return nil
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
func (r *fakeTrainingRepository) GetPlanByTreatmentRevision(context.Context, uuid.UUID, uuid.UUID) (*model.TrainingPlan, error) {
	return r.byRevision, nil
}
func (r *fakeTrainingRepository) SupersedePlansExcept(context.Context, uuid.UUID, uuid.UUID) error {
	r.supersedeCalled = true
	return r.supersedeErr
}
func (r *fakeTrainingRepository) GetOrCreateLog(context.Context, *model.TrainingPlan, time.Time) (*model.TrainingLog, error) {
	return &model.TrainingLog{ID: uuid.New(), Exercises: json.RawMessage(`[]`)}, nil
}
func (r *fakeTrainingRepository) SaveLog(context.Context, *model.TrainingLog) error { return nil }
func (r *fakeTrainingRepository) CheckInAndGet(context.Context, *model.TrainingPlan, time.Time) (*model.TrainingLog, error) {
	return &model.TrainingLog{ID: uuid.New()}, nil
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

type fakeTrainingTreatmentService struct {
	accepted  *model.Treatment
	acceptErr error
}

func (s *fakeTrainingTreatmentService) AcceptProposal(context.Context, uuid.UUID, uuid.UUID) (*model.Treatment, error) {
	if s.acceptErr != nil {
		return nil, s.acceptErr
	}
	return s.accepted, nil
}
func (s *fakeTrainingTreatmentService) RecordOutcome(context.Context, uuid.UUID, model.Outcome) (*model.Outcome, bool, error) {
	return nil, false, nil
}
func (s *fakeTrainingTreatmentService) GetCurrent(context.Context, uuid.UUID) (*model.Treatment, error) {
	return s.accepted, nil
}
func (s *fakeTrainingTreatmentService) GenerateProposal(context.Context, uuid.UUID, TreatmentProposalInput) (*model.TreatmentRevision, error) {
	return nil, nil
}

func acceptedTreatment(userID uuid.UUID) *model.Treatment {
	treatmentID := uuid.New()
	revisionID := uuid.New()
	plan, _ := json.Marshal(model.TreatmentPlanContent{
		Summary: "neck plan", Goal: "reduce neck load", DurationWeeks: 4,
		Interventions: []model.TreatmentInterventionDraft{{
			Kind: "exercise", Title: "chin tuck", Description: "controlled",
			Prescription: map[string]any{"sets": 3, "reps": 10},
		}},
	})
	return &model.Treatment{
		ID: treatmentID, UserID: userID, Status: model.TreatmentStatusActive,
		Current: &model.TreatmentRevision{
			ID: revisionID, TreatmentID: treatmentID, Revision: 1,
			AcceptanceState: model.TreatmentAcceptanceAccepted,
			LifecycleState:  model.TreatmentStatusActive,
			Goal:            "reduce neck load", DurationWeeks: 4, Plan: plan,
			Interventions: []model.Intervention{{
				ID: uuid.New(), UserID: userID, TreatmentID: treatmentID,
				TreatmentRevisionID: revisionID, Kind: "exercise", Title: "chin tuck",
				Description: "controlled", Prescription: datatypes.JSON(`{"sets":3,"reps":10}`),
			}},
		},
	}
}

func TestAcceptTreatmentAndEnsurePlanUsesOneUnitOfWork(t *testing.T) {
	userID := uuid.New()
	called := false
	repo := &fakeTrainingRepository{}
	treatment := acceptedTreatment(userID)
	svc := NewTrainingService(
		repo,
		&fakeTrainingTreatmentService{accepted: treatment},
		testTreatmentUnitOfWork{called: &called},
	)

	accepted, plan, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(), userID, treatment.Current.ID, nil)
	if err != nil {
		t.Fatalf("AcceptTreatmentAndEnsurePlan returned error: %v", err)
	}
	if !called || accepted == nil || plan == nil {
		t.Fatalf("expected one coordinated activation: called=%v accepted=%#v plan=%#v", called, accepted, plan)
	}
	if plan.TreatmentRevisionID == nil || *plan.TreatmentRevisionID != treatment.Current.ID {
		t.Fatalf("training plan did not pin accepted revision: %#v", plan)
	}
	if !repo.supersedeCalled {
		t.Fatal("activation must supersede older active training projections")
	}
}

func TestAcceptTreatmentAndEnsurePlanSurfacesProjectionFailure(t *testing.T) {
	userID := uuid.New()
	treatment := acceptedTreatment(userID)
	svc := NewTrainingService(
		&fakeTrainingRepository{createErr: errors.New("insert failed")},
		&fakeTrainingTreatmentService{accepted: treatment},
		testTreatmentUnitOfWork{},
	)

	_, _, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(), userID, treatment.Current.ID, nil)
	if !errors.Is(err, ErrTrainingProjectionFailed) {
		t.Fatalf("expected projection error, got %v", err)
	}
}

func TestEnsurePlanForTreatmentIsIdempotent(t *testing.T) {
	userID := uuid.New()
	treatment := acceptedTreatment(userID)
	existing := &model.TrainingPlan{ID: uuid.New(), UserID: userID, Status: "active", TreatmentRevisionID: &treatment.Current.ID}
	repo := &fakeTrainingRepository{byRevision: existing, active: existing}
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment}, testTreatmentUnitOfWork{})

	plan, err := svc.EnsurePlanForTreatment(context.Background(), userID, nil, treatment)
	if err != nil || plan.ID != existing.ID {
		t.Fatalf("expected existing projection, plan=%#v err=%v", plan, err)
	}
	if repo.created != nil {
		t.Fatal("idempotent projection lookup must not create a duplicate row")
	}
	if !repo.supersedeCalled {
		t.Fatal("idempotent recovery must re-run superseding cleanup")
	}
}
