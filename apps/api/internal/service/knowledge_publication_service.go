package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const KnowledgePublicationMinQualityScore = 0.90

type PublishKnowledgeBatchInput struct {
	PublicationKey string
	BatchKey       string
	UnitKeys       []string
	PublishedBy    string
	Summary        string
}

type RollbackKnowledgeBatchInput struct {
	PublicationKey         string
	RollbackPublicationKey string
	RolledBackBy           string
	Reason                 string
}

type knowledgeUnitPreState struct {
	UnitID           int64      `json:"unit_id"`
	UnitKey          string     `json:"unit_key"`
	LifecycleStatus  string     `json:"lifecycle_status"`
	ReviewStatus     string     `json:"review_status"`
	QualityScore     float64    `json:"quality_score"`
	ContentHash      string     `json:"content_hash"`
	PublicationID    *uuid.UUID `json:"publication_id,omitempty"`
	PublishedVersion *int       `json:"published_version,omitempty"`
}

type knowledgePublicationMetadata struct {
	UnitKeys  []string                `json:"unit_keys"`
	PreStates []knowledgeUnitPreState `json:"pre_states"`
}

type KnowledgePublicationService struct {
	repo       *repository.KnowledgePublicationRepository
	txManager  *database.TransactionManager
	minQuality float64
}

func NewKnowledgePublicationService(
	repo *repository.KnowledgePublicationRepository,
	txManager *database.TransactionManager,
) *KnowledgePublicationService {
	return &KnowledgePublicationService{
		repo:       repo,
		txManager:  txManager,
		minQuality: KnowledgePublicationMinQualityScore,
	}
}

func uniqueSortedUnitKeys(keys []string) ([]string, error) {
	seen := make(map[string]struct{}, len(keys))
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			return nil, errors.New("knowledge publication unit_key must not be empty")
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	if len(result) == 0 {
		return nil, errors.New("knowledge publication requires at least one unit_key")
	}
	sort.Strings(result)
	return result, nil
}

func decodeKnowledgeMetadata(raw datatypes.JSON) (map[string]any, error) {
	metadata := map[string]any{}
	if len(raw) == 0 {
		return metadata, nil
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, fmt.Errorf("decode knowledge unit metadata: %w", err)
	}
	return metadata, nil
}

func publicationMapValue(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	return mapped
}

func reviewedExternalSupportExists(value any) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, raw := range items {
		item := publicationMapValue(raw)
		if item["support_status"] == "reviewed_support" &&
			item["admissibility_status"] == "admissible_for_claim_review" &&
			item["license_status"] != "rejected" {
			return true
		}
	}
	return false
}

// ValidateKnowledgeUnitForPublication is the hard gate before a reviewed unit can become visible.
func ValidateKnowledgeUnitForPublication(
	unit repository.KnowledgePublicationUnit,
	minQuality float64,
) error {
	if unit.LifecycleStatus != "reviewed" {
		return fmt.Errorf("unit %s lifecycle_status=%s, want reviewed", unit.UnitKey, unit.LifecycleStatus)
	}
	if unit.ReviewStatus != "reviewed" && unit.ReviewStatus != "approved" && unit.ReviewStatus != "curated" {
		return fmt.Errorf("unit %s review_status=%s is not publishable", unit.UnitKey, unit.ReviewStatus)
	}
	if unit.QualityScore < minQuality {
		return fmt.Errorf("unit %s quality_score=%.3f below %.3f", unit.UnitKey, unit.QualityScore, minQuality)
	}
	if unit.ContentHash == nil || *unit.ContentHash == "" {
		return fmt.Errorf("unit %s has no content_hash", unit.UnitKey)
	}
	digest := sha256.Sum256([]byte(unit.BodyMarkdown))
	actualHash := hex.EncodeToString(digest[:])
	if actualHash != *unit.ContentHash {
		return fmt.Errorf("unit %s content_hash does not match body_markdown", unit.UnitKey)
	}
	if unit.SourceLicenseStatus == "rejected" {
		return fmt.Errorf("unit %s source license is rejected", unit.UnitKey)
	}
	metadata, err := decodeKnowledgeMetadata(unit.Metadata)
	if err != nil {
		return err
	}
	admissibility := publicationMapValue(metadata["claim_admissibility"])
	if admissibility["publication_eligible"] != true || admissibility["status"] != "claim_reviewed" {
		return fmt.Errorf("unit %s claim is not publication-eligible", unit.UnitKey)
	}
	claimReview := publicationMapValue(metadata["claim_review"])
	if claimReview["decision"] != "approved" || claimReview["review_status"] != "reviewed" {
		return fmt.Errorf("unit %s has no approved claim review", unit.UnitKey)
	}
	if !reviewedExternalSupportExists(metadata["external_evidence_candidates"]) {
		return fmt.Errorf("unit %s has no reviewed direct external support", unit.UnitKey)
	}
	if unit.SourceType == "thought_forest_note" {
		locator := publicationMapValue(metadata["source_locator"])
		if locator["locator_type"] != "markdown_lines" || locator["git_commit"] == "" || locator["path"] == "" {
			return fmt.Errorf("unit %s has incomplete Markdown provenance", unit.UnitKey)
		}
	}
	if unit.TranscriptExcerpt == "" {
		return fmt.Errorf("unit %s has no evidence excerpt", unit.UnitKey)
	}
	return nil
}

