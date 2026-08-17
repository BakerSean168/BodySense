package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListRevisionsAfter returns semantic changes newer than the pinned revision in
// ascending order. It is used by explicit freshness/review policies rather than
// treating every revision-number difference as invalidation.
func (r *BodyStateRepository) ListRevisionsAfter(
	ctx context.Context,
	userID uuid.UUID,
	afterRevision int64,
	limit int,
) ([]model.BodyStateRevision, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var revisions []model.BodyStateRevision
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND revision > ?", userID, afterRevision).
		Order("revision ASC").
		Limit(limit).
		Find(&revisions).Error
	return revisions, err
}

// UpsertEvidence persists traceable source material independently from Facts.
// A stable UUID derived from the source identity lets Python and Go reference the
// same evidence before/after the retrieval snapshot is committed.
func (r *BodyStateRepository) UpsertEvidence(
	ctx context.Context,
	userID uuid.UUID,
	evidence model.BodyStateEvidence,
) (*model.BodyStateEvidence, error) {
	evidence.SourceType = strings.TrimSpace(evidence.SourceType)
	evidence.SourceKey = strings.TrimSpace(evidence.SourceKey)
	if evidence.SourceType == "" || evidence.SourceKey == "" {
		return nil, errors.New("evidence source_type and source_key are required")
	}
	identity := strings.Join([]string{
		"bodysense:evidence",
		userID.String(),
		evidence.SourceType,
		evidence.SourceKey,
		evidence.SourceVersion,
	}, ":")
	evidence.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(identity))
	evidence.UserID = userID
	if len(evidence.Metadata) == 0 {
		evidence.Metadata = datatypes.JSON(`{}`)
	}
	if evidence.RetrievedAt.IsZero() {
		evidence.RetrievedAt = time.Now().UTC()
	}
	if evidence.CreatedAt.IsZero() {
		evidence.CreatedAt = evidence.RetrievedAt
	}

	err := database.FromContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "source_type"},
			{Name: "source_key"},
			{Name: "source_version"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "summary", "excerpt", "metadata", "retrieved_at",
		}),
	}).Create(&evidence).Error
	if err != nil {
		return nil, err
	}
	// ON CONFLICT can leave the proposed ID in memory even when an older row was
	// updated. Reload by semantic identity so callers always receive the stored ID.
	var stored model.BodyStateEvidence
	if err := database.FromContext(ctx, r.db).
		Where(
			"user_id = ? AND source_type = ? AND source_key = ? AND source_version = ?",
			userID,
			evidence.SourceType,
			evidence.SourceKey,
			evidence.SourceVersion,
		).
		First(&stored).Error; err != nil {
		return nil, err
	}
	return &stored, nil
}

func (r *BodyStateRepository) ListEvidence(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]model.BodyStateEvidence, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var items []model.BodyStateEvidence
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("retrieved_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *BodyStateRepository) GetEvidenceByIDs(
	ctx context.Context,
	userID uuid.UUID,
	ids []uuid.UUID,
) ([]model.BodyStateEvidence, error) {
	if len(ids) == 0 {
		return []model.BodyStateEvidence{}, nil
	}
	var items []model.BodyStateEvidence
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND id IN ?", userID, ids).
		Find(&items).Error
	return items, err
}

// AddHypothesis commits an AI explanation into the Hypothesis collection, never
// into Facts. The mutation itself advances BodyState because hypotheses are part
// of the durable longitudinal reasoning state.
func (r *BodyStateRepository) AddHypothesis(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	hypothesis model.BodyStateHypothesis,
	source string,
) (*model.BodyStateHypothesis, *model.BodyStateRevision, error) {
	var stored model.BodyStateHypothesis
	var committed *model.BodyStateRevision
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		hypothesis.ID = uuid.New()
		hypothesis.UserID = userID
		hypothesis.ConcernKey = bodyStateDefaultString(hypothesis.ConcernKey, "general")
		hypothesis.Statement = strings.TrimSpace(hypothesis.Statement)
		if hypothesis.Statement == "" {
			return errors.New("hypothesis statement is required")
		}
		hypothesis.LifecycleState = bodyStateDefaultString(hypothesis.LifecycleState, "active")
		if !bodyStateAllowedHypothesisState(hypothesis.LifecycleState) {
			return errors.New("invalid hypothesis lifecycle state")
		}
		bodyStateNormalizeHypothesisJSON(&hypothesis)
		hypothesis.CreatedRevision = next
		hypothesis.UpdatedRevision = next
		now := time.Now().UTC()
		hypothesis.CreatedAt = now
		hypothesis.UpdatedAt = now
		if err := tx.WithContext(ctx).Create(&hypothesis).Error; err != nil {
			return err
		}
		stored = hypothesis
		committed, err = bodyStatePersistRevision(
			ctx,
			tx,
			state,
			next,
			"hypothesis.added",
			source,
			map[string]any{"hypothesis": hypothesis},
		)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

func (r *BodyStateRepository) UpdateHypothesisLifecycle(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	hypothesisID uuid.UUID,
	lifecycleState string,
	counterevidenceIDs datatypes.JSON,
	source string,
) (*model.BodyStateHypothesis, *model.BodyStateRevision, error) {
	var stored model.BodyStateHypothesis
	var committed *model.BodyStateRevision
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, expectedRevision)
		if err != nil {
			return err
		}
		var before model.BodyStateHypothesis
		if err := tx.WithContext(ctx).
			Where("id = ? AND user_id = ?", hypothesisID, userID).
			First(&before).Error; err != nil {
			return err
		}
		lifecycleState = bodyStateDefaultString(lifecycleState, before.LifecycleState)
		if !bodyStateAllowedHypothesisState(lifecycleState) {
			return errors.New("invalid hypothesis lifecycle state")
		}
		if len(counterevidenceIDs) == 0 {
			counterevidenceIDs = before.CounterevidenceIDs
		}
		if before.LifecycleState == lifecycleState && string(before.CounterevidenceIDs) == string(counterevidenceIDs) {
			stored = before
			return nil
		}
		if err := tx.WithContext(ctx).Model(&model.BodyStateHypothesis{}).
			Where("id = ? AND user_id = ?", hypothesisID, userID).
			Updates(map[string]any{
				"lifecycle_state":     lifecycleState,
				"counterevidence_ids": counterevidenceIDs,
				"updated_revision":    next,
				"updated_at":          time.Now().UTC(),
			}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", hypothesisID).First(&stored).Error; err != nil {
			return err
		}
		committed, err = bodyStatePersistRevision(
			ctx,
			tx,
			state,
			next,
			"hypothesis.lifecycle_changed",
			source,
			map[string]any{"hypothesis_id": hypothesisID, "before": before, "after": stored},
		)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, committed, nil
}

func bodyStateAllowedHypothesisState(value string) bool {
	switch value {
	case "active", "strengthened", "weakened", "unsupported", "retired":
		return true
	default:
		return false
	}
}

func bodyStateNormalizeHypothesisJSON(hypothesis *model.BodyStateHypothesis) {
	fields := []*datatypes.JSON{
		&hypothesis.SupportingFactIDs,
		&hypothesis.SupportingObservationIDs,
		&hypothesis.SupportingEvidenceIDs,
		&hypothesis.CounterevidenceIDs,
	}
	for _, field := range fields {
		if len(*field) == 0 || !json.Valid(*field) {
			*field = datatypes.JSON(`[]`)
		}
	}
	if len(hypothesis.Provenance) == 0 || !json.Valid(hypothesis.Provenance) {
		hypothesis.Provenance = datatypes.JSON(`{}`)
	}
}
