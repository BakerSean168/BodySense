package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// benchMessage is a minimal projection of public.messages. Only synthetic rows
// are written; the harness deletes them by synthetic user e-mail afterwards.
type benchMessage struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null"`
	TurnID         uuid.UUID `gorm:"type:uuid;not null"`
	Role           string    `gorm:"size:20;not null"`
	Seq            int       `gorm:"not null"`
	ContentText    string    `gorm:"type:text"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

func (benchMessage) TableName() string { return "messages" }

type benchUser struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email     string    `gorm:"size:255;not null"`
	Role      string    `gorm:"size:30;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func (benchUser) TableName() string { return "users" }

type benchConversation struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID              uuid.UUID `gorm:"type:uuid;not null"`
	Status              string    `gorm:"size:20;not null"`
	ProcessedThroughSeq int       `gorm:"not null"`
	ContextStartSeq     int       `gorm:"not null"`
	PrunedThroughSeq    int       `gorm:"not null"`
	CreatedAt           time.Time `gorm:"not null"`
	UpdatedAt           time.Time `gorm:"not null"`
}

func (benchConversation) TableName() string { return "conversations" }

type modeResult struct {
	Mode              string   `json:"mode"`
	Cap               string   `json:"cap"`
	PrunePasses       int      `json:"prune_passes"`
	PruneP50MS        float64  `json:"prune_p50_ms"`
	PruneP95MS        float64  `json:"prune_p95_ms"`
	PruneMaxMS        float64  `json:"prune_max_ms"`
	NotePasses        int      `json:"prune_passes_with_note"`
	NotePruneP95MS    float64  `json:"prune_with_note_p95_ms"`
	CreatedNotes      int      `json:"notes_created"`
	RolledOutNotes    int      `json:"notes_rolled_out"`
	FinalNotes        int      `json:"final_notes"`
	FinalRunes        int      `json:"final_characters"`
	PerUserNotes      []int    `json:"per_user_notes"`
	PerUserCharacters []int    `json:"per_user_characters"`
	PerUserBinding    []string `json:"per_user_binding"`
}

func openDB(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
}

func dbVersion(db *gorm.DB) (string, error) {
	var v string
	err := db.Raw("select version()").Scan(&v).Error
	return v, err
}

func resetSyntheticData(db *gorm.DB) error {
	return db.Exec("DELETE FROM users WHERE email LIKE 'episodic-bench-%@example.invalid'").Error
}

func seedCorpus(db *gorm.DB, c *corpus) error {
	now := c.StartDate
	for _, u := range c.Users {
		user := benchUser{
			ID: u.ID, Email: fmt.Sprintf("episodic-bench-u%d@example.invalid", u.Index),
			Role: "member", CreatedAt: now,
		}
		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("seed user %d: %w", u.Index, err)
		}
		maxSeq := 0
		if len(u.Segments) > 0 {
			maxSeq = u.Segments[len(u.Segments)-1].SeqTo
		}
		conv := benchConversation{
			ID: u.ConversationID, UserID: u.ID, Status: "active",
			ProcessedThroughSeq: maxSeq, ContextStartSeq: maxSeq + 1, PrunedThroughSeq: 0,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&conv).Error; err != nil {
			return fmt.Errorf("seed conversation %d: %w", u.Index, err)
		}
		var msgs []benchMessage
		for _, s := range u.Segments {
			for _, m := range s.Messages {
				msgs = append(msgs, benchMessage{
					ID: m.ID, ConversationID: u.ConversationID, TurnID: m.TurnID,
					Role: m.Role, Seq: m.Seq, ContentText: m.Text, CreatedAt: m.At, UpdatedAt: m.At,
				})
			}
		}
		if err := db.CreateInBatches(msgs, 500).Error; err != nil {
			return fmt.Errorf("seed messages %d: %w", u.Index, err)
		}
	}
	return nil
}

func applyNote(tx *gorm.DB, seg *segment, cap retentionCap) (created bool, rolled int, err error) {
	if !seg.Eligible || seg.NoteText == "" {
		return false, 0, nil
	}
	var old []model.EpisodicNote
	if err := tx.Where("user_id = ?", seg.User.ID).Order("created_at ASC").Order("id ASC").Find(&old).Error; err != nil {
		return false, 0, err
	}
	total := utf8.RuneCountInString(seg.NoteText)
	for _, n := range old {
		total += utf8.RuneCountInString(n.ContentText)
	}
	for len(old)+1 > cap.Count || total > cap.Runes {
		if len(old) == 0 {
			return false, 0, nil
		}
		first := old[0]
		if err := tx.Where("user_id = ? AND id = ?", seg.User.ID, first.ID).Delete(&model.EpisodicNote{}).Error; err != nil {
			return false, 0, err
		}
		total -= utf8.RuneCountInString(first.ContentText)
		old = old[1:]
		rolled++
	}
	note := model.EpisodicNote{
		ID: seg.NoteID, UserID: seg.User.ID, ConversationID: seg.User.ConversationID,
		FromSeq: seg.SeqFrom, ToSeq: seg.SeqTo,
		SourceMessageIDs:       messageIDs(seg),
		SourceAssetIDs:         pq.StringArray{},
		GeneratorVersion:       "episodic-bench-stub-v1",
		RetentionPolicyVersion: "episodic-bench-" + cap.Name,
		ContentText:            seg.NoteText, Status: "shadow",
		CreatedAt: seg.At, UpdatedAt: seg.At,
	}
	if err := tx.Create(&note).Error; err != nil {
		return false, 0, err
	}
	return true, rolled, nil
}

