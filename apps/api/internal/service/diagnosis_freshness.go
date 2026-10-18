package service

import (
	"strconv"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Diagnosis freshness is a derived review signal, never a Diagnosis state.
// It compares the BodyState revision a Condition (or an open formal run) was
// analysed at with the immutable BodyState revisions that followed, and asks
// the same source authority that admits commits and plans whether every cited
// source still qualifies. It writes nothing: a stale Condition stays exactly
// as the user adopted it until a new formal run and the user's decision change
// it (diagnosis.md §3.6 "reviewRecommended").
const (
	DiagnosisFreshnessCurrent           = "current"
	DiagnosisFreshnessReviewRecommended = "review_recommended"
	DiagnosisFreshnessStale             = "stale"
)

// Reason codes. Stale codes mean the subject rests on something that no
// longer qualifies (a commit or plan admission would refuse it); review codes
// mean something relevant changed after the analysis and a new formal run is
// recommended. Reasons carry identifiers and revisions only, no health text.
const (
	freshnessCitedEvidenceCorrected   = "cited_evidence_corrected"
	freshnessCitedEvidenceWithdrawn   = "cited_evidence_withdrawn"
	freshnessCitedEvidenceUnavailable = "cited_evidence_unavailable"
	freshnessSourceClaimUnavailable   = "source_claim_unavailable"
	freshnessAssessmentUnavailable    = "assessment_source_unavailable"
	freshnessRequestUnavailable       = "request_source_unavailable"
	freshnessDiagnosisRevisionChanged = "diagnosis_revision_changed"
	freshnessConditionChanged         = "condition_changed"
	freshnessSafetyChanged            = "safety_changed"
	freshnessRelatedFactRecorded      = "related_fact_recorded"
	freshnessRelatedFactCorrected     = "related_fact_corrected"
	freshnessRelatedFactWithdrawn     = "related_fact_withdrawn"
	freshnessCitedEpisodeChanged      = "cited_episode_changed"
	freshnessRelatedEpisodeChanged    = "related_episode_changed"
	freshnessChangeWindowExceeded     = "change_window_exceeded"
)

var diagnosisFreshnessStaleCodes = map[string]bool{
	freshnessCitedEvidenceCorrected: true, freshnessCitedEvidenceWithdrawn: true, freshnessCitedEvidenceUnavailable: true,
	freshnessSourceClaimUnavailable: true, freshnessAssessmentUnavailable: true, freshnessRequestUnavailable: true,
	freshnessDiagnosisRevisionChanged: true, freshnessConditionChanged: true,
}

// DiagnosisFreshnessReason is one auditable cause, reproducible from the
// immutable BodyState/Diagnosis revisions it names.
type DiagnosisFreshnessReason struct {
	Code              string                       `json:"code"`
	BodyStateRevision *int64                       `json:"bodyStateRevision,omitempty"`
	ChangeType        string                       `json:"changeType,omitempty"`
	EvidenceType      *model.DiagnosisEvidenceType `json:"evidenceType,omitempty"`
	EvidenceID        *uuid.UUID                   `json:"evidenceId,omitempty"`
	ClaimID           string                       `json:"claimId,omitempty"`
	RunID             *uuid.UUID                   `json:"runId,omitempty"`
	ConditionID       *uuid.UUID                   `json:"conditionId,omitempty"`
	DiagnosisRevision *int64                       `json:"diagnosisRevision,omitempty"`
}

// DiagnosisFreshness is the stamp comparison for one Condition or open run.
type DiagnosisFreshness struct {
	State                      string                     `json:"state"`
	BasedOnBodyStateRevision   int64                      `json:"basedOnBodyStateRevision"`
	EvaluatedBodyStateRevision int64                      `json:"evaluatedBodyStateRevision"`
	EvaluatedDiagnosisRevision int64                      `json:"evaluatedDiagnosisRevision"`
	Reasons                    []DiagnosisFreshnessReason `json:"reasons"`
}

// Stale reports whether the subject rests on a source that no longer qualifies.
func (f *DiagnosisFreshness) Stale() bool { return f != nil && f.State == DiagnosisFreshnessStale }

// diagnosisFreshnessBasis is what a subject was analysed against.
type diagnosisFreshnessBasis struct {
	BodyStateRevision int64
	Evidence          []model.DiagnosisEvidenceRef
	// Concerns are the concern keys (see bodyFactConcernKey) the subject covers.
	Concerns map[string]bool
}

// diagnosisFreshnessChanges is the BodyState window after the oldest basis.
type diagnosisFreshnessChanges struct {
	Head      int64
	Revisions []model.BodyStateRevision // ascending
	// Complete is false when the bounded read did not reach Head.
	Complete bool
	Facts    BodyStateFactIndex
	// Unavailable holds evidenceKey()s that the live source authority refused.
	Unavailable map[string]bool
}

type diagnosisFreshnessBuilder struct {
	freshness DiagnosisFreshness
	seen      map[string]bool
}

func newDiagnosisFreshnessBuilder(basis, head, diagnosisRevision int64) *diagnosisFreshnessBuilder {
	return &diagnosisFreshnessBuilder{
		freshness: DiagnosisFreshness{
			State: DiagnosisFreshnessCurrent, BasedOnBodyStateRevision: basis,
			EvaluatedBodyStateRevision: head, EvaluatedDiagnosisRevision: diagnosisRevision,
			Reasons: []DiagnosisFreshnessReason{},
		},
		seen: map[string]bool{},
	}
}

func (b *diagnosisFreshnessBuilder) add(reason DiagnosisFreshnessReason) {
	key := reason.Code + "|" + reason.ChangeType + "|" + reason.ClaimID
	if reason.BodyStateRevision != nil {
		key += "|r" + strconv.FormatInt(*reason.BodyStateRevision, 10)
	}
	if reason.EvidenceID != nil {
		key += "|e" + reason.EvidenceID.String()
	}
	if reason.RunID != nil {
		key += "|u" + reason.RunID.String()
	}
	if reason.ConditionID != nil {
		key += "|c" + reason.ConditionID.String()
	}
	if b.seen[key] {
		return
	}
	b.seen[key] = true
	b.freshness.Reasons = append(b.freshness.Reasons, reason)
	if diagnosisFreshnessStaleCodes[reason.Code] {
		b.freshness.State = DiagnosisFreshnessStale
	} else if b.freshness.State == DiagnosisFreshnessCurrent {
		b.freshness.State = DiagnosisFreshnessReviewRecommended
	}
}

func (b *diagnosisFreshnessBuilder) result() DiagnosisFreshness { return b.freshness }

// evaluateDiagnosisFreshness is the pure, deterministic stamp comparison.
// Unrelated revisions never change the result; a revision-number difference
// alone is not evidence against a Condition (diagnosis.md §3.5 rule 6).
func evaluateDiagnosisFreshness(b *diagnosisFreshnessBuilder, basis diagnosisFreshnessBasis, changes diagnosisFreshnessChanges) {
	cited := map[string]model.DiagnosisEvidenceType{}
	for _, ref := range basis.Evidence {
		cited[ref.EvidenceID.String()] = ref.EvidenceType
	}
	citedAs := func(id string, kinds ...model.DiagnosisEvidenceType) bool {
		kind, ok := cited[id]
		if !ok {
			return false
		}
		for _, k := range kinds {
			if kind == k {
				return true
			}
		}
		return false
	}
	related := func(ids ...string) (string, bool) {
		for _, id := range ids {
			if concern := changes.Facts.concern(id); concern != "" && basis.Concerns[concern] {
				return id, true
			}
		}
		return "", false
	}
	revisionFlagged := map[string]bool{}
	for _, revision := range changes.Revisions {
		if revision.Revision <= basis.BodyStateRevision || revision.Revision > changes.Head {
			continue
		}
		change := decodeBodyStateChange(revision)
		at := revision.Revision
		reason := func(code string, kind model.DiagnosisEvidenceType, id string) DiagnosisFreshnessReason {
			r := DiagnosisFreshnessReason{Code: code, BodyStateRevision: &at, ChangeType: change.ChangeType}
			if parsed, err := uuid.Parse(id); err == nil && parsed != uuid.Nil {
				k := kind
				r.EvidenceType, r.EvidenceID = &k, &parsed
			}
			return r
		}
		switch change.Kind {
		case bodyStateChangeSafety:
			b.add(DiagnosisFreshnessReason{Code: freshnessSafetyChanged, BodyStateRevision: &at, ChangeType: change.ChangeType})
		case bodyStateChangeFactsRecorded:
			if id, ok := related(change.FactIDs...); ok {
				b.add(reason(freshnessRelatedFactRecorded, model.DiagnosisBodyFact, id))
			}
		case bodyStateChangeFactCorrected, bodyStateChangeFactWithdrawn:
			code, relatedCode := freshnessCitedEvidenceCorrected, freshnessRelatedFactCorrected
			if change.Kind == bodyStateChangeFactWithdrawn {
				code, relatedCode = freshnessCitedEvidenceWithdrawn, freshnessRelatedFactWithdrawn
			}
			if citedAs(change.TargetID, model.DiagnosisBodyFact) {
				revisionFlagged[change.TargetID] = true
				b.add(reason(code, model.DiagnosisBodyFact, change.TargetID))
			} else if _, ok := related(append([]string{change.TargetID}, change.FactIDs...)...); ok {
				b.add(reason(relatedCode, model.DiagnosisBodyFact, change.TargetID))
			}
		case bodyStateChangeObservationCorrected, bodyStateChangeObservationWithdrawn:
			if citedAs(change.TargetID, model.DiagnosisBodyObservation) {
				code := freshnessCitedEvidenceCorrected
				if change.Kind == bodyStateChangeObservationWithdrawn {
					code = freshnessCitedEvidenceWithdrawn
				}
				revisionFlagged[change.TargetID] = true
				b.add(reason(code, model.DiagnosisBodyObservation, change.TargetID))
			}
		case bodyStateChangeEpisode:
			flagged := false
			for _, id := range change.EpisodeIDs {
				if citedAs(id, model.DiagnosisSymptomEpisode) {
					b.add(reason(freshnessCitedEpisodeChanged, model.DiagnosisSymptomEpisode, id))
					flagged = true
				}
			}
			if !flagged {
				if id, ok := related(change.FactIDs...); ok {
					b.add(reason(freshnessRelatedEpisodeChanged, model.DiagnosisBodyFact, id))
				}
			}
		}
	}
	if !changes.Complete && basis.BodyStateRevision < changes.Head {
		b.add(DiagnosisFreshnessReason{Code: freshnessChangeWindowExceeded})
	}
	// The live source authority catches what revisions cannot name: an
	// erased statement, a removed report, an episode that was merged away.
	for _, ref := range basis.Evidence {
		if !changes.Unavailable[evidenceKey(ref)] || revisionFlagged[ref.EvidenceID.String()] {
			continue
		}
		kind, id := ref.EvidenceType, ref.EvidenceID
		b.add(DiagnosisFreshnessReason{Code: freshnessCitedEvidenceUnavailable, EvidenceType: &kind, EvidenceID: &id})
	}
}

// diagnosisConditionConcerns derives the concerns a Condition covers from its
// cited facts and its own region qualifiers.
func diagnosisConditionConcerns(refs []model.DiagnosisEvidenceRef, qualifiers *model.ConditionQualifiers, facts BodyStateFactIndex) map[string]bool {
	concerns := map[string]bool{}
	for _, ref := range refs {
		if ref.EvidenceType == model.DiagnosisBodyFact {
			if concern := facts.concern(ref.EvidenceID.String()); concern != "" {
				concerns[concern] = true
			}
		}
	}
	if qualifiers != nil {
		for _, region := range qualifiers.RegionIDs {
			if region = strings.TrimSpace(region); region != "" {
				concerns["region:"+region] = true
			}
		}
	}
	return concerns
}
