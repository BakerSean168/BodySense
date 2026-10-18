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
	otherPlan       *model.TrainingPlan
	log             *model.TrainingLog
	savedLog        *model.TrainingLog
	lockCalls       int
	lastLockedUser  uuid.UUID
	lockErr         error
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
func (r *fakeTrainingRepository) LockOwnerForPlanActivation(_ context.Context, userID uuid.UUID) error {
	r.lockCalls++
	r.lastLockedUser = userID
	return r.lockErr
}
func (r *fakeTrainingRepository) SupersedePlansExcept(context.Context, uuid.UUID, uuid.UUID) error {
	r.supersedeCalled = true
	if r.supersedeErr == nil && r.otherPlan != nil {
		r.otherPlan.Status = "superseded"
	}
	return r.supersedeErr
}
func (r *fakeTrainingRepository) GetOrCreateLog(context.Context, *model.TrainingPlan, time.Time) (*model.TrainingLog, error) {
	if r.log != nil {
		copy := *r.log
		return &copy, nil
	}
	return &model.TrainingLog{ID: uuid.New(), Exercises: json.RawMessage(`[]`)}, nil
}
func (r *fakeTrainingRepository) SaveLog(_ context.Context, log *model.TrainingLog) error {
	copy := *log
	r.savedLog = &copy
	return nil
}
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
	accepted    *model.Treatment
	acceptErr   error
	acceptCalls int
	onAccept    func() error
}

func (s *fakeTrainingTreatmentService) AcceptProposal(context.Context, uuid.UUID, uuid.UUID) (*model.Treatment, error) {
	s.acceptCalls++
	if s.onAccept != nil {
		if err := s.onAccept(); err != nil {
			return nil, err
		}
	}
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
		nil, testTreatmentUnitOfWork{called: &called},
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
		nil, testTreatmentUnitOfWork{},
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
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment}, nil, testTreatmentUnitOfWork{})

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

func TestIndependentTrainingProjectionPreservesOtherActivePlanAndIdempotentRetry(t *testing.T) {
	userID := uuid.New()
	old := &model.TrainingPlan{ID: uuid.New(), UserID: userID, Status: "active"}
	treatment := acceptedTreatment(userID)
	repo := &fakeTrainingRepository{active: old, otherPlan: old}
	svc := NewTrainingService(
		repo, &fakeTrainingTreatmentService{accepted: treatment},
		nil, testTreatmentUnitOfWork{}, WithIndependentTrainingPlans(),
	)

	_, first, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(), userID, treatment.Current.ID, nil)
	if err != nil || first == nil || first.TreatmentRevisionID == nil ||
		*first.TreatmentRevisionID != treatment.Current.ID {
		t.Fatalf("independent projection failed: plan=%#v err=%v", first, err)
	}
	if old.Status != "active" || repo.supersedeCalled {
		t.Fatal("independent projection must not alter another active plan")
	}
	_, second, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(), userID, treatment.Current.ID, nil)
	if err != nil || second == nil || second.ID != first.ID {
		t.Fatalf("idempotent retry changed projection: plan=%#v err=%v", second, err)
	}
	if old.Status != "active" || repo.supersedeCalled {
		t.Fatal("idempotent independent retry must not supersede unrelated plans")
	}
	if repo.lockCalls != 2 || repo.lastLockedUser != userID {
		t.Fatalf("every accepted projection/retry must lock its owner: calls=%d owner=%s",
			repo.lockCalls, repo.lastLockedUser)
	}
}

func TestIndependentTrainingProjectionSkipsLegacySupersedeFailure(t *testing.T) {
	userID := uuid.New()
	treatment := acceptedTreatment(userID)
	existing := &model.TrainingPlan{
		ID: uuid.New(), UserID: userID, Status: "active",
		TreatmentRevisionID: &treatment.Current.ID,
	}
	repo := &fakeTrainingRepository{
		byRevision: existing, supersedeErr: errors.New("legacy supersede failure"),
	}
	svc := NewTrainingService(
		repo, &fakeTrainingTreatmentService{accepted: treatment},
		nil, testTreatmentUnitOfWork{}, WithIndependentTrainingPlans(),
	)
	_, plan, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(), userID, treatment.Current.ID, nil)
	if err != nil || plan == nil || plan.ID != existing.ID {
		t.Fatalf("independent existing projection should be read-only: %#v %v", plan, err)
	}
	if repo.supersedeCalled || repo.created != nil {
		t.Fatal("existing independent projection must not write or supersede")
	}
}

