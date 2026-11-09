package main

import (
	"path/filepath"
	"testing"
)

// Gate for #517 S4 serving: >=80% correct non-clinical episodic recall on a
// multi-week synthetic set, zero ghost memories after a user deletion, and a
// bounded added latency. All fixture content is fabricated.
func TestS4SyntheticMultiweekRecall(t *testing.T) {
	fx, err := loadS4Fixture(filepath.Join("fixtures", "s4_multiweek.json"))
	if err != nil {
		t.Fatal(err)
	}

	known := map[string]bool{}
	for _, note := range fx.Notes {
		known[note.ID] = true
	}
	for _, q := range fx.Queries {
		for _, id := range q.ExpectedNoteIDs {
			if !known[id] {
				t.Fatalf("query %s expects unknown note %s", q.ID, id)
			}
		}
		if q.Week < 1 || q.Week > fx.Weeks {
			t.Fatalf("query %s week %d outside fixture", q.ID, q.Week)
		}
	}

	report := runS4Eval(fx)
	t.Logf("S4 synthetic recall: recall@%d=%.4f hit=%.4f ghosts=%d control_ghosts=%d p50=%.3fms p95=%.3fms",
		report.K, report.RecallAtK, report.HitRateAtK, report.GhostHits,
		report.NaiveControlGhostHits, report.LatencyP50MS, report.LatencyP95MS)

	if report.RecallAtK < 0.80 {
		t.Fatalf("recall@%d = %.4f, want >= 0.80 (missed: %v)", report.K, report.RecallAtK, report.MissedQueries)
	}
	if report.GhostHits != 0 {
		t.Fatalf("ghost hits after user deletion = %d, want 0", report.GhostHits)
	}
	if report.NaiveControlGhostHits == 0 {
		t.Fatalf("fixture does not exercise a deleted-note ghost risk; control hits = 0")
	}
	if report.LatencyP95MS > 50 {
		t.Fatalf("in-process recall p95 = %.3fms, want <= 50ms", report.LatencyP95MS)
	}
	if report.RecallProbes < 15 {
		t.Fatalf("only %d recall probes; fixture is too small", report.RecallProbes)
	}
}
