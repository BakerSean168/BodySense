package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// The query fixture is the human-reviewable annotation set. Every entry names
// the exact note id it must recall, the slice it belongs to and why the query
// and the note are the same subject. It is generated deterministically so a
// reviewer can regenerate and diff it.

const (
	sliceOriginal   = "original_term"
	sliceParaphrase = "paraphrase"
	sliceTemporal   = "temporal_reference"
	fixtureVersion  = 1
)

type fixtureQuery struct {
	ID               string   `json:"id"`
	Slice            string   `json:"slice"`
	UserIndex        int      `json:"user_index"`
	Anchor           string   `json:"anchor"`
	Query            string   `json:"query"`
	ExpectedNoteIDs  []string `json:"expected_note_ids"`
	ExpectedNoteDate string   `json:"expected_note_date"`
	Rationale        string   `json:"rationale"`
}

type queryFixture struct {
	Version      int            `json:"version"`
	Corpus       string         `json:"corpus"`
	Seed         uint64         `json:"seed"`
	Users        int            `json:"users"`
	Weeks        int            `json:"weeks"`
	Segments     int            `json:"segments"`
	Eligible     int            `json:"note_eligible_segments"`
	RetentionCap string         `json:"fixture_retention_cap"`
	Note         string         `json:"note"`
	Queries      []fixtureQuery `json:"queries"`
}

func buildQueries(c *corpus) []fixtureQuery {
	var out []fixtureQuery
	n := 0
	for ai := range anchors {
		a := anchors[ai]
		seg := c.segmentForAnchor(a.ID)
		if seg == nil {
			continue
		}
		date := seg.At.Format("2006-01-02")
		phrase := temporalPhrase(c.Now, seg.At, ai)
		mk := func(slice, q, why string) {
			n++
			out = append(out, fixtureQuery{
				ID:               fmt.Sprintf("q-%03d", n),
				Slice:            slice,
				UserIndex:        seg.User.Index,
				Anchor:           a.ID,
				Query:            q,
				ExpectedNoteIDs:  []string{seg.NoteID.String()},
				ExpectedNoteDate: date,
				Rationale:        why,
			})
		}
		mk(sliceOriginal, fmt.Sprintf("你还记得我之前说过的%s吗？", a.Term),
			fmt.Sprintf("原词命中：查询直接包含笔记锚点「%s」。", a.Term))
		mk(sliceOriginal, fmt.Sprintf("之前聊到%s的时候，我说了什么？", a.Term),
			fmt.Sprintf("原词命中：查询直接包含笔记锚点「%s」。", a.Term))
		mk(sliceParaphrase, fmt.Sprintf("上次提到%s，你还记得吗？", a.Alias),
			fmt.Sprintf("改写同义：用「%s」替换原词「%s」，二者指同一件事。", a.Alias, a.Term))
		mk(sliceParaphrase, fmt.Sprintf("我之前是不是讲过%s的事？", a.Alias),
			fmt.Sprintf("改写同义：用「%s」替换原词「%s」。", a.Alias, a.Term))
		mk(sliceTemporal, fmt.Sprintf("%s我说过%s的事，帮我回忆一下。", phrase, a.Alias),
			fmt.Sprintf("时间指代：用「%s」指代 %s 的笔记，内容用「%s」。", phrase, date, a.Alias))
		mk(sliceTemporal, fmt.Sprintf("你还记得%s我提到的%s吗？", phrase, a.Alias),
			fmt.Sprintf("时间指代：用「%s」指代 %s 的笔记。", phrase, date))
	}
	// 20 additional variants keep each slice above 100 while never pointing at
	// a note outside the retained suffix.
	for i := 0; i < 20 && i < len(anchors); i++ {
		a := anchors[i]
		seg := c.segmentForAnchor(a.ID)
		if seg == nil {
			continue
		}
		n++
		q := fixtureQuery{
			ID:               fmt.Sprintf("q-%03d", n),
			UserIndex:        seg.User.Index,
			Anchor:           a.ID,
			ExpectedNoteIDs:  []string{seg.NoteID.String()},
			ExpectedNoteDate: seg.At.Format("2006-01-02"),
		}
		if i%2 == 0 {
			q.Slice = sliceParaphrase
			q.Query = fmt.Sprintf("我说的%s那件事，你还有印象吗？", a.Alias)
			q.Rationale = fmt.Sprintf("改写同义：用「%s」替换原词「%s」。", a.Alias, a.Term)
		} else {
			q.Slice = sliceTemporal
			phrase := temporalPhrase(c.Now, seg.At, i)
			q.Query = fmt.Sprintf("%s聊到%s，你还记得细节吗？", phrase, a.Alias)
			q.Rationale = fmt.Sprintf("时间指代：用「%s」指代 %s 的笔记。", phrase, seg.At.Format("2006-01-02"))
		}
		out = append(out, q)
	}
	return out
}

