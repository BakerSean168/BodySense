package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxReviewPromptConditions = 1000

func (r *DiagnosisRepository) FindReviewPromptDismissalByKey(ctx context.Context, owner uuid.UUID, key string) (*model.DiagnosisReviewPromptDismissal, error) {
	var row model.DiagnosisReviewPromptDismissal
	err := r.ownerDB(ctx, owner).Where("idempotency_key=?", key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// InsertReviewPromptDismissal appends one dismissal. A concurrent retry of the
// same idempotency key surfaces as a unique violation for the caller to replay.
func (r *DiagnosisRepository) InsertReviewPromptDismissal(ctx context.Context, row *model.DiagnosisReviewPromptDismissal) error {
	if row == nil || row.UserID == uuid.Nil || row.ID == uuid.Nil || row.ConditionID == uuid.Nil || len(row.ReasonCodes) == 0 {
		return fmt.Errorf("incomplete review prompt dismissal")
	}
	return database.FromContext(ctx, r.db).Create(row).Error
}

// LatestReviewPromptDismissals returns the newest dismissal per Condition.
// Foreign or unknown Conditions are absent.
func (r *DiagnosisRepository) LatestReviewPromptDismissals(ctx context.Context, owner uuid.UUID, conditions []uuid.UUID) (map[uuid.UUID]model.DiagnosisReviewPromptDismissal, error) {
	result := make(map[uuid.UUID]model.DiagnosisReviewPromptDismissal, len(conditions))
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if len(conditions) == 0 {
		return result, nil
	}
	if len(conditions) > maxReviewPromptConditions {
		return nil, fmt.Errorf("too many conditions for one review prompt read")
	}
	var rows []model.DiagnosisReviewPromptDismissal
	if err := database.FromContext(ctx, r.db).Raw(`SELECT DISTINCT ON (condition_id) *
  FROM diagnosis_review_prompt_dismissals
 WHERE user_id = ? AND condition_id IN ?
 ORDER BY condition_id, dismissed_at DESC, id DESC`, owner, conditions).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UserID == owner {
			result[row.ConditionID] = row
		}
	}
	return result, nil
}

// BindAnalysisRequest turns one of the owner's own completed chat messages into
// the versioned request statement a formal Diagnosis analysis must cite. The
// excerpt is the message text verbatim; nothing is inferred or rewritten.
// Binding the same message again returns the evidence already bound to it.
func (r *DiagnosisRepository) BindAnalysisRequest(ctx context.Context, owner, messageID uuid.UUID, now time.Time) (*model.StatementEvidence, bool, error) {
	if owner == uuid.Nil || messageID == uuid.Nil || now.IsZero() {
		return nil, false, model.ErrDiagnosisRequestSourceNotFound
	}
	var bound *model.StatementEvidence
	replayed := false
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var messages []struct {
			model.Message
			SourceRunID uuid.UUID
		}
		if err := tx.Table("public.messages AS m").Select("m.*, r.id AS source_run_id").
			Joins("JOIN public.conversations AS c ON c.id=m.conversation_id").
			Joins("JOIN public.runs AS r ON r.turn_id=m.turn_id AND r.conversation_id=m.conversation_id").
			Where("m.id=? AND c.user_id=? AND r.user_id=? AND m.role='user' AND m.status='completed'", messageID, owner, owner).
			Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "m"}}).
			Limit(2).Find(&messages).Error; err != nil {
			return err
		}
		if len(messages) != 1 || messages[0].SourceRunID == uuid.Nil {
			return model.ErrDiagnosisRequestSourceNotFound
		}
		source := messages[0]
		var metadata struct {
			IsAnswer bool `json:"is_interaction_answer"`
		}
		if len(source.Metadata) > 0 && json.Unmarshal(source.Metadata, &metadata) != nil {
			return model.ErrDiagnosisRequestSourceInvalid
		}
		text := messageTextFromParts(source.Parts)
		if metadata.IsAnswer || text == "" || !utf8.ValidString(text) || utf8.RuneCountInString(text) > model.DiagnosisAnalysisRequestExcerptMax {
			return model.ErrDiagnosisRequestSourceInvalid
		}
		// One request binding per message even under concurrent clicks.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "diagnosis-request:"+owner.String()+":"+messageID.String()).Error; err != nil {
			return err
		}
		var existing model.StatementEvidence
		err := tx.Where("user_id=? AND source_message_id=? AND source_kind='user_message' AND source_version=1 AND availability='available' AND excerpt=?",
			owner, messageID, text).Order("recorded_at, id").Take(&existing).Error
		if err == nil {
			bound, replayed = &existing, true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		evidence := model.StatementEvidence{
			ID: uuid.New(), UserID: owner, SourceKind: "user_message", SourceVersion: 1,
			SourceConversationID: &source.ConversationID, SourceRunID: &source.SourceRunID, SourceMessageID: &source.ID,
			Excerpt: text, Availability: "available", RecordedAt: now.UTC().Truncate(time.Microsecond),
		}
		if err := tx.Create(&evidence).Error; err != nil {
			return err
		}
		bound = &evidence
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return bound, replayed, nil
}
