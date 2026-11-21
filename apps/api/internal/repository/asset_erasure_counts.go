package repository

import (
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type assetCopyCounter func(tx *gorm.DB, userID, assetID uuid.UUID) (int, error)

func countRows(tx *gorm.DB, query string, args ...any) (int, error) {
	var n int64
	if err := tx.Raw(query, args...).Scan(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// lockAssetForPin takes the asset row lock and reports whether an adopted
// condition still pins the asset's clinical content. A Diagnosis settlement
// that would add a citation qualifies the asset under a share lock, so the
// answer holds for this transaction.
func lockAssetForPin(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	if err := tx.Exec(`SELECT 1 FROM user_assets WHERE user_id = ? AND id = ? FOR UPDATE`, userID, assetID).Error; err != nil {
		return 0, err
	}
	return countAssetPinningConditions(tx, userID, assetID)
}

func verifyCount(tx *gorm.DB, erased int, query string, args ...any) (model.AssetErasureOutcome, error) {
	held, err := countRows(tx, query, args...)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if held > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: held, FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: erased}, nil
}

// countAssetEpisodicNotes counts narrative copies pinned to the asset's
// explicit source ID. It is never a raw chat retention citation.
func countAssetEpisodicNotes(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	return countRows(tx, `SELECT count(*) FROM episodic_notes
WHERE user_id = ? AND source_asset_ids @> ARRAY[?::uuid]`, userID, assetID)
}

// countAssetDocumentCopies counts extraction runs and reviews that still hold
// content; tombstones are identity only.
func countAssetDocumentCopies(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	return countRows(tx, `SELECT (SELECT count(*) FROM document_extraction_runs WHERE user_id = ? AND asset_id = ? AND erased_at IS NULL)
 + (SELECT count(*) FROM document_indicator_reviews WHERE user_id = ? AND asset_id = ? AND erased_at IS NULL)`, userID, assetID, userID, assetID)
}

// assetPinningConditionsSQL selects the adopted condition entries citing the
// asset: a current evidence link to one of its indicator reviews, or a
// clinical_document claim naming it. Withdrawn (inactive) conditions and the
// immutable Diagnosis audit trail (ids and AI rationale, never document
// content) do not pin.
const assetPinningConditionsSQL = `SELECT c.id FROM condition_entries c
WHERE c.user_id = @user AND c.disposition = 'adopted' AND (
  EXISTS (SELECT 1 FROM condition_entry_evidence e
    JOIN document_indicator_reviews r ON r.user_id = e.user_id AND r.id = e.document_review_id
    WHERE e.user_id = c.user_id AND e.condition_id = c.id AND r.asset_id = @asset)
  OR EXISTS (SELECT 1 FROM jsonb_array_elements(c.source_claim_refs) ref
    WHERE ref->>'sourceKind' = 'clinical_document' AND ref->>'sourceId' = @asset_text))
ORDER BY c.id`

func assetPinningConditions(tx *gorm.DB, userID, assetID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := tx.Raw(assetPinningConditionsSQL, map[string]any{"user": userID, "asset": assetID, "asset_text": assetID.String()}).Scan(&ids).Error
	return ids, err
}

func countAssetPinningConditions(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	ids, err := assetPinningConditions(tx, userID, assetID)
	return len(ids), err
}

func countAssetAssessmentRefs(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	return countRowsMentioning(tx, "assessment_reports", userID, []string{assetID.String()})
}

// countRowsMentioning matches the whole owner-scoped row as JSON text, so a
// reference nested anywhere (FK column, evidence ref, frozen replay input) counts.
func countRowsMentioning(tx *gorm.DB, table string, userID uuid.UUID, needles []string) (int, error) {
	if len(needles) == 0 {
		return 0, nil
	}
	clauses := make([]string, 0, len(needles))
	args := []any{userID}
	for _, needle := range needles {
		clauses = append(clauses, "strpos(to_jsonb(t)::text, ?) > 0")
		args = append(args, needle)
	}
	query := fmt.Sprintf(`SELECT count(*) FROM %s t WHERE t.user_id = ? AND (%s)`, table, strings.Join(clauses, " OR "))
	return countRows(tx, query, args...)
}
