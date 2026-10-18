package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func freshnessRevision(revision int64, changeType, changes string) model.BodyStateRevision {
	return model.BodyStateRevision{Revision: revision, ChangeType: changeType, Changes: datatypes.JSON(changes)}
}

func evaluateTestFreshness(basis diagnosisFreshnessBasis, changes diagnosisFreshnessChanges) DiagnosisFreshness {
	builder := newDiagnosisFreshnessBuilder(basis.BodyStateRevision, changes.Head, 3)
	evaluateDiagnosisFreshness(builder, basis, changes)
	return builder.result()
}

func freshnessCodes(f DiagnosisFreshness) []string {
	codes := make([]string, 0, len(f.Reasons))
	for _, reason := range f.Reasons {
		codes = append(codes, reason.Code)
	}
	return codes
}

func TestDiagnosisFreshnessPolicyClassifiesChangesAfterTheBasis(t *testing.T) {
	cited, related, unrelated, observation, episode := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	basis := diagnosisFreshnessBasis{
		BodyStateRevision: 4,
		Evidence: []model.DiagnosisEvidenceRef{
			{EvidenceType: model.DiagnosisBodyFact, EvidenceID: cited, Version: "2"},
			{EvidenceType: model.DiagnosisBodyObservation, EvidenceID: observation, Version: "3"},
			{EvidenceType: model.DiagnosisSymptomEpisode, EvidenceID: episode, Version: "4"},
		},
		Concerns: map[string]bool{"region:knee": true},
	}
	facts := BodyStateFactIndex{
		cited.String(): {Kind: "symptom", ConcernKey: "region:knee"}, related.String(): {Kind: "symptom", ConcernKey: "region:knee"},
		unrelated.String(): {Kind: "symptom", ConcernKey: "region:neck"},
	}
	base := diagnosisFreshnessChanges{Head: 12, Complete: true, Facts: facts, Unavailable: map[string]bool{}}

	t.Run("unrelated and pre-basis revisions keep it current", func(t *testing.T) {
		changes := base
		changes.Revisions = []model.BodyStateRevision{
			freshnessRevision(3, "safety.screened", `{}`),
			freshnessRevision(4, "facts.withdrawn", `{"target_fact_id":"`+cited.String()+`"}`),
			freshnessRevision(5, "facts.recorded", `{"fact_ids":["`+unrelated.String()+`"]}`),
			freshnessRevision(6, "observation.withdrawn", `{"target_observation_id":"`+uuid.NewString()+`"}`),
			freshnessRevision(7, "hypothesis.recorded", `{}`),
		}
		got := evaluateTestFreshness(basis, changes)
		if got.State != DiagnosisFreshnessCurrent || len(got.Reasons) != 0 || got.BasedOnBodyStateRevision != 4 || got.EvaluatedBodyStateRevision != 12 {
			t.Fatalf("a revision-number difference alone must not flag review: %+v", got)
		}
	})

	t.Run("safety and related evidence recommend review", func(t *testing.T) {
		changes := base
		changes.Revisions = []model.BodyStateRevision{
			freshnessRevision(5, "safety.restriction_resolved", `{}`),
			freshnessRevision(6, "facts.recorded", `{"fact_ids":["`+unrelated.String()+`","`+related.String()+`"]}`),
			freshnessRevision(7, "episode.status", `{"episode_id":"`+episode.String()+`","fact_id":"`+related.String()+`"}`),
			freshnessRevision(8, "episode.attach", `{"episode_id":"`+uuid.NewString()+`","fact_id":"`+related.String()+`"}`),
			freshnessRevision(9, "facts.withdrawn", `{"target_fact_id":"`+related.String()+`"}`),
		}
		got := evaluateTestFreshness(basis, changes)
		want := []string{freshnessSafetyChanged, freshnessRelatedFactRecorded, freshnessCitedEpisodeChanged, freshnessRelatedEpisodeChanged, freshnessRelatedFactWithdrawn}
		if got.State != DiagnosisFreshnessReviewRecommended || len(got.Reasons) != len(want) {
			t.Fatalf("review reasons %+v", got)
		}
		for i, code := range want {
			if got.Reasons[i].Code != code || got.Reasons[i].BodyStateRevision == nil || *got.Reasons[i].BodyStateRevision != int64(i+5) {
				t.Fatalf("reason %d = %+v, want %s at revision %d", i, got.Reasons[i], code, i+5)
			}
		}
		if got.Reasons[1].EvidenceID == nil || *got.Reasons[1].EvidenceID != related {
			t.Fatalf("related fact reason must name the related fact: %+v", got.Reasons[1])
		}
	})

	t.Run("a corrected or withdrawn cited source is stale", func(t *testing.T) {
		changes := base
		replacement := uuid.New()
		changes.Facts = BodyStateFactIndex{cited.String(): facts[cited.String()], replacement.String(): {Kind: "symptom", ConcernKey: "region:knee"}}
		changes.Revisions = []model.BodyStateRevision{
			freshnessRevision(5, "facts.corrected", `{"target_fact_id":"`+cited.String()+`","fact_ids":["`+replacement.String()+`"]}`),
			freshnessRevision(6, "observation.withdrawn", `{"target_observation_id":"`+observation.String()+`"}`),
		}
		changes.Unavailable = map[string]bool{
			evidenceKey(basis.Evidence[0]): true, evidenceKey(basis.Evidence[1]): true,
		}
		got := evaluateTestFreshness(basis, changes)
		codes := freshnessCodes(got)
		if got.State != DiagnosisFreshnessStale || len(codes) != 2 || codes[0] != freshnessCitedEvidenceCorrected || codes[1] != freshnessCitedEvidenceWithdrawn {
			t.Fatalf("revision-attributed reasons must not be repeated by the live check: %+v", got)
		}
		if got.Reasons[0].EvidenceType == nil || *got.Reasons[0].EvidenceType != model.DiagnosisBodyFact || *got.Reasons[0].EvidenceID != cited {
			t.Fatalf("stale reason must name the cited fact: %+v", got.Reasons[0])
		}
	})

	t.Run("the live source authority catches unattributed loss", func(t *testing.T) {
		changes := base
		changes.Unavailable = map[string]bool{evidenceKey(basis.Evidence[2]): true}
		got := evaluateTestFreshness(basis, changes)
		if got.State != DiagnosisFreshnessStale || len(got.Reasons) != 1 || got.Reasons[0].Code != freshnessCitedEvidenceUnavailable ||
			*got.Reasons[0].EvidenceID != episode || got.Reasons[0].BodyStateRevision != nil {
			t.Fatalf("unavailable episode %+v", got)
		}
	})

	t.Run("an incomplete window recommends review instead of claiming current", func(t *testing.T) {
		changes := base
		changes.Complete = false
		if got := evaluateTestFreshness(basis, changes); got.State != DiagnosisFreshnessReviewRecommended || got.Reasons[0].Code != freshnessChangeWindowExceeded {
			t.Fatalf("truncated window %+v", got)
		}
		basis := basis
		basis.BodyStateRevision = 12
		if got := evaluateTestFreshness(basis, changes); got.State != DiagnosisFreshnessCurrent {
			t.Fatalf("a basis at head has nothing unread: %+v", got)
		}
	})
}

