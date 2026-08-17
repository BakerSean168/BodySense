package repository

import (
	"context"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageContextRepository performs bounded historical retrieval. It never
// returns the entire transcript and always verifies conversation ownership.
type MessageContextRepository struct {
	db *gorm.DB
}

func NewMessageContextRepository(db *gorm.DB) *MessageContextRepository {
	return &MessageContextRepository{db: db}
}

func (r *MessageContextRepository) SearchRelevant(
	ctx context.Context,
	userID, conversationID uuid.UUID,
	terms []string,
	excludeRecent int,
	limit int,
) ([]model.Message, error) {
	if len(terms) == 0 {
		return []model.Message{}, nil
	}
	if excludeRecent < 0 {
		excludeRecent = 0
	}
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	var maxSeq int
	if err := r.db.WithContext(ctx).Model(&model.Message{}).
		Where("conversation_id = ?", conversationID).
		Select("COALESCE(MAX(seq), 0)").
		Scan(&maxSeq).Error; err != nil {
		return nil, err
	}
	cutoff := maxSeq - excludeRecent
	if cutoff <= 0 {
		return []model.Message{}, nil
	}

	query := r.db.WithContext(ctx).
		Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("messages.conversation_id = ? AND conversations.user_id = ?", conversationID, userID).
		Where("messages.seq <= ?", cutoff).
		Where("messages.status = ?", "completed").
		Where("messages.role IN ?", []string{"user", "assistant"}).
		Where("COALESCE(messages.content_text, '') <> ''")

	var termClause *gorm.DB
	for _, rawTerm := range terms {
		term := strings.TrimSpace(rawTerm)
		if term == "" {
			continue
		}
		pattern := "%" + escapeLike(term) + "%"
		if termClause == nil {
			termClause = r.db.Where("messages.content_text ILIKE ? ESCAPE '\\\\'", pattern)
		} else {
			termClause = termClause.Or("messages.content_text ILIKE ? ESCAPE '\\\\'", pattern)
		}
	}
	if termClause == nil {
		return []model.Message{}, nil
	}
	query = query.Where(termClause)

	var messages []model.Message
	if err := query.Order("messages.seq DESC").Limit(limit).Find(&messages).Error; err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
