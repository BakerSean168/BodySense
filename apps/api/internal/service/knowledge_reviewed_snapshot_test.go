package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/repository"
	"gorm.io/datatypes"
)

func reviewedArtifactFixture() ReviewedKnowledgeSnapshotArtifact {
	return ReviewedKnowledgeSnapshotArtifact{
		SchemaVersion:            ReviewedKnowledgeSnapshotSchemaV1,
		ReviewedSnapshotID:       "reviewed-knowledge:cohort",
		SourceSnapshotID:         "thought-forest:abc123:manifest",
		SourceGitCommit:          "abc123",
		ExternalEvidenceReviewID: "evidence-review-1",
		ClaimReviewID:            "claim-review-1",
		Units: []ReviewedKnowledgeSnapshotUnit{{
			UnitKey: "tfu-1", ClaimID: "tfc-1",
			ClaimContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ReviewStatus:     "reviewed", LifecycleStatus: "reviewed", QualityScore: 0.96,
			PublicationEligible: true,
			SourceLocator: ReviewedKnowledgeSourceLocator{
				LocatorType: "markdown_lines", Repository: "thought-forest", GitCommit: "abc123",
				Path: "z/pain.md", LineStart: 10, LineEnd: 20, HeadingPath: []string{"Pain", "Definition"},
			},
			ClaimReview: ReviewedKnowledgeClaimReview{
				ReviewID: "claim-review-1", Decision: "approved", ReviewStatus: "reviewed",
				QualityScore: 0.96, ExternalEvidenceReviewID: "evidence-review-1",
			},
		}},
	}
}

func TestParseReviewedKnowledgeSnapshotAcceptsExactArtifact(t *testing.T) {
	fixture := reviewedArtifactFixture()
	payload, _ := json.Marshal(fixture)
	parsed, err := ParseReviewedKnowledgeSnapshot(payload)
	if err != nil {
		t.Fatalf("ParseReviewedKnowledgeSnapshot: %v", err)
	}
	if len(parsed.UnitKeys()) != 1 || parsed.UnitKeys()[0] != "tfu-1" {
		t.Fatalf("unexpected unit keys: %+v", parsed.UnitKeys())
	}
}

func TestValidateReviewedKnowledgeSnapshotFailsClosedOnDrift(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ReviewedKnowledgeSnapshotArtifact)
	}{
		{"duplicate-unit", func(a *ReviewedKnowledgeSnapshotArtifact) { a.Units = append(a.Units, a.Units[0]) }},
		{"not-eligible", func(a *ReviewedKnowledgeSnapshotArtifact) { a.Units[0].PublicationEligible = false }},
		{"source-commit", func(a *ReviewedKnowledgeSnapshotArtifact) { a.Units[0].SourceLocator.GitCommit = "other" }},
		{"claim-review", func(a *ReviewedKnowledgeSnapshotArtifact) { a.Units[0].ClaimReview.ReviewID = "other" }},
		{"invalid-hash", func(a *ReviewedKnowledgeSnapshotArtifact) { a.Units[0].ClaimContentHash = strings.Repeat("z", 64) }},
		{"snapshot-commit", func(a *ReviewedKnowledgeSnapshotArtifact) { a.SourceSnapshotID = "thought-forest:other:manifest" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := reviewedArtifactFixture()
			tc.mutate(&fixture)
			if err := ValidateReviewedKnowledgeSnapshotArtifact(&fixture); err == nil {
				t.Fatal("expected reviewed snapshot validation failure")
			}
		})
	}
}

