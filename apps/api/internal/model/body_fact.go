package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrInvalidBodyFact             = errors.New("invalid body fact")
	ErrBodyFactIdempotencyConflict = errors.New("body fact idempotency conflict")
	ErrBodyFactNotFound            = errors.New("body fact not found")
	ErrBodyFactNotCurrent          = errors.New("body fact is no longer current")
)

// FactTemporal preserves the user's occurrence time separately from persistence time.
// In particular, unknown time must never be filled with the current clock time.
type FactTemporal struct {
	Start              string `json:"start,omitempty"`
	End                string `json:"end,omitempty"`
	Precision          string `json:"precision"`
	OriginalExpression string `json:"original_expression,omitempty"`
	Timezone           string `json:"timezone,omitempty"`
}

type SymptomFactDetails struct {
	SymptomType string `json:"symptom_type,omitempty"`
	// Explicit user-authored report for this symptom course; never inferred
	// from presence=absent, silence, improvement or a model guess.
	EpisodeCourseReport string        `json:"episode_course_report,omitempty"`
	Presence            string        `json:"presence,omitempty"`
	Laterality          string        `json:"laterality,omitempty"`
	Severity            *FactSeverity `json:"severity,omitempty"`
	Pattern             string        `json:"pattern,omitempty"`
	Triggers            []string      `json:"triggers,omitempty"`
	RelievingFactors    []string      `json:"relieving_factors,omitempty"`
	OccurrenceContext   string        `json:"occurrence_context,omitempty"`
}

type FactSeverity struct {
	RawValue string `json:"raw_value"`
	Scale    string `json:"scale,omitempty"`
	Unit     string `json:"unit,omitempty"`
}

type HistoryFactDetails struct {
	HistoryKind      string `json:"history_kind,omitempty"`
	EventDescription string `json:"event_description,omitempty"`
	Outcome          string `json:"outcome,omitempty"`
	SourceContext    string `json:"source_context,omitempty"`
}

type ActivityFactDetails struct {
	ActivityType string `json:"activity_type,omitempty"`
	Duration     string `json:"duration,omitempty"`
	Volume       string `json:"volume,omitempty"`
	Intensity    string `json:"intensity,omitempty"`
}

type LifestyleFactDetails struct {
	HabitType       string `json:"habit_type,omitempty"`
	Frequency       string `json:"frequency,omitempty"`
	Pattern         string `json:"pattern,omitempty"`
	TypicalDuration string `json:"typical_duration,omitempty"`
}

type TrainingFeedbackFactDetails struct {
	SessionLogID *uuid.UUID `json:"session_log_id,omitempty"`
	ExerciseID   *uuid.UUID `json:"exercise_id,omitempty"`
	FeedbackKind string     `json:"feedback_kind,omitempty"`
}

// BodyFactInput is a domain command value, not a generated transport model.
// This initial command records new statements only; correction/withdrawal have
// separate authorization and revision semantics and are not disguised as updates.
type BodyFactInput struct {
	Kind          string          `json:"kind"`
	Statement     string          `json:"statement"`
	BodyRegionIDs []string        `json:"body_region_ids"`
	Temporal      FactTemporal    `json:"temporal"`
	Details       json.RawMessage `json:"details"`
}

// AmendBodyFactCommand is an explicit user action on exactly one assertion.
// Replacement is present only for correction; withdrawal keeps historical data.
type AmendBodyFactCommand struct {
	IdempotencyKey   string         `json:"idempotency_key"`
	ExpectedRevision int64          `json:"expected_revision"`
	Replacement      *BodyFactInput `json:"replacement,omitempty"`
}

func (c AmendBodyFactCommand) Validate(correction bool) error {
	if c.ExpectedRevision < 0 || len(c.IdempotencyKey) < 1 || len(c.IdempotencyKey) > 128 || strings.TrimSpace(c.IdempotencyKey) != c.IdempotencyKey {
		return fmt.Errorf("%w: explicit command identity and non-negative revision are required", ErrInvalidBodyFact)
	}
	if correction {
		if c.Replacement == nil {
			return fmt.Errorf("%w: correction requires a replacement", ErrInvalidBodyFact)
		}
		return (RecordBodyFactsCommand{IdempotencyKey: c.IdempotencyKey, ExpectedRevision: c.ExpectedRevision, Facts: []BodyFactInput{*c.Replacement}}).Validate()
	}
	if c.Replacement != nil {
		return fmt.Errorf("%w: withdrawal cannot contain a replacement", ErrInvalidBodyFact)
	}
	return nil
}

type RecordBodyFactsCommand struct {
	IdempotencyKey   string          `json:"idempotency_key"`
	ExpectedRevision int64           `json:"expected_revision"`
	Facts            []BodyFactInput `json:"facts"`
}