func TestDiagnosisFreshnessBuilderDeduplicatesAndRanksStale(t *testing.T) {
	at := int64(5)
	b := newDiagnosisFreshnessBuilder(1, 5, 2)
	b.add(DiagnosisFreshnessReason{Code: freshnessSafetyChanged, BodyStateRevision: &at, ChangeType: "safety.screened"})
	b.add(DiagnosisFreshnessReason{Code: freshnessSafetyChanged, BodyStateRevision: &at, ChangeType: "safety.screened"})
	if got := b.result(); got.State != DiagnosisFreshnessReviewRecommended || len(got.Reasons) != 1 {
		t.Fatalf("duplicate reason %+v", got)
	}
	b.add(DiagnosisFreshnessReason{Code: freshnessSourceClaimUnavailable, ClaimID: "claim-1"})
	b.add(DiagnosisFreshnessReason{Code: freshnessRelatedFactRecorded, BodyStateRevision: &at})
	if got := b.result(); got.State != DiagnosisFreshnessStale || len(got.Reasons) != 3 || !(&got).Stale() {
		t.Fatalf("stale must dominate review %+v", got)
	}
}

func TestDiagnosisConditionConcernsUseCitedFactsAndRegions(t *testing.T) {
	fact, observation := uuid.New(), uuid.New()
	concerns := diagnosisConditionConcerns([]model.DiagnosisEvidenceRef{
		{EvidenceType: model.DiagnosisBodyFact, EvidenceID: fact, Version: "1"},
		{EvidenceType: model.DiagnosisBodyObservation, EvidenceID: observation, Version: "1"},
	}, &model.ConditionQualifiers{RegionIDs: []string{"shoulder_left", " "}}, BodyStateFactIndex{
		fact.String(): {Kind: "symptom", ConcernKey: "general"}, observation.String(): {Kind: "symptom", ConcernKey: "region:x"},
	})
	if len(concerns) != 2 || !concerns["general"] || !concerns["region:shoulder_left"] {
		t.Fatalf("concerns %#v", concerns)
	}
}

func TestDiagnosisSourceRefusedSeparatesInfrastructureFailures(t *testing.T) {
	if refused, err := diagnosisSourceRefused(nil); refused || err != nil {
		t.Fatal("nil error is a qualified source")
	}
	if refused, err := diagnosisSourceRefused(errors.New("condition fact is no longer asserted")); !refused || err != nil {
		t.Fatal("a policy refusal is an unavailable source")
	}
	if refused, err := diagnosisSourceRefused(context.DeadlineExceeded); refused || err == nil {
		t.Fatal("a timeout must surface as an error, not as stale")
	}
}

