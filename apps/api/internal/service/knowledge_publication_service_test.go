package service

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/bodysense/api/internal/repository"
	"gorm.io/datatypes"
)

func publishableUnit(t *testing.T) repository.KnowledgePublicationUnit {
	t.Helper()
	body := "## Definition\n\nReviewed claim text."
	digest := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(digest[:])
	return repository.KnowledgePublicationUnit{
		ID: 1, UnitKey: "tfu-pilot", BodyMarkdown: body,
		TranscriptExcerpt: body, LifecycleStatus: "reviewed", ReviewStatus: "reviewed",
		QualityScore: 0.95, ContentHash: &hash, SourceType: "thought_forest_note",
		SourceLicenseStatus: "unknown",
		Metadata: datatypes.JSON(`{
			"claim_admissibility":{"status":"claim_reviewed","publication_eligible":true},
			"claim_review":{"decision":"approved","review_status":"reviewed"},
			"external_evidence_candidates":[{
				"support_status":"reviewed_support",
				"admissibility_status":"admissible_for_claim_review",
				"license_status":"citation_only"
			}],
			"source_locator":{"locator_type":"markdown_lines","git_commit":"abc123","path":"z/sample.md"}
		}`),
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

func TestValidateKnowledgeUnitForPublicationRejectsRejectedSourceLicense(t *testing.T) {
	unit := publishableUnit(t)
	unit.SourceLicenseStatus = "rejected"
	if err := ValidateKnowledgeUnitForPublication(unit, KnowledgePublicationMinQualityScore); err == nil {
		t.Fatal("expected rejected source license to fail closed")
	}
}
