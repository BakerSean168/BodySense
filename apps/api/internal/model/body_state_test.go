package model

import (
	"encoding/json"
	"testing"
)

func TestBodyStateFactProjectionIncludesOptionalCanonicalRegionID(t *testing.T) {
	legacy, err := json.Marshal(BodyStateFact{BodyRegion: "肩颈"})
	if err != nil {
		t.Fatalf("marshal legacy fact: %v", err)
	}
	var legacyJSON map[string]any
	if err := json.Unmarshal(legacy, &legacyJSON); err != nil {
		t.Fatalf("unmarshal legacy fact: %v", err)
	}
	value, ok := legacyJSON["body_region_id"]
	if !ok || value != nil {
		t.Fatalf("legacy projection must include body_region_id=null, got %s", legacy)
	}

	canonical := "shoulder.right"
	encoded, err := json.Marshal(BodyStateFact{BodyRegion: "右肩", BodyRegionID: &canonical})
	if err != nil {
		t.Fatalf("marshal canonical fact: %v", err)
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		t.Fatalf("unmarshal canonical fact: %v", err)
	}
	if projected["body_region"] != "右肩" || projected["body_region_id"] != "shoulder.right" {
		t.Fatalf("API projection lost additive region contract: %s", encoded)
	}
}

func TestBodyStateObservationProjectionIncludesOptionalCanonicalRegionID(t *testing.T) {
	canonical := "knee.left"
	encoded, err := json.Marshal(BodyStateObservation{BodyRegion: "左膝", BodyRegionID: &canonical})
	if err != nil {
		t.Fatalf("marshal observation: %v", err)
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		t.Fatalf("unmarshal observation: %v", err)
	}
	if projected["body_region_id"] != "knee.left" {
		t.Fatalf("observation projection lost canonical laterality: %s", encoded)
	}
}
