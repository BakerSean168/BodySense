package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Retrieval is implemented here, in evaluation code only. It is not an API,
// service or runtime path, and nothing in this file is imported by the server.
//
// Candidate generation is deliberately the keyword baseline the ADR asks us
// to measure: OR full-text search over episodic_notes with ts_rank_cd, then a
// pg_trgm fallback for tokens the PostgreSQL text-search parser cannot split.
// Stock PostgreSQL has no Chinese word segmenter, so `to_tsvector('simple')`
// treats a whole Chinese sentence as one lexeme; the report states how much of
// recall therefore comes from the trigram fallback.
//
// Because pg_trgm needs three consecutive characters, a two-character Chinese
// term has no trigram overlap with an embedded occurrence ("早市" vs "去了早市
// 买菜"). The harness therefore also runs a clearly-labelled evaluation-only
// character-bigram control. It is a fairer "optimized keyword" ceiling, not a
// production design and not the prescribed pg_trgm path.

const recallLimit = 8

var queryStopwords = mustSortedStopwords([]string{
	"你还记得", "我提到的", "我之前说过的", "我说了什么", "帮我回忆一下",
	"回忆一下", "还有印象", "有印象", "是不是", "我之前的", "我之前", "说过的",
	"我说过的", "讲过", "聊到", "提到的", "提到过", "提到", "说过", "记得",
	"上次", "之前", "那次", "的时候", "时候", "后来", "怎么样", "什么",
	"细节", "事情", "事", "吗", "呢", "的", "了", "我", "你", "这个", "那个",
	"这段", "这段时间", "前阵子", "上个月", "两三个月前", "前几个月",
	"三个月前", "四个多月前", "半年前", "半年多前", "年初", "前一阵",
	"去年这个时候", "去年冬天", "去年", "关于", "我们", "可以", "以后", "请", "帮",
})

func mustSortedStopwords(in []string) []string {
	out := append([]string(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if len([]rune(out[i])) != len([]rune(out[j])) {
			return len([]rune(out[i])) > len([]rune(out[j]))
		}
		return out[i] < out[j]
	})
	return out
}

func extractTerms(q string) []string {
	s := q
	for _, w := range queryStopwords {
		s = strings.ReplaceAll(s, w, "|")
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) || unicode.IsSpace(r) {
			return '|'
		}
		return r
	}, s)
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(s, "|") {
		part = strings.TrimSpace(part)
		if utf8.RuneCountInString(part) < 2 || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	if len(out) == 0 {
		if t := strings.TrimSpace(q); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func tsqueryText(terms []string) string {
	quoted := make([]string, 0, len(terms))
	for _, t := range terms {
		quoted = append(quoted, "'"+strings.ReplaceAll(t, "'", "''")+"'")
	}
	return strings.Join(quoted, " | ")
}

type rankedNote struct {
	ID    uuid.UUID `gorm:"column:id"`
	Score float64   `gorm:"column:score"`
}

func ftsSearch(db *gorm.DB, userID uuid.UUID, terms []string, limit int) ([]rankedNote, error) {
	if len(terms) == 0 {
		return nil, nil
	}
	q := tsqueryText(terms)
	var rows []rankedNote
	err := db.Raw(`
SELECT id,
       ts_rank_cd(to_tsvector('simple', content_text), to_tsquery('simple', ?)) AS score
FROM public.episodic_notes
WHERE user_id = ? AND status = 'shadow'
  AND to_tsvector('simple', content_text) @@ to_tsquery('simple', ?)
ORDER BY score DESC, created_at DESC
LIMIT ?`, q, userID, q, limit).Scan(&rows).Error
	return rows, err
}

func trigramSearch(db *gorm.DB, userID uuid.UUID, terms []string, limit int) ([]rankedNote, error) {
	if len(terms) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(terms))
	args := make([]any, 0, len(terms)+1)
	for i, t := range terms {
		placeholders[i] = "?"
		args = append(args, t)
	}
	args = append(args, userID)
	sql := fmt.Sprintf(`
SELECT n.id, max(word_similarity(t.term, n.content_text)) AS score
FROM public.episodic_notes n
CROSS JOIN unnest(ARRAY[%s]::text[]) AS t(term)
WHERE n.user_id = ? AND n.status = 'shadow'
GROUP BY n.id, n.created_at
ORDER BY score DESC, n.created_at DESC
LIMIT %d`, strings.Join(placeholders, ","), limit)
	var rows []rankedNote
	err := db.Raw(sql, args...).Scan(&rows).Error
	return rows, err
}

// bigramSearch is the evaluation-only control described above: OR overlap over
// character bigrams, ranked by how many bigrams occur in the note.
func bigramSearch(db *gorm.DB, userID uuid.UUID, terms []string, limit int) ([]uuid.UUID, error) {
	tokens := bigrams(terms)
	if len(tokens) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(tokens))
	args := make([]any, 0, len(tokens)+1)
	args = append(args, userID)
	for i, t := range tokens {
		placeholders[i] = "?"
		args = append(args, t)
	}
	sql := fmt.Sprintf(`
SELECT n.id
FROM public.episodic_notes n
WHERE n.user_id = ? AND n.status = 'shadow'
ORDER BY (SELECT count(*) FROM unnest(ARRAY[%s]::text[]) AS g(tok)
          WHERE strpos(n.content_text, g.tok) > 0) DESC,
         n.created_at DESC
LIMIT %d`, strings.Join(placeholders, ","), limit)
	var rows []struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	err := db.Raw(sql, args...).Scan(&rows).Error
	out := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out, err
}

