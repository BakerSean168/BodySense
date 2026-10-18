package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Trainer runtime errors (TP-003). NotFound is owner-masked: a foreign plan or
// session reads exactly like a missing one.
var (
	ErrTrainerNotFound          = errors.New("training plan or session not found")
	ErrTrainerInvalid           = errors.New("invalid trainer command")
	ErrTrainerConflict          = errors.New("training session revision or idempotency conflict")
	ErrTrainerSafetyStale       = errors.New("safety state changed since the user saw it")
	ErrTrainerPlanNotActive     = errors.New("training plan is not active")
	ErrTrainerSessionInProgress = errors.New("another training session is in progress")
	ErrTrainerSessionEnded      = errors.New("training session already ended")
	ErrTrainerSetRecorded       = errors.New("this set already has a recorded action")
)

// TrainerGateError is a Go TrainerGate decision that did not permit the
// action. Nothing was written; Decision carries the reasons to show the user.
type TrainerGateError struct{ Decision safety.Decision }

func (e *TrainerGateError) Error() string { return "trainer gate: " + string(e.Decision.Result) }

const (
	TrainerSessionInProgress = "in_progress"
	TrainerSessionEnded      = "ended"
	TrainerSetPerformed      = "performed"
	TrainerSetSkipped        = "skipped"
	TrainerEndedFinished     = "finished"
	TrainerEndedStoppedEarly = "stopped_early"
	trainerSessionListLimit  = 50
)

// StartTrainerSessionCommand is the authenticated user's explicit start of
// one template of an owned accepted plan. ExpectedSafetyRevision is the
// Safety revision of the gate the user was shown; any other current revision
// is stale and blocks.
type StartTrainerSessionCommand struct {
	PlanID                 uuid.UUID
	ExpectedPlanRevisionID uuid.UUID
	TemplateID             string
	TimeZone               string
	ExpectedSafetyRevision int64
	IdempotencyKey         string
}

// TrainerSetActionCommand records one set the user explicitly marked as
// performed or skipped. It is never derived from navigation or the Agent.
type TrainerSetActionCommand struct {
	SessionID               uuid.UUID
	PlanExerciseID          string
	SetNumber               int
	Kind                    string
	ExpectedSessionRevision int64
	ExpectedSafetyRevision  int64
	IdempotencyKey          string
}

// FinishTrainerSessionCommand is the user's explicit end of a session. Ending
// is never gated (stopping is always allowed) and never marks remaining sets.
type FinishTrainerSessionCommand struct {
	SessionID               uuid.UUID
	EndedReason             string
	ExpectedSessionRevision int64
	IdempotencyKey          string
}

// TrainerSessionView is one owned session with its receipts in revision order.
type TrainerSessionView struct {
	Session model.TrainingSession
	Actions []model.TrainingSetAction
}

// TrainerSessionService is the Go-owned Trainer runtime. Every start and set
// action re-evaluates the TrainerGate inside the command transaction against
// the exact Safety revision the user saw; plans are only read, never written.
type TrainerSessionService struct {
	repo  *repository.TrainerSessionRepository
	plans *repository.IndependentTrainingPlanRepository
	owner *repository.TrainingRepository
	body  *BodyStateService
	tx    *database.TransactionManager
	now   func() time.Time
}

func NewTrainerSessionService(db *gorm.DB) *TrainerSessionService {
	return &TrainerSessionService{
		repo: repository.NewTrainerSessionRepository(db), plans: repository.NewIndependentTrainingPlanRepository(db),
		owner: repository.NewTrainingRepository(db), body: NewBodyStateService(repository.NewBodyStateRepository(db)),
		tx: database.NewTransactionManager(db), now: func() time.Time { return time.Now().UTC() },
	}
}

// TrainerTimeZone accepts only a loadable IANA zone name. "Local" and empty
// names would silently follow the server clock and are rejected.
func TrainerTimeZone(name string) (*time.Location, bool) {
	if name == "" || len(name) > 64 || strings.TrimSpace(name) != name || name == "Local" {
		return nil, false
	}
	loc, err := time.LoadLocation(name)
	return loc, err == nil
}

