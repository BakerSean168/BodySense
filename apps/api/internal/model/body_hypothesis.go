package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidBodyHypothesis             = errors.New("invalid body hypothesis")
	ErrBodyHypothesisIdempotencyConflict = errors.New("body hypothesis idempotency conflict")
	ErrBodyHypothesisNotFound            = errors.New("body hypothesis not found")
	ErrBodyHypothesisNotCurrent          = errors.New("body hypothesis is no longer current")
	ErrBodyHypothesisFactNotFound        = errors.New("body hypothesis linked fact not found")
)

const maxBodyHypothesisFactLinks = 20

// BodyHypothesis is ONLY the user's own stated guess about a cause or a
// possible condition ("I think it's from running too much"). It carries no
// confidence, no clinical status and no AI provenance: hypotheses proposed by
// the Agent/Diagnosis belong to the ConditionAnalysisRun audit, never here.
type BodyHypothesis struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID            uuid.UUID `gorm:"type:uuid;not null"`
	Statement         string    `gorm:"not null"`
	SourceEvidenceID  uuid.UUID `gorm:"type:uuid;not null"`
	AssertionStatus   string    `gorm:"not null"`
	CreatedRevision   int64     `gorm:"not null"`
	RecordedAt        time.Time `gorm:"not null"`
	WithdrawnAt       *time.Time
	WithdrawnRevision *int64
	WithdrawalReason  *string
}

func (BodyHypothesis) TableName() string { return "body_hypotheses" }

// BodyHypothesisFactLink is an owner-scoped reference to an existing BodyFact;
// the composite FK makes a cross-tenant link impossible at the database layer.
type BodyHypothesisFactLink struct {
	UserID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	HypothesisID uuid.UUID `gorm:"type:uuid;primaryKey"`
	FactID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	Ordinal      int16     `gorm:"not null"`
}

func (BodyHypothesisFactLink) TableName() string { return "body_hypothesis_fact_links" }

// Id-only durable receipt; the statement and withdrawal reason are never
// copied into receipts, revisions or the outbox.
type BodyHypothesisCommit struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	IdempotencyKey   string    `gorm:"not null" json:"-"`
	Fingerprint      string    `gorm:"not null" json:"-"`
	Operation        string    `gorm:"not null" json:"operation"`
	HypothesisID     uuid.UUID `gorm:"type:uuid;not null" json:"hypothesis_id"`
	SourceEvidenceID uuid.UUID `gorm:"type:uuid;not null" json:"source_evidence_id"`
	Revision         int64     `gorm:"not null" json:"revision"`
	RevisionID       uuid.UUID `gorm:"type:uuid;not null" json:"revision_id"`
	EventID          uuid.UUID `gorm:"type:uuid;not null" json:"event_id"`
	CommittedAt      time.Time `gorm:"not null" json:"committed_at"`
}

func (BodyHypothesisCommit) TableName() string { return "body_hypothesis_commits" }

type RecordBodyHypothesisCommand struct {
	IdempotencyKey   string      `json:"idempotency_key"`
	ExpectedRevision int64       `json:"expected_revision"`
	Statement        string      `json:"statement"`
	RelatedFactIDs   []uuid.UUID `json:"related_fact_ids,omitempty"`
}

func validBodyHypothesisKey(key string, revision int64) bool {
	return len(key) >= 1 && len(key) <= 128 && key == strings.TrimSpace(key) && revision >= 0
}

func (c RecordBodyHypothesisCommand) Validate() error {
	invalid := func(s string) error { return fmt.Errorf("%w: %s", ErrInvalidBodyHypothesis, s) }
	if !validBodyHypothesisKey(c.IdempotencyKey, c.ExpectedRevision) {
		return invalid("nonempty exact idempotency key and revision are required")
	}
	n := utf8.RuneCountInString(c.Statement)
	if n < 1 || n > 2000 || c.Statement != strings.TrimSpace(c.Statement) {
		return invalid("an original, bounded user-authored statement is required")
	}
	if len(c.RelatedFactIDs) > maxBodyHypothesisFactLinks {
		return invalid("too many related facts")
	}
	seen := make(map[uuid.UUID]struct{}, len(c.RelatedFactIDs))
	for _, id := range c.RelatedFactIDs {
		if id == uuid.Nil {
			return invalid("related fact id required")
		}
		if _, dup := seen[id]; dup {
			return invalid("duplicate related fact")
		}
		seen[id] = struct{}{}
	}
	return nil
}

// WithdrawBodyHypothesisCommand retracts exactly one user guess. It preserves
// the original row, source and fact links (history), and never withdraws or
// edits the linked facts or observations.
type WithdrawBodyHypothesisCommand struct {
	IdempotencyKey   string `json:"idempotency_key"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason,omitempty"`
}

func (c WithdrawBodyHypothesisCommand) Validate() error {
	if !validBodyHypothesisKey(c.IdempotencyKey, c.ExpectedRevision) {
		return fmt.Errorf("%w: exact idempotency key and expected revision required", ErrInvalidBodyHypothesis)
	}
	if utf8.RuneCountInString(c.Reason) > 500 || c.Reason != strings.TrimSpace(c.Reason) {
		return fmt.Errorf("%w: unbounded or padded withdrawal reason", ErrInvalidBodyHypothesis)
	}
	return nil
}

type BodyHypothesisSource struct {
	Actor   string    `json:"actor"` // always "user"
	Kind    string    `json:"kind"`  // statement_evidence source kind
	ID      uuid.UUID `json:"id"`
	Version int64     `json:"version"`
}

type BodyHypothesisRecord struct {
	ID              uuid.UUID            `json:"id"`
	UserID          uuid.UUID            `json:"user_id"`
	Statement       string               `json:"statement"`
	Source          BodyHypothesisSource `json:"source"`
	RelatedFactIDs  []uuid.UUID          `json:"related_fact_ids"`
	AssertionStatus string               `json:"assertion_status"`
	CreatedRevision int64                `json:"created_revision"`
	RecordedAt      time.Time            `json:"recorded_at"`
}

type BodyHypothesisList struct {
	UserID          uuid.UUID              `json:"user_id"`
	CurrentRevision int64                  `json:"current_revision"`
	Hypotheses      []BodyHypothesisRecord `json:"hypotheses"`
}
