package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func validFactCommand() RecordBodyFactsCommand {
	return RecordBodyFactsCommand{IdempotencyKey: "test", Facts: []BodyFactInput{{
		Kind: "symptom", Statement: "My shoulder hurt yesterday.", BodyRegionIDs: []string{},
		Temporal: FactTemporal{Precision: "unknown", OriginalExpression: "yesterday"}, Details: json.RawMessage(`{}`),
	}}}
}

func TestBodyFactFiveKindsKeepOptionalDetailsUnknown(t *testing.T) {
	for _, kind := range []string{"symptom", "health_history", "activity", "lifestyle", "training_feedback"} {
		t.Run(kind, func(t *testing.T) {
			command := validFactCommand()
			command.Facts[0].Kind = kind
			if err := command.Validate(); err != nil {
				t.Fatal(err)
			}
			if command.Facts[0].Temporal.Start != "" {
				t.Fatal("validation invented occurrence time")
			}
		})
	}
}

func TestBodyFactRejectsLegacyOrUnboundClaims(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*RecordBodyFactsCommand)
	}{
		{"legacy_kind", func(c *RecordBodyFactsCommand) { c.Facts[0].Kind = "discomfort" }},
		{"legacy_lifecycle", func(c *RecordBodyFactsCommand) {
			c.Facts[0].Details = json.RawMessage(`{"lifecycle_state":"resolved"}`)
		}},
		{"wrong_detail_kind", func(c *RecordBodyFactsCommand) { c.Facts[0].Details = json.RawMessage(`{"habit_type":"sleep"}`) }},
		{"clinical_presence", func(c *RecordBodyFactsCommand) { c.Facts[0].Details = json.RawMessage(`{"presence":"confirmed"}`) }},
		{"array_details", func(c *RecordBodyFactsCommand) { c.Facts[0].Details = json.RawMessage(`[]`) }},
		{"null_details", func(c *RecordBodyFactsCommand) { c.Facts[0].Details = json.RawMessage(`null`) }},
		{"extra_document", func(c *RecordBodyFactsCommand) { c.Facts[0].Details = json.RawMessage(`{} {}`) }},
		{"blank_statement", func(c *RecordBodyFactsCommand) { c.Facts[0].Statement = " " }},
		{"blank_key", func(c *RecordBodyFactsCommand) { c.IdempotencyKey = " " }},
		{"negative_revision", func(c *RecordBodyFactsCommand) { c.ExpectedRevision = -1 }},
		{"null_regions", func(c *RecordBodyFactsCommand) { c.Facts[0].BodyRegionIDs = nil }},
		{"duplicate_regions", func(c *RecordBodyFactsCommand) { c.Facts[0].BodyRegionIDs = []string{"shoulder.left", "shoulder.left"} }},
		{"unknown_with_date", func(c *RecordBodyFactsCommand) { c.Facts[0].Temporal.Start = "2026-10-12" }},
		{"reversed_time", func(c *RecordBodyFactsCommand) {
			c.Facts[0].Temporal = FactTemporal{Precision: "range", Start: "2026-10-13", End: "2026-10-12"}
		}},
		{"invalid_timezone", func(c *RecordBodyFactsCommand) { c.Facts[0].Temporal.Timezone = "Local" }},
		{"arbitrary_training_id", func(c *RecordBodyFactsCommand) {
			c.Facts[0].Kind = "training_feedback"
			c.Facts[0].Details = json.RawMessage(`{"session_log_id":"00000000-0000-0000-0000-000000000001"}`)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			command := validFactCommand()
			test.mutate(&command)
			if err := command.Validate(); !errors.Is(err, ErrInvalidBodyFact) {
				t.Fatalf("got %v, want invalid fact", err)
			}
		})
	}
}

func TestBodyFactNegativeSymptomDoesNotBecomeEpisodeResolution(t *testing.T) {
	command := validFactCommand()
	command.Facts[0].Details = json.RawMessage(`{"presence":"absent","pattern":"unknown","severity":{"raw_value":"0","scale":"0-10"}}`)
	command.Facts[0].Temporal = FactTemporal{Precision: "day", Start: "2026-10-13", Timezone: "Asia/Tokyo"}
	if err := command.Validate(); err != nil {
		t.Fatal(err)
	}
	var details map[string]any
	if err := json.Unmarshal(command.Facts[0].Details, &details); err != nil {
		t.Fatal(err)
	}
	if _, exists := details["episode_state"]; exists {
		t.Fatal("a negative observation must not imply the episode ended")
	}
}

func TestExplicitEpisodeCourseReportRequiresARealSymptomStatementAndMatchingPresence(t *testing.T) {
	tests := []struct {
		name    string
		details string
		invalid bool
	}{
		{"explicit_current", `{"symptom_type":"shoulder pain","presence":"present","episode_course_report":"explicit_current_report"}`, false},
		{"explicit_end", `{"symptom_type":"shoulder pain","presence":"absent","episode_course_report":"explicit_resolution"}`, false},
		{"ordinary_negative", `{"symptom_type":"shoulder pain","presence":"absent"}`, false},
		{"generic_negative_cannot_be_reinterpreted", `{"presence":"absent","episode_course_report":"explicit_current_report"}`, true},
		{"positive_is_not_resolution", `{"presence":"present","episode_course_report":"explicit_resolution"}`, true},
		{"arbitrary_clinical_state", `{"presence":"absent","episode_course_report":"recovered"}`, true},
		{"missing_presence", `{"episode_course_report":"explicit_resolution"}`, true},
		{"empty_presence", `{"presence":"","episode_course_report":"explicit_current_report"}`, true},
		{"injected_assessment", `{"presence":"absent","episode_state":"ended"}`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validFactCommand()
			c.Facts[0].Details = json.RawMessage(tc.details)
			err := c.Validate()
			if tc.invalid && !errors.Is(err, ErrInvalidBodyFact) {
				t.Fatalf("invalid assertion accepted: %v", err)
			}
			if !tc.invalid && err != nil {
				t.Fatalf("legitimate explicit input refused: %v", err)
			}
		})
	}
}
