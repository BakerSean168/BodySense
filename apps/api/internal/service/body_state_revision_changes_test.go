package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestResolveBodyStateRevisionFactsIsOwnerScopedAndKeysConcerns(t *testing.T) {
	owner, foreign := uuid.New(), uuid.New()
	fact := func(user uuid.UUID, kind, regions, details string) model.BodyFact {
		return model.BodyFact{ID: uuid.New(), UserID: user, Kind: kind, BodyRegionIDs: datatypes.JSON(regions), Details: datatypes.JSON(details)}
	}
	neck := fact(owner, "symptom", `["neck"]`, `{"presence":"present"}`)
	unplaced := fact(owner, "symptom", `[]`, `{"presence":"present"}`)
	injury := fact(owner, "health_history", `[]`, `{"history_kind":"injury_summary"}`)
	sleep := fact(owner, "lifestyle", `[]`, `{"habit_type":"sleep"}`)
	other := fact(foreign, "symptom", `["neck"]`, `{"presence":"present"}`)
	ledger := &memoryBodyContextLedger{facts: []model.BodyFact{neck, unplaced, injury, sleep, other}}

	ids, _ := json.Marshal([]uuid.UUID{neck.ID, unplaced.ID, injury.ID, other.ID})
	revisions := []model.BodyStateRevision{
		{Revision: 2, ChangeType: "facts.recorded", Changes: datatypes.JSON(`{"fact_ids":` + string(ids) + `}`)},
		{Revision: 3, ChangeType: "facts.withdrawn", Changes: datatypes.JSON(`{"target_fact_id":"` + sleep.ID.String() + `","fact_ids":[]}`)},
		{Revision: 4, ChangeType: "observation.withdrawn", Changes: datatypes.JSON(`{"target_observation_id":"` + uuid.NewString() + `"}`)},
	}
	index, err := resolveBodyStateRevisionFacts(context.Background(), ledger, owner, revisions)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{neck.ID.String(): "region:neck", unplaced.ID.String(): "general", injury.ID.String(): "history:injury", sleep.ID.String(): "lifestyle:sleep"}
	if len(index) != len(want) {
		t.Fatalf("index must hold only the owner's facts: %#v", index)
	}
	for id, concern := range want {
		if index.concern(id) != concern {
			t.Fatalf("fact %s concern=%q want %q", id, index.concern(id), concern)
		}
	}
	if _, leaked := index[other.ID.String()]; leaked {
		t.Fatal("foreign fact resolved")
	}
}

func TestDecodeBodyStateChangeIgnoresLegacyAndMalformedPayloads(t *testing.T) {
	for _, revision := range []model.BodyStateRevision{
		{ChangeType: "fact.added", Changes: datatypes.JSON(`{"fact":{"id":"x"}}`)},
		{ChangeType: "observation.added", Changes: datatypes.JSON(`{}`)},
		{ChangeType: "current_context.updated", Changes: datatypes.JSON(`{}`)},
		{ChangeType: "hypothesis.withdrawn", Changes: datatypes.JSON(`{}`)},
	} {
		if change := decodeBodyStateChange(revision); change.Kind != bodyStateChangeIgnored {
			t.Fatalf("%s must be ignored, got %#v", revision.ChangeType, change)
		}
	}
	if change := decodeBodyStateChange(model.BodyStateRevision{ChangeType: "facts.recorded", Changes: datatypes.JSON(`not json`)}); change.Kind != bodyStateChangeFactsRecorded || len(change.FactIDs) != 0 {
		t.Fatalf("malformed metadata resolves no ids: %#v", change)
	}
	if change := decodeBodyStateChange(model.BodyStateRevision{ChangeType: "safety.screened", Changes: datatypes.JSON(`not json`)}); change.Kind != bodyStateChangeSafety {
		t.Fatalf("safety change must not depend on its payload: %#v", change)
	}
}