func TestLegacyTrainingProjectionStillSupersedesByDefault(t *testing.T) {
	userID := uuid.New()
	other := &model.TrainingPlan{ID: uuid.New(), UserID: userID, Status: "active"}
	treatment := acceptedTreatment(userID)
	repo := &fakeTrainingRepository{otherPlan: other}
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment}, nil, testTreatmentUnitOfWork{})
	if _, err := svc.EnsurePlanForTreatment(context.Background(), userID, nil, treatment); err != nil {
		t.Fatal(err)
	}
	if !repo.supersedeCalled || other.Status != "superseded" {
		t.Fatalf("default legacy projection must preserve existing behavior: called=%v status=%s",
			repo.supersedeCalled, other.Status)
	}
}

func TestIndependentTrainingProjectionRejectsUnacceptedRevision(t *testing.T) {
	userID := uuid.New()
	treatment := acceptedTreatment(userID)
	treatment.Current.AcceptanceState = model.TreatmentAcceptanceProposed
	repo := &fakeTrainingRepository{}
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment},
		nil, testTreatmentUnitOfWork{}, WithIndependentTrainingPlans())
	_, plan, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(), userID, treatment.Current.ID, nil)
	if err == nil || plan != nil || repo.created != nil || repo.supersedeCalled {
		t.Fatalf("unaccepted revision was projected: plan=%#v err=%v", plan, err)
	}
}

func TestIndependentTrainingProjectionRejectsDirectUnlockedWrites(t *testing.T) {
	userID := uuid.New()
	treatment := acceptedTreatment(userID)
	repo := &fakeTrainingRepository{}
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment},
		nil, testTreatmentUnitOfWork{}, WithIndependentTrainingPlans())
	plan, err := svc.EnsurePlanForTreatment(context.Background(), userID, nil, treatment)
	if plan != nil || !errors.Is(err, ErrIndependentTrainingOwnerLockRequired) {
		t.Fatalf("independent direct projection must fail closed: plan=%#v err=%v", plan, err)
	}
	if repo.lockCalls != 0 || repo.created != nil || repo.supersedeCalled {
		t.Fatal("unlocked direct projection reached any write path")
	}
}

func TestIndependentTrainingProjectionRejectsAnotherOwnersLock(t *testing.T) {
	userID, other := uuid.New(), uuid.New()
	treatment := acceptedTreatment(userID)
	repo := &fakeTrainingRepository{}
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment},
		nil, testTreatmentUnitOfWork{}, WithIndependentTrainingPlans())
	lockedCtx, err := svc.lockIndependentTrainingOwner(context.Background(), other)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := svc.EnsurePlanForTreatment(lockedCtx, userID, nil, treatment)
	if plan != nil || !errors.Is(err, ErrIndependentTrainingOwnerLockRequired) {
		t.Fatalf("other owner's witness must not authorize plan write: %#v %v", plan, err)
	}
	if repo.created != nil || repo.supersedeCalled || repo.lastLockedUser != other {
		t.Fatal("mismatched owner reached plan write path")
	}
}

func TestIndependentTrainingAcceptanceStopsBeforeProposalOnLockError(t *testing.T) {
	userID := uuid.New()
	treatment := acceptedTreatment(userID)
	repo := &fakeTrainingRepository{lockErr: errors.New("synthetic missing user")}
	tracker := &fakeTrainingTreatmentService{accepted: treatment}
	svc := NewTrainingService(repo, tracker,
		nil, testTreatmentUnitOfWork{}, WithIndependentTrainingPlans())
	accepted, plan, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(), userID,
		treatment.Current.ID, nil)
	if accepted != nil || plan != nil || !errors.Is(err, repo.lockErr) {
		t.Fatalf("lock failure must abort activation: treatment=%#v plan=%#v err=%v",
			accepted, plan, err)
	}
	if repo.created != nil || repo.supersedeCalled || repo.lockCalls != 1 || tracker.acceptCalls != 0 {
		t.Fatal("lock failure reached proposal acceptance or plan mutation")
	}
}

