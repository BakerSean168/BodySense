package service

import (
	"context"
	"sort"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type settledDiagnosisCondition struct {
	entry              model.ConditionEntry
	clinical           diagnosisClinicalContent
	evidence           []model.ConditionEntryEvidence
	create             bool
	dirty              bool
	contentUpdated     bool
	dispositionUpdated bool
	evidenceChanged    bool
}
type diagnosisSettlementPlan struct {
	conditions   map[uuid.UUID]*settledDiagnosisCondition
	relations    map[string]*model.ConditionRelationRecord
	dispositions []*model.ConditionDispositionChange
	patch        []map[string]any
}

func newDiagnosisSettlementPlan() *diagnosisSettlementPlan {
	return &diagnosisSettlementPlan{conditions: map[uuid.UUID]*settledDiagnosisCondition{}, relations: map[string]*model.ConditionRelationRecord{}, dispositions: []*model.ConditionDispositionChange{}, patch: []map[string]any{}}
}
func (p *diagnosisSettlementPlan) conditionIDs() []uuid.UUID {
	ids := map[uuid.UUID]bool{}
	for id, c := range p.conditions {
		if c.dirty {
			ids[id] = true
		}
	}
	for _, r := range p.relations {
		ids[r.FromConditionID] = true
		ids[r.ToConditionID] = true
	}
	out := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
func (s *DiagnosisSettlementService) loadSettlementCondition(ctx context.Context, owner uuid.UUID, run model.ConditionAnalysisRun, plan *diagnosisSettlementPlan, id uuid.UUID, head int64) (*settledDiagnosisCondition, error) {
	if id == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if existing := plan.conditions[id]; existing != nil {
		return existing, nil
	}
	entry, err := s.repo.GetCondition(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	// An explicit partial-update scope cannot silently expand to unrelated
	// conditions. A full analysis with no named subset still has exact targets.
	if len(run.Scope.ConditionIDs) > 0 {
		allowed := false
		for _, target := range run.Scope.ConditionIDs {
			if target == id {
				allowed = true
			}
		}
		if !allowed {
			return nil, ErrDiagnosisDecisionConflict
		}
	}
	if entry.CommittedRevision <= 0 || entry.CommittedRevision > head {
		return nil, ErrDiagnosisDecisionConflict
	}
	evidence, err := s.repo.ListEvidence(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	out := &settledDiagnosisCondition{entry: *entry, clinical: diagnosisClinicalSnapshot(*entry, evidence), evidence: evidence}
	plan.conditions[id] = out
	return out, nil
}
func assignDiagnosisClinical(entry *model.ConditionEntry, c diagnosisClinicalContent) {
	entry.MedicalName = c.MedicalName
	entry.ConceptRef = c.ConceptRef
	entry.Qualifiers = c.Qualifiers
	entry.Assessment = c.Assessment
	entry.SourceClaimRefs = c.SourceClaimRefs
	entry.CauseAnalysis = c.CauseAnalysis
	entry.InformationGaps = c.InformationGaps
	entry.HandlingPlan = c.HandlingPlan
}
func (s *DiagnosisSettlementService) planSettlement(ctx context.Context, owner uuid.UUID, run model.ConditionAnalysisRun, choices diagnosisRunChoices, head, bodyRevision int64, now time.Time) (*diagnosisSettlementPlan, error) {
	plan := newDiagnosisSettlementPlan()
	for _, choice := range choices.effective {
		p, d := choice.proposal, choice.decision
		if d.Decision == model.DiagnosisReject {
			continue
		}
		if d.Decision != model.DiagnosisAccept {
			return nil, ErrDiagnosisSettlementInvalid
		}
		var envelope diagnosisContentEnvelope
		if err := strictDiagnosisJSON(p.Content, &envelope); err != nil {
			return nil, err
		}
		switch p.ChangeKind {
		case model.DiagnosisCreate:
			if p.TargetConditionID != nil || len(envelope.ProposedEntry) == 0 || len(envelope.ChangeSet) > 0 || envelope.NewDisposition != nil || envelope.DispositionReason != "" || envelope.RelationChange != nil {
				return nil, ErrDiagnosisSettlementInvalid
			}
			var content diagnosisClinicalContent
			if err := strictDiagnosisJSON(envelope.ProposedEntry, &content); err != nil {
				return nil, err
			}
			normalizeDiagnosisClinical(&content)
			links, err := s.validateSettlementClinical(ctx, owner, run, p, content, diagnosisClinicalFields, bodyRevision)
			if err != nil {
				return nil, err
			}
			id := uuid.New()
			entry := model.ConditionEntry{UserID: owner, ID: id, Disposition: model.ConditionAdopted, LatestDecisionID: d.ID, LastAnalysisRunID: &run.ID, CommittedRevision: head + 1, CreatedAt: now, UpdatedAt: now}
			assignDiagnosisClinical(&entry, content)
			for i := range links {
				links[i].ConditionID = id
			}
			plan.conditions[id] = &settledDiagnosisCondition{entry: entry, clinical: content, evidence: links, create: true, dirty: true, contentUpdated: true, evidenceChanged: true}
			plan.patch = append(plan.patch, map[string]any{"kind": "create", "conditionId": id, "proposalId": p.ID, "decisionId": d.ID, "after": content, "disposition": model.ConditionAdopted})
		case model.DiagnosisUpdate:
			if p.TargetConditionID == nil || envelope.NewDisposition != nil || envelope.DispositionReason != "" || envelope.RelationChange != nil {
				return nil, ErrDiagnosisSettlementInvalid
			}
			stage, err := s.loadSettlementCondition(ctx, owner, run, plan, *p.TargetConditionID, head)
			if err != nil {
				return nil, err
			}
			if stage.contentUpdated {
				return nil, ErrDiagnosisDecisionConflict
			}
			stage.contentUpdated = true
			before := stage.clinical
			after, changed, err := diagnosisUpdatedClinical(before, envelope)
			if err != nil {
				return nil, err
			}
			if !sameDiagnosisMedicalIdentity(before, after) {
				return nil, ErrDiagnosisMedicalIdentityChanged
			}
			links, err := s.validateSettlementClinical(ctx, owner, run, p, after, changed, bodyRevision)
			if err != nil {
				return nil, err
			}
			if diagnosisJSONEqual(before, after) {
				continue
			}
			assignDiagnosisClinical(&stage.entry, after)
			stage.clinical = after
			stage.dirty = true
			stage.entry.LatestDecisionID = d.ID
			stage.entry.LastAnalysisRunID = &run.ID
			stage.entry.CommittedRevision = head + 1
			stage.entry.UpdatedAt = now
			if changed["evidenceLinks"] {
				for i := range links {
					links[i].ConditionID = stage.entry.ID
				}
				stage.evidence = links
				stage.evidenceChanged = true
			}
			plan.patch = append(plan.patch, map[string]any{"kind": "update", "conditionId": stage.entry.ID, "proposalId": p.ID, "decisionId": d.ID, "before": before, "after": after})
		case model.DiagnosisSetDisposition:
			if p.TargetConditionID == nil || envelope.NewDisposition == nil || len(envelope.ProposedEntry) > 0 || len(envelope.ChangeSet) > 0 || envelope.RelationChange != nil {
				return nil, ErrDiagnosisSettlementInvalid
			}
			next := *envelope.NewDisposition
			validReason := false
			if next == model.ConditionAdopted {
				validReason = envelope.DispositionReason == "readopted"
			} else if next == model.ConditionInactive {
				switch envelope.DispositionReason {
				case "user_withdrawal", "superseded_by_new_conclusion", "no_longer_applicable":
					validReason = true
				}
			}
			if !validReason {
				return nil, ErrDiagnosisSettlementInvalid
			}
			stage, err := s.loadSettlementCondition(ctx, owner, run, plan, *p.TargetConditionID, head)
			if err != nil {
				return nil, err
			}
			if stage.dispositionUpdated {
				return nil, ErrDiagnosisDecisionConflict
			}
			stage.dispositionUpdated = true
			if stage.entry.Disposition == next {
				continue
			}
			if next == model.ConditionAdopted {
				// Re-adoption is a new source-qualified decision in this run, not an
				// undo that resurrects an unavailable old evidence chain.
				if _, err := s.validateSettlementClinical(ctx, owner, run, p, stage.clinical, map[string]bool{"evidenceLinks": true}, bodyRevision); err != nil {
					return nil, err
				}
			}
			old := stage.entry.Disposition
			stage.entry.Disposition = next
			stage.entry.LatestDecisionID = d.ID
			stage.entry.LastAnalysisRunID = &run.ID
			stage.entry.CommittedRevision = head + 1
			stage.entry.UpdatedAt = now
			stage.dirty = true
			plan.dispositions = append(plan.dispositions, &model.ConditionDispositionChange{UserID: owner, ID: uuid.New(), ConditionID: stage.entry.ID, FromDisposition: old, ToDisposition: next, Reason: envelope.DispositionReason, EvidenceRefs: d.DecidedEvidenceRefs, DecisionID: &d.ID, CommittedRevision: head + 1, ChangedAt: now})
			plan.patch = append(plan.patch, map[string]any{"kind": "set_disposition", "conditionId": stage.entry.ID, "proposalId": p.ID, "decisionId": d.ID, "from": old, "to": next, "reason": envelope.DispositionReason})
		case model.DiagnosisUpdateRelation:
			if envelope.RelationChange == nil || len(envelope.ProposedEntry) > 0 || len(envelope.ChangeSet) > 0 || envelope.NewDisposition != nil || envelope.DispositionReason != "" {
				return nil, ErrDiagnosisSettlementInvalid
			}
			relation := *envelope.RelationChange
			if relation.FromConditionID == uuid.Nil || relation.ToConditionID == uuid.Nil || relation.FromConditionID == relation.ToConditionID || (p.TargetConditionID != nil && *p.TargetConditionID != relation.FromConditionID) {
				return nil, ErrDiagnosisSettlementInvalid
			}
			switch relation.Kind {
			case model.ConditionDifferentialWith, model.ConditionSupersedes, model.ConditionCoexisting:
			default:
				return nil, ErrDiagnosisSettlementInvalid
			}
			// Relations never change either condition's adoption or clinical fields.
			for _, id := range []uuid.UUID{relation.FromConditionID, relation.ToConditionID} {
				if _, err := s.loadSettlementCondition(ctx, owner, run, plan, id, head); err != nil {
					return nil, err
				}
			}
			refs, err := normalizedDiagnosisRefs(relation.BasisEvidenceRefs)
			if err != nil || len(refs) == 0 {
				return nil, ErrDiagnosisSettlementInvalid
			}
			relation.BasisEvidenceRefs = refs
			decided := map[string]bool{}
			for _, ref := range d.DecidedEvidenceRefs {
				decided[evidenceKey(ref)] = true
			}
			for _, ref := range refs {
				if !decided[evidenceKey(ref)] {
					return nil, ErrDiagnosisDecisionConflict
				}
				if err := s.sources.Evidence(ctx, owner, ref, bodyRevision); err != nil {
					return nil, diagnosisEvidenceError(err)
				}
			}
			key := relation.FromConditionID.String() + ":" + relation.ToConditionID.String() + ":" + string(relation.Kind)
			if _, exists := plan.relations[key]; exists {
				return nil, ErrDiagnosisDecisionConflict
			}
			before, err := s.repo.GetSettledRelation(ctx, owner, relation.FromConditionID, relation.ToConditionID, relation.Kind)
			if err != nil {
				return nil, err
			}
			if before != nil && sameDiagnosisRefs(before.BasisEvidenceRefs, relation.BasisEvidenceRefs) && before.Rationale == relation.Rationale {
				continue
			}
			plan.relations[key] = &model.ConditionRelationRecord{UserID: owner, FromConditionID: relation.FromConditionID, ToConditionID: relation.ToConditionID, Kind: relation.Kind, BasisEvidenceRefs: relation.BasisEvidenceRefs, Rationale: relation.Rationale, LatestDecisionID: d.ID, CommittedRevision: head + 1, RecordedAt: now}
			plan.patch = append(plan.patch, map[string]any{"kind": "update_relation", "proposalId": p.ID, "decisionId": d.ID, "before": before, "after": relation})
		default:
			return nil, ErrDiagnosisSettlementInvalid
		}
	}
	return plan, nil
}
func (s *DiagnosisSettlementService) writeSettlementPlan(ctx context.Context, owner uuid.UUID, plan *diagnosisSettlementPlan) error {
	for _, id := range plan.conditionIDs() {
		stage := plan.conditions[id]
		if stage == nil || !stage.dirty {
			continue
		}
		if err := s.repo.WriteSettledCondition(ctx, &stage.entry, stage.create); err != nil {
			return err
		}
		if stage.evidenceChanged {
			if err := s.repo.ReplaceSettledEvidence(ctx, owner, id, stage.evidence); err != nil {
				return err
			}
		}
	}
	keys := make([]string, 0, len(plan.relations))
	for key := range plan.relations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := s.repo.WriteSettledRelation(ctx, plan.relations[key]); err != nil {
			return err
		}
	}
	for _, change := range plan.dispositions {
		if err := s.repo.InsertDispositionHistory(ctx, change); err != nil {
			return err
		}
	}
	return nil
}
