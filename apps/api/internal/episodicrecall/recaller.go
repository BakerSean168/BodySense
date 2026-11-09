package episodicrecall

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// RecalledNote is one bounded, provenance-carrying episodic note. Its Text is
// the note's canonical text, byte-for-byte from episodic_notes; the recall path
// never rewrites, summarises or edits it.
type RecalledNote struct {
	ID               uuid.UUID
	ConversationID   uuid.UUID
	FromSeq          int
	ToSeq            int
	SourceMessageIDs []uuid.UUID
	Text             string
	CreatedAt        time.Time
	Score            int
}

// CandidateStore is the only data dependency of the recall path. The SQL
// adapter is internal/repository.EpisodicRecallRepository. It must be
// owner-scoped, tombstone-filtered and bounded by limit.
type CandidateStore interface {
	ListOwnedRecallCandidates(ctx context.Context, owner uuid.UUID, limit int) ([]model.EpisodicNote, error)
}

const (
	// EnabledEnv mirrors the EPISODIC_* kill-switch style. Only an explicit
	// true/1 opts in; unset is today's exact behavior.
	EnabledEnv = "EPISODIC_RECALL_ENABLED"

	// TopK is the approved serving bound (episodic-memory-hybrid-plan.md §6.1).
	TopK = 8

	// CandidateLimit covers the approved per-owner retention cap (96 notes).
	// A user cannot have more retained notes, so no older note is excluded
	// from ranking by this bound.
	CandidateLimit = 96

	// MinScore drops notes with no query-bigram overlap. The baseline ranked
	// zero-overlap candidates by time, which it identified as accidental
	// hits rather than relevance; serving must not inject unrelated notes.
	MinScore = 1

	// RecallTimeout is the hard budget for retrieval + injection. On expiry
	// the turn silently continues with no episodic notes.
	RecallTimeout = 100 * time.Millisecond
)

// EpisodicRecallEnabledFromEnv reads the default-off switch.
func EpisodicRecallEnabledFromEnv(getenv func(string) string) bool {
	value := strings.TrimSpace(getenv(EnabledEnv))
	return strings.EqualFold(value, "true") || value == "1"
}

// Recaller performs real-time, per-turn retrieval. It is deliberately
// stateless: no precomputation, no cross-request cache. Every call either
// returns bounded notes or nothing; it never returns an error to the caller,
// because episodic memory must never block or change the consultation path.
type Recaller struct {
	store    CandidateStore
	timeout  time.Duration
	topK     int
	minScore int
	limit    int
	logf     func(format string, args ...any)
}

func NewRecaller(store CandidateStore) *Recaller {
	return &Recaller{
		store:    store,
		timeout:  RecallTimeout,
		topK:     TopK,
		minScore: MinScore,
		limit:    CandidateLimit,
		logf:     log.Printf,
	}
}

// Recall returns the most relevant notes for one owner and one query turn.
// Timeout, cancellation and any store error all degrade to an empty result.
func (r *Recaller) Recall(ctx context.Context, owner uuid.UUID, query string) []RecalledNote {
	if r == nil || r.store == nil || owner == uuid.Nil {
		return nil
	}
	tokens := QueryTokens(query)
	if len(tokens) == 0 {
		return nil
	}

	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	notes, err := r.store.ListOwnedRecallCandidates(callCtx, owner, r.limit)
	if err != nil {
		if r.logf != nil {
			r.logf("episodic recall degraded for user %s: %v", owner, err)
		}
		return nil
	}
	if callCtx.Err() != nil {
		if r.logf != nil {
			r.logf("episodic recall timed out for user %s", owner)
		}
		return nil
	}

	type scored struct {
		note  model.EpisodicNote
		score int
	}
	ranked := make([]scored, 0, len(notes))
	for _, note := range notes {
		score := Score(note.ContentText, tokens)
		if score < r.minScore {
			continue
		}
		ranked = append(ranked, scored{note: note, score: score})
	}
	// Deterministic ranking: relevance first, then newest, then id, so equal
	// scores never depend on database row order.
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if !ranked[i].note.CreatedAt.Equal(ranked[j].note.CreatedAt) {
			return ranked[i].note.CreatedAt.After(ranked[j].note.CreatedAt)
		}
		return ranked[i].note.ID.String() > ranked[j].note.ID.String()
	})
	if len(ranked) > r.topK {
		ranked = ranked[:r.topK]
	}

	out := make([]RecalledNote, 0, len(ranked))
	for _, item := range ranked {
		sourceIDs := make([]uuid.UUID, 0, len(item.note.SourceMessageIDs))
		for _, raw := range item.note.SourceMessageIDs {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				// Unverifiable provenance is never served (ADR rule 4).
				sourceIDs = nil
				break
			}
			sourceIDs = append(sourceIDs, parsed)
		}
		if len(sourceIDs) == 0 {
			continue
		}
		out = append(out, RecalledNote{
			ID:               item.note.ID,
			ConversationID:   item.note.ConversationID,
			FromSeq:          item.note.FromSeq,
			ToSeq:            item.note.ToSeq,
			SourceMessageIDs: sourceIDs,
			Text:             item.note.ContentText,
			CreatedAt:        item.note.CreatedAt,
			Score:            item.score,
		})
	}
	return out
}
