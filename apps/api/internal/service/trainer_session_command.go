package service

import (
	"strings"
	"unicode/utf8"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
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

// validTrainerFeedback accepts verbatim UTF-8 text of 1..2000 characters that
// is not only whitespace. It is not trimmed or rewritten.
func validTrainerFeedback(text string) bool {
	return utf8.ValidString(text) && strings.TrimSpace(text) != "" && !strings.ContainsRune(text, 0) &&
		utf8.RuneCountInString(text) <= trainerMaxFeedbackRunes
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
