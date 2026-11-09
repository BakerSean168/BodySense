package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bodysense/api/internal/episodicrecall"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// S4 real-time recall evaluation over a fully synthetic multi-week fixture.
// It exercises the shipping Recaller (same tokenizer, threshold, ranking and
// 100ms budget) with an in-memory candidate store. It never opens a database,
// contacts a model, or reads real user data.
//
// The fixture is human-reviewable JSON. Note and user ids are stable strings
// mapped to deterministic UUIDs here, so no identifier in the fixture is real.

var s4Namespace = uuid.MustParse("7a1d4f92-3c58-5e0b-8b21-6d9a0f4c2e31")

type s4User struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type s4Note struct {
	ID        string `json:"id"`
	User      string `json:"user"`
	Week      int    `json:"week"`
	FromSeq   int    `json:"from_seq"`
	ToSeq     int    `json:"to_seq"`
	CreatedAt string `json:"created_at"`
	Text      string `json:"text"`
}

type s4Deletion struct {
	User    string `json:"user"`
	FromSeq int    `json:"from_seq"`
	ToSeq   int    `json:"to_seq"`
	Note    string `json:"note"`
}

type s4Query struct {
	ID              string   `json:"id"`
	User            string   `json:"user"`
	Week            int      `json:"week"`
	Query           string   `json:"query"`
	ExpectedNoteIDs []string `json:"expected_note_ids"`
}

type s4Fixture struct {
	Version   int          `json:"version"`
	Synthetic bool         `json:"synthetic"`
	Note      string       `json:"note"`
	Weeks     int          `json:"weeks"`
	K         int          `json:"k"`
	Users     []s4User     `json:"users"`
	Notes     []s4Note     `json:"notes"`
	Deletions []s4Deletion `json:"deletions"`
	Queries   []s4Query    `json:"queries"`
}

func s4UUID(kind, id string) uuid.UUID {
	return uuid.NewSHA1(s4Namespace, []byte(kind+":"+id))
}

func loadS4Fixture(path string) (*s4Fixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fx s4Fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		return nil, err
	}
	if !fx.Synthetic {
		return nil, fmt.Errorf("fixture is not marked synthetic")
	}
	if fx.K != episodicrecall.TopK {
		return nil, fmt.Errorf("fixture k=%d does not match runtime top-k=%d", fx.K, episodicrecall.TopK)
	}
	users := map[string]bool{}
	for _, u := range fx.Users {
		users[u.ID] = true
	}
	for _, n := range fx.Notes {
		if !users[n.User] {
			return nil, fmt.Errorf("note %s references unknown user %s", n.ID, n.User)
		}
		if n.FromSeq <= 0 || n.ToSeq < n.FromSeq {
			return nil, fmt.Errorf("note %s has invalid seq range", n.ID)
		}
		if _, err := time.Parse(time.RFC3339, n.CreatedAt); err != nil {
			return nil, fmt.Errorf("note %s created_at: %w", n.ID, err)
		}
	}
	return &fx, nil
}

type s4Store struct {
	notes []model.EpisodicNote
}