// MessageFactSource identifies a Go-created consultation envelope. It is NOT
// trusted simply because an AI tool passes these IDs: persistence verifies the
// run, conversation owner, user-role message and matching turn in the database.
// Each persisted fact must quote a literal excerpt from that user's message.
type MessageFactSource struct {
	RunID     uuid.UUID
	MessageID uuid.UUID
}

// StatementEvidence is the minimum durable user-statement source. The public
// direct-edit command creates these server-side; callers cannot claim a message,
// clinical source, other user's identity or arbitrary source version.
type StatementEvidence struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID               uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	SourceKind           string     `gorm:"not null" json:"source_kind"`
	SourceVersion        int64      `gorm:"not null" json:"source_version"`
	SourceConversationID *uuid.UUID `gorm:"type:uuid" json:"source_conversation_id,omitempty"`
	SourceRunID          *uuid.UUID `gorm:"type:uuid" json:"source_run_id,omitempty"`
	SourceMessageID      *uuid.UUID `gorm:"type:uuid" json:"source_message_id,omitempty"`
	SourceInteractionID  *uuid.UUID `gorm:"type:uuid" json:"source_interaction_id,omitempty"`
	SourceToolCallID     *string    `json:"source_tool_call_id,omitempty"`
	QuestionExcerpt      *string    `json:"question_excerpt,omitempty"`
	Excerpt              string     `gorm:"not null" json:"excerpt"`
	Availability         string     `gorm:"not null" json:"availability"`
	RecordedAt           time.Time  `gorm:"not null" json:"recorded_at"`
}

func (StatementEvidence) TableName() string { return "statement_evidence" }

// BodyFact is the approved five-kind assertion model. It intentionally has no
// clinical review state, universal symptom lifecycle or inferred trend.
type BodyFact struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	Subject          string         `gorm:"not null" json:"subject"`
	Kind             string         `gorm:"not null" json:"kind"`
	Statement        string         `gorm:"not null" json:"statement"`
	BodyRegionIDs    datatypes.JSON `gorm:"type:jsonb;not null" json:"body_region_ids"`
	Temporal         datatypes.JSON `gorm:"type:jsonb;not null" json:"temporal"`
	Details          datatypes.JSON `gorm:"type:jsonb;not null" json:"details"`
	SourceEvidenceID uuid.UUID      `gorm:"type:uuid;not null" json:"source_evidence_id"`
	SupersedesFactID *uuid.UUID     `gorm:"type:uuid" json:"supersedes_fact_id,omitempty"`
	AssertionStatus  string         `gorm:"not null" json:"assertion_status"`
	CreatedRevision  int64          `gorm:"not null" json:"created_revision"`
	RecordedAt       time.Time      `gorm:"not null" json:"recorded_at"`
}

func (BodyFact) TableName() string { return "body_facts" }

// BodyFactCommit stores the replayable receipt of one atomic statement batch.
// IDs and a fingerprint are sufficient; the receipt must not duplicate health text.
type BodyFactCommit struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	IdempotencyKey string         `gorm:"not null" json:"-"`
	Fingerprint    string         `gorm:"not null" json:"-"`
	Revision       int64          `gorm:"not null" json:"revision"`
	RevisionID     uuid.UUID      `gorm:"type:uuid;not null" json:"revision_id"`
	EventID        uuid.UUID      `gorm:"type:uuid;not null" json:"event_id"`
	FactIDs        datatypes.JSON `gorm:"type:jsonb;not null" json:"fact_ids"`
	CommittedAt    time.Time      `gorm:"not null" json:"committed_at"`
}

func (BodyFactCommit) TableName() string { return "body_fact_commits" }

// BodyStateOutbox is a producer-side durable event. Delivery/consumer checkpoints
// are implemented separately; the presence of this row is not a delivery claim.
type BodyStateOutbox struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	Revision  int64          `gorm:"not null" json:"revision"`
	EventType string         `gorm:"not null" json:"event_type"`
	Payload   datatypes.JSON `gorm:"type:jsonb;not null" json:"payload"`
	CreatedAt time.Time      `gorm:"not null" json:"created_at"`
}

func (BodyStateOutbox) TableName() string { return "body_state_outbox" }

type BodyFactPage struct {
	UserID             uuid.UUID  `json:"user_id"`
	CurrentRevision    int64      `json:"current_revision"`
	Facts              []BodyFact `json:"facts"`
	HasMore            bool       `json:"has_more"`
	NextBeforeRevision *int64     `json:"next_before_revision,omitempty"`
}

