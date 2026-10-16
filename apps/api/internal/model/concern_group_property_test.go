package model

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// Property-style tests for the DC-032 ConcernGroup identity rule
// (concern-group-anchor-v2). Every property runs over many seeded random
// BodyState snapshots so failures are reproducible by seed.

const concernPropertyRuns = 400

func randomConcernSnapshot(rng *rand.Rand) ConcernGroupSnapshot {
	s := ConcernGroupSnapshot{UserID: uuid.New()}
	var rev int64
	next := func() int64 { rev += 1 + int64(rng.Intn(2)); return rev }
	for i, n := 0, rng.Intn(8); i < n; i++ {
		s.Facts = append(s.Facts, ConcernGroupNode{ID: uuid.New(), CreatedRevision: next()})
	}
	for i, n := 0, rng.Intn(5); i < n; i++ {
		s.Observations = append(s.Observations, ConcernGroupNode{ID: uuid.New(), CreatedRevision: next()})
	}
	for i, n := 0, rng.Intn(4); i < n; i++ {
		s.Episodes = append(s.Episodes, ConcernGroupNode{ID: uuid.New(), CreatedRevision: next(), Label: fmt.Sprintf("episode %d", i)})
	}
	for i, n := 0, rng.Intn(6); i < n; i++ {
		created := next()
		h := ConcernGroupNode{ID: uuid.New(), CreatedRevision: created, Label: fmt.Sprintf("guess %d", i)}
		if rng.Intn(3) == 0 { // the current member of a corrected lineage
			h.AnchorID, h.AnchorRevision = uuid.New(), 1+rng.Int63n(created)
		}
		s.Hypotheses = append(s.Hypotheses, h)
	}
	for _, h := range s.Hypotheses {
		for _, f := range s.Facts {
			if rng.Intn(4) == 0 {
				s.HypothesisFactLinks = append(s.HypothesisFactLinks, ConcernGroupLink{FromID: h.ID, FactID: f.ID})
			}
		}
		for _, o := range s.Observations {
			if rng.Intn(3) == 0 {
				s.HypothesisObservationLinks = append(s.HypothesisObservationLinks, ConcernGroupObservationLink{HypothesisID: h.ID, ObservationID: o.ID})
			}
		}
	}
	for _, e := range s.Episodes {
		for _, f := range s.Facts {
			if rng.Intn(4) == 0 {
				s.EpisodeFactLinks = append(s.EpisodeFactLinks, ConcernGroupLink{FromID: e.ID, FactID: f.ID})
			}
		}
	}
	s.Revision = rev
	return s
}

func cloneConcernSnapshot(s ConcernGroupSnapshot) ConcernGroupSnapshot {
	c := s
	c.Hypotheses = append([]ConcernGroupNode(nil), s.Hypotheses...)
	c.Facts = append([]ConcernGroupNode(nil), s.Facts...)
	c.Observations = append([]ConcernGroupNode(nil), s.Observations...)
	c.Episodes = append([]ConcernGroupNode(nil), s.Episodes...)
	c.HypothesisFactLinks = append([]ConcernGroupLink(nil), s.HypothesisFactLinks...)
	c.HypothesisObservationLinks = append([]ConcernGroupObservationLink(nil), s.HypothesisObservationLinks...)
	c.EpisodeFactLinks = append([]ConcernGroupLink(nil), s.EpisodeFactLinks...)
	return c
}

func groupHolding(list *DerivedConcernGroupList, member uuid.UUID) *DerivedConcernGroup {
	for i := range list.Groups {
		g := &list.Groups[i]
		for _, ids := range [][]uuid.UUID{g.RelatedHypothesisIDs, g.RelatedEpisodeIDs, g.RelatedFactIDs, g.RelatedObservationIDs} {
			for _, id := range ids {
				if id == member {
					return g
				}
			}
		}
	}
	return nil
}

// anchorMember returns the anchor-eligible member (hypothesis or episode) of g
// under the documented ordering (lineage start revision, lineage id).
func anchorMember(s ConcernGroupSnapshot, g DerivedConcernGroup) ConcernGroupNode {
	byID := map[uuid.UUID]ConcernGroupNode{}
	for _, n := range append(append([]ConcernGroupNode(nil), s.Hypotheses...), s.Episodes...) {
		byID[n.ID] = n
	}
	var best *ConcernGroupNode
	for _, id := range append(append([]uuid.UUID(nil), g.RelatedHypothesisIDs...), g.RelatedEpisodeIDs...) {
		n := byID[id]
		if best == nil || concernAnchorLess(n, *best) {
			best = &n
		}
	}
	return *best
}

func groupIDSet(list *DerivedConcernGroupList) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, g := range list.Groups {
		out[g.ID] = true
	}
	return out
}

func anchorEligible(g DerivedConcernGroup) []uuid.UUID {
	return append(append([]uuid.UUID(nil), g.RelatedHypothesisIDs...), g.RelatedEpisodeIDs...)
}

