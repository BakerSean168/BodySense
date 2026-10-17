package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FindAnalysisRunByKey returns an owner's formal run for an ingestion retry.
// A foreign owner's key is invisible; uniqueness is (user_id, idempotency_key).
func (r *DiagnosisRepository) FindAnalysisRunByKey(ctx context.Context, owner uuid.UUID, key string) (*model.ConditionAnalysisRun, error) {
	var row model.ConditionAnalysisRun
	err := r.ownerDB(ctx, owner).Where("idempotency_key=?", key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListRunProposalIDs returns the immutable proposal identities of one run in
// creation order, used to rebuild an idempotent ingestion receipt.
func (r *DiagnosisRepository) ListRunProposalIDs(ctx context.Context, owner, run uuid.UUID) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	err := r.ownerDB(ctx, owner).Model(&model.DiagnosisProposal{}).Where("run_id=?", run).Order("created_at,id").Limit(1001).Pluck("id", &ids).Error
	return ids, err
}

// RunTerminalKind returns the run's single terminal audit kind, if any.
func (r *DiagnosisRepository) RunTerminalKind(ctx context.Context, owner, run uuid.UUID) (string, error) {
	kinds := []string{}
	err := r.ownerDB(ctx, owner).Model(&model.ConditionAnalysisEvent{}).
		Where("run_id=? AND kind IN ?", run, []string{"committed", "closed_without_change", "safety_blocked", "insufficient_evidence"}).
		Order("sequence").Pluck("kind", &kinds).Error
	if err != nil || len(kinds) == 0 {
		return "", err
	}
	if len(kinds) > 1 {
		return "", fmt.Errorf("formal run has multiple terminal outcomes")
	}
	return kinds[0], nil
}

// ResolveAnalysisRequest reads the owner's versioned user statement that asks
// for the formal analysis. Foreign and missing statements are indistinguishable.
func (r *DiagnosisRepository) ResolveAnalysisRequest(ctx context.Context, owner, evidence uuid.UUID) (*model.StatementEvidence, error) {
	var row model.StatementEvidence
	if err := database.FromContext(ctx, r.db).Clauses(clause.Locking{Strength: "SHARE"}).
		Where("user_id=? AND id=?", owner, evidence).Take(&row).Error; err != nil {
		return nil, diagnosisReadError(err)
	}
	return &row, nil
}

// InsertAnalysisRun writes the immutable formal run base. Proposals and audit
// events follow in the same caller transaction; nothing here adopts a condition.
func (r *DiagnosisRepository) InsertAnalysisRun(ctx context.Context, run *model.ConditionAnalysisRun) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("formal analysis ingestion requires a transaction")
	}
	return database.FromContext(ctx, r.db).Create(run).Error
}

// InsertProposal appends one immutable Agent proposal and its proposal_created event.
func (r *DiagnosisRepository) InsertProposal(ctx context.Context, proposal *model.DiagnosisProposal, bodyRevision int64) error {
	if !database.HasTransaction(ctx) {
		return fmt.Errorf("formal analysis ingestion requires a transaction")
	}
	if err := database.FromContext(ctx, r.db).Create(proposal).Error; err != nil {
		return err
	}
	return r.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{
		UserID: proposal.UserID, RunID: proposal.RunID, Kind: "proposal_created", ProposalID: &proposal.ID,
		BodyStateRevision: bodyRevision, Details: datatypes.JSON(`{}`), RecordedAt: proposal.CreatedAt,
	})
}
