package model

import (
	"time"

	"github.com/google/uuid"
)

// TrainingSession is one Trainer execution instance (TP-003). It pins the
// accepted plan revision and template it executes, the user's IANA timezone
// snapshot and the local training date at start, plus the start receipt: the
// idempotency key and the exact Safety revision the Go TrainerGate admitted.
// Only Revision advances while in progress; ending it is a single transition.
type TrainingSession struct {
	ID                       uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID                   uuid.UUID `gorm:"type:uuid"`
	PlanID                   uuid.UUID `gorm:"type:uuid"`
	PlanRevisionID           uuid.UUID `gorm:"type:uuid"`
	TemplateID               string
	TimeZone                 string
	TrainingDate             DateOnly `gorm:"type:date"`
	State                    string
	EndedReason              *string
	Revision                 int64
	StartedAt                time.Time
	EndedAt                  *time.Time
	StartIdempotencyKey      string
	StartRequestFingerprint  string
	StartSafetyRevision      int64
	StartGateResult          string
	StartGatePolicyRevision  string
	FinishIdempotencyKey     *string
	FinishRequestFingerprint *string
}

func (TrainingSession) TableName() string { return "training_sessions" }

// TrainingSetAction is one immutable, user-originated receipt admitted by the
// TrainerGate at SafetyRevision: a set performed or skipped, a whole exercise
// skipped (Scope "exercise", no SetNumber), or a correction that retracts an
// earlier receipt of the same target (CorrectsActionID). It is never inferred
// from navigation, playback or assistant output.
type TrainingSetAction struct {
	ID                    uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID                uuid.UUID `gorm:"type:uuid"`
	SessionID             uuid.UUID `gorm:"type:uuid"`
	PlanExerciseID        string
	ExerciseID            string
	Scope                 string
	SetNumber             *int
	Kind                  string
	CorrectsActionID      *uuid.UUID `gorm:"type:uuid"`
	ActualReps            *int
	ActualDurationSeconds *int
	SessionRevision       int64
	IdempotencyKey        string
	RequestFingerprint    string
	SafetyRevision        int64
	GateResult            string
	GatePolicyRevision    string
	RecordedAt            time.Time
}

func (TrainingSetAction) TableName() string { return "training_set_actions" }

// TrainingSessionTransition is the immutable receipt of one explicit pause or
// resume. A resume carries the Safety revision and permitting gate result it
// was admitted at; a pause is never gated and carries none.
type TrainingSessionTransition struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID             uuid.UUID `gorm:"type:uuid"`
	SessionID          uuid.UUID `gorm:"type:uuid"`
	Kind               string
	SessionRevision    int64
	IdempotencyKey     string
	RequestFingerprint string
	SafetyRevision     *int64
	GateResult         *string
	GatePolicyRevision *string
	RecordedAt         time.Time
}

func (TrainingSessionTransition) TableName() string { return "training_session_transitions" }

// TrainingSessionFeedback is the user's own words about one ended session,
// stored verbatim at most once (TP-D10). It pins the session's plan revision
// for the review projection and is never interpreted into a BodyState fact,
// a Safety change or a plan change.
type TrainingSessionFeedback struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID             uuid.UUID `gorm:"type:uuid"`
	SessionID          uuid.UUID `gorm:"type:uuid"`
	PlanID             uuid.UUID `gorm:"type:uuid"`
	PlanRevisionID     uuid.UUID `gorm:"type:uuid"`
	Text               string
	IdempotencyKey     string
	RequestFingerprint string
	RecordedAt         time.Time
}

func (TrainingSessionFeedback) TableName() string { return "training_session_feedback" }
