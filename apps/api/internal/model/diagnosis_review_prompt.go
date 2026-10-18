package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// DiagnosisReviewPromptDismissal is the user's "not now" on one derived
// Diagnosis review prompt (diagnosis.md §3.6). It records only what the user
// saw: the Condition, its analysis basis, the BodyState revision the prompt
// was evaluated at and the reason codes then shown. It is never a Diagnosis
// state and never edits a Condition or its freshness; newer substantive
// evidence raises the prompt again. Append-only.
type DiagnosisReviewPromptDismissal struct {
	UserID                            uuid.UUID `gorm:"primaryKey"`
	ID                                uuid.UUID `gorm:"primaryKey"`
	ConditionID                       uuid.UUID
	BasedOnBodyStateRevision          int64
	DismissedThroughBodyStateRevision int64
	DiagnosisRevision                 int64
	ReasonCodes                       []string `gorm:"serializer:json;type:jsonb"`
	IdempotencyKey                    string
	DismissedAt                       time.Time
}

func (DiagnosisReviewPromptDismissal) TableName() string { return "diagnosis_review_prompt_dismissals" }

var (
	// ErrDiagnosisRequestSourceNotFound masks absent, foreign, unfinished and
	// non-user messages alike.
	ErrDiagnosisRequestSourceNotFound = errors.New("diagnosis analysis request message not found")
	// ErrDiagnosisRequestSourceInvalid: the message cannot stand as the user's
	// own request statement (no text, too long, or a structured answer).
	ErrDiagnosisRequestSourceInvalid = errors.New("diagnosis analysis request message is not a request statement")
)

// DiagnosisAnalysisRequestExcerptMax mirrors statement_evidence_excerpt_check.
const DiagnosisAnalysisRequestExcerptMax = 2000
