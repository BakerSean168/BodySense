package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestSafetyEnvelopeSharedContract(t *testing.T) {
	raw, err := os.ReadFile("../../../../contracts/internal/diagnosis/safety-envelope-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		ProvenanceRoundTrip json.RawMessage `json:"provenance_round_trip"`
		Cases               []struct {
			Name     string            `json:"name"`
			Snapshot BodyStateSnapshot `json:"snapshot"`
			Envelope json.RawMessage   `json:"envelope"`
			Error    bool              `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			got, err := ProjectSafetyEnvelopeV2(&test.Snapshot)
			if test.Error {
				if err == nil {
					t.Fatal("expected fail-closed error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var actual, expected any
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(test.Envelope, &expected); err != nil {
				t.Fatal(err)
			}
			actualJSON, _ := json.Marshal(actual)
			expectedJSON, _ := json.Marshal(expected)
			if string(actualJSON) != string(expectedJSON) {
				t.Fatalf("envelope mismatch\ngot %s\nwant %s", actualJSON, expectedJSON)
			}
		})
	}
	var withProvenance SafetyEnvelopeV2
	if err := json.Unmarshal(fixture.ProvenanceRoundTrip, &withProvenance); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(withProvenance)
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture.ProvenanceRoundTrip, &expected); err != nil {
		t.Fatal(err)
	}
	actualJSON, _ := json.Marshal(actual)
	expectedJSON, _ := json.Marshal(expected)
	if string(actualJSON) != string(expectedJSON) {
		t.Fatalf("provenance round trip mismatch\ngot %s\nwant %s", actualJSON, expectedJSON)
	}
}

func TestSafetyEnvelopeIgnoresProseAndExcludedFacts(t *testing.T) {
	snapshot := &BodyStateSnapshot{CurrentRevision: 1, SafetyState: json.RawMessage(`{}`), Facts: []model.BodyStateFact{
		{ID: uuid.New(), Value: "头晕，外伤", Details: datatypes.JSON(`{"additional_notes":"trauma"}`), LifecycleState: "active", ReviewState: "confirmed"},
		{ID: uuid.New(), Details: datatypes.JSON(`{"dizziness":true}`), LifecycleState: "active", ReviewState: "unverified", ExcludedFromReasoning: true},
	}}
	envelope, err := ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Assertions) != 0 || envelope.RequiresReview {
		t.Fatalf("prose or excluded fact acquired authority: %+v", envelope)
	}
}

func TestSafetyEnvelopeUnverifiedPositiveRequiresReview(t *testing.T) {
	snapshot := &BodyStateSnapshot{CurrentRevision: 1, SafetyState: json.RawMessage(`{}`), Facts: []model.BodyStateFact{{ID: uuid.New(), Details: datatypes.JSON(`{"dizziness":true}`), LifecycleState: "active", ReviewState: "unverified"}}}
	envelope, err := ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !envelope.RequiresReview || len(envelope.ActiveBlockers) != 1 {
		t.Fatalf("unverified positive did not fail closed: %+v", envelope)
	}
}

func TestSafetyEnvelopeLegacyMonitoringAndUnknown(t *testing.T) {
	for _, test := range []struct {
		raw     string
		blocked bool
		invalid bool
	}{
		{`{"has_red_flags":true,"status":"monitoring"}`, false, false},
		{`{"has_red_flags":true,"status":"active","flags":[{"category":"unmapped"}]}`, true, false},
		{`{"has_red_flags":true,"status":"surprise"}`, false, true},
	} {
		envelope, err := ProjectSafetyEnvelopeV2(&BodyStateSnapshot{CurrentRevision: 1, SafetyState: json.RawMessage(test.raw)})
		if (err != nil) != test.invalid {
			t.Fatalf("unexpected error for %s: %v", test.raw, err)
		}
		if err == nil && envelope.RequiresReview != test.blocked {
			t.Fatalf("unexpected blocker for %s", test.raw)
		}
		if err == nil && test.blocked && envelope.Assertions[0].Concept != SafetyUnknownRedFlag {
			t.Fatalf("unmapped legacy concept was lost")
		}
	}
}
