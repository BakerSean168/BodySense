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

// DiagnosisReadSnapshot is one consistent view of an owner's committed
// Diagnosis head, entries and typed evidence. Uncommitted Agent proposals and
// merely recorded decisions are intentionally not part of this projection.
type DiagnosisReadSnapshot struct {
	Head       *model.Diagnosis
	Conditions []model.ConditionEntry
	Evidence   map[uuid.UUID][]model.ConditionEntryEvidence
}

// ReadSnapshot never creates the Diagnosis aggregate on GET. A user with no
// committed diagnosis receives the empty revision-zero view, while every SQL
// predicate explicitly includes user_id. Repeatable Read prevents an update
// between the head, entries and their evidence from leaking a mixed revision.
func (r *DiagnosisRepository) ReadSnapshot(ctx context.Context, owner uuid.UUID) (*DiagnosisReadSnapshot, error) {
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	snapshot := &DiagnosisReadSnapshot{
		Conditions: []model.ConditionEntry{},
		Evidence:   make(map[uuid.UUID][]model.ConditionEntryEvidence),
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var head model.Diagnosis
		err := tx.Where("user_id = ?", owner).Take(&head).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// A missing aggregate is a valid empty projection; no mutation on GET.
			return nil
		}
		if err != nil {
			return fmt.Errorf("read diagnosis head: %w", err)
		}
		if head.UserID != owner || head.CurrentRevision < 0 {
			return fmt.Errorf("invalid diagnosis head")
		}
		snapshot.Head = &head
		if err := tx.Where("user_id = ?", owner).Order("created_at, id").Find(&snapshot.Conditions).Error; err != nil {
			return fmt.Errorf("read diagnosis conditions: %w", err)
		}
		for _, condition := range snapshot.Conditions {
			if condition.UserID != owner || condition.ID == uuid.Nil || condition.CommittedRevision <= 0 || condition.CommittedRevision > head.CurrentRevision {
				return fmt.Errorf("invalid diagnosis condition revision")
			}
			snapshot.Evidence[condition.ID] = []model.ConditionEntryEvidence{}
		}
		var evidence []model.ConditionEntryEvidence
		if err := tx.Where("user_id = ?", owner).Order("condition_id, evidence_type, evidence_id, evidence_version, role").Find(&evidence).Error; err != nil {
			return fmt.Errorf("read diagnosis evidence: %w", err)
		}
		for _, ref := range evidence {
			if ref.UserID != owner || ref.EvidenceVersion <= 0 {
				return fmt.Errorf("invalid diagnosis evidence")
			}
			if _, exists := snapshot.Evidence[ref.ConditionID]; !exists {
				return fmt.Errorf("diagnosis evidence without owner-visible condition")
			}
			snapshot.Evidence[ref.ConditionID] = append(snapshot.Evidence[ref.ConditionID], ref)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}
