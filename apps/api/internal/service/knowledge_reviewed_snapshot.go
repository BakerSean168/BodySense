package service

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/bodysense/api/internal/repository"
)

const ReviewedKnowledgeSnapshotSchemaV1 = "bodysense.reviewed-knowledge-snapshot.v1"

type ReviewedKnowledgeSourceLocator struct {
	LocatorType string   `json:"locator_type"`
	Repository  string   `json:"repository"`
	GitCommit   string   `json:"git_commit"`
	Path        string   `json:"path"`
	LineStart   int      `json:"line_start"`
	LineEnd     int      `json:"line_end"`
	HeadingPath []string `json:"heading_path"`
}

type ReviewedKnowledgeClaimReview struct {
	ReviewID                 string  `json:"review_id"`
	Decision                 string  `json:"decision"`
	ReviewStatus             string  `json:"review_status"`
	QualityScore             float64 `json:"quality_score"`
	ExternalEvidenceReviewID string  `json:"external_evidence_review_id"`
}

type ReviewedKnowledgeSnapshotUnit struct {
	UnitKey             string                         `json:"unit_key"`
	ClaimID             string                         `json:"claim_id"`
	ClaimContentHash    string                         `json:"claim_content_hash"`
	ReviewStatus        string                         `json:"review_status"`
	LifecycleStatus     string                         `json:"lifecycle_status"`
	QualityScore        float64                        `json:"quality_score"`
	PublicationEligible bool                           `json:"publication_eligible"`
	SourceLocator       ReviewedKnowledgeSourceLocator `json:"source_locator"`
	ClaimReview         ReviewedKnowledgeClaimReview   `json:"claim_review"`
}

type ReviewedKnowledgeSnapshotArtifact struct {
	SchemaVersion            string                          `json:"schema_version"`
	ReviewedSnapshotID       string                          `json:"reviewed_snapshot_id"`
	SourceSnapshotID         string                          `json:"source_snapshot_id"`
	SourceGitCommit          string                          `json:"source_git_commit"`
	ExternalEvidenceReviewID string                          `json:"external_evidence_review_id"`
	ClaimReviewID            string                          `json:"claim_review_id"`
	Units                    []ReviewedKnowledgeSnapshotUnit `json:"units"`
}

func ParseReviewedKnowledgeSnapshot(payload []byte) (*ReviewedKnowledgeSnapshotArtifact, error) {
	var artifact ReviewedKnowledgeSnapshotArtifact
	if err := json.Unmarshal(payload, &artifact); err != nil {
		return nil, fmt.Errorf("decode reviewed knowledge snapshot: %w", err)
	}
	if err := ValidateReviewedKnowledgeSnapshotArtifact(&artifact); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func ValidateReviewedKnowledgeSnapshotArtifact(artifact *ReviewedKnowledgeSnapshotArtifact) error {
	if artifact == nil {
		return errors.New("reviewed knowledge snapshot is required")
	}
	if artifact.SchemaVersion != ReviewedKnowledgeSnapshotSchemaV1 {
		return fmt.Errorf("unsupported reviewed knowledge snapshot schema %q", artifact.SchemaVersion)
	}
	if strings.TrimSpace(artifact.ReviewedSnapshotID) == "" ||
		strings.TrimSpace(artifact.SourceSnapshotID) == "" ||
		strings.TrimSpace(artifact.SourceGitCommit) == "" ||
		strings.TrimSpace(artifact.ExternalEvidenceReviewID) == "" ||
		strings.TrimSpace(artifact.ClaimReviewID) == "" {
		return errors.New("reviewed snapshot identity fields are required")
	}
	if !strings.Contains(artifact.SourceSnapshotID, ":"+artifact.SourceGitCommit+":") {
		return errors.New("reviewed snapshot source_snapshot_id does not bind source_git_commit")
	}
	if len(artifact.Units) == 0 {
		return errors.New("reviewed snapshot must contain at least one unit")
	}
	seen := make(map[string]struct{}, len(artifact.Units))
	seenClaims := make(map[string]struct{}, len(artifact.Units))
	for _, unit := range artifact.Units {
		if strings.TrimSpace(unit.UnitKey) == "" || strings.TrimSpace(unit.ClaimID) == "" || len(unit.ClaimContentHash) != 64 {
			return fmt.Errorf("reviewed snapshot contains incomplete unit identity for %q", unit.UnitKey)
		}
		if _, err := hex.DecodeString(unit.ClaimContentHash); err != nil {
			return fmt.Errorf("reviewed snapshot unit %s has invalid claim content hash", unit.UnitKey)
		}
		if _, exists := seen[unit.UnitKey]; exists {
			return fmt.Errorf("reviewed snapshot contains duplicate unit_key %s", unit.UnitKey)
		}
		seen[unit.UnitKey] = struct{}{}
		claimIdentity := unit.ClaimID + ":" + unit.ClaimContentHash
		if _, exists := seenClaims[claimIdentity]; exists {
			return fmt.Errorf("reviewed snapshot contains duplicate claim identity %s", unit.ClaimID)
		}
		seenClaims[claimIdentity] = struct{}{}
		if !unit.PublicationEligible || unit.LifecycleStatus != "reviewed" || unit.ReviewStatus != "reviewed" {
			return fmt.Errorf("reviewed snapshot unit %s is not publication-eligible reviewed knowledge", unit.UnitKey)
		}
		if unit.SourceLocator.LocatorType != "markdown_lines" ||
			strings.TrimSpace(unit.SourceLocator.Repository) == "" ||
			unit.SourceLocator.GitCommit != artifact.SourceGitCommit ||
			strings.TrimSpace(unit.SourceLocator.Path) == "" ||
			len(unit.SourceLocator.HeadingPath) == 0 ||
			unit.SourceLocator.LineStart <= 0 ||
			unit.SourceLocator.LineEnd < unit.SourceLocator.LineStart {
			return fmt.Errorf("reviewed snapshot unit %s has invalid source locator", unit.UnitKey)
		}
		if unit.ClaimReview.ReviewID != artifact.ClaimReviewID ||
			unit.ClaimReview.Decision != "approved" ||
			unit.ClaimReview.ReviewStatus != "reviewed" ||
			unit.ClaimReview.ExternalEvidenceReviewID != artifact.ExternalEvidenceReviewID {
			return fmt.Errorf("reviewed snapshot unit %s has inconsistent claim review identity", unit.UnitKey)
		}
		if math.Abs(unit.ClaimReview.QualityScore-unit.QualityScore) > 1e-9 {
			return fmt.Errorf("reviewed snapshot unit %s quality score differs from claim review", unit.UnitKey)
		}
	}
	return nil
}

func (artifact ReviewedKnowledgeSnapshotArtifact) UnitKeys() []string {
	keys := make([]string, 0, len(artifact.Units))
	for _, unit := range artifact.Units {
		keys = append(keys, unit.UnitKey)
	}
	sort.Strings(keys)
	return keys
}

func reviewedSnapshotUnitMap(artifact *ReviewedKnowledgeSnapshotArtifact) map[string]ReviewedKnowledgeSnapshotUnit {
	units := make(map[string]ReviewedKnowledgeSnapshotUnit, len(artifact.Units))
	for _, unit := range artifact.Units {
		units[unit.UnitKey] = unit
	}
	return units
}

func metadataInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func reviewedExternalSupportMatches(value any, reviewID string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, raw := range items {
		item := publicationMapValue(raw)
		if item["support_status"] == "reviewed_support" &&
			item["admissibility_status"] == "admissible_for_claim_review" &&
			item["external_review_id"] == reviewID &&
			item["license_status"] != "rejected" {
			return true
		}
	}
	return false
}

func metadataStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil
		}
		result = append(result, text)
	}
	return result
}

func equalReviewedSnapshotStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func ValidateKnowledgeUnitAgainstReviewedSnapshot(
	unit repository.KnowledgePublicationUnit,
	artifactUnit ReviewedKnowledgeSnapshotUnit,
	artifact *ReviewedKnowledgeSnapshotArtifact,
) error {
	if unit.UnitKey != artifactUnit.UnitKey {
		return fmt.Errorf("reviewed snapshot unit key mismatch: db=%s artifact=%s", unit.UnitKey, artifactUnit.UnitKey)
	}
	if unit.ContentHash == nil || *unit.ContentHash != artifactUnit.ClaimContentHash {
		return fmt.Errorf("unit %s content hash differs from reviewed snapshot", unit.UnitKey)
	}
	if math.Abs(unit.QualityScore-artifactUnit.QualityScore) > 1e-6 {
		return fmt.Errorf("unit %s quality score differs from reviewed snapshot", unit.UnitKey)
	}
	metadata, err := decodeKnowledgeMetadata(unit.Metadata)
	if err != nil {
		return err
	}
	if metadata["snapshot_id"] != artifact.SourceSnapshotID {
		return fmt.Errorf("unit %s source snapshot identity mismatch", unit.UnitKey)
	}
	claim := publicationMapValue(metadata["claim_candidate"])
	if claim["claim_id"] != artifactUnit.ClaimID {
		return fmt.Errorf("unit %s claim identity mismatch", unit.UnitKey)
	}
	claimReview := publicationMapValue(metadata["claim_review"])
	if claimReview["review_id"] != artifact.ClaimReviewID ||
		claimReview["decision"] != "approved" ||
		claimReview["review_status"] != "reviewed" ||
		claimReview["external_evidence_review_id"] != artifact.ExternalEvidenceReviewID {
		return fmt.Errorf("unit %s claim review identity mismatch", unit.UnitKey)
	}
	locator := publicationMapValue(metadata["source_locator"])
	if locator["locator_type"] != artifactUnit.SourceLocator.LocatorType ||
		locator["repository"] != artifactUnit.SourceLocator.Repository ||
		locator["git_commit"] != artifactUnit.SourceLocator.GitCommit ||
		locator["path"] != artifactUnit.SourceLocator.Path ||
		!equalReviewedSnapshotStringSlices(metadataStringSlice(locator["heading_path"]), artifactUnit.SourceLocator.HeadingPath) ||
		metadataInt(locator["line_start"]) != artifactUnit.SourceLocator.LineStart ||
		metadataInt(locator["line_end"]) != artifactUnit.SourceLocator.LineEnd {
		return fmt.Errorf("unit %s source locator differs from reviewed snapshot", unit.UnitKey)
	}
	if !reviewedExternalSupportMatches(metadata["external_evidence_candidates"], artifact.ExternalEvidenceReviewID) {
		return fmt.Errorf("unit %s external evidence review identity mismatch", unit.UnitKey)
	}
	return nil
}
