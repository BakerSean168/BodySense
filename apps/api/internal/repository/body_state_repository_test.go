package repository

import (
	"testing"

	"github.com/bodysense/api/internal/model"
	"gorm.io/datatypes"
)

func TestBodyStateSameFactIgnoresJSONFormattingAndKeyOrder(t *testing.T) {
	a := model.BodyStateFact{
		ConcernKey: "region:neck", Kind: "discomfort", BodyRegion: "neck", Value: "same",
		Details:    datatypes.JSON(`{"severity":"mild","trigger":"sitting"}`),
		Provenance: datatypes.JSON(`{"source_type":"test","nested":{"a":1,"b":2}}`),
		Origin:     "user_reported", ReviewState: "confirmed", LifecycleState: "active", Trend: "stable",
	}
	b := a
	b.Details = datatypes.JSON(`{ "trigger": "sitting", "severity": "mild" }`)
	b.Provenance = datatypes.JSON(`{"nested":{"b":2,"a":1},"source_type":"test"}`)
	if !bodyStateSameFact(a, b) {
		t.Fatal("semantically identical JSON must not create a BodyState revision")
	}
}

func TestBodyStateSameJSONDetectsSemanticChange(t *testing.T) {
	if bodyStateSameJSON(datatypes.JSON(`{"status":"requires_review"}`), datatypes.JSON(`{"status":"cleared"}`), `{}`) {
		t.Fatal("different JSON values must not compare equal")
	}
}

func TestBodyStateSameFactTreatsCanonicalLateralityAsSemanticIdentity(t *testing.T) {
	left := "shoulder.left"
	right := "shoulder.right"
	base := model.BodyStateFact{
		ConcernKey: "region:shoulder", Kind: "discomfort", BodyRegion: "肩部", Value: "疼痛",
		Details: datatypes.JSON(`{}`), Provenance: datatypes.JSON(`{}`),
		Origin: "user_reported", ReviewState: "confirmed", LifecycleState: "active", Trend: "stable",
	}
	a := base
	a.BodyRegionID = &left
	b := base
	b.BodyRegionID = &right
	if bodyStateSameFact(a, b) {
		t.Fatal("left and right canonical regions must be different semantic facts")
	}
}

func TestBodyStateSameObservationTreatsCanonicalLateralityAsSemanticIdentity(t *testing.T) {
	left := "knee.left"
	right := "knee.right"
	base := model.BodyStateObservation{
		ConcernKey: "region:knee", Kind: "self_measurement", BodyRegion: "膝部", Method: "self_test",
		Value: datatypes.JSON(`{}`), Condition: datatypes.JSON(`{}`), Provenance: datatypes.JSON(`{}`),
		ReviewState: "confirmed", LifecycleState: "active",
	}
	a := base
	a.BodyRegionID = &left
	b := base
	b.BodyRegionID = &right
	if bodyStateSameObservation(a, b) {
		t.Fatal("left and right canonical observation regions must be different semantic identity")
	}
}
