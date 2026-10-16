package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// V2 BodyState revisions carry identifiers only (no health text). Review and
// freshness policies decode them into a bodyStateChange and resolve the named
// fact ids to their kind and concern through BodyStateFactIndex.
type bodyStateChangeKind int

const (
	bodyStateChangeIgnored bodyStateChangeKind = iota
	bodyStateChangeSafety
	bodyStateChangeFactsRecorded
	bodyStateChangeFactCorrected
	bodyStateChangeFactWithdrawn
	bodyStateChangeObservationCorrected
	bodyStateChangeObservationWithdrawn
	bodyStateChangeEpisode
)

type bodyStateChange struct {
	Revision   int64
	ChangeType string
	Kind       bodyStateChangeKind
	// FactIDs are recorded facts, a correction's replacement, or the facts
	// an episode command is based on.
	FactIDs []string
	// TargetID is the corrected/withdrawn fact or observation.
	TargetID string
}

// BodyStateFactRef is the policy-relevant projection of one V2 fact.
type BodyStateFactRef struct {
	Kind       string
	ConcernKey string
}

// BodyStateFactIndex maps a V2 fact id to its kind and concern key.
type BodyStateFactIndex map[string]BodyStateFactRef

func (index BodyStateFactIndex) concern(id string) string {
	return index[id].ConcernKey
}

type bodyStateRevisionMetadata struct {
	FactIDs             []string `json:"fact_ids"`
	TargetFactID        string   `json:"target_fact_id"`
	TargetObservationID string   `json:"target_observation_id"`
	FactID              string   `json:"fact_id"`
	BasisFactID         string   `json:"basis_fact_id"`
}

// decodeBodyStateChange classifies one revision. Legacy change types
// (fact.*, observation.added/updated, current_context.updated) are not
// BodyState authority any more and are ignored; legacy safety.updated still
// counts as a Safety change (fail closed).
func decodeBodyStateChange(revision model.BodyStateRevision) bodyStateChange {
	change := bodyStateChange{Revision: revision.Revision, ChangeType: revision.ChangeType}
	if strings.HasPrefix(revision.ChangeType, "safety.") {
		change.Kind = bodyStateChangeSafety
		return change
	}
	var metadata bodyStateRevisionMetadata
	_ = json.Unmarshal(revision.Changes, &metadata)
	switch {
	case revision.ChangeType == "facts.recorded":
		change.Kind, change.FactIDs = bodyStateChangeFactsRecorded, metadata.FactIDs
	case revision.ChangeType == "facts.corrected":
		change.Kind, change.FactIDs, change.TargetID = bodyStateChangeFactCorrected, metadata.FactIDs, metadata.TargetFactID
	case revision.ChangeType == "facts.withdrawn":
		change.Kind, change.TargetID = bodyStateChangeFactWithdrawn, metadata.TargetFactID
	case revision.ChangeType == "observation.corrected":
		change.Kind, change.TargetID = bodyStateChangeObservationCorrected, metadata.TargetObservationID
	case revision.ChangeType == "observation.withdrawn":
		change.Kind, change.TargetID = bodyStateChangeObservationWithdrawn, metadata.TargetObservationID
	case strings.HasPrefix(revision.ChangeType, "episode."):
		change.Kind = bodyStateChangeEpisode
		for _, id := range []string{metadata.FactID, metadata.BasisFactID} {
			if id != "" {
				change.FactIDs = append(change.FactIDs, id)
			}
		}
	}
	return change
}

