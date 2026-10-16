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
	ErrBodyHypothesisObservationNotFound = errors.New("body hypothesis linked observation not found")
)

const (
	maxBodyHypothesisFactLinks        = 20
	maxBodyHypothesisObservationLinks = 20
)

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
	// SupersedesHypothesisID is set only on a direct user correction; the
	// corrected row stays as history with assertion_status 'superseded'.
	SupersedesHypothesisID *uuid.UUID `gorm:"type:uuid"`
	// LineageID is the id of the first hypothesis in a correction chain (its
	// own id when never corrected). It is the stable identity a correction
	// carries forward and anchors the derived ConcernGroup id (DC-032).
	LineageID uuid.UUID `gorm:"type:uuid;not null"`
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

// BodyHypothesisObservationLink is an owner-scoped reference to one of the
// user's own BodyObservations; the composite FK rejects cross-tenant links.
type BodyHypothesisObservationLink struct {
	UserID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	HypothesisID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	ObservationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Ordinal       int16     `gorm:"not null"`
}

func (BodyHypothesisObservationLink) TableName() string { return "body_hypothesis_observation_links" }

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
	// TargetHypothesisID is the exact hypothesis a correct/withdraw acted on
	// (for correct, HypothesisID is the new successor); nil for record.
	TargetHypothesisID *uuid.UUID `gorm:"type:uuid" json:"target_hypothesis_id,omitempty"`
}

func (BodyHypothesisCommit) TableName() string { return "body_hypothesis_commits" }

type RecordBodyHypothesisCommand struct {
	IdempotencyKey   string      `json:"idempotency_key"`
	ExpectedRevision int64       `json:"expected_revision"`
	Statement        string      `json:"statement"`
	RelatedFactIDs   []uuid.UUID `json:"related_fact_ids,omitempty"`
	// RelatedObservationIDs must be the owner's own current observations.
	RelatedObservationIDs []uuid.UUID `json:"related_observation_ids,omitempty"`
}

func validBodyHypothesisKey(key string, revision int64) bool {
	return len(key) >= 1 && len(key) <= 128 && key == strings.TrimSpace(key) && revision >= 0
}

func (c RecordBodyHypothesisCommand) Validate() error {
	invalid := func(s string) error { return fmt.Errorf("%w: %s", ErrInvalidBodyHypothesis, s) }
	if !validBodyHypothesisKey(c.IdempotencyKey, c.ExpectedRevision) {
		return invalid("nonempty exact idempotency key and revision are required")
	}
	return validateBodyHypothesisContent(c.Statement, c.RelatedFactIDs, c.RelatedObservationIDs)
}

func validateBodyHypothesisContent(statement string, facts, observations []uuid.UUID) error {
	invalid := func(s string) error { return fmt.Errorf("%w: %s", ErrInvalidBodyHypothesis, s) }
	n := utf8.RuneCountInString(statement)
	if n < 1 || n > 2000 || statement != strings.TrimSpace(statement) {
		return invalid("an original, bounded user-authored statement is required")
	}
	if err := validateBodyHypothesisLinkIDs(facts, maxBodyHypothesisFactLinks, "fact"); err != nil {
		return err
	}
	return validateBodyHypothesisLinkIDs(observations, maxBodyHypothesisObservationLinks, "observation")
}

func validateBodyHypothesisLinkIDs(ids []uuid.UUID, limit int, name string) error {
	if len(ids) > limit {
		return fmt.Errorf("%w: too many related %ss", ErrInvalidBodyHypothesis, name)
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return fmt.Errorf("%w: related %s id required", ErrInvalidBodyHypothesis, name)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: duplicate related %s", ErrInvalidBodyHypothesis, name)
		}
		seen[id] = struct{}{}
	}
	return nil
}

// BodyHypothesisReplacement is the complete new content of a corrected
// guess. Link lists are explicit (possibly empty): a correction never
// silently inherits or drops links of the superseded hypothesis.
type BodyHypothesisReplacement struct {
	Statement             string      `json:"statement"`
	RelatedFactIDs        []uuid.UUID `json:"related_fact_ids"`
	RelatedObservationIDs []uuid.UUID `json:"related_observation_ids"`
}

// CorrectBodyHypothesisCommand is the user's direct edit of exactly one of
// their own current guesses. The old row is kept as 'superseded' history and
// a new hypothesis in the same lineage carries the replacement content.
type CorrectBodyHypothesisCommand struct {
	IdempotencyKey   string                     `json:"idempotency_key"`
	ExpectedRevision int64                      `json:"expected_revision"`
	Replacement      *BodyHypothesisReplacement `json:"replacement"`
}

func (c CorrectBodyHypothesisCommand) Validate() error {
	if !validBodyHypothesisKey(c.IdempotencyKey, c.ExpectedRevision) {
		return fmt.Errorf("%w: exact idempotency key and expected revision required", ErrInvalidBodyHypothesis)
	}
	if c.Replacement == nil {
		return fmt.Errorf("%w: an explicit replacement is required for correction", ErrInvalidBodyHypothesis)
	}
	r := c.Replacement
	return validateBodyHypothesisContent(r.Statement, r.RelatedFactIDs, r.RelatedObservationIDs)
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
	ID                     uuid.UUID            `json:"id"`
	UserID                 uuid.UUID            `json:"user_id"`
	LineageID              uuid.UUID            `json:"lineage_id"`
	SupersedesHypothesisID *uuid.UUID           `json:"supersedes_hypothesis_id,omitempty"`
	Statement              string               `json:"statement"`
	Source                 BodyHypothesisSource `json:"source"`
	RelatedFactIDs         []uuid.UUID          `json:"related_fact_ids"`
	RelatedObservationIDs  []uuid.UUID          `json:"related_observation_ids"`
	AssertionStatus        string               `json:"assertion_status"`
	CreatedRevision        int64                `json:"created_revision"`
	RecordedAt             time.Time            `json:"recorded_at"`
}

type BodyHypothesisList struct {
	UserID          uuid.UUID              `json:"user_id"`
	CurrentRevision int64                  `json:"current_revision"`
	Hypotheses      []BodyHypothesisRecord `json:"hypotheses"`
}