type fakeDiagnosisFreshness struct {
	state string
	err   error
	calls int
}

func (f *fakeDiagnosisFreshness) EvaluateConditions(_ context.Context, owner uuid.UUID, view *NativeDiagnosisView) (map[uuid.UUID]DiagnosisFreshness, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	result := map[uuid.UUID]DiagnosisFreshness{}
	for _, condition := range view.ConditionEntries {
		result[condition.ConditionID] = DiagnosisFreshness{State: f.state, EvaluatedDiagnosisRevision: view.CurrentRevision, Reasons: []DiagnosisFreshnessReason{}}
	}
	return result, nil
}

func TestTreatmentGenerationRefusesAStaleAdoptedCondition(t *testing.T) {
	for _, tc := range []struct {
		state   string
		err     error
		wantErr error
	}{
		{state: DiagnosisFreshnessStale, wantErr: ErrTreatmentDiagnosisStale},
		{state: DiagnosisFreshnessReviewRecommended},
		{state: DiagnosisFreshnessCurrent},
		{err: errors.New("freshness unavailable")},
	} {
		h := newTreatmentGenerationHarness(7)
		freshness := &fakeDiagnosisFreshness{state: tc.state, err: tc.err}
		svc := h.service(treatmentAgentRawWithPlan(treatmentPlanBlockV1))
		svc.AttachDiagnosisFreshness(freshness)
		_, err := svc.GeneratePlanProposal(context.Background(), h.userID, TreatmentPlanGenerationInput{IdempotencyKey: "gen-" + uuid.NewString(), UserConstraints: map[string]any{}})
		switch {
		case tc.err != nil:
			if err == nil || h.captured.ConfigurationID != "" {
				t.Fatalf("a failed freshness read must fail closed before the Agent: %v", err)
			}
		case tc.wantErr != nil:
			if !errors.Is(err, tc.wantErr) || h.captured.ConfigurationID != "" || len(h.plans.inputs) != 0 {
				t.Fatalf("%s: expected DIAGNOSIS_STALE before the Agent, got %v", tc.state, err)
			}
		default:
			if err != nil || len(h.plans.inputs) != 1 {
				t.Fatalf("%s must not block generation: %v", tc.state, err)
			}
		}
		if freshness.calls != 1 {
			t.Fatalf("freshness evaluated %d times", freshness.calls)
		}
	}
	// The pinned-revision check still runs first and never consults freshness.
	h := newTreatmentGenerationHarness(7)
	presented := int64(1)
	freshness := &fakeDiagnosisFreshness{state: DiagnosisFreshnessCurrent}
	svc := h.service(treatmentAgentRawWithPlan(treatmentPlanBlockV1))
	svc.AttachDiagnosisFreshness(freshness)
	if _, err := svc.GeneratePlanProposal(context.Background(), h.userID, TreatmentPlanGenerationInput{ExpectedDiagnosisRevision: &presented, IdempotencyKey: "gen-" + uuid.NewString()}); !errors.Is(err, ErrTreatmentDiagnosisStale) || freshness.calls != 0 {
		t.Fatalf("pinned revision must return DIAGNOSIS_STALE first: %v calls=%d", err, freshness.calls)
	}
}

func TestWorkspaceStaleAdoptedConditionOwesReviewAndBlocksGeneration(t *testing.T) {
	for state, want := range map[string]bool{DiagnosisFreshnessStale: true, DiagnosisFreshnessReviewRecommended: false, DiagnosisFreshnessCurrent: false} {
		diagnosis := workspaceDiagnosis(model.ConditionAdopted)
		diagnosis.ConditionEntries[0].Freshness = &DiagnosisFreshness{State: state}
		caps := deriveWorkspaceCapabilities(screenedWorkspaceBodyState(7), diagnosis, nil, nil)
		if caps.RequiresDiagnosisReview != want || caps.CanGenerateTreatment == want {
			t.Fatalf("%s: review=%v generate=%v", state, caps.RequiresDiagnosisReview, caps.CanGenerateTreatment)
		}
	}
	// An inactive stale Condition shapes no plan and owes nothing.
	diagnosis := workspaceDiagnosis(model.ConditionAdopted)
	inactive := testAdoptedCondition(diagnosis.UserID, 2, model.ConditionInactive)
	inactive.Freshness = &DiagnosisFreshness{State: DiagnosisFreshnessStale}
	diagnosis.ConditionEntries = append(diagnosis.ConditionEntries, inactive)
	if caps := deriveWorkspaceCapabilities(screenedWorkspaceBodyState(7), diagnosis, nil, nil); caps.RequiresDiagnosisReview || !caps.CanGenerateTreatment {
		t.Fatalf("inactive stale Condition gated the loop: %+v", caps)
	}
}
