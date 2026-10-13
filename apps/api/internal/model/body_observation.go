package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrInvalidBodyObservation             = errors.New("invalid body observation")
	ErrBodyObservationIdempotencyConflict = errors.New("body observation idempotency conflict")
)

// ObservationKind is deliberately an organizational label and NOT a clinical
// reference range, interpretation, diagnosis, or device catalog.
var directObservationKind = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

// RecordBodyObservationInput is a user's original direct report. The HTTP
// contract intentionally has NO source_kind, normalized_value, unit conversion,
// confidence, diagnosis or evidence_ref_id; assigning those requires a
// separately admitted device/document/measurement workflow.
type RecordBodyObservationInput struct {
	Kind               string       `json:"kind"`
	Statement          string       `json:"statement"`
	RawValue           string       `json:"raw_value,omitempty"`
	RawUnit            string       `json:"raw_unit,omitempty"`
	Method             string       `json:"method,omitempty"`
	MeasurementContext string       `json:"measurement_context,omitempty"`
	ObservedAt         FactTemporal `json:"observed_at"`
}

type RecordBodyObservationCommand struct {
	IdempotencyKey   string                     `json:"idempotency_key"`
	ExpectedRevision int64                      `json:"expected_revision"`
	Observation      RecordBodyObservationInput `json:"observation"`
}

func (c RecordBodyObservationCommand) Validate() error {
	invalid := func(s string) error { return fmt.Errorf("%w: %s", ErrInvalidBodyObservation, s) }
	if len(c.IdempotencyKey) < 1 || len(c.IdempotencyKey) > 128 ||
		c.IdempotencyKey != strings.TrimSpace(c.IdempotencyKey) || c.ExpectedRevision < 0 {
		return invalid("nonempty exact idempotency key and revision are required")
	}
	input := c.Observation
	if !directObservationKind.MatchString(input.Kind) {
		return invalid("kind must be a bounded lowercase observation label")
	}
	if utf8.RuneCountInString(input.Statement) < 1 || utf8.RuneCountInString(input.Statement) > 2000 ||
		input.Statement != strings.TrimSpace(input.Statement) {
		return invalid("an original, bounded user-authored statement is required")
	}
	fields := []struct {
		value string
		max   int
		name  string
	}{
		{input.RawValue, 120, "raw_value"}, {input.RawUnit, 48, "raw_unit"},
		{input.Method, 240, "method"}, {input.MeasurementContext, 500, "measurement_context"},
	}
	for _, field := range fields {
		if utf8.RuneCountInString(field.value) > field.max || field.value != strings.TrimSpace(field.value) {
			return invalid("unbounded or padded " + field.name)
		}
	}
	if input.RawUnit != "" && input.RawValue == "" {
		return invalid("raw measurement unit without an original reported value")
	}
	if err := input.ObservedAt.Validate(); err != nil {
		return invalid("unsupported user-reported time: " + err.Error())
	}
	return nil
}

// No duplicate source statement is stored on the observation row: it lives in
// the shared statement_evidence authoritative source and is joined for reads.
type BodyObservation struct {
	ID                      uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID                  uuid.UUID `gorm:"type:uuid;not null"`
	Kind                    string
	RawValue                string
	RawUnit                 string
	Method                  string
	MeasurementContext      string
	ObservedAt              datatypes.JSON `gorm:"type:jsonb;not null"`
	SourceEvidenceID        uuid.UUID      `gorm:"type:uuid;not null"`
	AssertionStatus         string
	SupersedesObservationID *uuid.UUID `gorm:"type:uuid"`
	CreatedRevision         int64
	RecordedAt              time.Time
}

func (BodyObservation) TableName() string { return "body_observations" }

// One exact semantic command yields one id-only durable commit. No original
// statement, raw value or unit is copied to the generic audit/outbox.
type BodyObservationCommit struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	IdempotencyKey   string    `gorm:"not null" json:"-"`
	Fingerprint      string    `gorm:"not null" json:"-"`
	ObservationID    uuid.UUID `gorm:"type:uuid;not null" json:"observation_id"`
	SourceEvidenceID uuid.UUID `gorm:"type:uuid;not null" json:"source_evidence_id"`
	Revision         int64     `gorm:"not null" json:"revision"`
	RevisionID       uuid.UUID `gorm:"type:uuid;not null" json:"revision_id"`
	EventID          uuid.UUID `gorm:"type:uuid;not null" json:"event_id"`
	CommittedAt      time.Time `gorm:"not null" json:"committed_at"`
}

func (BodyObservationCommit) TableName() string { return "body_observation_commits" }

// Direct edit is a user_report observation SOURCE TYPE; it never means a
// laboratory or device authenticated measurement. Source version is assigned
// server-side and this read type may be safely masked/blocked by provenance.
type BodyObservationSource struct {
	Kind    string    `json:"kind"`
	ID      uuid.UUID `json:"id"`
	Version int64     `json:"version"`
}
type BodyObservationRecord struct {
	ID                 uuid.UUID             `json:"id"`
	UserID             uuid.UUID             `json:"user_id"`
	Kind               string                `json:"kind"`
	Statement          string                `json:"statement"`
	RawValue           string                `json:"raw_value,omitempty"`
	RawUnit            string                `json:"raw_unit,omitempty"`
	Method             string                `json:"method,omitempty"`
	MeasurementContext string                `json:"measurement_context,omitempty"`
	ObservedAt         FactTemporal          `json:"observed_at"`
	Source             BodyObservationSource `json:"source"`
	AssertionStatus    string                `json:"assertion_status"`
	CreatedRevision    int64                 `json:"created_revision"`
	RecordedAt         time.Time             `json:"recorded_at"`
}

type BodyObservationPage struct {
	UserID             uuid.UUID               `json:"user_id"`
	CurrentRevision    int64                   `json:"current_revision"`
	Observations       []BodyObservationRecord `json:"observations"`
	HasMore            bool                    `json:"has_more"`
	NextBeforeRevision *int64                  `json:"next_before_revision,omitempty"`
}
