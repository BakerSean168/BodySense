package model

import (
	"errors"
	"strings"
	"testing"
)

func validObservationCommand() RecordBodyObservationCommand {
	return RecordBodyObservationCommand{IdempotencyKey: "measure-20261013-weight",
		ExpectedRevision: 0, Observation: RecordBodyObservationInput{
			Kind: "body_weight", Statement: "I measured my body weight as 67.5 kilograms",
			RawValue: "67.5", RawUnit: "kg", Method: "home scale",
			ObservedAt: FactTemporal{Precision: "day", Start: "2026-10-13"},
		},
	}
}

func TestBodyObservationExplicitUserReportOnlyValidation(t *testing.T) {
	valid := validObservationCommand()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid direct user observation rejected: %v", err)
	}
	unknown := valid
	unknown.Observation.ObservedAt = FactTemporal{Precision: "unknown"}
	unknown.Observation.RawValue = ""
	unknown.Observation.RawUnit = ""
	unknown.Observation.Kind = "posture_note"
	unknown.Observation.Statement = "I noticed a change in my shoulder posture"
	if err := unknown.Validate(); err != nil {
		t.Fatalf("valid nonnumeric observation refused: %v", err)
	}
	// OpenAPI maxLength and PostgreSQL char_length count Unicode scalar
	// values, not UTF-8 bytes: long legitimate Chinese reports are valid.
	multilingual := valid
	multilingual.Observation.Statement = strings.Repeat("测", 1800)
	multilingual.Observation.Method = strings.Repeat("测", 200)
	if err := multilingual.Validate(); err != nil {
		t.Fatalf("Unicode report valid in OpenAPI and PG rejected in Go: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*RecordBodyObservationCommand)
	}{
		{"empty_key", func(c *RecordBodyObservationCommand) { c.IdempotencyKey = "" }},
		{"trimmed_key", func(c *RecordBodyObservationCommand) { c.IdempotencyKey = " padding " }},
		{"negative_revision", func(c *RecordBodyObservationCommand) { c.ExpectedRevision = -1 }},
		{"invalid_kind_separator", func(c *RecordBodyObservationCommand) { c.Observation.Kind = "diagnosed:hypertension" }},
		{"invalid_uppercase_kind", func(c *RecordBodyObservationCommand) { c.Observation.Kind = "Weight" }},
		{"missing_statement", func(c *RecordBodyObservationCommand) { c.Observation.Statement = "" }},
		{"oversize_unicode_statement", func(c *RecordBodyObservationCommand) { c.Observation.Statement = strings.Repeat("测", 2001) }},
		{"padded_statement", func(c *RecordBodyObservationCommand) { c.Observation.Statement = " padded " }},
		{"unit_without_reported_value", func(c *RecordBodyObservationCommand) { c.Observation.RawValue = ""; c.Observation.RawUnit = "mmHg" }},
		{"invented_time", func(c *RecordBodyObservationCommand) {
			c.Observation.ObservedAt.Precision = "unknown"
			c.Observation.ObservedAt.Start = "2026-10-13"
		}},
		{"wrong_time_order", func(c *RecordBodyObservationCommand) { c.Observation.ObservedAt.End = "2026-10-01" }},
		{"server_local_timezone", func(c *RecordBodyObservationCommand) { c.Observation.ObservedAt.Timezone = "Local" }},
		{"oversize_method", func(c *RecordBodyObservationCommand) { c.Observation.Method = string(make([]byte, 241)) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.mutate(&c)
			if err := c.Validate(); !errors.Is(err, ErrInvalidBodyObservation) {
				t.Fatalf("invalid observation accepted: %v", err)
			}
		})
	}
}