func eachConcernSeed(t *testing.T, f func(t *testing.T, rng *rand.Rand, s ConcernGroupSnapshot)) {
	t.Helper()
	for seed := int64(1); seed <= concernPropertyRuns; seed++ {
		rng := rand.New(rand.NewSource(seed))
		s := randomConcernSnapshot(rng)
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) { f(t, rng, s) })
	}
}

// Ids are unique per revision, v5, and equal ConcernGroupID(anchor lineage).
func TestConcernGroupIDPropertyUniqueAnchored(t *testing.T) {
	eachConcernSeed(t, func(t *testing.T, _ *rand.Rand, s ConcernGroupSnapshot) {
		got := DeriveConcernGroups(s)
		if len(groupIDSet(got)) != len(got.Groups) {
			t.Fatalf("duplicate group ids: %#v", got.Groups)
		}
		for _, g := range got.Groups {
			lineage, _ := anchorMember(s, g).anchor()
			if g.ID.Version() != 5 || g.ID != ConcernGroupID(lineage) {
				t.Fatalf("group id is not the anchor lineage id: %#v", g)
			}
			if len(g.RelatedHypothesisIDs)+len(g.RelatedEpisodeIDs) == 0 {
				t.Fatalf("group without hypothesis or episode: %#v", g)
			}
		}
	})
}

// Linking new facts/observations into a group and removing leaf
// facts/observations (single link) never changes any group id.
func TestConcernGroupIDPropertyStableUnderEvidenceChurn(t *testing.T) {
	eachConcernSeed(t, func(t *testing.T, rng *rand.Rand, s ConcernGroupSnapshot) {
		before := DeriveConcernGroups(s)
		c := cloneConcernSnapshot(s)
		rev := s.Revision
		for _, g := range before.Groups {
			rev++
			f := ConcernGroupNode{ID: uuid.New(), CreatedRevision: rev}
			c.Facts = append(c.Facts, f)
			if len(g.RelatedHypothesisIDs) > 0 && (len(g.RelatedEpisodeIDs) == 0 || rng.Intn(2) == 0) {
				h := g.RelatedHypothesisIDs[rng.Intn(len(g.RelatedHypothesisIDs))]
				c.HypothesisFactLinks = append(c.HypothesisFactLinks, ConcernGroupLink{FromID: h, FactID: f.ID})
				o := ConcernGroupNode{ID: uuid.New(), CreatedRevision: rev}
				c.Observations = append(c.Observations, o)
				c.HypothesisObservationLinks = append(c.HypothesisObservationLinks, ConcernGroupObservationLink{HypothesisID: h, ObservationID: o.ID})
			} else {
				e := g.RelatedEpisodeIDs[rng.Intn(len(g.RelatedEpisodeIDs))]
				c.EpisodeFactLinks = append(c.EpisodeFactLinks, ConcernGroupLink{FromID: e, FactID: f.ID})
			}
		}
		// Remove every observation linked exactly once (a leaf): withdrawn,
		// corrected or unlinked observations cannot split a group.
		degree := map[uuid.UUID]int{}
		for _, l := range c.HypothesisObservationLinks {
			degree[l.ObservationID]++
		}
		var kept []ConcernGroupObservationLink
		for _, l := range c.HypothesisObservationLinks {
			if degree[l.ObservationID] != 1 {
				kept = append(kept, l)
			}
		}
		c.HypothesisObservationLinks = kept
		c.Revision = rev
		after := DeriveConcernGroups(c)
		if !reflect.DeepEqual(groupIDSet(before), groupIDSet(after)) {
			t.Fatalf("evidence churn changed ids:\nbefore %#v\nafter  %#v", before.Groups, after.Groups)
		}
		for _, g := range before.Groups {
			if ng := groupHolding(after, anchorEligible(g)[0]); ng == nil || ng.ID != g.ID {
				t.Fatalf("member moved to a different id: %#v -> %#v", g, ng)
			}
		}
	})
}

