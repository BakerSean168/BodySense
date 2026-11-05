package main

import (
	"testing"
	"unicode/utf8"
)

func TestCorpusIsDeterministic(t *testing.T) {
	a := buildCorpus(defaultSeed, defaultUsers, defaultWeeks)
	b := buildCorpus(defaultSeed, defaultUsers, defaultWeeks)
	if len(a.Segments) != len(b.Segments) {
		t.Fatalf("segment count differs: %d vs %d", len(a.Segments), len(b.Segments))
	}
	for i := range a.Segments {
		if a.Segments[i].NoteID != b.Segments[i].NoteID || a.Segments[i].NoteText != b.Segments[i].NoteText {
			t.Fatalf("segment %d differs between identical runs", i)
		}
	}
	c := buildCorpus(defaultSeed+1, defaultUsers, defaultWeeks)
	if c.Segments[0].NoteText == a.Segments[0].NoteText {
		t.Fatal("a different seed produced identical note text")
	}
}

func TestCorpusNotesFitSchemaBounds(t *testing.T) {
	c := buildCorpus(defaultSeed, defaultUsers, defaultWeeks)
	notes := 0
	for _, s := range c.Segments {
		if !s.Eligible {
			if s.NoteText != "" {
				t.Fatalf("clinical segment %d unexpectedly carries a note", s.SeqFrom)
			}
			continue
		}
		notes++
		runes := utf8.RuneCountInString(s.NoteText)
		if runes < 1 || runes > 800 {
			t.Fatalf("note for segment %d has %d runes, outside 1..800", s.SeqFrom, runes)
		}
		if s.NoteID.String() == "" || s.SeqTo-s.SeqFrom+1 != len(s.Messages) {
			t.Fatalf("segment %d has inconsistent provenance", s.SeqFrom)
		}
	}
	if notes < 700 {
		t.Fatalf("corpus produced only %d note-eligible segments", notes)
	}
}

func TestFixtureCoversEverySliceAndStaysInsideTestCap(t *testing.T) {
	c := buildCorpus(defaultSeed, defaultUsers, defaultWeeks)
	queries := buildQueries(c)
	if len(queries) < 300 {
		t.Fatalf("only %d queries", len(queries))
	}
	perSlice := map[string]int{}
	for _, q := range queries {
		perSlice[q.Slice]++
		if len(q.ExpectedNoteIDs) == 0 {
			t.Fatalf("query %s has no expected note", q.ID)
		}
	}
	for _, slice := range []string{sliceOriginal, sliceParaphrase, sliceTemporal} {
		if perSlice[slice] < 100 {
			t.Fatalf("slice %s has %d queries, need >= 100", slice, perSlice[slice])
		}
	}
	retained := map[string]bool{}
	stat := c.simulateRollout(retentionCaps[0])
	for _, u := range stat.PerUser {
		for _, id := range u.RetainedID {
			retained[id] = true
		}
	}
	for _, q := range queries {
		for _, id := range q.ExpectedNoteIDs {
			if !retained[id] {
				t.Fatalf("query %s points at a note rolled out at the test-only cap", q.ID)
			}
		}
	}
}
