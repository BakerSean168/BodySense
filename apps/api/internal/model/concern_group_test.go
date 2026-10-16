package model

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func concernFixture() (ConcernGroupSnapshot, map[string]uuid.UUID) {
	id := map[string]uuid.UUID{}
	for _, k := range []string{"h1", "h2", "h3", "f1", "f2", "f3", "f4", "f5", "e1", "e2", "o1", "o2", "x"} {
		id[k] = uuid.New()
	}
	s := ConcernGroupSnapshot{
		UserID: uuid.New(), Revision: 9,
		Hypotheses: []ConcernGroupNode{
			{ID: id["h1"], CreatedRevision: 3, Label: "older guess"},
			{ID: id["h2"], CreatedRevision: 7, Label: "newer guess"},
			{ID: id["h3"], CreatedRevision: 8, Label: "lonely guess"},
		},
		Facts: []ConcernGroupNode{
			{ID: id["f1"], CreatedRevision: 1}, {ID: id["f2"], CreatedRevision: 2},
			{ID: id["f3"], CreatedRevision: 4}, {ID: id["f4"], CreatedRevision: 5},
			{ID: id["f5"], CreatedRevision: 6}, // isolated
		},
		Observations: []ConcernGroupNode{
			{ID: id["o1"], CreatedRevision: 1}, {ID: id["o2"], CreatedRevision: 9}, // o2 isolated
		},
		HypothesisObservationLinks: []ConcernGroupObservationLink{
			{HypothesisID: id["h2"], ObservationID: id["o1"]}, {HypothesisID: id["x"], ObservationID: id["o2"]},
		},
		Episodes: []ConcernGroupNode{
			{ID: id["e1"], CreatedRevision: 2, Label: "knee pain"},
			{ID: id["e2"], CreatedRevision: 5, Label: "headache"},
		},
		// Component A: h1-f1-e1-f2-h2 (bridged through episode e1)
		// Component B: e2-f4 (episode only)  Component C: h3 alone
		// Link to unknown fact x and from unknown hypothesis must be ignored.
		HypothesisFactLinks: []ConcernGroupLink{
			{FromID: id["h1"], FactID: id["f1"]}, {FromID: id["h2"], FactID: id["f2"]},
			{FromID: id["h3"], FactID: id["x"]}, {FromID: id["x"], FactID: id["f3"]},
		},
		EpisodeFactLinks: []ConcernGroupLink{
			{FromID: id["e1"], FactID: id["f1"]}, {FromID: id["e1"], FactID: id["f2"]},
			{FromID: id["e2"], FactID: id["f4"]},
		},
	}
	return s, id
}

func TestDeriveConcernGroupsComponents(t *testing.T) {
	s, id := concernFixture()
	got := DeriveConcernGroups(s)
	if got.UserID != s.UserID || got.CurrentRevision != 9 || len(got.Groups) != 3 {
		t.Fatalf("groups: %#v", got)
	}
	// Order: latest_revision DESC → C(8), A(7), B(5).
	c, a, b := got.Groups[0], got.Groups[1], got.Groups[2]
	if c.LatestRevision != 8 || !reflect.DeepEqual(c.RelatedHypothesisIDs, []uuid.UUID{id["h3"]}) ||
		len(c.RelatedFactIDs) != 0 || len(c.RelatedEpisodeIDs) != 0 || c.DisplayTitle != "lonely guess" {
		t.Fatalf("lonely hypothesis group: %#v", c)
	}
	if a.LatestRevision != 7 || a.DisplayTitle != "newer guess" ||
		!reflect.DeepEqual(a.RelatedHypothesisIDs, []uuid.UUID{id["h1"], id["h2"]}) ||
		!reflect.DeepEqual(a.RelatedFactIDs, []uuid.UUID{id["f1"], id["f2"]}) ||
		!reflect.DeepEqual(a.RelatedEpisodeIDs, []uuid.UUID{id["e1"]}) ||
		!reflect.DeepEqual(a.RelatedObservationIDs, []uuid.UUID{id["o1"]}) {
		t.Fatalf("bridged group: %#v", a)
	}
	// Anchor = earliest anchor-eligible lineage (episode e1 at rev 2), never
	// the earlier fact f1: facts and observations do not anchor.
	if a.ID != ConcernGroupID(id["e1"]) || b.ID != ConcernGroupID(id["e2"]) || c.ID != ConcernGroupID(id["h3"]) {
		t.Fatalf("group ids must be UUIDv5 of their anchor lineage: %#v", got.Groups)
	}
	if a.ID.Version() != 5 {
		t.Fatalf("not uuidv5: %s", a.ID)
	}
	if b.DisplayTitle != "headache" || len(b.RelatedHypothesisIDs) != 0 ||
		!reflect.DeepEqual(b.RelatedEpisodeIDs, []uuid.UUID{id["e2"]}) ||
		!reflect.DeepEqual(b.RelatedFactIDs, []uuid.UUID{id["f4"]}) {
		t.Fatalf("episode-only group: %#v", b)
	}
	for _, g := range []DerivedConcernGroup{b, c} {
		if g.RelatedObservationIDs == nil || len(g.RelatedObservationIDs) != 0 {
			t.Fatalf("observation ids must be empty, non-nil: %#v", g)
		}
	}
	for _, g := range got.Groups {
		for _, o := range g.RelatedObservationIDs {
			if o == id["o2"] {
				t.Fatalf("isolated observation grouped: %#v", g)
			}
		}
		for _, f := range g.RelatedFactIDs {
			if f == id["f5"] || f == id["f3"] {
				t.Fatalf("isolated fact grouped: %#v", g)
			}
		}
	}
}