func (s *KnowledgePublicationService) PublishBatch(
	ctx context.Context,
	input PublishKnowledgeBatchInput,
) (*model.KnowledgePublication, error) {
	if input.PublicationKey == "" || input.BatchKey == "" || input.PublishedBy == "" {
		return nil, errors.New("publication_key, batch_key and published_by are required")
	}
	unitKeys, err := uniqueSortedUnitKeys(input.UnitKeys)
	if err != nil {
		return nil, err
	}

	var publication model.KnowledgePublication
	err = s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		units, err := s.repo.LockUnitsByKeys(txCtx, unitKeys)
		if err != nil {
			return err
		}
		if len(units) != len(unitKeys) {
			return fmt.Errorf("publication requested %d units but locked %d", len(unitKeys), len(units))
		}
		preStates := make([]knowledgeUnitPreState, 0, len(units))
		for _, unit := range units {
			if err := ValidateKnowledgeUnitForPublication(unit, s.minQuality); err != nil {
				return err
			}
			preStates = append(preStates, knowledgeUnitPreState{
				UnitID: unit.ID, UnitKey: unit.UnitKey,
				LifecycleStatus: unit.LifecycleStatus, ReviewStatus: unit.ReviewStatus,
				QualityScore: unit.QualityScore, ContentHash: *unit.ContentHash,
				PublicationID: unit.PublicationID, PublishedVersion: unit.PublishedVersion,
			})
		}
		batchVersion, err := s.repo.NextBatchVersion(txCtx, input.BatchKey)
		if err != nil {
			return err
		}
		metadataBytes, err := json.Marshal(knowledgePublicationMetadata{
			UnitKeys: unitKeys, PreStates: preStates,
		})
		if err != nil {
			return err
		}
		publication = model.KnowledgePublication{
			ID: uuid.New(), PublicationKey: input.PublicationKey, BatchKey: input.BatchKey,
			Title: fmt.Sprintf("Knowledge publication %s", input.BatchKey), Summary: input.Summary,
			UnitCount: len(units), PublishedVersion: batchVersion, PublishedAt: time.Now().UTC(),
			PublishedBy: input.PublishedBy, CreatedBy: input.PublishedBy,
			Status: "published", Metadata: datatypes.JSON(metadataBytes),
		}
		if err := s.repo.Create(txCtx, &publication); err != nil {
			return err
		}
		for _, unit := range units {
			if err := s.repo.UpdateUnitForPublication(txCtx, unit.ID, publication.ID, batchVersion); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &publication, nil
}

func (s *KnowledgePublicationService) RollbackBatch(
	ctx context.Context,
	input RollbackKnowledgeBatchInput,
) (*model.KnowledgePublication, error) {
	if input.PublicationKey == "" || input.RollbackPublicationKey == "" || input.RolledBackBy == "" {
		return nil, errors.New("publication_key, rollback_publication_key and rolled_back_by are required")
	}
	var rollback model.KnowledgePublication
	err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		publication, err := s.repo.LockPublicationByKey(txCtx, input.PublicationKey)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("publication %s not found", input.PublicationKey)
			}
			return err
		}
		if publication.Status != "published" {
			return fmt.Errorf("publication %s status=%s, want published", input.PublicationKey, publication.Status)
		}
		var metadata knowledgePublicationMetadata
		if err := json.Unmarshal(publication.Metadata, &metadata); err != nil {
			return fmt.Errorf("decode publication metadata: %w", err)
		}
		units, err := s.repo.LockUnitsByKeys(txCtx, metadata.UnitKeys)
		if err != nil {
			return err
		}
		if len(units) != len(metadata.PreStates) {
			return errors.New("rollback unit set no longer matches publication metadata")
		}
		unitsByKey := make(map[string]repository.KnowledgePublicationUnit, len(units))
		for _, unit := range units {
			unitsByKey[unit.UnitKey] = unit
		}
		for _, pre := range metadata.PreStates {
			unit, ok := unitsByKey[pre.UnitKey]
			if !ok || unit.PublicationID == nil || *unit.PublicationID != publication.ID {
				return fmt.Errorf("rollback identity drift for unit %s", pre.UnitKey)
			}
			if unit.LifecycleStatus != "published" || unit.PublishedVersion == nil ||
				*unit.PublishedVersion != publication.PublishedVersion {
				return fmt.Errorf("rollback lifecycle/version drift for unit %s", pre.UnitKey)
			}
			if unit.ContentHash == nil || *unit.ContentHash != pre.ContentHash {
				return fmt.Errorf("rollback content drift for unit %s", pre.UnitKey)
			}
		}

		rollbackMetadata, err := json.Marshal(map[string]any{
			"rollback_of": publication.ID.String(), "unit_keys": metadata.UnitKeys,
			"reason": input.Reason,
		})
		if err != nil {
			return err
		}
		rollback = model.KnowledgePublication{
			ID: uuid.New(), PublicationKey: input.RollbackPublicationKey,
			BatchKey: publication.BatchKey, RollbackOf: &publication.ID,
			Title: fmt.Sprintf("Rollback %s", publication.PublicationKey), Summary: input.Reason,
			UnitCount: len(metadata.PreStates), PublishedVersion: publication.PublishedVersion,
			PublishedAt: time.Now().UTC(), PublishedBy: input.RolledBackBy,
			CreatedBy: input.RolledBackBy, Status: "rollback", Metadata: datatypes.JSON(rollbackMetadata),
		}
		if err := s.repo.Create(txCtx, &rollback); err != nil {
			return err
		}
		for _, pre := range metadata.PreStates {
			if err := s.repo.RestoreUnitPublicationState(
				txCtx, pre.UnitID, pre.LifecycleStatus, pre.ReviewStatus,
				pre.QualityScore, pre.PublicationID, pre.PublishedVersion,
			); err != nil {
				return err
			}
		}
		return s.repo.UpdatePublicationStatus(txCtx, publication.ID, "rolled_back")
	})
	if err != nil {
		return nil, err
	}
	return &rollback, nil
}
