package repository

import (
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// eraseStatementBodyRecords clears the text of the derived records once
// none is asserted and nothing pins them. Rows are kept as tombstones
// (identity, kind and lineage) because withdrawn conditions, episodes and
// accepted plan revisions link to them; nothing that links to them changes.
func eraseStatementBodyRecords(tx *gorm.DB, userID, statementID uuid.UUID, pins statementPins) (model.AssetErasureOutcome, error) {
	if pins.derived.unerased() == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, nil
	}
	if active := pins.derived.active(); active > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: active, FailureCode: model.ErasureCodeBodyRecordActive}, nil
	}
	if len(pins.conditions) > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: pins.derived.unerased(), FailureCode: model.AssetErasureCodeDiagnosisPinned}, nil
	}
	if len(pins.plans) > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: pins.derived.unerased(), FailureCode: model.ErasureCodePlanReview}, nil
	}
	erased := int64(0)
	for _, statement := range []string{
		`UPDATE body_facts SET erased_at = now(), statement = '', details = '{}', body_region_ids = '[]',
    temporal = '{"precision": "unknown"}'
WHERE user_id = ? AND source_evidence_id = ? AND erased_at IS NULL AND assertion_status <> 'asserted'`,
		`UPDATE body_observations SET erased_at = now(), raw_value = '', raw_unit = '', method = '',
    measurement_context = '', observed_at = '{"precision": "unknown"}'
WHERE user_id = ? AND source_evidence_id = ? AND erased_at IS NULL AND assertion_status <> 'asserted'`,
		`UPDATE body_hypotheses SET erased_at = now(), statement = '', withdrawal_reason = NULL
WHERE user_id = ? AND source_evidence_id = ? AND erased_at IS NULL AND assertion_status <> 'asserted'`,
	} {
		result := tx.Exec(statement, userID, statementID)
		if result.Error != nil {
			return model.AssetErasureOutcome{}, result.Error
		}
		erased += result.RowsAffected
	}
	after, err := statementDerivedRecords(tx, userID, statementID, false)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if left := after.unerased(); left > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: left, FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: int(erased)}, nil
}