func TestDeriveConcernGroupsDeterministicUnderShuffle(t *testing.T) {
	s, _ := concernFixture()
	want := DeriveConcernGroups(s)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		c := s
		c.Hypotheses = append([]ConcernGroupNode(nil), s.Hypotheses...)
		c.Facts = append([]ConcernGroupNode(nil), s.Facts...)
		c.Episodes = append([]ConcernGroupNode(nil), s.Episodes...)
		c.Observations = append([]ConcernGroupNode(nil), s.Observations...)
		c.HypothesisObservationLinks = append([]ConcernGroupObservationLink(nil), s.HypothesisObservationLinks...)
		rng.Shuffle(len(c.Observations), func(a, b int) { c.Observations[a], c.Observations[b] = c.Observations[b], c.Observations[a] })
		rng.Shuffle(len(c.HypothesisObservationLinks), func(a, b int) {
			c.HypothesisObservationLinks[a], c.HypothesisObservationLinks[b] = c.HypothesisObservationLinks[b], c.HypothesisObservationLinks[a]
		})
		c.HypothesisFactLinks = append([]ConcernGroupLink(nil), s.HypothesisFactLinks...)
		c.EpisodeFactLinks = append([]ConcernGroupLink(nil), s.EpisodeFactLinks...)
		rng.Shuffle(len(c.Hypotheses), func(a, b int) { c.Hypotheses[a], c.Hypotheses[b] = c.Hypotheses[b], c.Hypotheses[a] })
		rng.Shuffle(len(c.Facts), func(a, b int) { c.Facts[a], c.Facts[b] = c.Facts[b], c.Facts[a] })
		rng.Shuffle(len(c.Episodes), func(a, b int) { c.Episodes[a], c.Episodes[b] = c.Episodes[b], c.Episodes[a] })
		rng.Shuffle(len(c.HypothesisFactLinks), func(a, b int) {
			c.HypothesisFactLinks[a], c.HypothesisFactLinks[b] = c.HypothesisFactLinks[b], c.HypothesisFactLinks[a]
		})
		rng.Shuffle(len(c.EpisodeFactLinks), func(a, b int) {
			c.EpisodeFactLinks[a], c.EpisodeFactLinks[b] = c.EpisodeFactLinks[b], c.EpisodeFactLinks[a]
		})
		if got := DeriveConcernGroups(c); !reflect.DeepEqual(got, want) {
			t.Fatalf("shuffle %d changed projection:\n got %#v\nwant %#v", i, got, want)
		}
	}
}

func TestDeriveConcernGroupsTieBreakAndEmpty(t *testing.T) {
	if got := DeriveConcernGroups(ConcernGroupSnapshot{}); got.Groups == nil || len(got.Groups) != 0 {
		t.Fatalf("empty snapshot must give empty, non-nil groups: %#v", got)
	}
	lo := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	hi := uuid.MustParse("ffffffff-0000-4000-8000-000000000001")
	s := ConcernGroupSnapshot{Revision: 2, Hypotheses: []ConcernGroupNode{
		{ID: hi, CreatedRevision: 2, Label: "b"}, {ID: lo, CreatedRevision: 2, Label: "a"}}}
	got := DeriveConcernGroups(s)
	if len(got.Groups) != 2 {
		t.Fatalf("%#v", got)
	}
	// Same latest revision → ordered by provisional id ascending.
	if bytesLess := got.Groups[0].ID.String() < got.Groups[1].ID.String(); !bytesLess {
		t.Fatalf("tie not broken by id: %s %s", got.Groups[0].ID, got.Groups[1].ID)
	}
	// Merging both hypotheses through one fact: earliest member by (rev,id) is lo.
	f := uuid.New()
	s.Facts = []ConcernGroupNode{{ID: f, CreatedRevision: 2}}
	s.HypothesisFactLinks = []ConcernGroupLink{{FromID: hi, FactID: f}, {FromID: lo, FactID: f}}
	merged := DeriveConcernGroups(s)
	if len(merged.Groups) != 1 || merged.Groups[0].ID != ConcernGroupID(lo) ||
		merged.Groups[0].DisplayTitle != "b" {
		t.Fatalf("merge: %#v", merged)
	}
}
