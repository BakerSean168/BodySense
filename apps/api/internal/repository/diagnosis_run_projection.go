package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const diagnosisRunReadMaxRows = 1000
const diagnosisRunReadMaxEvents = 10000

// DiagnosisRunSnapshot is one consistent read of a user's immutable formal run,
// its proposed contents, recorded choices and ordered audit. It is never an
// authority to commit or to decide whether evidence is still valid.
type DiagnosisRunSnapshot struct {
	Run                      model.ConditionAnalysisRun
	CurrentDiagnosisRevision int64
	Proposals                []model.DiagnosisProposal
	Decisions                []model.DiagnosisDecision
	Events                   []model.ConditionAnalysisEvent
	Settlement               *model.DiagnosisSettlement
}

type DiagnosisRunPage struct {
	Runs            []model.ConditionAnalysisRun
	TerminalEvents  []model.ConditionAnalysisEvent
	Settlements     []model.DiagnosisSettlement
	HasMore         bool
	NextBeforeRunID *uuid.UUID
}

// ReadRunSnapshot is read-only and owner-fenced at every table. Repeatable Read
// prevents mixing one run's presented fingerprint with a later decision,
// supersession or settlement, while avoiding any Diagnosis initialization on GET.
func (r *DiagnosisRepository) ReadRunSnapshot(ctx context.Context, owner, runID uuid.UUID) (*DiagnosisRunSnapshot, error) {
	if owner == uuid.Nil || runID == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	snapshot := &DiagnosisRunSnapshot{
		Proposals: []model.DiagnosisProposal{}, Decisions: []model.DiagnosisDecision{}, Events: []model.ConditionAnalysisEvent{},
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id=? AND id=?", owner, runID).Take(&snapshot.Run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrDiagnosisNotFound
			}
			return fmt.Errorf("read diagnosis run: %w", err)
		}
		var head model.Diagnosis
		if err := tx.Where("user_id=?", owner).Take(&head).Error; err != nil {
			return fmt.Errorf("read diagnosis head for formal run: %w", err)
		}
		snapshot.CurrentDiagnosisRevision = head.CurrentRevision
		if err := tx.Where("user_id=? AND run_id=?", owner, runID).Order("created_at,id").Limit(diagnosisRunReadMaxRows + 1).Find(&snapshot.Proposals).Error; err != nil {
			return fmt.Errorf("read diagnosis proposals: %w", err)
		}
		if err := tx.Where("user_id=? AND run_id=?", owner, runID).Order("decided_at,id").Limit(diagnosisRunReadMaxRows + 1).Find(&snapshot.Decisions).Error; err != nil {
			return fmt.Errorf("read diagnosis decisions: %w", err)
		}
		if err := tx.Where("user_id=? AND run_id=?", owner, runID).Order("sequence,id").Limit(diagnosisRunReadMaxEvents + 1).Find(&snapshot.Events).Error; err != nil {
			return fmt.Errorf("read diagnosis run events: %w", err)
		}
		if len(snapshot.Proposals) > diagnosisRunReadMaxRows || len(snapshot.Decisions) > diagnosisRunReadMaxRows || len(snapshot.Events) > diagnosisRunReadMaxEvents {
			return fmt.Errorf("diagnosis run exceeds bounded read size")
		}
		var settlement model.DiagnosisSettlement
		err := tx.Where("user_id=? AND run_id=?", owner, runID).Take(&settlement).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("read diagnosis settlement: %w", err)
		}
		if err == nil {
			snapshot.Settlement = &settlement
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// ListRunPage is the owner-facing recovery index. Only terminal event kinds and
// settlement receipts are read for a small bounded page, without exposing
// mutable summaries, raw replay inputs, or Agent execution credentials.
func (r *DiagnosisRepository) ListRunPage(ctx context.Context, owner uuid.UUID, limit int, before *uuid.UUID) (*DiagnosisRunPage, error) {
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if limit < 1 || limit > 50 {
		return nil, fmt.Errorf("diagnosis run limit out of bounds")
	}
	page := &DiagnosisRunPage{
		Runs: []model.ConditionAnalysisRun{}, TerminalEvents: []model.ConditionAnalysisEvent{}, Settlements: []model.DiagnosisSettlement{},
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("user_id=?", owner)
		if before != nil {
			if *before == uuid.Nil {
				return model.ErrDiagnosisNotFound
			}
			var anchor model.ConditionAnalysisRun
			if err := tx.Where("user_id=? AND id=?", owner, *before).Select("user_id,id,started_at").Take(&anchor).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return model.ErrDiagnosisNotFound
				}
				return fmt.Errorf("read diagnosis page cursor: %w", err)
			}
			query = query.Where("(started_at,id) < (?,?)", anchor.StartedAt, anchor.ID)
		}
		if err := query.Order("started_at DESC,id DESC").Limit(limit + 1).Find(&page.Runs).Error; err != nil {
			return fmt.Errorf("list formal runs: %w", err)
		}
		if len(page.Runs) > limit {
			page.HasMore = true
			page.Runs = page.Runs[:limit]
			last := page.Runs[len(page.Runs)-1].ID
			page.NextBeforeRunID = &last
		}
		if len(page.Runs) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, len(page.Runs))
		for i, row := range page.Runs {
			if row.UserID != owner || row.ID == uuid.Nil {
				return fmt.Errorf("foreign diagnosis run in index")
			}
			ids[i] = row.ID
		}
		terminal := []string{"committed", "closed_without_change", "safety_blocked", "insufficient_evidence"}
		if err := tx.Where("user_id=? AND run_id IN ? AND kind IN ?", owner, ids, terminal).
			Order("run_id,sequence").Find(&page.TerminalEvents).Error; err != nil {
			return fmt.Errorf("list terminal diagnosis events: %w", err)
		}
		if err := tx.Where("user_id=? AND run_id IN ?", owner, ids).
			Order("run_id").Find(&page.Settlements).Error; err != nil {
			return fmt.Errorf("list diagnosis settlements: %w", err)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return page, nil
}
