package service

import (
	"encoding/json"
	"os"
	"testing"
	"time"

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
			Name          string               `json:"name"`
			Snapshot      BodyStateReadModel   `json:"snapshot"`
			SafetyScreens []model.SafetyScreen `json:"safety_screens"`
			Envelope      json.RawMessage      `json:"envelope"`
			Error         bool                 `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			test.Snapshot.SafetyScreens = test.SafetyScreens
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

func TestSafetyEnvelopeIgnoresLegacyDetailBooleansAndProse(t *testing.T) {
	// Only safety screens carry safety authority. Free-form or legacy detail
	// booleans on facts (whatever their review state) are never assertions.
	snapshot := &BodyStateReadModel{CurrentRevision: 3, SafetyState: json.RawMessage(`{}`), Facts: []model.BodyFact{
		{ID: uuid.New(), Kind: "symptom", Statement: "头晕，外伤", Details: datatypes.JSON(`{"additional_notes":"trauma","dizziness":true,"safety_capture_revision":"body-state-safety-capture-v1"}`), AssertionStatus: "asserted", CreatedRevision: 2},
		{ID: uuid.New(), Details: datatypes.JSON(`{"dizziness":true}`), AssertionStatus: "asserted"},
	}}
	envelope, err := ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Assertions) != 0 || envelope.RequiresReview || envelope.Coverage.Complete || len(envelope.Coverage.IncompleteSourceRefs) != 1 {
		t.Fatalf("detail booleans acquired safety authority: %+v", envelope)
	}
}

func TestSafetyEnvelopeCoverageFollowsScreenRevision(t *testing.T) {
	owner := uuid.New()
	symptom := model.BodyFact{ID: uuid.New(), UserID: owner, Kind: "symptom", AssertionStatus: "asserted", Details: datatypes.JSON(`{"presence":"present"}`), CreatedRevision: 4}
	screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, BodyFactID: symptom.ID, BodyStateRevision: 4, RecordedAt: time.Now()}
	for name, test := range map[string]struct {
		screens  []model.SafetyScreen
		complete bool
	}{
		"no-screen":     {nil, false},
		"same-revision": {[]model.SafetyScreen{screen}, true},
		"older-screen":  {[]model.SafetyScreen{{ID: uuid.New(), UserID: owner, BodyFactID: symptom.ID, BodyStateRevision: 3}}, false},

		"older-and-newer": {[]model.SafetyScreen{{ID: uuid.New(), UserID: owner, BodyFactID: symptom.ID, BodyStateRevision: 2}, {ID: uuid.New(), UserID: owner, BodyFactID: symptom.ID, BodyStateRevision: 5}}, true},
	} {
		envelope, err := ProjectSafetyEnvelopeV2(&BodyStateReadModel{UserID: owner, CurrentRevision: 5, Facts: []model.BodyFact{symptom}, SafetyScreens: test.screens})
		if err != nil {
			t.Fatal(err)
		}
		if envelope.Coverage.Complete != test.complete {
			t.Fatalf("%s: coverage complete = %v, want %v (%+v)", name, envelope.Coverage.Complete, test.complete, envelope.Coverage)
		}
	}
	empty, err := ProjectSafetyEnvelopeV2(&BodyStateReadModel{UserID: owner, CurrentRevision: 5, SafetyScreens: []model.SafetyScreen{screen}})
	if err != nil || empty.Coverage.Complete {
		t.Fatalf("a screen without any symptom must not complete coverage: %+v %v", empty.Coverage, err)
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
		envelope, err := ProjectSafetyEnvelopeV2(&BodyStateReadModel{CurrentRevision: 1, SafetyState: json.RawMessage(test.raw)})
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
