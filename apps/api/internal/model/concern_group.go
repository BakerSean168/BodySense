package model

import (
	"bytes"
	"sort"

	"github.com/google/uuid"
)

// concernGroupIDNamespace scopes the derived ConcernGroup display ids.
var concernGroupIDNamespace = uuid.MustParse("6f1d7c2e-3b8a-5c41-9e0d-4a2b7f6c1d93")

// ConcernGroupIDRule names the DC-032 identity rule in force. It is part of
// the hashed name, so a future rule change can never collide with these ids.
const ConcernGroupIDRule = "concern-group-anchor-v2"

// ConcernGroupID is the stable derived id of a group anchored on one
// anchor-eligible lineage (a hypothesis lineage id or an episode id).
func ConcernGroupID(anchor uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(concernGroupIDNamespace, []byte(ConcernGroupIDRule+"/"+anchor.String()))
}

// ConcernGroupNode is one BodyState member visible in a revision snapshot.
// Label is only used for display titles (hypothesis statement or episode
// symptom label); facts and observations carry none.
//
// AnchorID/AnchorRevision identify the member's stable lineage: for a
// hypothesis the first hypothesis of its correction chain (lineage_id and
// that row's created_revision), for an episode the episode itself. Zero
// values default to ID/CreatedRevision. Facts and observations never anchor.
type ConcernGroupNode struct {
	ID              uuid.UUID
	CreatedRevision int64
	Label           string
	AnchorID        uuid.UUID
	AnchorRevision  int64
}

// ConcernGroupLink is one undirected edge between a fact and either a
// hypothesis or an episode.
type ConcernGroupLink struct {
	FromID uuid.UUID // hypothesis or episode
	FactID uuid.UUID
}

// ConcernGroupObservationLink is one undirected hypothesis–observation edge
// (the user linked their own observation to their own guess).
type ConcernGroupObservationLink struct {
	HypothesisID  uuid.UUID
	ObservationID uuid.UUID
}

// ConcernGroupSnapshot is everything read from ONE owner-scoped BodyState
// revision: current hypotheses/facts/observations/episodes plus their links.
// Links whose endpoints are not in the snapshot (withdrawn or superseded
// fact/observation, non-current episode, unavailable source) are ignored.
type ConcernGroupSnapshot struct {
	UserID                     uuid.UUID
	Revision                   int64
	Hypotheses                 []ConcernGroupNode
	Facts                      []ConcernGroupNode
	Observations               []ConcernGroupNode
	Episodes                   []ConcernGroupNode
	HypothesisFactLinks        []ConcernGroupLink
	HypothesisObservationLinks []ConcernGroupObservationLink
	EpisodeFactLinks           []ConcernGroupLink
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
	RelatedObservationIDs []uuid.UUID `json:"related_observation_ids"`
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
	concernObservation
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

func (n ConcernGroupNode) anchor() (uuid.UUID, int64) {
	id, rev := n.AnchorID, n.AnchorRevision
	if id == uuid.Nil {
		id = n.ID
	}
	if rev == 0 {
		rev = n.CreatedRevision
	}
	return id, rev
}

// concernAnchorLess orders anchor-eligible members by when their lineage
// began, then lineage id, then the member itself (total order).
func concernAnchorLess(a, b ConcernGroupNode) bool {
	aid, arev := a.anchor()
	bid, brev := b.anchor()
	if arev != brev {
		return arev < brev
	}
	if c := bytes.Compare(aid[:], bid[:]); c != 0 {
		return c < 0
	}
	return concernNodeLess(a, b)
}

// DeriveConcernGroups builds an undirected graph (hypothesis–fact,
// hypothesis–observation and fact–episode edges) and returns one group per
// connected component that contains at least one hypothesis or episode.
//
// Group id (DC-032): the anchor is the component's anchor-eligible member
// (current hypothesis or current episode; never a fact or observation) whose
// lineage began earliest, ordered by (lineage start revision, lineage id). A
// hypothesis's lineage survives its corrections, so id = ConcernGroupID(anchor
// lineage) stays the same when facts/observations/episodes are linked,
// unlinked, corrected or withdrawn, when any hypothesis is corrected or a
// newer one recorded, and on merge (the earlier anchor wins) or split (the
// part keeping the anchor keeps the id). It changes only when the anchor
// lineage itself leaves the group (anchor hypothesis withdrawn, anchor
// episode no longer current) or the group is merged into an earlier one.
//
// Output is independent of input order: members are sorted by
// (created_revision, id); groups by latest_revision DESC then id ASC.
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
	add(concernObservation, s.Observations)
	add(concernEpisode, s.Episodes)
	find := func(k concernNodeKey) concernNodeKey {
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
	for _, l := range s.HypothesisObservationLinks {
		union(concernNodeKey{concernHypothesis, l.HypothesisID}, concernNodeKey{concernObservation, l.ObservationID})
	}
	for _, l := range s.EpisodeFactLinks {
		union(concernNodeKey{concernEpisode, l.FromID}, concernNodeKey{concernFact, l.FactID})
	}

	type component struct{ hyp, fact, obs, ep []ConcernGroupNode }
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
		case concernObservation:
			c.obs = append(c.obs, n)
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
	type anchored struct {
		group  DerivedConcernGroup
		anchor ConcernGroupNode
	}
	var groups []anchored
	for _, c := range components {
		if len(c.hyp) == 0 && len(c.ep) == 0 {
			continue // isolated facts/observations form no group
		}
		g := DerivedConcernGroup{
			RelatedHypothesisIDs:  ids(c.hyp),
			RelatedFactIDs:        ids(c.fact),
			RelatedObservationIDs: ids(c.obs),
			RelatedEpisodeIDs:     ids(c.ep),
		}
		var anchor *ConcernGroupNode
		for _, list := range [][]ConcernGroupNode{c.hyp, c.ep} {
			for i := range list {
				if anchor == nil || concernAnchorLess(list[i], *anchor) {
					anchor = &list[i]
				}
			}
		}
		for _, list := range [][]ConcernGroupNode{c.hyp, c.fact, c.obs, c.ep} {
			for _, n := range list {
				if n.CreatedRevision > g.LatestRevision {
					g.LatestRevision = n.CreatedRevision
				}
			}
		}
		if len(c.hyp) > 0 {
			g.DisplayTitle = c.hyp[len(c.hyp)-1].Label // newest hypothesis (lists sorted ascending)
		} else {
			g.DisplayTitle = c.ep[len(c.ep)-1].Label
		}
		groups = append(groups, anchored{group: g, anchor: *anchor})
	}
	// Assign ids in anchor order. An anchor lineage has exactly one current
	// member, so collisions cannot occur from valid data; should one ever
	// appear, the later group falls back to its anchor member's own id so ids
	// stay unique within a revision.
	sort.Slice(groups, func(i, j int) bool { return concernAnchorLess(groups[i].anchor, groups[j].anchor) })
	used := map[uuid.UUID]struct{}{}
	for i := range groups {
		lineage, _ := groups[i].anchor.anchor()
		id := ConcernGroupID(lineage)
		if _, taken := used[id]; taken {
			id = uuid.NewSHA1(concernGroupIDNamespace, []byte(ConcernGroupIDRule+"/member/"+groups[i].anchor.ID.String()))
		}
		used[id] = struct{}{}
		groups[i].group.ID = id
		out.Groups = append(out.Groups, groups[i].group)
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