func publicationUnitForReviewedArtifact(t *testing.T, artifact ReviewedKnowledgeSnapshotArtifact) repository.KnowledgePublicationUnit {
	t.Helper()
	body := "reviewed body"
	digest := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(digest[:])
	artifact.Units[0].ClaimContentHash = hash
	artifactUnit := artifact.Units[0]
	metadata := map[string]any{
		"embedding_identity":  testEmbeddingIdentity(),
		"snapshot_id":         artifact.SourceSnapshotID,
		"claim_candidate":     map[string]any{"claim_id": artifactUnit.ClaimID},
		"claim_admissibility": map[string]any{"status": "claim_reviewed", "publication_eligible": true},
		"claim_review": map[string]any{
			"review_id": artifact.ClaimReviewID, "decision": "approved", "review_status": "reviewed",
			"external_evidence_review_id": artifact.ExternalEvidenceReviewID,
		},
		"external_evidence_candidates": []any{map[string]any{
			"support_status": "reviewed_support", "admissibility_status": "admissible_for_claim_review",
			"external_review_id": artifact.ExternalEvidenceReviewID, "license_status": "citation_only",
		}},
		"source_locator": map[string]any{
			"locator_type": artifactUnit.SourceLocator.LocatorType, "repository": artifactUnit.SourceLocator.Repository,
			"git_commit": artifactUnit.SourceLocator.GitCommit, "path": artifactUnit.SourceLocator.Path,
			"line_start": artifactUnit.SourceLocator.LineStart, "line_end": artifactUnit.SourceLocator.LineEnd,
			"heading_path": artifactUnit.SourceLocator.HeadingPath,
		},
	}
	raw, _ := json.Marshal(metadata)
	return repository.KnowledgePublicationUnit{
		ID: 1, UnitKey: artifactUnit.UnitKey, BodyMarkdown: body, TranscriptExcerpt: body,
		LifecycleStatus: "reviewed", ReviewStatus: "reviewed", QualityScore: artifactUnit.QualityScore,
		ContentHash: &hash, Metadata: datatypes.JSON(raw), SourceType: "thought_forest_note",
		SourceLicenseStatus: "citation_only", HasEmbedding: true,
	}
}

func TestValidateKnowledgeUnitAgainstReviewedSnapshotRequiresExactIdentity(t *testing.T) {
	artifact := reviewedArtifactFixture()
	unit := publicationUnitForReviewedArtifact(t, artifact)
	artifact.Units[0].ClaimContentHash = *unit.ContentHash
	// PostgreSQL stores knowledge_units.quality_score as REAL (float32). The
	// artifact is JSON float64, so publication identity compares at storage
	// precision rather than requiring impossible bit equality.
	unit.QualityScore = 0.96000003814697266
	if err := ValidateKnowledgeUnitAgainstReviewedSnapshot(unit, artifact.Units[0], &artifact); err != nil {
		t.Fatalf("exact reviewed snapshot binding rejected: %v", err)
	}
	unit.QualityScore = 0.961
	if err := ValidateKnowledgeUnitAgainstReviewedSnapshot(unit, artifact.Units[0], &artifact); err == nil {
		t.Fatal("expected material quality score drift to fail closed")
	}
	unit.QualityScore = artifact.Units[0].QualityScore

	var headingMetadata map[string]any
	_ = json.Unmarshal(unit.Metadata, &headingMetadata)
	headingMetadata["source_locator"].(map[string]any)["heading_path"] = []any{"Pain", "Drifted"}
	headingRaw, _ := json.Marshal(headingMetadata)
	unit.Metadata = datatypes.JSON(headingRaw)
	if err := ValidateKnowledgeUnitAgainstReviewedSnapshot(unit, artifact.Units[0], &artifact); err == nil {
		t.Fatal("expected heading path drift to fail closed")
	}
	unit = publicationUnitForReviewedArtifact(t, artifact)
	artifact.Units[0].ClaimContentHash = *unit.ContentHash

	var metadata map[string]any
	_ = json.Unmarshal(unit.Metadata, &metadata)
	metadata["claim_review"].(map[string]any)["review_id"] = "drifted-review"
	raw, _ := json.Marshal(metadata)
	unit.Metadata = datatypes.JSON(raw)
	if err := ValidateKnowledgeUnitAgainstReviewedSnapshot(unit, artifact.Units[0], &artifact); err == nil {
		t.Fatal("expected claim review drift to fail closed")
	}
}