// Correcting any hypothesis (new member, same lineage, same links) and
// recording a newer hypothesis linked into a group keep every id.
func TestConcernGroupIDPropertyStableUnderCorrectionAndNewHypothesis(t *testing.T) {
	eachConcernSeed(t, func(t *testing.T, rng *rand.Rand, s ConcernGroupSnapshot) {
		if len(s.Hypotheses) == 0 {
			return
		}
		before := DeriveConcernGroups(s)
		c := cloneConcernSnapshot(s)
		i := rng.Intn(len(c.Hypotheses))
		old := c.Hypotheses[i]
		lineage, start := old.anchor()
		successor := ConcernGroupNode{ID: uuid.New(), CreatedRevision: s.Revision + 1, Label: "corrected guess",
			AnchorID: lineage, AnchorRevision: start}
		c.Hypotheses[i] = successor
		for j := range c.HypothesisFactLinks {
			if c.HypothesisFactLinks[j].FromID == old.ID {
				c.HypothesisFactLinks[j].FromID = successor.ID
			}
		}
		for j := range c.HypothesisObservationLinks {
			if c.HypothesisObservationLinks[j].HypothesisID == old.ID {
				c.HypothesisObservationLinks[j].HypothesisID = successor.ID
			}
		}
		c.Revision = s.Revision + 1
		// A newer hypothesis joins a random group through one of its facts.
		var fresh *ConcernGroupNode
		for _, g := range before.Groups {
			if len(g.RelatedFactIDs) > 0 && rng.Intn(2) == 0 {
				c.Revision++
				fresh = &ConcernGroupNode{ID: uuid.New(), CreatedRevision: c.Revision, Label: "newer guess"}
				c.Hypotheses = append(c.Hypotheses, *fresh)
				c.HypothesisFactLinks = append(c.HypothesisFactLinks,
					ConcernGroupLink{FromID: fresh.ID, FactID: g.RelatedFactIDs[rng.Intn(len(g.RelatedFactIDs))]})
				break
			}
		}
		after := DeriveConcernGroups(c)
		if !reflect.DeepEqual(groupIDSet(before), groupIDSet(after)) {
			t.Fatalf("correction/new hypothesis changed ids:\nbefore %#v\nafter  %#v", before.Groups, after.Groups)
		}
		if prev, now := groupHolding(before, old.ID), groupHolding(after, successor.ID); prev == nil || now == nil || prev.ID != now.ID {
			t.Fatalf("corrected hypothesis left its group id: %#v -> %#v", prev, now)
		}
	})
}

// Merging two groups yields the id of the group whose anchor began earlier;
// all other groups keep their ids and the absorbed id disappears.
func TestConcernGroupIDPropertyMergeKeepsEarlierAnchor(t *testing.T) {
	eachConcernSeed(t, func(t *testing.T, rng *rand.Rand, s ConcernGroupSnapshot) {
		before := DeriveConcernGroups(s)
		if len(before.Groups) < 2 {
			return
		}
		p := rng.Perm(len(before.Groups))
		g1, g2 := before.Groups[p[0]], before.Groups[p[1]]
		c := cloneConcernSnapshot(s)
		c.Revision++
		bridge := ConcernGroupNode{ID: uuid.New(), CreatedRevision: c.Revision}
		c.Facts = append(c.Facts, bridge)
		for _, g := range []DerivedConcernGroup{g1, g2} {
			if len(g.RelatedHypothesisIDs) > 0 {
				c.HypothesisFactLinks = append(c.HypothesisFactLinks, ConcernGroupLink{FromID: g.RelatedHypothesisIDs[0], FactID: bridge.ID})
			} else {
				c.EpisodeFactLinks = append(c.EpisodeFactLinks, ConcernGroupLink{FromID: g.RelatedEpisodeIDs[0], FactID: bridge.ID})
			}
		}
		after := DeriveConcernGroups(c)
		if len(after.Groups) != len(before.Groups)-1 {
			t.Fatalf("merge count: %d -> %d", len(before.Groups), len(after.Groups))
		}
		winner, loser := g1, g2
		if concernAnchorLess(anchorMember(s, g2), anchorMember(s, g1)) {
			winner, loser = g2, g1
		}
		merged := groupHolding(after, bridge.ID)
		if merged == nil || merged.ID != winner.ID || groupIDSet(after)[loser.ID] {
			t.Fatalf("merge id: winner %s loser %s got %#v", winner.ID, loser.ID, merged)
		}
		for _, g := range before.Groups {
			if g.ID != g1.ID && g.ID != g2.ID && !groupIDSet(after)[g.ID] {
				t.Fatalf("unrelated group id changed by merge: %#v", g)
			}
		}
	})
}

// Withdrawing a hypothesis: groups not containing it are untouched; if it
// was not its group's anchor the part keeping the anchor keeps the id (split
// included); if it was the anchor, that lineage's id is never reused.
func TestConcernGroupIDPropertyWithdrawAndSplit(t *testing.T) {
	eachConcernSeed(t, func(t *testing.T, rng *rand.Rand, s ConcernGroupSnapshot) {
		if len(s.Hypotheses) == 0 {
			return
		}
		before := DeriveConcernGroups(s)
		c := cloneConcernSnapshot(s)
		i := rng.Intn(len(c.Hypotheses))
		gone := c.Hypotheses[i]
		c.Hypotheses = append(c.Hypotheses[:i], c.Hypotheses[i+1:]...)
		c.Revision++
		after := DeriveConcernGroups(c)
		after2 := groupIDSet(after)
		for _, g := range before.Groups {
			anchor := anchorMember(s, g)
			holder := groupHolding(after, anchor.ID)
			switch {
			case groupHolding(before, gone.ID) == nil || groupHolding(before, gone.ID).ID != g.ID:
				if holder == nil || holder.ID != g.ID {
					t.Fatalf("unrelated group changed: %#v -> %#v", g, holder)
				}
			case anchor.ID != gone.ID:
				if holder == nil || holder.ID != g.ID {
					t.Fatalf("part keeping the anchor lost the id: %#v -> %#v", g, holder)
				}
			default:
				if after2[g.ID] {
					t.Fatalf("withdrawn anchor lineage id reused: %s", g.ID)
				}
			}
		}
	})
}
