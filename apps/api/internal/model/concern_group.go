package model

import (
	"bytes"
	"sort"

	"github.com/google/uuid"
)

// concernGroupIDNamespace scopes the provisional ConcernGroup display ids.
// It is a fixed constant so the same earliest member always yields the same
// id, but the id carries no authority: it changes whenever a merge/split
// changes the earliest member (stability policy is DC-032).
var concernGroupIDNamespace = uuid.MustParse("6f1d7c2e-3b8a-5c41-9e0d-4a2b7f6c1d93")

// ConcernGroupNode is one BodyState member visible in a revision snapshot.
// Label is only used for display titles (hypothesis statement or episode
// symptom label); facts carry none.
type ConcernGroupNode struct {
	ID              uuid.UUID
	CreatedRevision int64
	Label           string
}

// ConcernGroupLink is one undirected edge between a fact and either a
// hypothesis or an episode.
type ConcernGroupLink struct {
	FromID uuid.UUID // hypothesis or episode
	FactID uuid.UUID
}

// ConcernGroupSnapshot is everything read from ONE owner-scoped BodyState
// revision: current hypotheses/facts/episodes plus their links. Links whose
// endpoints are not in the snapshot (withdrawn fact, non-current episode,
// unavailable source) are ignored by the derivation.
type ConcernGroupSnapshot struct {
	UserID              uuid.UUID
	Revision            int64
	Hypotheses          []ConcernGroupNode
	Facts               []ConcernGroupNode
	Episodes            []ConcernGroupNode
	HypothesisFactLinks []ConcernGroupLink
	EpisodeFactLinks    []ConcernGroupLink
}

// DerivedConcernGroup is a read-only organisational view: no table, no
// authority, never a Diagnosis FK target, never a clinical conclusion.
type DerivedConcernGroup struct {
	ID                    uuid.UUID   `json:"id"`
	DisplayTitle          string      `json:"display_title"`
	LatestRevision        int64       `json:"latest_revision"`
	RelatedHypothesisIDs  []uuid.UUID `json:"related_hypothesis_ids"`
	RelatedFactIDs        []uuid.UUID `json:"related_fact_ids"`
	RelatedEpisodeIDs     []uuid.UUID `json:"related_episode_ids"`
	RelatedObservationIDs []uuid.UUID `json:"related_observation_ids"` // not implemented: no V2 observation link yet
}

type DerivedConcernGroupList struct {
	UserID          uuid.UUID             `json:"user_id"`
	CurrentRevision int64                 `json:"current_revision"`
	Groups          []DerivedConcernGroup `json:"groups"`
}

type concernNodeKind uint8

const (
	concernHypothesis concernNodeKind = iota
	concernFact
	concernEpisode
)

type concernNodeKey struct {
	kind concernNodeKind
	id   uuid.UUID
}

func concernNodeLess(a, b ConcernGroupNode) bool {
	if a.CreatedRevision != b.CreatedRevision {
		return a.CreatedRevision < b.CreatedRevision
	}
	return bytes.Compare(a.ID[:], b.ID[:]) < 0
}

// DeriveConcernGroups builds an undirected graph (hypothesis–fact and
// fact–episode edges) and returns one group per connected component that
// contains at least one hypothesis or episode. Output is independent of input
// order: members are sorted by (created_revision, id); groups by
// latest_revision DESC then id ASC.
func DeriveConcernGroups(s ConcernGroupSnapshot) *DerivedConcernGroupList {
	out := &DerivedConcernGroupList{UserID: s.UserID, CurrentRevision: s.Revision, Groups: []DerivedConcernGroup{}}
	nodes := map[concernNodeKey]ConcernGroupNode{}
	parent := map[concernNodeKey]concernNodeKey{}
	add := func(kind concernNodeKind, list []ConcernGroupNode) {
		for _, n := range list {
			if n.ID == uuid.Nil {
				continue
			}
			k := concernNodeKey{kind, n.ID}
			nodes[k] = n
			parent[k] = k
		}
	}
	add(concernHypothesis, s.Hypotheses)
	add(concernFact, s.Facts)
	add(concernEpisode, s.Episodes)
	var find func(concernNodeKey) concernNodeKey
	find = func(k concernNodeKey) concernNodeKey {
		for parent[k] != k {
			parent[k] = parent[parent[k]]
			k = parent[k]
		}
		return k
	}
	union := func(a, b concernNodeKey) {
		if _, ok := parent[a]; !ok {
			return
		}
		if _, ok := parent[b]; !ok {
			return
		}
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for _, l := range s.HypothesisFactLinks {
		union(concernNodeKey{concernHypothesis, l.FromID}, concernNodeKey{concernFact, l.FactID})
	}
	for _, l := range s.EpisodeFactLinks {
		union(concernNodeKey{concernEpisode, l.FromID}, concernNodeKey{concernFact, l.FactID})
	}

	type component struct{ hyp, fact, ep []ConcernGroupNode }
	components := map[concernNodeKey]*component{}
	for k, n := range nodes {
		root := find(k)
		c := components[root]
		if c == nil {
			c = &component{}
			components[root] = c
		}
		switch k.kind {
		case concernHypothesis:
			c.hyp = append(c.hyp, n)
		case concernFact:
			c.fact = append(c.fact, n)
		case concernEpisode:
			c.ep = append(c.ep, n)
		}
	}
	ids := func(list []ConcernGroupNode) []uuid.UUID {
		sort.Slice(list, func(i, j int) bool { return concernNodeLess(list[i], list[j]) })
		r := make([]uuid.UUID, len(list))
		for i, n := range list {
			r[i] = n.ID
		}
		return r
	}
	for _, c := range components {
		if len(c.hyp) == 0 && len(c.ep) == 0 {
			continue // isolated facts form no group
		}
		g := DerivedConcernGroup{
			RelatedHypothesisIDs:  ids(c.hyp),
			RelatedFactIDs:        ids(c.fact),
			RelatedEpisodeIDs:     ids(c.ep),
			RelatedObservationIDs: []uuid.UUID{},
		}
		var earliest, newest *ConcernGroupNode
		for _, list := range [][]ConcernGroupNode{c.hyp, c.fact, c.ep} {
			for i := range list {
				n := list[i]
				if earliest == nil || concernNodeLess(n, *earliest) {
					earliest = &n
				}
				if newest == nil || concernNodeLess(*newest, n) {
					newest = &n
				}
			}
		}
		g.ID = uuid.NewSHA1(concernGroupIDNamespace, []byte(earliest.ID.String()))
		g.LatestRevision = newest.CreatedRevision
		if len(c.hyp) > 0 {
			g.DisplayTitle = c.hyp[len(c.hyp)-1].Label // newest hypothesis (lists sorted ascending)
		} else {
			g.DisplayTitle = c.ep[len(c.ep)-1].Label
		}
		out.Groups = append(out.Groups, g)
	}
	sort.Slice(out.Groups, func(i, j int) bool {
		a, b := out.Groups[i], out.Groups[j]
		if a.LatestRevision != b.LatestRevision {
			return a.LatestRevision > b.LatestRevision
		}
		return bytes.Compare(a.ID[:], b.ID[:]) < 0
	})
	return out
}
