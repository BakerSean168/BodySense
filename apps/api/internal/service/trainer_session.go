package service

import (
	"context"
	"errors"
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
	ErrTrainerSessionPaused     = errors.New("training session is paused")
	ErrTrainerSessionNotPaused  = errors.New("training session is not paused")
	ErrTrainerSetRecorded       = errors.New("this set or exercise already has a recorded action")
	ErrTrainerSessionNotEnded   = errors.New("training session has not ended")
	ErrTrainerFeedbackRecorded  = errors.New("feedback for this training session is already recorded")
)

// TrainerGateError is a Go TrainerGate decision that did not permit the
// action. Nothing was written; Decision carries the reasons to show the user.
type TrainerGateError struct{ Decision safety.Decision }

func (e *TrainerGateError) Error() string { return "trainer gate: " + string(e.Decision.Result) }

const (
	TrainerSessionInProgress = "in_progress"
	TrainerSessionPaused     = "paused"
	TrainerSessionEnded      = "ended"
	TrainerScopeSet          = "set"
	TrainerScopeExercise     = "exercise"
	TrainerSetPerformed      = "performed"
	TrainerSetSkipped        = "skipped"
	TrainerSetCorrected      = "corrected"
	TrainerTransitionPause   = "pause"
	TrainerTransitionResume  = "resume"
	TrainerEndedFinished     = "finished"
	TrainerEndedStoppedEarly = "stopped_early"
	trainerSessionListLimit  = 50
	trainerMaxActualReps     = 1000
	trainerMaxActualSeconds  = 86400
	trainerMaxFeedbackRunes  = 2000
)

// TrainerSessionView is one owned session with its receipts in revision order
// and its feedback, if the user gave any.
type TrainerSessionView struct {
	Session     model.TrainingSession
	Actions     []model.TrainingSetAction
	Transitions []model.TrainingSessionTransition
	Feedback    *model.TrainingSessionFeedback
}

// TrainerEffectiveActions drops corrections and the receipts they retracted:
// what remains is what the user's execution record currently says (TP-D8).
func TrainerEffectiveActions(actions []model.TrainingSetAction) []model.TrainingSetAction {
	retracted := map[uuid.UUID]bool{}
	for _, a := range actions {
		if a.CorrectsActionID != nil {
			retracted[*a.CorrectsActionID] = true
		}
	}
	effective := []model.TrainingSetAction{}
	for _, a := range actions {
		if a.Kind != TrainerSetCorrected && !retracted[a.ID] {
			effective = append(effective, a)
		}
	}
	return effective
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

func (s *TrainerSessionService) lock(ctx context.Context, owner uuid.UUID) error {
	return trainerError(s.owner.LockOwnerForPlanActivation(ctx, owner))
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

func (s *TrainerSessionService) view(ctx context.Context, owner uuid.UUID, session model.TrainingSession) (*TrainerSessionView, error) {
	views, err := s.views(ctx, owner, []model.TrainingSession{session})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// views attaches each session's receipts and transitions in revision order.
func (s *TrainerSessionService) views(ctx context.Context, owner uuid.UUID, sessions []model.TrainingSession) ([]TrainerSessionView, error) {
	ids := make([]uuid.UUID, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	actions, err := s.repo.Actions(ctx, owner, ids)
	if err != nil {
		return nil, err
	}
	transitions, err := s.repo.Transitions(ctx, owner, ids)
	if err != nil {
		return nil, err
	}
	feedback, err := s.repo.Feedback(ctx, owner, ids)
	if err != nil {
		return nil, err
	}
	views := make([]TrainerSessionView, 0, len(sessions))
	index := map[uuid.UUID]int{}
	for i, session := range sessions {
		index[session.ID] = i
		views = append(views, TrainerSessionView{Session: session, Actions: []model.TrainingSetAction{}, Transitions: []model.TrainingSessionTransition{}})
	}
	for _, a := range actions {
		views[index[a.SessionID]].Actions = append(views[index[a.SessionID]].Actions, a)
	}
	for _, t := range transitions {
		views[index[t.SessionID]].Transitions = append(views[index[t.SessionID]].Transitions, t)
	}
	for i := range feedback {
		views[index[feedback[i].SessionID]].Feedback = &feedback[i]
	}
	return views, nil
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
	if state != "" && state != TrainerSessionInProgress && state != TrainerSessionPaused && state != TrainerSessionEnded {
		return nil, ErrTrainerInvalid
	}
	views := []TrainerSessionView{}
	err := s.tx.WithinReadSnapshot(ctx, func(ctx context.Context) error {
		sessions, err := s.repo.List(ctx, owner, state, trainerSessionListLimit)
		if err != nil {
			return err
		}
		views, err = s.views(ctx, owner, sessions)
		return err
	})
	if err != nil {
		return nil, err
	}
	return views, nil
}
