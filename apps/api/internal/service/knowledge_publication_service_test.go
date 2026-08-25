package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bodysense/api/internal/repository"
	"gorm.io/datatypes"
)

func testEmbeddingIdentity() map[string]any {
	provider := "hashing"
	modelName := "bodysense-hashing-ngram"
	dimension := 1536
	revision := "sha256-char-word-ngram-v1"
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\n%s\n%d\n%s", provider, modelName, dimension, revision)))
	return map[string]any{
		"provider": provider, "model": modelName, "dimension": dimension,
		"revision": revision, "fingerprint": hex.EncodeToString(digest[:]),
	}
}

func publishableUnit(t *testing.T) repository.KnowledgePublicationUnit {
	t.Helper()
	body := "## Definition\n\nReviewed claim text."
	digest := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(digest[:])
	metadata, err := json.Marshal(map[string]any{
		"embedding_identity":  testEmbeddingIdentity(),
		"claim_admissibility": map[string]any{"status": "claim_reviewed", "publication_eligible": true},
		"claim_review":        map[string]any{"decision": "approved", "review_status": "reviewed"},
		"external_evidence_candidates": []any{map[string]any{
			"support_status": "reviewed_support", "admissibility_status": "admissible_for_claim_review",
			"license_status": "citation_only",
		}},
		"source_locator": map[string]any{"locator_type": "markdown_lines", "git_commit": "abc123", "path": "z/sample.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository.KnowledgePublicationUnit{
		ID: 1, UnitKey: "tfu-pilot", BodyMarkdown: body,
		TranscriptExcerpt: body, LifecycleStatus: "reviewed", ReviewStatus: "reviewed",
		QualityScore: 0.95, ContentHash: &hash, SourceType: "thought_forest_note",
		SourceLicenseStatus: "unknown", HasEmbedding: true, Metadata: datatypes.JSON(metadata),
	}
}

func TestValidateKnowledgeUnitForPublicationAcceptsReviewedClaim(t *testing.T) {
	unit := publishableUnit(t)
	if err := ValidateKnowledgeUnitForPublication(unit, KnowledgePublicationMinQualityScore); err != nil {
		t.Fatalf("expected publishable unit: %v", err)
	}
}

func TestValidateKnowledgeUnitForPublicationRejectsMerelyReviewedLifecycle(t *testing.T) {
	unit := publishableUnit(t)
	unit.Metadata = datatypes.JSON(`{
		"claim_admissibility":{"status":"evidence_ready_for_claim_review","publication_eligible":false},
		"claim_review":{"decision":"approved","review_status":"reviewed"},
		"external_evidence_candidates":[]
	}`)
	if err := ValidateKnowledgeUnitForPublication(unit, KnowledgePublicationMinQualityScore); err == nil {
		t.Fatal("expected publication gate to reject a non-eligible reviewed unit")
	}
}

func TestValidateKnowledgeUnitForPublicationRejectsContentDrift(t *testing.T) {
	unit := publishableUnit(t)
	bad := "deadbeef"
	unit.ContentHash = &bad
	if err := ValidateKnowledgeUnitForPublication(unit, KnowledgePublicationMinQualityScore); err == nil {
		t.Fatal("expected content hash mismatch to fail closed")
	}
}

func TestValidateKnowledgeUnitForPublicationRejectsMissingOrDriftedEmbeddingIdentity(t *testing.T) {
	unit := publishableUnit(t)
	unit.HasEmbedding = false
	if err := ValidateKnowledgeUnitForPublication(unit, KnowledgePublicationMinQualityScore); err == nil {
		t.Fatal("expected missing embedding to fail closed")
	}
	unit = publishableUnit(t)
	var metadata map[string]any
	if err := json.Unmarshal(unit.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	metadata["embedding_identity"].(map[string]any)["revision"] = "drifted-revision"
	raw, _ := json.Marshal(metadata)
	unit.Metadata = datatypes.JSON(raw)
	if err := ValidateKnowledgeUnitForPublication(unit, KnowledgePublicationMinQualityScore); err == nil {
		t.Fatal("expected embedding fingerprint drift to fail closed")
	}
}

func TestValidateKnowledgeUnitForPublicationRejectsRejectedSourceLicense(t *testing.T) {
	unit := publishableUnit(t)
	unit.SourceLicenseStatus = "rejected"
	if err := ValidateKnowledgeUnitForPublication(unit, KnowledgePublicationMinQualityScore); err == nil {
		t.Fatal("expected rejected source license to fail closed")
	}
}