// bodyStateRevisionFactIDs lists every fact id the revisions name, so one
// owner-scoped lookup can resolve them.
func bodyStateRevisionFactIDs(revisions []model.BodyStateRevision) []uuid.UUID {
	seen := map[uuid.UUID]struct{}{}
	ids := make([]uuid.UUID, 0)
	add := func(raw string) {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	for _, revision := range revisions {
		change := decodeBodyStateChange(revision)
		for _, id := range change.FactIDs {
			add(id)
		}
		if change.Kind == bodyStateChangeFactCorrected || change.Kind == bodyStateChangeFactWithdrawn {
			add(change.TargetID)
		}
	}
	return ids
}

type bodyFactLookup interface {
	GetBodyFactsByIDs(ctx context.Context, owner uuid.UUID, ids []uuid.UUID) ([]model.BodyFact, error)
}

func resolveBodyStateRevisionFacts(ctx context.Context, lookup bodyFactLookup, owner uuid.UUID, revisions []model.BodyStateRevision) (BodyStateFactIndex, error) {
	index := BodyStateFactIndex{}
	ids := bodyStateRevisionFactIDs(revisions)
	if len(ids) == 0 {
		return index, nil
	}
	facts, err := lookup.GetBodyFactsByIDs(ctx, owner, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve body state revision facts: %w", err)
	}
	for _, fact := range facts {
		if fact.UserID != owner {
			return nil, fmt.Errorf("resolve body state revision facts: owner mismatch")
		}
		index[fact.ID.String()] = BodyStateFactRef{Kind: fact.Kind, ConcernKey: bodyFactConcernKey(fact)}
	}
	return index, nil
}

// bodyFactConcernKey is the concern a V2 fact belongs to: a symptom's first
// canonical region ("region:<id>", "general" without one), the injury
// history, or a lifestyle section. Other facts have no concern.
func bodyFactConcernKey(fact model.BodyFact) string {
	if fact.Kind == "symptom" {
		var regions []string
		_ = json.Unmarshal(fact.BodyRegionIDs, &regions)
		if len(regions) > 0 && strings.TrimSpace(regions[0]) != "" {
			return "region:" + regions[0]
		}
		return "general"
	}
	switch key := bodyContextFactKey(fact); {
	case key == "history:"+bodyContextHistoryInjury:
		return "history:injury"
	case strings.HasPrefix(key, "lifestyle:"):
		return key
	}
	return ""
}

// diagnosisBasis is what an analysis rests on: its concern keys and the V2
// fact / observation ids its candidates cite.
type diagnosisBasis struct {
	concerns     map[string]struct{}
	facts        map[string]struct{}
	observations map[string]struct{}
}

func newDiagnosisBasis(analysis *model.DiagnosisAnalysisRecord) diagnosisBasis {
	basis := diagnosisBasis{concerns: map[string]struct{}{}, facts: map[string]struct{}{}, observations: map[string]struct{}{}}
	if analysis == nil {
		return basis
	}
	for _, candidate := range analysis.Candidates {
		if key := strings.TrimSpace(candidate.ConcernKey); key != "" && key != "general" {
			basis.concerns[key] = struct{}{}
		}
		for _, id := range decodeStringList(candidate.BasisFactIDs) {
			basis.facts[id] = struct{}{}
		}
		for _, id := range decodeStringList(candidate.BasisObservationIDs) {
			basis.observations[id] = struct{}{}
		}
	}
	return basis
}

func (b diagnosisBasis) citesFact(id string) bool {
	_, ok := b.facts[id]
	return id != "" && ok
}

func (b diagnosisBasis) citesObservation(id string) bool {
	_, ok := b.observations[id]
	return id != "" && ok
}

func (b diagnosisBasis) relatedConcern(concernKey string) bool {
	return concernMatches(b.concerns, concernKey)
}

// firstRelatedFact returns the first fact id that the analysis cites or whose
// concern it covers, with that fact's concern key.
func (b diagnosisBasis) firstRelatedFact(ids []string, facts BodyStateFactIndex) (string, string, bool) {
	for _, id := range ids {
		concern := facts.concern(id)
		if b.citesFact(id) || b.relatedConcern(concern) {
			return id, concern, true
		}
	}
	return "", "", false
}

func decodeStringList(raw datatypes.JSON) []string {
	var values []string
	_ = json.Unmarshal(raw, &values)
	return values
}

func concernMatches(concerns map[string]struct{}, concernKey string) bool {
	if concernKey == "" || concernKey == "general" || len(concerns) == 0 {
		return false
	}
	_, ok := concerns[concernKey]
	return ok
}

// GetBodyStateHead returns the aggregate head (revision + Safety state).
func (s *BodyStateService) GetBodyStateHead(ctx context.Context, userID uuid.UUID) (*model.BodyStateHead, error) {
	return s.repo.GetBodyStateHead(ctx, userID)
}

// ResolveRevisionFacts resolves the V2 fact ids named by revisions.
func (s *BodyStateService) ResolveRevisionFacts(ctx context.Context, userID uuid.UUID, revisions []model.BodyStateRevision) (BodyStateFactIndex, error) {
	return resolveBodyStateRevisionFacts(ctx, s.repo, userID, revisions)
}