func (s *s4Store) ListOwnedRecallCandidates(ctx context.Context, owner uuid.UUID, limit int) ([]model.EpisodicNote, error) {
	out := make([]model.EpisodicNote, 0, len(s.notes))
	for _, note := range s.notes {
		if note.UserID == owner {
			out = append(out, note)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (fx *s4Fixture) deletedRanges(user string) [][2]int {
	var out [][2]int
	for _, d := range fx.Deletions {
		if d.User == user {
			out = append(out, [2]int{d.FromSeq, d.ToSeq})
		}
	}
	return out
}

func overlaps(ranges [][2]int, from, to int) bool {
	for _, r := range ranges {
		if r[0] <= to && r[1] >= from {
			return true
		}
	}
	return false
}

// candidateNotes mirrors the repository contract: same owner, shadow status,
// and tombstoned source ranges excluded when applyDeletions is true.
func (fx *s4Fixture) candidateNotes(applyDeletions bool) []model.EpisodicNote {
	ranges := map[string][][2]int{}
	if applyDeletions {
		for _, u := range fx.Users {
			ranges[u.ID] = fx.deletedRanges(u.ID)
		}
	}
	notes := make([]model.EpisodicNote, 0, len(fx.Notes))
	for _, n := range fx.Notes {
		if applyDeletions && overlaps(ranges[n.User], n.FromSeq, n.ToSeq) {
			continue
		}
		created, err := time.Parse(time.RFC3339, n.CreatedAt)
		if err != nil {
			panic(err)
		}
		sources := make(pq.StringArray, 0, n.ToSeq-n.FromSeq+1)
		for seq := n.FromSeq; seq <= n.ToSeq; seq++ {
			sources = append(sources, s4UUID("message", fmt.Sprintf("%s:%d", n.ID, seq)).String())
		}
		notes = append(notes, model.EpisodicNote{
			ID:               s4UUID("note", n.ID),
			UserID:           s4UUID("user", n.User),
			ConversationID:   s4UUID("conversation", n.User),
			FromSeq:          n.FromSeq,
			ToSeq:            n.ToSeq,
			SourceMessageIDs: sources,
			ContentText:      n.Text,
			Status:           "shadow",
			CreatedAt:        created,
		})
	}
	return notes
}

type s4Report struct {
	FixtureVersion        int      `json:"fixture_version"`
	Synthetic             bool     `json:"synthetic"`
	K                     int      `json:"k"`
	Weeks                 int      `json:"weeks"`
	Notes                 int      `json:"notes"`
	CandidateNotes        int      `json:"live_candidate_notes"`
	Queries               int      `json:"queries"`
	RecallProbes          int      `json:"recall_probes"`
	RecallAtK             float64  `json:"recall_at_k"`
	HitRateAtK            float64  `json:"hit_rate_at_k"`
	GhostProbes           int      `json:"ghost_probes"`
	GhostHits             int      `json:"ghost_hits"`
	NaiveControlGhostHits int      `json:"naive_control_ghost_hits"`
	LatencyP50MS          float64  `json:"recall_p50_ms"`
	LatencyP95MS          float64  `json:"recall_p95_ms"`
	BudgetMS              float64  `json:"hard_timeout_ms"`
	MissedQueries         []string `json:"missed_query_ids,omitempty"`
}

// runS4Eval scores the fixture twice: the live path applies the S6-A
// tombstone exclusion (ghost hits must be 0), while the naive control omits
// it to prove the fixture contains a genuine deleted-note ghost risk.
func runS4Eval(fx *s4Fixture) s4Report {
	live := fx.candidateNotes(true)
	naive := fx.candidateNotes(false)
	deleted := map[uuid.UUID]bool{}
	liveIDs := map[uuid.UUID]bool{}
	for _, note := range live {
		liveIDs[note.ID] = true
	}
	for _, note := range naive {
		if !liveIDs[note.ID] {
			deleted[note.ID] = true
		}
	}

	recaller := episodicrecall.NewRecaller(&s4Store{notes: live})
	naiveRecaller := episodicrecall.NewRecaller(&s4Store{notes: naive})
	report := s4Report{
		FixtureVersion: fx.Version,
		Synthetic:      fx.Synthetic,
		K:              episodicrecall.TopK,
		Weeks:          fx.Weeks,
		Notes:          len(fx.Notes),
		Queries:        len(fx.Queries),
		BudgetMS:       float64(episodicrecall.RecallTimeout) / float64(time.Millisecond),
	}

	latencies := make([]time.Duration, 0, len(fx.Queries))
	var recallSum, hitSum float64
	for _, q := range fx.Queries {
		owner := s4UUID("user", q.User)
		started := time.Now()
		got := recaller.Recall(context.Background(), owner, q.Query)
		latencies = append(latencies, time.Since(started))

		if len(q.ExpectedNoteIDs) > 0 {
			expected := map[uuid.UUID]bool{}
			for _, id := range q.ExpectedNoteIDs {
				expected[s4UUID("note", id)] = true
			}
			found := 0
			for _, note := range got {
				if expected[note.ID] {
					found++
				}
			}
			report.RecallProbes++
			recallSum += float64(found) / float64(len(expected))
			if found > 0 {
				hitSum++
			} else {
				report.MissedQueries = append(report.MissedQueries, q.ID)
			}
		} else {
			report.GhostProbes++
		}
		for _, note := range got {
			if deleted[note.ID] {
				report.GhostHits++
			}
		}

		for _, note := range naiveRecaller.Recall(context.Background(), owner, q.Query) {
			if deleted[note.ID] {
				report.NaiveControlGhostHits++
				break
			}
		}
	}
	if report.RecallProbes > 0 {
		report.RecallAtK = round(recallSum / float64(report.RecallProbes))
		report.HitRateAtK = round(hitSum / float64(report.RecallProbes))
	}
	report.LatencyP50MS = ms(percentile(latencies, 0.50))
	report.LatencyP95MS = ms(percentile(latencies, 0.95))
	report.CandidateNotes = len(live)
	return report
}
