package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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

// TrainerSetActionCommand records one explicit user action on a target of
// the pinned template: a set (Scope "set", SetNumber >= 1) performed or
// skipped, a whole exercise skipped (Scope "exercise", SetNumber 0), or a
// correction retracting CorrectsActionID on that same target. Actual reps
// and duration are optional and only describe a performed set. It is never
// derived from navigation or the Agent.
type TrainerSetActionCommand struct {
	SessionID               uuid.UUID
	PlanExerciseID          string
	Scope                   string
	SetNumber               int
	Kind                    string
	CorrectsActionID        uuid.UUID
	ActualReps              *int
	ActualDurationSeconds   *int
	ExpectedSessionRevision int64
	ExpectedSafetyRevision  int64
	IdempotencyKey          string
}

// TrainerTransitionCommand pauses or resumes a session by explicit user
// command. Only a resume carries ExpectedSafetyRevision: it re-runs the gate.
type TrainerTransitionCommand struct {
	SessionID               uuid.UUID
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

// TrainerFeedbackCommand records the user's own words about one ended
// session (TP-D10). Text is stored exactly as given; it is never gated,
// never interpreted and never changes the plan, BodyState or Safety.
type TrainerFeedbackCommand struct {
	SessionID      uuid.UUID
	Text           string
	IdempotencyKey string
}

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

func validTrainerActual(value *int, limit int) bool {
	return value == nil || (*value >= 1 && *value <= limit)
}

func (cmd TrainerSetActionCommand) valid() bool {
	if cmd.SessionID == uuid.Nil || !validTrainerText(cmd.PlanExerciseID) || cmd.ExpectedSessionRevision < 0 ||
		cmd.ExpectedSafetyRevision <= 0 || !validPlanKey(cmd.IdempotencyKey) {
		return false
	}
	switch cmd.Scope {
	case TrainerScopeSet:
		if cmd.SetNumber < 1 {
			return false
		}
	case TrainerScopeExercise:
		if cmd.SetNumber != 0 || cmd.Kind == TrainerSetPerformed {
			return false
		}
	default:
		return false
	}
	switch cmd.Kind {
	case TrainerSetPerformed, TrainerSetSkipped, TrainerSetCorrected:
	default:
		return false
	}
	if (cmd.Kind == TrainerSetCorrected) != (cmd.CorrectsActionID != uuid.Nil) {
		return false
	}
	if cmd.Kind != TrainerSetPerformed && (cmd.ActualReps != nil || cmd.ActualDurationSeconds != nil) {
		return false
	}
	return validTrainerActual(cmd.ActualReps, trainerMaxActualReps) && validTrainerActual(cmd.ActualDurationSeconds, trainerMaxActualSeconds)
}

func trainerSetNumber(a model.TrainingSetAction) int {
	if a.SetNumber == nil {
		return 0
	}
	return *a.SetNumber
}

// checkTrainerTarget keeps at most one effective receipt per set and one
// effective skip per exercise. A correction must retract an effective, not
// yet corrected receipt of exactly the same target; it never targets another
// correction. actions are the session's receipts for this plan exercise.
func checkTrainerTarget(actions []model.TrainingSetAction, cmd TrainerSetActionCommand, sets int) error {
	effective := TrainerEffectiveActions(actions)
	if cmd.Kind == TrainerSetCorrected {
		for _, a := range actions {
			if a.ID != cmd.CorrectsActionID {
				continue
			}
			if a.Kind == TrainerSetCorrected || a.Scope != cmd.Scope || trainerSetNumber(a) != cmd.SetNumber {
				return ErrTrainerInvalid
			}
			for _, e := range effective {
				if e.ID == a.ID {
					return nil
				}
			}
			return ErrTrainerConflict
		}
		return ErrTrainerInvalid
	}
	recordedSets := 0
	for _, e := range effective {
		switch {
		case e.Scope == TrainerScopeExercise:
			return ErrTrainerSetRecorded
		case cmd.Scope == TrainerScopeSet && trainerSetNumber(e) == cmd.SetNumber:
			return ErrTrainerSetRecorded
		}
		recordedSets++
	}
	if cmd.Scope == TrainerScopeExercise && recordedSets >= sets {
		return ErrTrainerSetRecorded
	}
	return nil
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

// validTrainerFeedback accepts verbatim UTF-8 text of 1..2000 characters that
// is not only whitespace. It is not trimmed or rewritten.
func validTrainerFeedback(text string) bool {
	return utf8.ValidString(text) && strings.TrimSpace(text) != "" && !strings.ContainsRune(text, 0) &&
		utf8.RuneCountInString(text) <= trainerMaxFeedbackRunes
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

// templateGateAction is a session-scope gate action over every exercise of
// the template (start and resume).
func templateGateAction(kind safety.ActionKind, plan uuid.UUID, template *model.PlanSessionTemplate) safety.Action {
	action := safety.Action{Kind: kind, PlanID: plan}
	for _, stage := range template.Stages {
		for _, exercise := range stage.Exercises {
			action.Exercises = append(action.Exercises, gateExercise(exercise))
		}
	}
	return action
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
		decision, err := s.admit(ctx, owner, cmd.ExpectedSafetyRevision, templateGateAction(safety.ActionStartSession, plan.ID, template))
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
		result = &TrainerSessionView{Session: session, Actions: []model.TrainingSetAction{}, Transitions: []model.TrainingSessionTransition{}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RecordSetAction appends one user-originated receipt at the next session
// revision after the TrainerGate admits that exercise. Only an in-progress
// session accepts actions; a paused one must be resumed first.
func (s *TrainerSessionService) RecordSetAction(ctx context.Context, owner uuid.UUID, cmd TrainerSetActionCommand) (*model.TrainingSetAction, error) {
	if owner == uuid.Nil || !cmd.valid() {
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
		if err := trainerLiveState(session, TrainerSessionInProgress); err != nil {
			return err
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
		prior, err := s.repo.ExerciseActions(ctx, owner, session.ID, cmd.PlanExerciseID)
		if err != nil {
			return err
		}
		if err := checkTrainerTarget(prior, cmd, exercise.Sets); err != nil {
			return err
		}
		action := safety.Action{Kind: safety.ActionPerformSet, PlanID: session.PlanID, Exercises: []safety.Exercise{gateExercise(*exercise)}}
		decision, err := s.admit(ctx, owner, cmd.ExpectedSafetyRevision, action)
		if err != nil {
			return err
		}
		row := model.TrainingSetAction{
			ID: uuid.New(), UserID: owner, SessionID: session.ID, PlanExerciseID: exercise.PlanExerciseID, ExerciseID: exercise.ExerciseID,
			Scope: cmd.Scope, Kind: cmd.Kind, ActualReps: cmd.ActualReps, ActualDurationSeconds: cmd.ActualDurationSeconds,
			SessionRevision: session.Revision + 1, IdempotencyKey: cmd.IdempotencyKey, RequestFingerprint: fingerprint,
			SafetyRevision: decision.SafetyRevision, GateResult: string(decision.Result), GatePolicyRevision: decision.PolicyRevision, RecordedAt: s.now(),
		}
		if cmd.Scope == TrainerScopeSet {
			number := cmd.SetNumber
			row.SetNumber = &number
		}
		if cmd.Kind == TrainerSetCorrected {
			target := cmd.CorrectsActionID
			row.CorrectsActionID = &target
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

// trainerLiveState requires the session to be in want; ended and the other
// live state are distinct, visible refusals.
func trainerLiveState(session *model.TrainingSession, want string) error {
	switch {
	case session.State == want:
		return nil
	case session.State == TrainerSessionEnded:
		return ErrTrainerSessionEnded
	case session.State == TrainerSessionPaused:
		return ErrTrainerSessionPaused
	}
	return ErrTrainerSessionNotPaused
}

// Pause moves an in-progress session to paused. Pausing is never gated and
// leaves every receipt as it is; it never marks a set or ends the session.
func (s *TrainerSessionService) Pause(ctx context.Context, owner uuid.UUID, cmd TrainerTransitionCommand) (*TrainerSessionView, error) {
	return s.transition(ctx, owner, TrainerTransitionPause, cmd)
}

// Resume moves a paused session back to in progress after re-running the
// TrainerGate over the template against exactly ExpectedSafetyRevision, as a
// start does. The plan must still be active; another session in progress
// blocks it.
func (s *TrainerSessionService) Resume(ctx context.Context, owner uuid.UUID, cmd TrainerTransitionCommand) (*TrainerSessionView, error) {
	return s.transition(ctx, owner, TrainerTransitionResume, cmd)
}

func (s *TrainerSessionService) transition(ctx context.Context, owner uuid.UUID, kind string, cmd TrainerTransitionCommand) (*TrainerSessionView, error) {
	resume := kind == TrainerTransitionResume
	if owner == uuid.Nil || cmd.SessionID == uuid.Nil || cmd.ExpectedSessionRevision < 0 || cmd.ExpectedSafetyRevision < 0 ||
		resume != (cmd.ExpectedSafetyRevision > 0) || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	from, to := TrainerSessionInProgress, TrainerSessionPaused
	if resume {
		from, to = TrainerSessionPaused, TrainerSessionInProgress
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   TrainerTransitionCommand
	}{"trainer_" + kind, cmd})
	var result *TrainerSessionView
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.TransitionByKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			session, err := s.repo.Session(ctx, owner, existing.SessionID, false)
			if err != nil {
				return trainerError(err)
			}
			result, err = s.view(ctx, owner, *session)
			return err
		}
		session, err := s.repo.Session(ctx, owner, cmd.SessionID, true)
		if err != nil {
			return trainerError(err)
		}
		if err := trainerLiveState(session, from); err != nil {
			return err
		}
		if session.Revision != cmd.ExpectedSessionRevision {
			return ErrTrainerConflict
		}
		row := model.TrainingSessionTransition{
			ID: uuid.New(), UserID: owner, SessionID: session.ID, Kind: kind, SessionRevision: session.Revision + 1,
			IdempotencyKey: cmd.IdempotencyKey, RequestFingerprint: fingerprint, RecordedAt: s.now(),
		}
		if resume {
			plan, err := s.plans.Plan(ctx, owner, session.PlanID)
			if err != nil {
				return trainerError(err)
			}
			if plan.Status != "active" {
				return ErrTrainerPlanNotActive
			}
			template, err := s.pinnedTemplate(ctx, owner, session.PlanID, session.PlanRevisionID, session.TemplateID)
			if err != nil {
				return err
			}
			decision, err := s.admit(ctx, owner, cmd.ExpectedSafetyRevision, templateGateAction(safety.ActionResumeSession, session.PlanID, template))
			if err != nil {
				return err
			}
			gate, policy := string(decision.Result), decision.PolicyRevision
			row.SafetyRevision, row.GateResult, row.GatePolicyRevision = &decision.SafetyRevision, &gate, &policy
		}
		if err := s.repo.Transition(ctx, &row, from, to); err != nil {
			return trainerError(err)
		}
		session.State, session.Revision = to, row.SessionRevision
		result, err = s.view(ctx, owner, *session)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Finish ends an in-progress or paused session once, as finished or stopped
// early. Completion is derived from the effective receipts; nothing is
// backfilled.
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
		if session.State == TrainerSessionEnded {
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

// RecordFeedback stores the user's verbatim feedback on one ended session,
// once. It is never gated (the user can always say how a session went) and
// only feeds the plan review projection: no plan, revision, receipt, BodyState
// or Safety row is written. Body symptoms are recorded through BodyState.
func (s *TrainerSessionService) RecordFeedback(ctx context.Context, owner uuid.UUID, cmd TrainerFeedbackCommand) (*TrainerSessionView, error) {
	if owner == uuid.Nil || cmd.SessionID == uuid.Nil || !validTrainerFeedback(cmd.Text) || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   TrainerFeedbackCommand
	}{"trainer_feedback", cmd})
	var result *TrainerSessionView
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.FeedbackByKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			session, err := s.repo.Session(ctx, owner, existing.SessionID, false)
			if err != nil {
				return trainerError(err)
			}
			result, err = s.view(ctx, owner, *session)
			return err
		}
		session, err := s.repo.Session(ctx, owner, cmd.SessionID, true)
		if err != nil {
			return trainerError(err)
		}
		if session.State != TrainerSessionEnded {
			return ErrTrainerSessionNotEnded
		}
		prior, err := s.repo.Feedback(ctx, owner, []uuid.UUID{session.ID})
		if err != nil {
			return err
		}
		if len(prior) > 0 {
			return ErrTrainerFeedbackRecorded
		}
		row := model.TrainingSessionFeedback{
			ID: uuid.New(), UserID: owner, SessionID: session.ID, PlanID: session.PlanID, PlanRevisionID: session.PlanRevisionID,
			Text: cmd.Text, IdempotencyKey: cmd.IdempotencyKey, RequestFingerprint: fingerprint, RecordedAt: s.now(),
		}
		if err := s.repo.InsertFeedback(ctx, &row); err != nil {
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