func TestIndependentTrainingLocksBeforeProposalAcceptance(t *testing.T) {
	userID := uuid.New()
	repo := &fakeTrainingRepository{}
	treatment := acceptedTreatment(userID)
	tracker := &fakeTrainingTreatmentService{
		accepted: treatment,
		onAccept: func() error {
			if repo.lockCalls != 1 || repo.lastLockedUser != userID {
				return errors.New("proposal acceptance started without the correct owner lock")
			}
			return nil
		},
	}
	svc := NewTrainingService(repo, tracker, nil, testTreatmentUnitOfWork{}, WithIndependentTrainingPlans())
	accepted, plan, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(), userID,
		treatment.Current.ID, nil)
	if err != nil || accepted == nil || plan == nil || tracker.acceptCalls != 1 {
		t.Fatalf("owner must lock before proposal acceptance: plan=%#v err=%v calls=%d",
			plan, err, tracker.acceptCalls)
	}
}

func TestIndependentTrainingRecoveryStopsBeforeCurrentLookupOnLockFailure(t *testing.T) {
	owner := uuid.New()
	treatment := acceptedTreatment(owner)
	repo := &fakeTrainingRepository{lockErr: errors.New("synthetic owner lock failed")}
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment},
		nil, testTreatmentUnitOfWork{}, WithIndependentTrainingPlans())
	got, err := svc.EnsureCurrentPlan(context.Background(), owner, nil)
	if got != nil || !errors.Is(err, repo.lockErr) || repo.created != nil {
		t.Fatalf("lock failure must not recover a plan: plan=%#v err=%v", got, err)
	}
}

func TestIndependentTrainingCurrentRecoveryIsOwnerSerialized(t *testing.T) {
	userID := uuid.New()
	treatment := acceptedTreatment(userID)
	repo := &fakeTrainingRepository{}
	called := false
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment},
		nil, testTreatmentUnitOfWork{called: &called}, WithIndependentTrainingPlans())
	first, err := svc.EnsureCurrentPlan(context.Background(), userID, nil)
	if err != nil || first == nil || first.TreatmentRevisionID == nil ||
		*first.TreatmentRevisionID != treatment.Current.ID {
		t.Fatalf("first recovery failed: plan=%#v err=%v", first, err)
	}
	second, err := svc.EnsureCurrentPlan(context.Background(), userID, nil)
	if err != nil || second == nil || second.ID != first.ID {
		t.Fatalf("repeated recovery changed plan identity: %#v err=%v", second, err)
	}
	if !called || repo.lockCalls != 2 || repo.supersedeCalled {
		t.Fatalf("recovery must lock before each lookup and never supersede: tx=%v locks=%d supersede=%v",
			called, repo.lockCalls, repo.supersedeCalled)
	}
}

func TestLegacyTrainingAcceptanceDoesNotAcquireNewV2Lock(t *testing.T) {
	userID := uuid.New()
	treatment := acceptedTreatment(userID)
	repo := &fakeTrainingRepository{lockErr: errors.New("must not be called")}
	svc := NewTrainingService(repo, &fakeTrainingTreatmentService{accepted: treatment},
		nil, testTreatmentUnitOfWork{})
	accepted, plan, err := svc.AcceptTreatmentAndEnsurePlan(context.Background(),
		userID, treatment.Current.ID, nil)
	if err != nil || accepted == nil || plan == nil || repo.lockCalls != 0 || !repo.supersedeCalled {
		t.Fatalf("legacy activation unexpectedly invoked V2 lock: plan=%#v err=%v lockCalls=%d",
			plan, err, repo.lockCalls)
	}
}

func TestUpdateLogWithFeedbackPreservesExercisesWhenReassessmentOmitsThem(t *testing.T) {
	userID := uuid.New()
	plan := &model.TrainingPlan{ID: uuid.New(), UserID: userID, Status: "active"}
	original := json.RawMessage(`[{
  "intervention_id":"11111111-1111-4111-8111-111111111111",
  "name":"chin tuck",
  "completed":true
}]`)
	repo := &fakeTrainingRepository{active: plan, log: &model.TrainingLog{ID: uuid.New(), UserID: userID, PlanID: plan.ID, Exercises: original}}
	svc := NewTrainingService(repo, nil, nil, testTreatmentUnitOfWork{})

	if _, err := svc.UpdateLogWithFeedback(context.Background(), plan.ID, userID, TrainingFeedbackInput{
		Notes: "复评反馈", TrainingFeeling: "更稳定",
	}); err != nil {
		t.Fatalf("UpdateLogWithFeedback returned error: %v", err)
	}
	if repo.savedLog == nil || string(repo.savedLog.Exercises) != string(original) {
		t.Fatalf("reassessment overwrote exercises: got=%s want=%s", repo.savedLog.Exercises, original)
	}
}
