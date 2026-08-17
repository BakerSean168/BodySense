package service

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type messageContextSearch interface {
	SearchRelevant(ctx context.Context, userID, conversationID uuid.UUID, terms []string, excludeRecent, limit int) ([]model.Message, error)
}

// ConsultationHistoricalMessage is a bounded quoted excerpt. It is context,
// never durable health truth, and current BodyState always takes precedence.
type ConsultationHistoricalMessage struct {
	MessageID    uuid.UUID `json:"message_id"`
	Role         string    `json:"role"`
	Sequence     int       `json:"sequence"`
	Content      string    `json:"content"`
	MatchedTerms []string  `json:"matched_terms"`
}

type ContextRetrievalService struct {
	repo messageContextSearch
}

func NewContextRetrievalService(repo messageContextSearch) *ContextRetrievalService {
	return &ContextRetrievalService{repo: repo}
}

func (s *ContextRetrievalService) Retrieve(
	ctx context.Context,
	userID, conversationID uuid.UUID,
	queryText string,
	bodyState *BodyStateSnapshot,
) ([]ConsultationHistoricalMessage, error) {
	terms := BuildHistoricalContextTerms(queryText, bodyState)
	messages, err := s.repo.SearchRelevant(ctx, userID, conversationID, terms, 24, 8)
	if err != nil {
		return nil, err
	}
	result := make([]ConsultationHistoricalMessage, 0, len(messages))
	for _, message := range messages {
		matched := make([]string, 0)
		contentLower := strings.ToLower(message.ContentText)
		for _, term := range terms {
			if strings.Contains(contentLower, strings.ToLower(term)) {
				matched = append(matched, term)
			}
		}
		content := strings.TrimSpace(message.ContentText)
		if len([]rune(content)) > 600 {
			content = string([]rune(content)[:600]) + "…"
		}
		result = append(result, ConsultationHistoricalMessage{
			MessageID: message.ID, Role: message.Role, Sequence: message.Seq,
			Content: content, MatchedTerms: matched,
		})
	}
	return result, nil
}

// BuildHistoricalContextTerms combines current-turn language with durable active
// BodyState anchors. Generic filler words are discarded and the search remains
// intentionally bounded.
func BuildHistoricalContextTerms(queryText string, bodyState *BodyStateSnapshot) []string {
	seen := map[string]struct{}{}
	terms := make([]string, 0, 12)
	add := func(value string) {
		value = strings.TrimSpace(value)
		runes := []rune(value)
		if len(runes) < 2 || len(runes) > 40 {
			return
		}
		lower := strings.ToLower(value)
		if _, exists := seen[lower]; exists || historicalStopTerm(lower) {
			return
		}
		seen[lower] = struct{}{}
		terms = append(terms, value)
	}
	for _, token := range strings.FieldsFunc(queryText, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("，。！？；：、,.!?;:()（）[]【】\"'", r)
	}) {
		add(token)
	}
	if bodyState != nil {
		for _, fact := range bodyState.Facts {
			add(fact.BodyRegion)
			add(fact.Value)
			add(fact.ConcernKey)
		}
		for _, observation := range bodyState.Observations {
			add(observation.BodyRegion)
			add(observation.ConcernKey)
		}
	}
	if len(terms) > 12 {
		terms = terms[:12]
	}
	// Stable order makes retrieval/test behavior reproducible when only BodyState
	// terms are present; current-turn tokens retain their leading positions.
	if strings.TrimSpace(queryText) == "" {
		sort.Strings(terms)
	}
	return terms
}

func historicalStopTerm(value string) bool {
	switch value {
	case "这个", "那个", "现在", "今天", "感觉", "请问", "可以", "还是", "然后", "因为", "但是", "我的", "一下":
		return true
	default:
		return false
	}
}
