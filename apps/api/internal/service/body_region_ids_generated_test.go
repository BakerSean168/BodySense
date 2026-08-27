package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedCanonicalBodyRegionIDsMatchOntologyJSON(t *testing.T) {
	path := filepath.Join("..", "..", "..", "web", "src", "features", "body-explorer", "data", "body-regions.v1.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read BodyRegionOntology source: %v", err)
	}
	var ontology struct {
		SchemaVersion   int `json:"schemaVersion"`
		OntologyVersion int `json:"ontologyVersion"`
		Regions         []struct {
			ID string `json:"id"`
		} `json:"regions"`
	}
	if err := json.Unmarshal(raw, &ontology); err != nil {
		t.Fatalf("decode BodyRegionOntology source: %v", err)
	}
	if ontology.SchemaVersion != 1 || ontology.OntologyVersion != 1 {
		t.Fatalf("unexpected ontology version: schema=%d ontology=%d", ontology.SchemaVersion, ontology.OntologyVersion)
	}
	if len(ontology.Regions) != len(canonicalBodyRegionIDs) {
		t.Fatalf("generated region count %d does not match ontology %d", len(canonicalBodyRegionIDs), len(ontology.Regions))
	}
	for _, region := range ontology.Regions {
		if !IsCanonicalBodyRegionID(region.ID) {
			t.Fatalf("generated validator is missing canonical region %q", region.ID)
		}
	}
}
