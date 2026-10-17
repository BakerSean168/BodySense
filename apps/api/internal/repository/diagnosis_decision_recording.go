package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LockDecisionFrame follows the same ordering as plan admission: owner,
// BodyState, Diagnosis. NO KEY UPDATE avoids upgrading a parent FK key lock.
func (r *DiagnosisRepository) LockDecisionFrame(ctx context.Context, owner uuid.UUID) (*model.BodyState, *model.Diagnosis, error) {
	if !database.HasTransaction(ctx) {
		return nil, nil, fmt.Errorf("diagnosis decisions require a transaction")
	}
	if owner == uuid.Nil {
		return nil, nil, model.ErrDiagnosisNotFound
	}
	db := database.FromContext(ctx, r.db)
	var user struct{ ID uuid.UUID }
	if err := db.Table("users").Select("id").Where("id=?", owner).Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Take(&user).Error; err != nil {
		return nil, nil, diagnosisReadError(err)
	}
	var body model.BodyState
	if err := db.Where("user_id=?", owner).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&body).Error; err != nil {
		return nil, nil, diagnosisReadError(err)
	}
	var head model.Diagnosis
	if err := db.Where("user_id=?", owner).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&head).Error; err != nil {
		return nil, nil, diagnosisReadError(err)
	}
	return &body, &head, nil
}
func (r *DiagnosisRepository) FindDecisionByKey(ctx context.Context, owner uuid.UUID, key string) (*model.DiagnosisDecision, error) {
	var row model.DiagnosisDecision
	err := r.ownerDB(ctx, owner).Where("idempotency_key=?", key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r *DiagnosisRepository) FindDecisionForProposal(ctx context.Context, owner, proposal uuid.UUID) (*model.DiagnosisDecision, error) {
	var row model.DiagnosisDecision
	err := r.ownerDB(ctx, owner).Where("proposal_id=?", proposal).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r *DiagnosisRepository) AnalysisClosed(ctx context.Context, owner, run uuid.UUID) (bool, error) {
	var count int64
	if err := r.ownerDB(ctx, owner).Model(&model.DiagnosisRevision{}).Where("run_id=?", run).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	err := r.ownerDB(ctx, owner).Model(&model.ConditionAnalysisEvent{}).Where("run_id=? AND kind IN ?", run, []string{"committed", "closed_without_change", "safety_blocked"}).Count(&count).Error
	return count > 0, err
}
func (r *DiagnosisRepository) ProposalUnavailable(ctx context.Context, owner, run, proposal uuid.UUID) (bool, error) {
	var count int64
	if err := r.ownerDB(ctx, owner).Model(&model.DiagnosisProposal{}).Where("run_id=? AND supersedes_proposal_id=?", run, proposal).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	err := r.ownerDB(ctx, owner).Model(&model.ConditionAnalysisEvent{}).Where("run_id=? AND proposal_id=? AND kind IN ?", run, proposal, []string{"proposal_superseded", "proposal_expired"}).Count(&count).Error
	return count > 0, err
}
func (r *DiagnosisRepository) AppendAnalysisEvent(ctx context.Context, event *model.ConditionAnalysisEvent) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("analysis events require a transaction")
	}
	db := database.FromContext(ctx, r.db)
	var run model.ConditionAnalysisRun
	if err := r.ownerDB(ctx, event.UserID).Select("user_id,id").Where("id=?", event.RunID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&run).Error; err != nil {
		return diagnosisReadError(err)
	}
	if err := r.ownerDB(ctx, event.UserID).Model(&model.ConditionAnalysisEvent{}).Select("coalesce(max(sequence),0)+1").Where("run_id=?", event.RunID).Scan(&event.Sequence).Error; err != nil {
		return err
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	return db.Create(event).Error
}
func (r *DiagnosisRepository) InsertDecisionWithEvent(ctx context.Context, decision *model.DiagnosisDecision, bodyRevision int64) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("diagnosis decisions require a transaction")
	}
	if err := database.FromContext(ctx, r.db).Create(decision).Error; err != nil {
		return err
	}
	return r.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: decision.UserID, ID: uuid.New(), RunID: decision.RunID, Kind: "decision_recorded", ProposalID: &decision.ProposalID, DecisionID: &decision.ID, BodyStateRevision: bodyRevision, Details: datatypes.JSON(`{}`), RecordedAt: decision.DecidedAt})
}
func (r *DiagnosisRepository) BlockAnalysisForUrgent(ctx context.Context, owner, run uuid.UUID, bodyRevision int64, now time.Time) error {
	return r.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: owner, ID: uuid.New(), RunID: run, Kind: "safety_blocked", BodyStateRevision: bodyRevision, Details: datatypes.JSON(`{"reason":"active_urgent_safety_restriction"}`), RecordedAt: now})
}
