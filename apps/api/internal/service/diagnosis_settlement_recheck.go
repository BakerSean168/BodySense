package service

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type diagnosisChoice struct {
	proposal model.DiagnosisProposal
	decision model.DiagnosisDecision
}
type diagnosisRunChoices struct {
	effective []diagnosisChoice
	pending   []uuid.UUID
	events    []model.ConditionAnalysisEvent
}

func newDiagnosisRunChoices(proposals []model.DiagnosisProposal, decisions []model.DiagnosisDecision, events []model.ConditionAnalysisEvent) (diagnosisRunChoices, error) {
	result := diagnosisRunChoices{effective: []diagnosisChoice{}, pending: []uuid.UUID{}, events: events}
	byID := map[uuid.UUID]model.DiagnosisProposal{}
	unavailable := map[uuid.UUID]bool{}
	byProposal := map[uuid.UUID]model.DiagnosisDecision{}
	for _, p := range proposals {
		if p.ID == uuid.Nil {
			return result, ErrDiagnosisSettlementInvalid
		}
		if _, duplicate := byID[p.ID]; duplicate {
			return result, ErrDiagnosisSettlementInvalid
		}
		byID[p.ID] = p
	}
	// Supersession is a lineage, not a cycle or an ambiguous fork that could
	// make all prior user choices disappear from the effective set.
	for _, p := range proposals {
		seen := map[uuid.UUID]bool{}
		for current := p; ; {
			if seen[current.ID] {
				return result, ErrDiagnosisSettlementInvalid
			}
			seen[current.ID] = true
			if current.SupersedesProposalID == nil {
				break
			}
			parent, ok := byID[*current.SupersedesProposalID]
			if !ok || parent.UserID != p.UserID || parent.RunID != p.RunID {
				return result, ErrDiagnosisSettlementInvalid
			}
			current = parent
		}
	}
	for _, p := range proposals {
		if p.SupersedesProposalID != nil {
			if parent, ok := byID[*p.SupersedesProposalID]; !ok || parent.RunID != p.RunID || parent.ID == p.ID || unavailable[parent.ID] {
				return result, ErrDiagnosisSettlementInvalid
			}
			unavailable[*p.SupersedesProposalID] = true
		}
	}
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			return result, ErrDiagnosisSettlementInvalid
		}
		if event.Kind == "proposal_superseded" || event.Kind == "proposal_expired" {
			if event.ProposalID == nil {
				return result, ErrDiagnosisSettlementInvalid
			}
			unavailable[*event.ProposalID] = true
		}
	}
	for _, d := range decisions {
		p, ok := byID[d.ProposalID]
		if !ok || p.RunID != d.RunID || p.UserID != d.UserID {
			return result, ErrDiagnosisSettlementInvalid
		}
		if _, exists := byProposal[d.ProposalID]; exists {
			return result, ErrDiagnosisSettlementInvalid
		}
		byProposal[d.ProposalID] = d
	}
	for _, p := range proposals {
		if unavailable[p.ID] {
			continue
		}
		if d, ok := byProposal[p.ID]; ok {
			result.effective = append(result.effective, diagnosisChoice{p, d})
		} else {
			result.pending = append(result.pending, p.ID)
		}
	}
	return result, nil
}
func (c diagnosisRunChoices) decisionIDs() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(c.effective))
	for _, choice := range c.effective {
		ids = append(ids, choice.decision.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids
}

type diagnosisRecheckDetails struct {
	CheckedEvidenceRefs []model.DiagnosisEvidenceRef `json:"checkedEvidenceRefs"`
	Result              string                       `json:"result"`
}
type diagnosisSettlementRecheck struct {
	proposal uuid.UUID
	decision uuid.UUID
	details  diagnosisRecheckDetails
}

func evidenceKey(ref model.DiagnosisEvidenceRef) string {
	return string(ref.EvidenceType) + ":" + ref.EvidenceID.String() + ":" + ref.Version
}
func normalizedDiagnosisRefs(refs []model.DiagnosisEvidenceRef) ([]model.DiagnosisEvidenceRef, error) {
	if len(refs) > 2000 {
		return nil, ErrDiagnosisSettlementInvalid
	}
	byKey := map[string]model.DiagnosisEvidenceRef{}
	for _, ref := range refs {
		v, err := strconv.ParseInt(ref.Version, 10, 64)
		if err != nil || v <= 0 || ref.Version != strconv.FormatInt(v, 10) || ref.EvidenceID == uuid.Nil {
			return nil, ErrDiagnosisSettlementInvalid
		}
		switch ref.EvidenceType {
		case model.DiagnosisBodyFact, model.DiagnosisBodyObservation, model.DiagnosisSymptomEpisode, model.DiagnosisDocumentEvidence:
		default:
			return nil, ErrDiagnosisSettlementInvalid
		}
		byKey[evidenceKey(ref)] = ref
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]model.DiagnosisEvidenceRef, 0, len(keys))
	for _, k := range keys {
		result = append(result, byKey[k])
	}
	return result, nil
}
func sameDiagnosisRefs(a, b []model.DiagnosisEvidenceRef) bool {
	x, xe := normalizedDiagnosisRefs(a)
	y, ye := normalizedDiagnosisRefs(b)
	if xe != nil || ye != nil || len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// Review coverage is explicit run evidence, not an arbitrary BodyState counter.
// A prior affected verdict cannot be silently overwritten by a later unaffected
// event: the user must receive and decide a new superseding proposal.
func (s *DiagnosisSettlementService) recheckChoices(ctx context.Context, owner uuid.UUID, run model.ConditionAnalysisRun, choices diagnosisRunChoices, bodyRevision int64) ([]diagnosisSettlementRecheck, error) {
	out := make([]diagnosisSettlementRecheck, 0, len(choices.effective))
	for _, choice := range choices.effective {
		p, d := choice.proposal, choice.decision
		hash, err := model.DiagnosisProposalContentFingerprint(&p)
		if err != nil || hash != p.ContentFingerprint || d.ContentFingerprint != p.ContentFingerprint || d.ExpectedRevision != run.BasedOnDiagnosisRevision || p.BaseRevision != run.BasedOnDiagnosisRevision {
			return nil, ErrDiagnosisDecisionConflict
		}
		refs, err := diagnosisDecidedEvidence(&p)
		if err != nil {
			return nil, err
		}
		if !sameDiagnosisRefs(refs, d.DecidedEvidenceRefs) {
			return nil, ErrDiagnosisDecisionConflict
		}
		var recordedSequence int64
		for _, event := range choices.events {
			if event.Kind == "decision_recorded" && event.DecisionID != nil && *event.DecisionID == d.ID && event.ProposalID != nil && *event.ProposalID == p.ID {
				recordedSequence = event.Sequence
			}
		}
		if recordedSequence == 0 {
			return nil, ErrDiagnosisSettlementInvalid
		}
		decided := map[string]bool{}
		all := map[string]model.DiagnosisEvidenceRef{}
		addedAt := map[string]int64{}
		checkedAt := map[string]int64{}
		for _, ref := range refs {
			key := evidenceKey(ref)
			decided[key] = true
			all[key] = ref
		}
		for _, event := range choices.events {
			if event.Sequence <= recordedSequence {
				continue
			}
			switch event.Kind {
			case "evidence_added":
				if event.ProposalID != nil && *event.ProposalID != p.ID {
					continue
				}
				var data struct {
					EvidenceRefs []model.DiagnosisEvidenceRef `json:"evidenceRefs"`
				}
				if json.Unmarshal(event.Details, &data) != nil || len(data.EvidenceRefs) == 0 {
					return nil, ErrDiagnosisRecheckRequired
				}
				added, err := normalizedDiagnosisRefs(data.EvidenceRefs)
				if err != nil {
					return nil, ErrDiagnosisRecheckRequired
				}
				for _, ref := range added {
					key := evidenceKey(ref)
					all[key] = ref
					if !decided[key] && addedAt[key] == 0 {
						addedAt[key] = event.Sequence
					}
				}
			case "decision_rechecked":
				if event.DecisionID == nil || *event.DecisionID != d.ID {
					continue
				}
				var check diagnosisRecheckDetails
				if json.Unmarshal(event.Details, &check) != nil || check.Result != "unaffected" {
					return nil, ErrDiagnosisRecheckRequired
				}
				covered, err := normalizedDiagnosisRefs(check.CheckedEvidenceRefs)
				if err != nil {
					return nil, ErrDiagnosisRecheckRequired
				}
				for _, ref := range covered {
					checkedAt[evidenceKey(ref)] = event.Sequence
				}
			}
		}
		for key, sequence := range addedAt {
			if checkedAt[key] < sequence {
				return nil, ErrDiagnosisRecheckRequired
			}
		}
		allRefs := make([]model.DiagnosisEvidenceRef, 0, len(all))
		for _, ref := range all {
			allRefs = append(allRefs, ref)
		}
		allRefs, err = normalizedDiagnosisRefs(allRefs)
		if err != nil {
			return nil, err
		}
		for _, ref := range allRefs {
			if err := s.sources.Evidence(ctx, owner, ref, bodyRevision); err != nil {
				return nil, diagnosisEvidenceError(err)
			}
		}
		out = append(out, diagnosisSettlementRecheck{p.ID, d.ID, diagnosisRecheckDetails{allRefs, "unaffected"}})
	}
	return out, nil
}
func settlementCheckedRefs(checks []diagnosisSettlementRecheck) []model.DiagnosisEvidenceRef {
	all := map[string]model.DiagnosisEvidenceRef{}
	for _, check := range checks {
		for _, ref := range check.details.CheckedEvidenceRefs {
			all[evidenceKey(ref)] = ref
		}
	}
	keys := make([]string, 0, len(all))
	for key := range all {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	refs := make([]model.DiagnosisEvidenceRef, 0, len(keys))
	for _, key := range keys {
		refs = append(refs, all[key])
	}
	return refs
}
