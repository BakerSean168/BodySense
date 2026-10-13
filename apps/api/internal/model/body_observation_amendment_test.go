package model

import (
	"errors"
	"strings"
	"testing"
)

func TestAmendBodyObservationRequiresExactAuthorizationAndReplacementShape(t *testing.T) {
	source := validObservationCommand().Observation
	valid := AmendBodyObservationCommand{IdempotencyKey: "correction-1", ExpectedRevision: 1, Replacement: &source}
	if err := valid.Validate(true); err != nil {
		t.Fatalf("valid direct correction denied: %v", err)
	}
	if err := (AmendBodyObservationCommand{IdempotencyKey: "withdrawal-1", ExpectedRevision: 2}).Validate(false); err != nil {
		t.Fatalf("valid direct withdrawal denied: %v", err)
	}
	cases := []struct {
		name       string
		correction bool
		mutate     func(*AmendBodyObservationCommand)
	}{
		{"missing_key", true, func(c *AmendBodyObservationCommand) { c.IdempotencyKey = "" }},
		{"padded_key", true, func(c *AmendBodyObservationCommand) { c.IdempotencyKey = " padded " }},
		{"oversize_key", true, func(c *AmendBodyObservationCommand) { c.IdempotencyKey = strings.Repeat("x", 129) }},
		{"stale_negative_revision", true, func(c *AmendBodyObservationCommand) { c.ExpectedRevision = -1 }},
		{"correction_without_new_original", true, func(c *AmendBodyObservationCommand) { c.Replacement = nil }},
		{"injected_time", true, func(c *AmendBodyObservationCommand) {
			v := *c.Replacement
			v.ObservedAt.Precision = "unknown"
			c.Replacement = &v
		}},
		{"unit_without_value", true, func(c *AmendBodyObservationCommand) { v := *c.Replacement; v.RawValue = ""; c.Replacement = &v }},
		{"withdraw_smuggled_replacement", false, func(c *AmendBodyObservationCommand) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := valid
			tc.mutate(&input)
			if err := input.Validate(tc.correction); !errors.Is(err, ErrInvalidBodyObservation) {
				t.Fatalf("invalid amendment accepted: %v", err)
			}
		})
	}
}