// TrainingDate is the user's local calendar date of instant in zone; it is
// fixed at session start and never recomputed (TP-D6).
func TrainingDate(instant time.Time, zone *time.Location) model.DateOnly {
	local := instant.In(zone)
	return model.DateOnly(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC))
}

func validTrainerText(value string) bool {
	return value != "" && len(value) <= 256
}

func trainerError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrIndependentPlanNotFound), errors.Is(err, repository.ErrTrainerSessionNotFound),
		errors.Is(err, repository.ErrTrainingPlanOwnerNotFound):
		return ErrTrainerNotFound
	case errors.Is(err, repository.ErrTrainerSessionInProgress):
		return ErrTrainerSessionInProgress
	case errors.Is(err, repository.ErrTrainerSessionRace):
		return ErrTrainerConflict
	}
	return err
}

func (s *TrainerSessionService) lock(ctx context.Context, owner uuid.UUID) error {
	return trainerError(s.owner.LockOwnerForPlanActivation(ctx, owner))
}

// admit share-locks the owner's Safety head, requires it to be exactly the
// revision the user saw, and requires the Go TrainerGate to permit action at
// that revision. Unavailable or unscreened Safety never permits.
func (s *TrainerSessionService) admit(ctx context.Context, owner uuid.UUID, expected int64, action safety.Action) (safety.Decision, error) {
	current, err := s.repo.SafetyRevision(ctx, owner)
	if err != nil {
		return safety.Decision{}, err
	}
	if current != expected {
		return safety.Decision{}, ErrTrainerSafetyStale
	}
	decision := s.body.EvaluateTrainerGate(ctx, owner, action, s.now())
	if !decision.Permits() {
		return decision, &TrainerGateError{Decision: decision}
	}
	if decision.SafetyRevision != expected {
		return decision, ErrTrainerSafetyStale
	}
	return decision, nil
}

// pinnedTemplate reads the session template from one immutable revision.
func (s *TrainerSessionService) pinnedTemplate(ctx context.Context, owner, plan, revision uuid.UUID, templateID string) (*model.PlanSessionTemplate, error) {
	rev, err := s.plans.Revision(ctx, owner, plan, revision)
	if err != nil {
		return nil, trainerError(err)
	}
	var draft model.TrainingPlanDraft
	if err := json.Unmarshal(rev.Snapshot, &draft); err != nil {
		return nil, fmt.Errorf("training plan revision snapshot: %w", err)
	}
	for i := range draft.Content.Sessions {
		if draft.Content.Sessions[i].TemplateID == templateID {
			return &draft.Content.Sessions[i], nil
		}
	}
	return nil, ErrTrainerInvalid
}

func gateExercise(e model.PlanExercise) safety.Exercise {
	return safety.Exercise{ExerciseID: e.ExerciseID, Custom: strings.HasPrefix(e.ExerciseID, "custom:")}
}

func (s *TrainerSessionService) view(ctx context.Context, owner uuid.UUID, session model.TrainingSession) (*TrainerSessionView, error) {
	actions, err := s.repo.Actions(ctx, owner, []uuid.UUID{session.ID})
	if err != nil {
		return nil, err
	}
	return &TrainerSessionView{Session: session, Actions: actions}, nil
}