func messageIDs(seg *segment) pq.StringArray {
	ids := make(pq.StringArray, 0, len(seg.Messages))
	for _, m := range seg.Messages {
		ids = append(ids, m.ID.String())
	}
	return ids
}

// runMode replays the corpus week by week. Each weekly pass is one prune
// transaction (delete the raw messages of that week, advance cursors) and,
// when extraction is enabled, best-effort note generation in the same pass.
// The extraction stub builds text deterministically; no model is called.
func runMode(db *gorm.DB, c *corpus, cap retentionCap, extract bool) (modeResult, error) {
	mode := "prune_only"
	if extract {
		mode = "prune_with_extraction_stub"
	}
	res := modeResult{Mode: mode, Cap: cap.Name}
	var prune []time.Duration
	var notePrune []time.Duration
	perUserNotes := make([]int, len(c.Users))
	perUserChars := make([]int, len(c.Users))
	perUserBinding := make([]string, len(c.Users))

	for week := 0; week < c.Weeks; week++ {
		for _, u := range c.Users {
			var segs []*segment
			for _, s := range u.Segments {
				if s.Week == week {
					segs = append(segs, s)
				}
			}
			if len(segs) == 0 {
				continue
			}
			boundary := segs[len(segs)-1].SeqTo
			started := time.Now()
			err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("DELETE FROM messages WHERE conversation_id = ? AND seq <= ?", u.ConversationID, boundary).Error; err != nil {
					return err
				}
				if err := tx.Exec("UPDATE conversations SET pruned_through_seq = ?, context_start_seq = ?, retention_revision = retention_revision + 1 WHERE id = ?",
					boundary, boundary+1, u.ConversationID).Error; err != nil {
					return err
				}
				if extract {
					for _, s := range segs {
						created, rolled, err := applyNote(tx, s, cap)
						if err != nil {
							return err
						}
						if created {
							res.CreatedNotes++
						}
						res.RolledOutNotes += rolled
					}
				}
				return nil
			})
			if err != nil {
				return res, fmt.Errorf("week %d user %d: %w", week, u.Index, err)
			}
			d := time.Since(started)
			prune = append(prune, d)
			if extract && hasEligible(segs) {
				notePrune = append(notePrune, d)
			}
		}
	}

	res.PrunePasses = len(prune)
	res.PruneP50MS = ms(percentile(prune, 0.50))
	res.PruneP95MS = ms(percentile(prune, 0.95))
	res.PruneMaxMS = ms(percentile(prune, 1.0))
	res.NotePasses = len(notePrune)
	res.NotePruneP95MS = ms(percentile(notePrune, 0.95))

	for _, u := range c.Users {
		var notes []model.EpisodicNote
		if err := db.Where("user_id = ?", u.ID).Order("created_at ASC").Find(&notes).Error; err != nil {
			return res, err
		}
		chars := 0
		for _, n := range notes {
			chars += utf8.RuneCountInString(n.ContentText)
		}
		perUserNotes[u.Index] = len(notes)
		perUserChars[u.Index] = chars
		perUserBinding[u.Index] = bindingFor(len(notes), chars, cap)
		res.FinalNotes += len(notes)
		res.FinalRunes += chars
	}
	res.PerUserNotes = perUserNotes
	res.PerUserCharacters = perUserChars
	res.PerUserBinding = perUserBinding
	return res, nil
}

func hasEligible(segs []*segment) bool {
	for _, s := range segs {
		if s.Eligible {
			return true
		}
	}
	return false
}

func bindingFor(notes, chars int, cap retentionCap) string {
	if notes >= cap.Count {
		return "count"
	}
	if chars >= cap.Runes {
		return "characters"
	}
	return "none"
}

func percentile(d []time.Duration, p float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	idx := int(math.Ceil(p*float64(len(s)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
}

func ms(d time.Duration) float64 {
	return math.Round(float64(d.Microseconds())/1000*1000) / 1000
}

// writeIndexes creates the retrieval indexes used only by this disposable
// benchmark database. They are NOT migrations and are not part of the runtime
// schema; the report states this explicitly.
func writeIndexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS pg_trgm`,
		`CREATE INDEX IF NOT EXISTS episodic_notes_bench_trgm_idx ON public.episodic_notes USING gin (content_text gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS episodic_notes_bench_fts_idx ON public.episodic_notes USING gin (to_tsvector('simple', content_text))`,
		`CREATE INDEX IF NOT EXISTS messages_bench_conversation_seq_idx ON public.messages (conversation_id, seq)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return nil
}

func requireSchema(db *gorm.DB) error {
	var n int64
	if err := db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='episodic_notes'").Scan(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("episodic_notes is missing: apply apps/api/migrations to a disposable PostgreSQL 18 database first")
	}
	return nil
}

func gitCommit() string {
	if v := os.Getenv("GIT_COMMIT"); v != "" {
		return v
	}
	return "unknown"
}