func bigrams(terms []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, term := range terms {
		r := []rune(term)
		for i := 0; i+1 < len(r); i++ {
			b := string(r[i : i+2])
			if seen[b] {
				continue
			}
			seen[b] = true
			out = append(out, b)
		}
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// hybridSearch is the keyword baseline: FTS hits in ts_rank_cd order, then
// trigram-ranked notes that FTS could not produce, deduplicated, capped at 8.
func hybridSearch(db *gorm.DB, userID uuid.UUID, query string) ([]uuid.UUID, []rankedNote, bool, error) {
	terms := extractTerms(query)
	fts, err := ftsSearch(db, userID, terms, recallLimit)
	if err != nil {
		return nil, nil, false, err
	}
	out := make([]uuid.UUID, 0, recallLimit)
	seen := map[uuid.UUID]bool{}
	for _, h := range fts {
		if !seen[h.ID] {
			seen[h.ID] = true
			out = append(out, h.ID)
		}
	}
	zeroOverlap := true
	if len(out) < recallLimit {
		tri, err := trigramSearch(db, userID, terms, recallLimit*4)
		if err != nil {
			return nil, nil, false, err
		}
		for _, h := range tri {
			if h.Score > 0 {
				zeroOverlap = false
			}
			if seen[h.ID] {
				continue
			}
			seen[h.ID] = true
			out = append(out, h.ID)
			if len(out) == recallLimit {
				break
			}
		}
	} else {
		zeroOverlap = false
	}
	if len(out) > recallLimit {
		out = out[:recallLimit]
	}
	return out, fts, zeroOverlap, nil
}

type sliceResult struct {
	Queries      int     `json:"queries"`
	Recall       float64 `json:"recall_at_8"`
	HitRate      float64 `json:"hit_rate_at_8"`
	FTSRecall    float64 `json:"fts_only_recall_at_8"`
	BigramRecall float64 `json:"cjk_bigram_control_recall_at_8"`
}

type recallResult struct {
	Cap                string                 `json:"cap"`
	Queries            int                    `json:"queries"`
	Recall             float64                `json:"recall_at_8"`
	HitRate            float64                `json:"hit_rate_at_8"`
	FTSRecall          float64                `json:"fts_only_recall_at_8"`
	BigramRecall       float64                `json:"cjk_bigram_control_recall_at_8"`
	FTSCovered         int                    `json:"fts_covered_queries"`
	TrigramOnly        int                    `json:"trigram_only_queries"`
	ZeroTrigramOverlap int                    `json:"zero_trigram_overlap_queries"`
	LatencyP50MS       float64                `json:"query_p50_ms"`
	LatencyP95MS       float64                `json:"query_p95_ms"`
	BySlice            map[string]sliceResult `json:"by_slice"`
	UnmatchedQuery     []string               `json:"unmatched_query_ids,omitempty"`
}

func runRecall(db *gorm.DB, c *corpus, fx *queryFixture, capName string) (recallResult, error) {
	res := recallResult{Cap: capName, BySlice: map[string]sliceResult{}}
	sums := map[string]float64{}
	hits := map[string]int{}
	ftsSums := map[string]float64{}
	bigramSums := map[string]float64{}
	counts := map[string]int{}
	var latencies []time.Duration

	for _, q := range fx.Queries {
		if q.UserIndex < 0 || q.UserIndex >= len(c.Users) {
			return res, fmt.Errorf("query %s references unknown user %d", q.ID, q.UserIndex)
		}
		userID := c.Users[q.UserIndex].ID
		started := time.Now()
		got, ftsRows, zeroOverlap, err := hybridSearch(db, userID, q.Query)
		if err != nil {
			return res, fmt.Errorf("query %s: %w", q.ID, err)
		}
		bigramIDs, err := bigramSearch(db, userID, extractTerms(q.Query), recallLimit)
		if err != nil {
			return res, fmt.Errorf("query %s bigram control: %w", q.ID, err)
		}
		latencies = append(latencies, time.Since(started))

		expected := map[uuid.UUID]bool{}
		for _, id := range q.ExpectedNoteIDs {
			u, err := uuid.Parse(id)
			if err != nil {
				return res, fmt.Errorf("query %s: bad expected id %q: %w", q.ID, id, err)
			}
			expected[u] = true
		}
		found := 0
		for _, id := range got {
			if expected[id] {
				found++
			}
		}
		bigramFound := 0
		for _, id := range bigramIDs {
			if expected[id] {
				bigramFound++
			}
		}
		recall := float64(found) / float64(len(expected))
		hit := 0.0
		if found > 0 {
			hit = 1.0
		}
		counts[q.Slice]++
		sums[q.Slice] += recall
		hits[q.Slice] += int(hit)
		bigramSums[q.Slice] += float64(bigramFound) / float64(len(expected))

		// FTS-only recall: how much the stock text-search path found by itself.
		ftsHits := 0
		if len(ftsRows) > 0 {
			for _, h := range ftsRows {
				if expected[h.ID] {
					ftsHits++
				}
			}
			res.FTSCovered++
		} else {
			res.TrigramOnly++
		}
		if zeroOverlap {
			res.ZeroTrigramOverlap++
		}
		ftsSums[q.Slice] += float64(ftsHits) / float64(len(expected))
		if found == 0 {
			res.UnmatchedQuery = append(res.UnmatchedQuery, q.ID)
		}
		res.Queries++
	}

	var recallTotal, hitTotal, ftsTotal, bigramTotal float64
	for slice, n := range counts {
		sr := sliceResult{
			Queries:      n,
			Recall:       round(sums[slice] / float64(n)),
			HitRate:      round(float64(hits[slice]) / float64(n)),
			FTSRecall:    round(ftsSums[slice] / float64(n)),
			BigramRecall: round(bigramSums[slice] / float64(n)),
		}
		res.BySlice[slice] = sr
		recallTotal += sums[slice]
		hitTotal += float64(hits[slice])
		ftsTotal += ftsSums[slice]
		bigramTotal += bigramSums[slice]
	}
	if res.Queries > 0 {
		res.Recall = round(recallTotal / float64(res.Queries))
		res.HitRate = round(hitTotal / float64(res.Queries))
		res.FTSRecall = round(ftsTotal / float64(res.Queries))
		res.BigramRecall = round(bigramTotal / float64(res.Queries))
	}
	res.LatencyP50MS = ms(percentile(latencies, 0.50))
	res.LatencyP95MS = ms(percentile(latencies, 0.95))
	return res, nil
}

func round(f float64) float64 {
	return float64(int(f*10000+0.5)) / 10000
}