// Start opens one session bound to the plan's current accepted revision.
func (s *TrainerSessionService) Start(ctx context.Context, owner uuid.UUID, cmd StartTrainerSessionCommand) (*TrainerSessionView, error) {
	zone, ok := TrainerTimeZone(cmd.TimeZone)
	if owner == uuid.Nil || cmd.PlanID == uuid.Nil || cmd.ExpectedPlanRevisionID == uuid.Nil || !validTrainerText(cmd.TemplateID) ||
		!ok || cmd.ExpectedSafetyRevision <= 0 || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   StartTrainerSessionCommand
	}{"trainer_start", cmd})
	var result *TrainerSessionView
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.SessionByStartKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.StartRequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			result, err = s.view(ctx, owner, *existing)
			return err
		}
		plan, err := s.plans.Plan(ctx, owner, cmd.PlanID)
		if err != nil {
			return trainerError(err)
		}
		if plan.Status != "active" {
			return ErrTrainerPlanNotActive
		}
		if *plan.CurrentAcceptedRevisionID != cmd.ExpectedPlanRevisionID {
			return ErrTrainerConflict
		}
		template, err := s.pinnedTemplate(ctx, owner, plan.ID, cmd.ExpectedPlanRevisionID, cmd.TemplateID)
		if err != nil {
			return err
		}
		action := safety.Action{Kind: safety.ActionStartSession, PlanID: plan.ID}
		for _, stage := range template.Stages {
			for _, exercise := range stage.Exercises {
				action.Exercises = append(action.Exercises, gateExercise(exercise))
			}
		}
		decision, err := s.admit(ctx, owner, cmd.ExpectedSafetyRevision, action)
		if err != nil {
			return err
		}
		now := s.now()
		session := model.TrainingSession{
			ID: uuid.New(), UserID: owner, PlanID: plan.ID, PlanRevisionID: cmd.ExpectedPlanRevisionID, TemplateID: cmd.TemplateID,
			TimeZone: cmd.TimeZone, TrainingDate: TrainingDate(now, zone), State: TrainerSessionInProgress, StartedAt: now,
			StartIdempotencyKey: cmd.IdempotencyKey, StartRequestFingerprint: fingerprint, StartSafetyRevision: decision.SafetyRevision,
			StartGateResult: string(decision.Result), StartGatePolicyRevision: decision.PolicyRevision,
		}
		if err := s.repo.InsertSession(ctx, &session); err != nil {
			return trainerError(err)
		}
		result = &TrainerSessionView{Session: session, Actions: []model.TrainingSetAction{}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RecordSetAction appends one user-originated set receipt at the next
// session revision after the TrainerGate admits that exercise.
func (s *TrainerSessionService) RecordSetAction(ctx context.Context, owner uuid.UUID, cmd TrainerSetActionCommand) (*model.TrainingSetAction, error) {
	if owner == uuid.Nil || cmd.SessionID == uuid.Nil || !validTrainerText(cmd.PlanExerciseID) || cmd.SetNumber < 1 ||
		(cmd.Kind != TrainerSetPerformed && cmd.Kind != TrainerSetSkipped) || cmd.ExpectedSessionRevision < 0 ||
		cmd.ExpectedSafetyRevision <= 0 || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   TrainerSetActionCommand
	}{"trainer_set_action", cmd})
	var result *model.TrainingSetAction
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.ActionByKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			result = existing
			return nil
		}
		session, err := s.repo.Session(ctx, owner, cmd.SessionID, true)
		if err != nil {
			return trainerError(err)
		}
		if session.State != TrainerSessionInProgress {
			return ErrTrainerSessionEnded
		}
		if session.Revision != cmd.ExpectedSessionRevision {
			return ErrTrainerConflict
		}
		template, err := s.pinnedTemplate(ctx, owner, session.PlanID, session.PlanRevisionID, session.TemplateID)
		if err != nil {
			return err
		}
		var exercise *model.PlanExercise
		for _, stage := range template.Stages {
			for i := range stage.Exercises {
				if stage.Exercises[i].PlanExerciseID == cmd.PlanExerciseID {
					exercise = &stage.Exercises[i]
				}
			}
		}
		if exercise == nil || cmd.SetNumber > exercise.Sets {
			return ErrTrainerInvalid
		}
		recorded, err := s.repo.SetRecorded(ctx, owner, session.ID, cmd.PlanExerciseID, cmd.SetNumber)
		if err != nil {
			return err
		}
		if recorded {
			return ErrTrainerSetRecorded
		}
		action := safety.Action{Kind: safety.ActionPerformSet, PlanID: session.PlanID, Exercises: []safety.Exercise{gateExercise(*exercise)}}
		decision, err := s.admit(ctx, owner, cmd.ExpectedSafetyRevision, action)
		if err != nil {
			return err
		}
		row := model.TrainingSetAction{
			ID: uuid.New(), UserID: owner, SessionID: session.ID, PlanExerciseID: exercise.PlanExerciseID, ExerciseID: exercise.ExerciseID,
			SetNumber: cmd.SetNumber, Kind: cmd.Kind, SessionRevision: session.Revision + 1, IdempotencyKey: cmd.IdempotencyKey,
			RequestFingerprint: fingerprint, SafetyRevision: decision.SafetyRevision, GateResult: string(decision.Result),
			GatePolicyRevision: decision.PolicyRevision, RecordedAt: s.now(),
		}
		if err := s.repo.AppendAction(ctx, &row); err != nil {
			return trainerError(err)
		}
		result = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Finish ends an in-progress session once, as finished or stopped early.
