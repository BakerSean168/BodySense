package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
)

type corpusSummary struct {
	Version           string  `json:"version"`
	Seed              uint64  `json:"seed"`
	Users             int     `json:"users"`
	Weeks             int     `json:"weeks"`
	Segments          int     `json:"segments"`
	EligibleNotes     int     `json:"note_eligible_segments"`
	ClinicalGaps      int     `json:"clinical_generation_gaps"`
	Messages          int     `json:"messages"`
	AvgNoteCharacters float64 `json:"average_note_characters"`
}

type capReport struct {
	Cap        string        `json:"cap"`
	CountLimit int           `json:"count_limit"`
	CharLimit  int           `json:"character_limit"`
	PolicyNote string        `json:"policy_note"`
	Modes      []modeResult  `json:"modes"`
	Recall     *recallResult `json:"recall"`
	Simulated  rolloutStat   `json:"simulated_rollout"`
}

type report struct {
	GeneratedAt    string        `json:"generated_at"`
	GitCommit      string        `json:"git_commit"`
	Postgres       string        `json:"postgres_version"`
	Corpus         corpusSummary `json:"corpus"`
	Caps           []capReport   `json:"caps"`
	RetrievalNotes []string      `json:"retrieval_notes"`
}

func main() {
	dsn := flag.String("dsn", os.Getenv("BODYSENSE_EPISODIC_BENCH_DSN"), "disposable PostgreSQL 18 DSN (synthetic data only)")
	seed := flag.Uint64("seed", defaultSeed, "deterministic corpus seed")
	users := flag.Int("users", defaultUsers, "synthetic user count")
	weeks := flag.Int("weeks", defaultWeeks, "simulated weeks (52 = 12 months)")
	fixturePath := flag.String("fixtures", filepath.Join("benchmarks", "episodic", "fixtures", "queries.json"), "query fixture path")
	outPath := flag.String("out", filepath.Join("benchmarks", "episodic", "results", "benchmark.json"), "result JSON path")
	writeFix := flag.Bool("write-fixtures", false, "regenerate the committed query fixture and exit (no database needed)")
	s4Eval := flag.Bool("s4-eval", false, "score the synthetic S4 recall fixture and exit (no database needed)")
	s4Path := flag.String("s4-fixtures", filepath.Join("benchmarks", "episodic", "fixtures", "s4_multiweek.json"), "S4 recall fixture path")
	flag.Parse()

	if *s4Eval {
		fx, err := loadS4Fixture(*s4Path)
		if err != nil {
			fail(fmt.Errorf("load S4 fixture: %w", err))
		}
		report := runS4Eval(fx)
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fail(err)
		}
		fmt.Println(string(raw))
		return
	}

	c := buildCorpus(*seed, *users, *weeks)
	if *writeFix {
		if err := writeFixture(c, *fixturePath); err != nil {
			fail(err)
		}
		fmt.Printf("FIXTURE_WRITTEN path=%s queries=%d anchors=%d\n", *fixturePath, len(buildQueries(c)), len(anchors))
		return
	}
	if *dsn == "" {
		fail(fmt.Errorf("no DSN: pass -dsn or BODYSENSE_EPISODIC_BENCH_DSN (disposable PG18 only)"))
	}
	fx, err := loadFixture(*fixturePath)
	if err != nil {
		fail(fmt.Errorf("load fixture: %w (run with -write-fixtures once)", err))
	}

	db, err := openDB(*dsn)
	if err != nil {
		fail(err)
	}
	if err := requireSchema(db); err != nil {
		fail(err)
	}
	if err := writeIndexes(db); err != nil {
		fail(err)
	}
	pgVersion, err := dbVersion(db)
	if err != nil {
		fail(err)
	}

	rep := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		GitCommit:   gitCommit(),
		Postgres:    pgVersion,
		Corpus:      summarize(c),
		RetrievalNotes: []string{
			"Retrieval exists only in this offline harness; no API, service or runtime path was added.",
			"Candidate generation is OR full-text search over episodic_notes with ts_rank_cd, filled by a pg_trgm word_similarity fallback.",
			"Stock PostgreSQL has no Chinese word segmenter: to_tsvector('simple') keeps a whole Chinese sentence as one lexeme.",
			"pg_trgm and FTS indexes are created by this harness in the disposable benchmark database only; they are not migrations.",
		},
	}

	for _, cap := range retentionCaps {
		cr := capReport{
			Cap: cap.Name, CountLimit: cap.Count, CharLimit: cap.Runes, PolicyNote: cap.Source,
			Simulated: c.simulateRollout(cap),
		}
		for _, extract := range []bool{false, true} {
			if err := resetSyntheticData(db); err != nil {
				fail(err)
			}
			if err := seedCorpus(db, c); err != nil {
				fail(err)
			}
			started := time.Now()
			res, err := runMode(db, c, cap, extract)
			if err != nil {
				fail(err)
			}
			fmt.Printf("cap=%-11s mode=%-27s elapsed=%s passes=%d p95=%.3fms notes=%d chars=%d rolled=%d\n",
				cap.Name, res.Mode, time.Since(started).Round(time.Millisecond), res.PrunePasses,
				res.PruneP95MS, res.FinalNotes, res.FinalRunes, res.RolledOutNotes)
			cr.Modes = append(cr.Modes, res)
			if extract {
				if impl := cr.Simulated.Retained; impl != res.FinalNotes {
					fail(fmt.Errorf("cap %s: postgres retained %d notes but in-memory simulation expects %d", cap.Name, res.FinalNotes, impl))
				}
				rec, err := runRecall(db, c, fx, cap.Name)
				if err != nil {
					fail(err)
				}
				cr.Recall = &rec
				fmt.Printf("recall cap=%-11s queries=%d recall@8=%.4f hit=%.4f paraphrase=%.4f temporal=%.4f fts_only=%.4f p95=%.3fms\n",
					cap.Name, rec.Queries, rec.Recall, rec.HitRate,
					rec.BySlice[sliceParaphrase].Recall, rec.BySlice[sliceTemporal].Recall,
					rec.FTSRecall, rec.LatencyP95MS)
				fmt.Printf("recall cap=%-11s bigram_control=%.4f (paraphrase %.4f, temporal %.4f) zero_trigram_overlap=%d/%d\n",
					cap.Name, rec.BigramRecall, rec.BySlice[sliceParaphrase].BigramRecall,
					rec.BySlice[sliceTemporal].BigramRecall, rec.ZeroTrigramOverlap, rec.Queries)
			}
		}
		rep.Caps = append(rep.Caps, cr)
	}

	if err := writeReport(rep, *outPath); err != nil {
		fail(err)
	}
	fmt.Printf("REPORT_WRITTEN path=%s\n", *outPath)
}

func summarize(c *corpus) corpusSummary {
	s := corpusSummary{
		Version: c.Version, Seed: c.Seed, Users: len(c.Users), Weeks: c.Weeks,
		Segments: len(c.Segments), EligibleNotes: countEligible(c),
		ClinicalGaps: len(c.Segments) - countEligible(c),
	}
	totalRunes := 0
	notes := 0
	for _, seg := range c.Segments {
		s.Messages += len(seg.Messages)
		if seg.Eligible {
			totalRunes += len([]rune(seg.NoteText))
			notes++
		}
	}
	if notes > 0 {
		s.AvgNoteCharacters = float64(totalRunes) / float64(notes)
	}
	return s
}

func writeReport(rep report, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "EPISODIC_BENCH=FAIL %v\n", err)
	os.Exit(1)
}