// Validate rejects unsupported/legacy state fields instead of silently mapping
// them into the new model. Optional details remain optional, never inferred.
func (c RecordBodyFactsCommand) Validate() error {
	invalid := func(message string) error { return fmt.Errorf("%w: %s", ErrInvalidBodyFact, message) }
	if len(c.IdempotencyKey) < 1 || len(c.IdempotencyKey) > 128 || strings.TrimSpace(c.IdempotencyKey) != c.IdempotencyKey {
		return invalid("idempotency_key must be 1..128 non-blank characters")
	}
	if c.ExpectedRevision < 0 || len(c.Facts) < 1 || len(c.Facts) > 20 {
		return invalid("expected_revision must be non-negative and facts must contain 1..20 items")
	}
	for i, fact := range c.Facts {
		if strings.TrimSpace(fact.Statement) == "" || utf8.RuneCountInString(fact.Statement) > 2000 {
			return invalid(fmt.Sprintf("facts[%d].statement must contain 1..2000 characters", i))
		}
		if fact.BodyRegionIDs == nil || len(fact.BodyRegionIDs) > 20 {
			return invalid("body_region_ids must be an array of at most 20 canonical identifiers")
		}
		seen := map[string]bool{}
		for _, region := range fact.BodyRegionIDs {
			if region == "" || seen[region] {
				return invalid("body_region_ids must contain unique non-empty identifiers")
			}
			seen[region] = true
		}
		if err := fact.Temporal.Validate(); err != nil {
			return err
		}
		if len(fact.Details) > 8192 || len(bytes.TrimSpace(fact.Details)) == 0 || bytes.TrimSpace(fact.Details)[0] != '{' {
			return invalid("details must be an object of at most 8192 bytes")
		}
		var target any
		symptom := &SymptomFactDetails{}
		switch fact.Kind {
		case "symptom":
			target = symptom
		case "health_history":
			target = &HistoryFactDetails{}
		case "activity":
			target = &ActivityFactDetails{}
		case "lifestyle":
			target = &LifestyleFactDetails{}
		case "training_feedback":
			target = &TrainingFeedbackFactDetails{}
		default:
			return invalid("unsupported fact kind")
		}
		decoder := json.NewDecoder(bytes.NewReader(fact.Details))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(target); err != nil {
			return invalid("details do not match the fact kind")
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return invalid("details must contain exactly one object")
		}
		if !oneOf(symptom.Presence, "", "present", "absent") || !oneOf(symptom.Pattern, "", "continuous", "intermittent", "unknown") ||
			!oneOf(symptom.Laterality, "", "left", "right", "bilateral", "not_applicable", "unknown") {
			return invalid("invalid symptom presence, pattern or laterality")
		}
		if !oneOf(symptom.EpisodeCourseReport, "", "explicit_current_report", "explicit_resolution") ||
			(symptom.EpisodeCourseReport == "explicit_current_report" && symptom.Presence != "present") ||
			(symptom.EpisodeCourseReport == "explicit_resolution" && symptom.Presence != "absent") {
			return invalid("explicit Episode course report must match user-reported presence")
		}
		if symptom.Severity != nil && strings.TrimSpace(symptom.Severity.RawValue) == "" {
			return invalid("severity must preserve a raw_value")
		}
		if feedback, ok := target.(*TrainingFeedbackFactDetails); ok && (feedback.SessionLogID != nil || feedback.ExerciseID != nil) {
			return invalid("training references require the verified Trainer binding; direct edits cannot attach arbitrary IDs")
		}
	}
	return nil
}

func (t FactTemporal) Validate() error {
	invalid := func(message string) error { return fmt.Errorf("%w: %s", ErrInvalidBodyFact, message) }
	if !oneOf(t.Precision, "exact", "day", "range", "approximate", "unknown") {
		return invalid("unsupported temporal precision")
	}
	if t.Precision == "unknown" && (t.Start != "" || t.End != "") {
		return invalid("unknown time cannot contain an invented normalized boundary")
	}
	var start, end time.Time
	for _, item := range []struct {
		value       string
		destination *time.Time
	}{{t.Start, &start}, {t.End, &end}} {
		if item.value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, item.value)
		if err != nil && t.Precision != "exact" {
			parsed, err = time.Parse("2006-01-02", item.value)
		}
		if err != nil {
			return invalid("temporal boundaries must be explicit ISO dates or timestamps")
		}
		*item.destination = parsed
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		return invalid("temporal end precedes start")
	}
	if t.Timezone != "" {
		if t.Timezone == "Local" {
			return invalid("timezone must not depend on the server locale")
		}
		if _, err := time.LoadLocation(t.Timezone); err != nil {
			return invalid("timezone must be an IANA identifier")
		}
	}
	if utf8.RuneCountInString(t.OriginalExpression) > 500 {
		return invalid("original time expression is too long")
	}
	return nil
}

func oneOf(value string, options ...string) bool {
	for _, candidate := range options {
		if candidate == value {
			return true
		}
	}
	return false
}
