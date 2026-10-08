package consultation

import (
	"encoding/json"
	"testing"
)

func TestNormalizeSpatialContextMetadataAcceptsCanonicalRegionAndPersistsSanitizedMetadata(t *testing.T) {
	metadata, spatial, err := normalizeSpatialContextMetadata(json.RawMessage(`{
		"body_explorer_context": {
			"body_region_id": " shoulder.right ",
			"body_region_label": " 右肩 ",
			"anatomy_id": " appendicular-skeleton-clavicle-right ",
			"anatomy_name": " Right clavicle "
		},
		"untrusted_extra": "discard-me"
	}`))
	if err != nil {
		t.Fatalf("normalize spatial context: %v", err)
	}
	if spatial == nil {
		t.Fatal("expected spatial context")
	}
	if spatial.BodyRegionID != "shoulder.right" || spatial.BodyRegionLabel != "右肩" {
		t.Fatalf("unexpected normalized region context: %#v", spatial)
	}
	if spatial.AnatomyID != "appendicular-skeleton-clavicle-right" || spatial.AnatomyName != "Right clavicle" {
		t.Fatalf("unexpected normalized anatomy context: %#v", spatial)
	}
	var persisted map[string]any
	if err := json.Unmarshal(metadata, &persisted); err != nil {
		t.Fatalf("decode persisted metadata: %v", err)
	}
	if _, ok := persisted["untrusted_extra"]; ok {
		t.Fatalf("unexpected untrusted metadata persisted: %#v", persisted)
	}
}

func TestNormalizeSpatialContextMetadataAcceptsMultipleRegionsAndReferenceMotion(t *testing.T) {
	metadata, spatial, err := normalizeSpatialContextMetadata(json.RawMessage(`{
		"body_explorer_context": {
			"body_region_id": " shoulder.right ",
			"body_region_ids": ["shoulder.right", "scapular.right", "shoulder.right"],
			"reference_motion": {
				"id": " arm_raise ",
				"label": " 客户端伪标签 ",
				"phase": 0.375,
				"paused": true,
				"source": " reference_animation "
			}
		}
	}`))
	if err != nil {
		t.Fatalf("normalize spatial context: %v", err)
	}
	if spatial == nil || len(spatial.BodyRegionIDs) != 2 {
		t.Fatalf("unexpected multi-region context: %#v", spatial)
	}
	if spatial.BodyRegionIDs[0] != "shoulder.right" || spatial.BodyRegionIDs[1] != "scapular.right" {
		t.Fatalf("unexpected region order: %#v", spatial.BodyRegionIDs)
	}
	if spatial.ReferenceMotion == nil || spatial.ReferenceMotion.ID != "arm_raise" ||
		spatial.ReferenceMotion.Label != "抬臂观察" ||
		spatial.ReferenceMotion.Source != "reference_animation" ||
		spatial.ReferenceMotion.Phase != 0.375 || !spatial.ReferenceMotion.Paused {
		t.Fatalf("unexpected reference motion: %#v", spatial.ReferenceMotion)
	}
	var persisted map[string]any
	if err := json.Unmarshal(metadata, &persisted); err != nil {
		t.Fatalf("decode persisted metadata: %v", err)
	}
	if _, ok := persisted["body_explorer_context"]; !ok {
		t.Fatalf("expected sanitized Body Canvas context: %#v", persisted)
	}
}

func TestNormalizeSpatialContextMetadataRejectsInvalidReferenceMotion(t *testing.T) {
	for _, payload := range []string{
		`{"body_explorer_context":{"reference_motion":{"id":"unknown","label":"unknown","phase":0.5,"paused":true,"source":"reference_animation"}}}`,
		`{"body_explorer_context":{"reference_motion":{"id":"stand","label":"站立","phase":1.2,"paused":true,"source":"reference_animation"}}}`,
		`{"body_explorer_context":{"reference_motion":{"id":"stand","label":"站立","phase":0.2,"paused":true,"source":"measured_motion"}}}`,
	} {
		if _, _, err := normalizeSpatialContextMetadata(json.RawMessage(payload)); err == nil {
			t.Fatalf("expected invalid reference motion for %s", payload)
		}
	}
}

func TestNormalizeSpatialContextMetadataRejectsUnknownCanonicalRegion(t *testing.T) {
	_, _, err := normalizeSpatialContextMetadata(json.RawMessage(`{
		"body_explorer_context": {"body_region_id": "shoulder.middle"}
	}`))
	if err == nil {
		t.Fatal("expected invalid spatial context error")
	}
}

func TestNormalizeSpatialContextMetadataTreatsMissingContextAsEmpty(t *testing.T) {
	metadata, spatial, err := normalizeSpatialContextMetadata(json.RawMessage(`{"other":true}`))
	if err != nil {
		t.Fatalf("normalize metadata: %v", err)
	}
	if spatial != nil {
		t.Fatalf("expected no spatial context, got %#v", spatial)
	}
	if string(metadata) != `{}` {
		t.Fatalf("expected sanitized empty metadata, got %s", metadata)
	}
}