// Completion is derived from the recorded receipts; nothing is backfilled.
func (s *TrainerSessionService) Finish(ctx context.Context, owner uuid.UUID, cmd FinishTrainerSessionCommand) (*TrainerSessionView, error) {
	if owner == uuid.Nil || cmd.SessionID == uuid.Nil || (cmd.EndedReason != TrainerEndedFinished && cmd.EndedReason != TrainerEndedStoppedEarly) ||
		cmd.ExpectedSessionRevision < 0 || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   FinishTrainerSessionCommand
	}{"trainer_finish", cmd})
	var result *TrainerSessionView
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.SessionByFinishKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if *existing.FinishRequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			result, err = s.view(ctx, owner, *existing)
			return err
		}
		session, err := s.repo.Session(ctx, owner, cmd.SessionID, true)
		if err != nil {
			return trainerError(err)
		}
		if session.State != TrainerSessionInProgress {
			return ErrTrainerSessionEnded
		}
		if session.Revision != cmd.ExpectedSessionRevision {
			return ErrTrainerConflict
		}
		now := s.now()
		reason, key, digest := cmd.EndedReason, cmd.IdempotencyKey, fingerprint
		session.State, session.EndedReason, session.EndedAt, session.Revision = TrainerSessionEnded, &reason, &now, session.Revision+1
		session.FinishIdempotencyKey, session.FinishRequestFingerprint = &key, &digest
		if err := s.repo.End(ctx, session, cmd.ExpectedSessionRevision); err != nil {
			return trainerError(err)
		}
		result, err = s.view(ctx, owner, *session)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Get returns one owned session; another owner's id is not found.
func (s *TrainerSessionService) Get(ctx context.Context, owner, id uuid.UUID) (*TrainerSessionView, error) {
	var result *TrainerSessionView
	err := s.tx.WithinReadSnapshot(ctx, func(ctx context.Context) error {
		session, err := s.repo.Session(ctx, owner, id, false)
		if err != nil {
			return trainerError(err)
		}
		result, err = s.view(ctx, owner, *session)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// List returns the owner's newest sessions, optionally filtered by state.
func (s *TrainerSessionService) List(ctx context.Context, owner uuid.UUID, state string) ([]TrainerSessionView, error) {
	if state != "" && state != TrainerSessionInProgress && state != TrainerSessionEnded {
		return nil, ErrTrainerInvalid
	}
	views := []TrainerSessionView{}
	err := s.tx.WithinReadSnapshot(ctx, func(ctx context.Context) error {
		sessions, err := s.repo.List(ctx, owner, state, trainerSessionListLimit)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, 0, len(sessions))
		for _, session := range sessions {
			ids = append(ids, session.ID)
		}
		actions, err := s.repo.Actions(ctx, owner, ids)
		if err != nil {
			return err
		}
		bySession := map[uuid.UUID][]model.TrainingSetAction{}
		for _, action := range actions {
			bySession[action.SessionID] = append(bySession[action.SessionID], action)
		}
		for _, session := range sessions {
			own := bySession[session.ID]
			if own == nil {
				own = []model.TrainingSetAction{}
			}
			views = append(views, TrainerSessionView{Session: session, Actions: own})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return views, nil
}
