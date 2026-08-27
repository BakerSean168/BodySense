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
