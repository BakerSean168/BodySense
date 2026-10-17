package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *DiagnosisRepository) LockAnalysisRun(ctx context.Context, owner, run uuid.UUID) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("analysis settlement requires a transaction")
	}
	var row model.ConditionAnalysisRun
	if err := database.FromContext(ctx, r.db).Select("user_id,id").Where("user_id=? AND id=?", owner, run).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&row).Error; err != nil {
		return diagnosisReadError(err)
	}
	return nil
}
func (r *DiagnosisRepository) FindSettlementByKey(ctx context.Context, owner uuid.UUID, key string) (*model.DiagnosisSettlement, error) {
	var row model.DiagnosisSettlement
	err := r.ownerDB(ctx, owner).Where("idempotency_key=?", key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r *DiagnosisRepository) ReadRunChoices(ctx context.Context, owner, run uuid.UUID) ([]model.DiagnosisProposal, []model.DiagnosisDecision, []model.ConditionAnalysisEvent, error) {
	proposals := []model.DiagnosisProposal{}
	decisions := []model.DiagnosisDecision{}
	events := []model.ConditionAnalysisEvent{}
	if err := r.ownerDB(ctx, owner).Where("run_id=?", run).Order("created_at,id").Limit(1001).Find(&proposals).Error; err != nil {
		return nil, nil, nil, err
	}
	if err := r.ownerDB(ctx, owner).Where("run_id=?", run).Order("decided_at,id").Limit(1001).Find(&decisions).Error; err != nil {
		return nil, nil, nil, err
	}
	if err := r.ownerDB(ctx, owner).Where("run_id=?", run).Order("sequence").Limit(10001).Find(&events).Error; err != nil {
		return nil, nil, nil, err
	}
	if len(proposals) > 1000 || len(decisions) > 1000 || len(events) > 10000 {
		return nil, nil, nil, fmt.Errorf("analysis exceeds bounded settlement size")
	}
	return proposals, decisions, events, nil
}
func (r *DiagnosisRepository) ListConditionRelations(ctx context.Context, owner, condition uuid.UUID) ([]model.ConditionRelationRecord, error) {
	if _, err := r.GetCondition(ctx, owner, condition); err != nil {
		return nil, err
	}
	rows := []model.ConditionRelationRecord{}
	err := r.ownerDB(ctx, owner).Where("(from_condition_id=? OR to_condition_id=?)", condition, condition).Order("from_condition_id,to_condition_id,kind").Find(&rows).Error
	return rows, err
}
func (r *DiagnosisRepository) InsertSettlement(ctx context.Context, row *model.DiagnosisSettlement) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("analysis settlement requires a transaction")
	}
	return database.FromContext(ctx, r.db).Create(row).Error
}
func (r *DiagnosisRepository) InsertSettledRevision(ctx context.Context, row *model.DiagnosisRevision) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("diagnosis revision requires a transaction")
	}
	return database.FromContext(ctx, r.db).Create(row).Error
}
func (r *DiagnosisRepository) WriteSettledCondition(ctx context.Context, row *model.ConditionEntry, create bool) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("condition adoption requires a transaction")
	}
	db := database.FromContext(ctx, r.db)
	if create {
		return db.Create(row).Error
	}
	result := db.Model(&model.ConditionEntry{}).Where("user_id=? AND id=?", row.UserID, row.ID).Select("medical_name", "concept_ref", "qualifiers", "assessment", "source_claim_refs", "cause_analysis", "information_gaps", "handling_plan", "disposition", "latest_decision_id", "last_analysis_run_id", "committed_revision", "updated_at").Updates(row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return model.ErrDiagnosisNotFound
	}
	return nil
}
func (r *DiagnosisRepository) ReplaceSettledEvidence(ctx context.Context, owner, condition uuid.UUID, rows []model.ConditionEntryEvidence) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("condition evidence replacement requires a transaction")
	}
	db := database.FromContext(ctx, r.db)
	for _, row := range rows {
		if row.UserID != owner || row.ConditionID != condition {
			return model.ErrDiagnosisNotFound
		}
	}
	if err := db.Where("user_id=? AND condition_id=?", owner, condition).Delete(&model.ConditionEntryEvidence{}).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Create(&rows).Error
}

// A relation proposal changes exactly one explicitly addressed relation, never
// the entire relation set of either participating condition.
func (r *DiagnosisRepository) GetSettledRelation(ctx context.Context, owner, from, to uuid.UUID, kind model.ConditionRelationKind) (*model.ConditionRelationRecord, error) {
	var row model.ConditionRelationRecord
	err := r.ownerDB(ctx, owner).Where("from_condition_id=? AND to_condition_id=? AND kind=?", from, to, kind).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r *DiagnosisRepository) WriteSettledRelation(ctx context.Context, row *model.ConditionRelationRecord) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("condition relation update requires a transaction")
	}
	return database.FromContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "from_condition_id"}, {Name: "to_condition_id"}, {Name: "kind"}},
		DoUpdates: clause.AssignmentColumns([]string{"basis_evidence_refs", "rationale", "latest_decision_id", "committed_revision", "recorded_at"}),
	}).Create(row).Error
}
func (r *DiagnosisRepository) InsertDispositionHistory(ctx context.Context, row *model.ConditionDispositionChange) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("disposition history requires a transaction")
	}
	return database.FromContext(ctx, r.db).Create(row).Error
}
func (r *DiagnosisRepository) AdvanceDiagnosisHead(ctx context.Context, owner uuid.UUID, expected, next int64) error {
	if !database.HasTransaction(ctx) || next != expected+1 {
		return fmt.Errorf("invalid diagnosis head advance")
	}
	result := database.FromContext(ctx, r.db).Model(&model.Diagnosis{}).Where("user_id=? AND current_revision=?", owner, expected).Updates(map[string]any{"current_revision": next, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return model.ErrDiagnosisDecisionConflict
	}
	return nil
}

func (r *DiagnosisRepository) SettlementDocumentAction(ctx context.Context, owner, review uuid.UUID) (string, error) {
	var row model.DocumentIndicatorReview
	if err := r.ownerDB(ctx, owner).Select("action").Where("id=?", review).Take(&row).Error; err != nil {
		return "", diagnosisReadError(err)
	}
	if row.Action != "confirm" && row.Action != "correct" {
		return "", fmt.Errorf("document evidence is not admitted")
	}
	return row.Action, nil
}