func (c *corpus) segmentForAnchor(id string) *segment {
	for _, s := range c.Segments {
		if s.Anchor != nil && s.Anchor.ID == id {
			return s
		}
	}
	return nil
}

// temporalPhrase keeps the colloquial time reference consistent with the
// note date, so a reviewer can check it against expected_note_date.
func temporalPhrase(now, at time.Time, pick int) string {
	days := int(now.Sub(at).Hours() / 24)
	switch {
	case days <= 45:
		return []string{"前阵子", "上个月", "这段时间"}[(pick+days)%3]
	case days <= 100:
		return []string{"两三个月前", "前几个月", "三个月前"}[(pick+days)%3]
	case days <= 180:
		return []string{"四个多月前", "前一阵", "年初"}[(pick+days)%3]
	case days <= 270:
		return []string{"半年前", "半年多前"}[(pick+days)%2]
	default:
		return []string{"去年", "去年这个时候", "去年冬天"}[(pick+days)%3]
	}
}

func writeFixture(c *corpus, path string) error {
	queries := buildQueries(c)
	if len(queries) < 300 {
		return fmt.Errorf("query fixture has %d queries, need >= 300", len(queries))
	}
	retained := c.simulateRollout(retentionCaps[0])
	guard := map[string]bool{}
	for _, u := range retained.PerUser {
		for _, id := range u.RetainedID {
			guard[id] = true
		}
	}
	for _, q := range queries {
		for _, id := range q.ExpectedNoteIDs {
			if !guard[id] {
				return fmt.Errorf("query %s targets a note rolled out at cap %d/%d: %s", q.ID, retentionCaps[0].Count, retentionCaps[0].Runes, id)
			}
		}
	}
	sort.SliceStable(queries, func(i, j int) bool { return queries[i].ID < queries[j].ID })
	fx := queryFixture{
		Version:      fixtureVersion,
		Corpus:       c.Version,
		Seed:         c.Seed,
		Users:        len(c.Users),
		Weeks:        c.Weeks,
		Segments:     len(c.Segments),
		Eligible:     countEligible(c),
		RetentionCap: "48 notes / 16000 characters (historical S2-A test-only baseline)",
		Note:         "Every expected note is retained at the historical test-only baseline, so the same queries are answerable at every measured cap. All users and content are synthetic.",
		Queries:      queries,
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func loadFixture(path string) (*queryFixture, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fx queryFixture
	if err := json.Unmarshal(body, &fx); err != nil {
		return nil, err
	}
	if fx.Version != fixtureVersion {
		return nil, fmt.Errorf("unsupported fixture version %d", fx.Version)
	}
	if len(fx.Queries) < 300 {
		return nil, fmt.Errorf("query fixture has %d queries, need >= 300", len(fx.Queries))
	}
	return &fx, nil
}

func countEligible(c *corpus) int {
	n := 0
	for _, s := range c.Segments {
		if s.Eligible {
			n++
		}
	}
	return n
}
