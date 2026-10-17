package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakePlanProposer struct {
	byKey  map[string]*model.PlanProposal
	inputs []PlanProposalInput
	err    error
}

func (f *fakePlanProposer) Propose(_ context.Context, owner uuid.UUID, input PlanProposalInput) (*model.PlanProposal, error) {
	f.inputs = append(f.inputs, input)
	if f.err != nil {
		return nil, f.err
	}
	if err := validatePlanDraft(input.Draft); err != nil {
		return nil, err
	}
	if !validPlanGeneration(input) {
		return nil, ErrPlanInvalid
	}
	raw, _ := json.Marshal(input.Draft)
	proposal := &model.PlanProposal{ID: uuid.New(), UserID: owner, Kind: trainingplan.ProposalCreate, State: trainingplan.ProposalPending,
		InitiatedBy: input.InitiatedBy, SourceID: input.SourceID, Draft: raw, IdempotencyKey: input.IdempotencyKey,
		RequestFingerprint: planProposalRequestFingerprint(input), ContentFingerprint: planDigest(input.Draft)}
	if input.Generation != nil {
		proposal.GenerationProvenance = input.Generation.Provenance
		proposal.ReplayInput = input.Generation.ReplayInput
	}
	if f.byKey == nil {
		f.byKey = map[string]*model.PlanProposal{}
	}
	f.byKey[input.IdempotencyKey] = proposal
	return proposal, nil
}

func (f *fakePlanProposer) ProposalByIdempotencyKey(_ context.Context, _ uuid.UUID, key string) (*model.PlanProposal, error) {
	return f.byKey[key], nil
}

const treatmentPlanBlockV1 = `{
	"schemaRevision":"training-plan-agent-proposal-v1","outcome":"proposed",
	"title":"Neck plan","goal":"reduce load","summary":"gentle",
	"content":{"sessions":[{"title":"A","stages":[
		{"phase":"warmup","exercises":[{"exerciseId":"neck_rotation","nameSnapshot":"Neck rotation","sets":1,"target":{"kind":"duration","seconds":30},"side":"both"}]},
		{"phase":"main","exercises":[{"exerciseId":"chin_tuck","nameSnapshot":"Chin tuck","sets":2,"target":{"kind":"repetitions","min":8,"max":12},"restSeconds":30}]}
	]}],"frequency":{"sessionsPerWeek":3,"description":"every other day"},"constraints":["stop on pain"]},
	"evidenceIds":["selected-analysis-only"]
}`

func treatmentAgentRawWithPlan(block string) json.RawMessage {
	raw := `{
		"status":"proposed","summary":"plan","goal":"reduce load","duration_weeks":4,
		"evidence_ids":["selected-analysis-only"],
		"interventions":[{"kind":"exercise","title":"chin tuck","description":"controlled","prescription":{"sets":3}}],
		"daily_habits":[],"expected_timeline":"4 weeks","warning_signs":[],"review_triggers":[],"safety_notes":[],
		"governance":{"verdict":"accepted"}`
	if block != "" {
		raw += `,"plan_proposal":` + block
	}
	return json.RawMessage(raw + `}`)
}

type planGenerationFixture struct {
	userID   uuid.UUID
	analysis *model.DiagnosisAnalysisRecord
	repo     *fakeTreatmentRepo
	plans    *fakePlanProposer
	captured *TreatmentRecommendationRequest
	svc      *TreatmentService
}

func newPlanGenerationFixture(t *testing.T, raw json.RawMessage, body *fakeTreatmentBodyState) *planGenerationFixture {
	t.Helper()
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		DecisionTrace: datatypes.JSON(`{"evidence":{"body_state_authority":"none","body_state_refs":[],"citations":[{"evidence_id":"selected-analysis-only","summary":"audit citation","evidence_type":"body_fact","body_state_authority":"authoritative"}]}}`),
		Candidates:    []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	if body == nil {
		body = &fakeTreatmentBodyState{}
	}
	if body.snapshot == nil {
		body.snapshot = &BodyStateReadModel{UserID: userID, CurrentRevision: 9}
	}
	f := &planGenerationFixture{userID: userID, analysis: analysis, repo: &fakeTreatmentRepo{}, plans: &fakePlanProposer{}, captured: &TreatmentRecommendationRequest{}}
	f.svc = NewTreatmentService(
		f.repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		body,
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh},
		nil,
		fakeTreatmentReasoner{raw: raw, capture: f.captured},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)
	f.svc.AttachPlanProposer(f.plans)
	return f
}

func TestTreatmentPlanGenerationPersistsOnlyAPendingAgentProposal(t *testing.T) {
	f := newPlanGenerationFixture(t, treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil)
	result, err := f.svc.GeneratePlanProposal(context.Background(), f.userID, TreatmentPlanGenerationInput{
		UserConstraints: map[string]any{"minutes": 20}, ChangeReason: " first plan ", IdempotencyKey: "gen-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Replayed || result.Proposal == nil || result.Proposal.State != trainingplan.ProposalPending {
		t.Fatalf("result: %+v", result)
	}
	if f.repo.proposal != nil || f.repo.current != nil {
		t.Fatal("plan generation must not write a Treatment revision")
	}
	if len(f.plans.inputs) != 1 {
		t.Fatalf("Propose calls: %d", len(f.plans.inputs))
	}
	in := f.plans.inputs[0]
	if in.InitiatedBy != trainingplan.InitiatorAgent || in.SourceID != "treatment_agent:"+defaultTreatmentConfigurationID ||
		in.PlanID != uuid.Nil || in.ExpectedRevisionID != uuid.Nil || in.IdempotencyKey != "gen-1" {
		t.Fatalf("propose input: %+v", in)
	}
	if in.Generation == nil || in.Generation.FrozenBodyStateRevision != 9 || f.captured.BodyStateRevision != 9 {
		t.Fatalf("generation not pinned to the frozen BodyState: %+v", in.Generation)
	}
	frozen, err := decodeTreatmentReplayInput(in.Generation.ReplayInput)
	if err != nil || frozen.BodyStateRevision != 9 || !jsonBytesEqual(frozen.UserConstraints, f.captured.UserConstraints) || !jsonBytesEqual(frozen.BodyState, f.captured.BodyState) {
		t.Fatalf("frozen replay input differs from the Agent request: %v", err)
	}
	var provenance map[string]any
	if err := json.Unmarshal(in.Generation.Provenance, &provenance); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"agentConfiguration", "executionProvenance", "governance", "generationDecisionTrace", "rolloutProvenance", "replayInputDigest", "agentOutputDigest"} {
		if provenance[key] == nil {
			t.Fatalf("provenance lacks %s: %s", key, in.Generation.Provenance)
		}
	}
	if provenance["schemaRevision"] != TrainingPlanGenerationProvenanceV1 || provenance["sourceDiagnosisAnalysisId"] != f.analysis.ID.String() ||
		provenance["changeReason"] != "first plan" || provenance["agentConfigurationId"] != defaultTreatmentConfigurationID {
		t.Fatalf("provenance: %s", in.Generation.Provenance)
	}
	d := in.Draft
	if d.Title != "Neck plan" || d.Goal != "reduce load" || d.Content.Schedule.Mode != "frequency_only" ||
		d.Content.Frequency.SessionsPerWeek == nil || *d.Content.Frequency.SessionsPerWeek != 3 || len(d.Content.Constraints) != 1 ||
		len(d.RelatedConditionIDs) != 0 || len(d.RelatedBodyRefs) != 0 {
		t.Fatalf("draft: %+v", d)
	}
	session := d.Content.Sessions[0]
	if _, err := uuid.Parse(session.TemplateID); err != nil {
		t.Fatalf("template id not Go-assigned: %q", session.TemplateID)
	}
	main := session.Stages[1].Exercises[0]
	if _, err := uuid.Parse(main.PlanExerciseID); err != nil || main.ExerciseID != "chin_tuck" || main.Sets != 2 ||
		main.Target.Kind != "repetitions" || main.Target.Min != 8 || main.Target.Max == nil || *main.Target.Max != 12 ||
		main.RestSeconds == nil || *main.RestSeconds != 30 {
		t.Fatalf("exercise: %+v", main)
	}
}

func TestTreatmentPlanGenerationReplaysKeyWithoutCallingAgent(t *testing.T) {
	f := newPlanGenerationFixture(t, treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil)
	ctx := context.Background()
	input := TreatmentPlanGenerationInput{DiagnosisAnalysisID: f.analysis.ID, UserConstraints: map[string]any{"minutes": 20}, IdempotencyKey: "gen-replay"}
	first, err := f.svc.GeneratePlanProposal(ctx, f.userID, input)
	if err != nil {
		t.Fatal(err)
	}
	*f.captured = TreatmentRecommendationRequest{}
	again, err := f.svc.GeneratePlanProposal(ctx, f.userID, input)
	if err != nil || !again.Replayed || again.Proposal.ID != first.Proposal.ID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if f.captured.ConfigurationID != "" || len(f.plans.inputs) != 1 {
		t.Fatal("a replayed key must not re-run the Agent or propose again")
	}
	changed := input
	changed.UserConstraints = map[string]any{"minutes": 40}
	if _, err := f.svc.GeneratePlanProposal(ctx, f.userID, changed); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("changed command under the same key: %v", err)
	}
	f.plans.byKey["user-key"] = &model.PlanProposal{ID: uuid.New(), UserID: f.userID, IdempotencyKey: "user-key", InitiatedBy: trainingplan.InitiatorUser, RequestFingerprint: strings.Repeat("a", 64)}
	userKey := input
	userKey.IdempotencyKey = "user-key"
	if _, err := f.svc.GeneratePlanProposal(ctx, f.userID, userKey); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("user proposal key reused for generation: %v", err)
	}
	if f.captured.ConfigurationID != "" {
		t.Fatal("a conflicting key must be refused before the Agent runs")
	}
}

func TestTreatmentPlanGenerationRejectsMalformedAgentPlanWithNoWrite(t *testing.T) {
	replace := func(old, new string) string { return strings.Replace(treatmentPlanBlockV1, old, new, 1) }
	cases := map[string]struct {
		block string
		want  error
	}{
		"missing block":          {"", ErrTreatmentPlanUnavailable},
		"null block":             {"null", ErrTreatmentPlanUnavailable},
		"unavailable outcome":    {`{"schemaRevision":"training-plan-agent-proposal-v1","outcome":"unavailable","reason":"no_plan"}`, ErrTreatmentPlanUnavailable},
		"unknown schema":         {replace(`training-plan-agent-proposal-v1`, `training-plan-agent-proposal-v9`), ErrTreatmentPlanOutputInvalid},
		"unknown outcome":        {replace(`"outcome":"proposed"`, `"outcome":"accepted"`), ErrTreatmentPlanOutputInvalid},
		"agent template id":      {replace(`{"title":"A",`, `{"templateId":"agent-template","title":"A",`), ErrTreatmentPlanOutputInvalid},
		"agent entry id":         {replace(`{"exerciseId":"chin_tuck",`, `{"planExerciseId":"agent-entry","exerciseId":"chin_tuck",`), ErrTreatmentPlanOutputInvalid},
		"agent plan id":          {replace(`"title":"Neck plan",`, `"planId":"11111111-1111-4111-8111-111111111111","title":"Neck plan",`), ErrTreatmentPlanOutputInvalid},
		"agent decision":         {replace(`"title":"Neck plan",`, `"state":"accepted","title":"Neck plan",`), ErrTreatmentPlanOutputInvalid},
		"agent schedule":         {replace(`"constraints":["stop on pain"]`, `"constraints":[],"schedule":{"mode":"calendar"}`), ErrTreatmentPlanOutputInvalid},
		"missing content":        {`{"schemaRevision":"training-plan-agent-proposal-v1","outcome":"proposed","title":"t","goal":"g","summary":"s","evidenceIds":[]}`, ErrTreatmentPlanOutputInvalid},
		"foreign evidence":       {replace(`"evidenceIds":["selected-analysis-only"]`, `"evidenceIds":["never-cited"]`), ErrTreatmentPlanOutputInvalid},
		"zero sets":              {replace(`"sets":2`, `"sets":0`), ErrTreatmentPlanOutputInvalid},
		"bad phase":              {replace(`"phase":"main"`, `"phase":"stretch"`), ErrTreatmentPlanOutputInvalid},
		"target kind mismatch":   {replace(`{"kind":"duration","seconds":30}`, `{"kind":"duration","min":30}`), ErrTreatmentPlanOutputInvalid},
		"empty title":            {replace(`"title":"Neck plan"`, `"title":" "`), ErrTreatmentPlanOutputInvalid},
		"no sessions":            {`{"schemaRevision":"training-plan-agent-proposal-v1","outcome":"proposed","title":"t","goal":"g","summary":"s","content":{"sessions":[],"frequency":{},"constraints":[]},"evidenceIds":[]}`, ErrTreatmentPlanOutputInvalid},
		"zero sessions per week": {replace(`"sessionsPerWeek":3`, `"sessionsPerWeek":0`), ErrTreatmentPlanOutputInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPlanGenerationFixture(t, treatmentAgentRawWithPlan(tc.block), nil)
			_, err := f.svc.GeneratePlanProposal(context.Background(), f.userID, TreatmentPlanGenerationInput{IdempotencyKey: "gen-bad"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if len(f.plans.inputs) != 0 || f.repo.proposal != nil {
				t.Fatal("a malformed Agent plan must persist nothing")
			}
		})
	}
}

func TestTreatmentPlanGenerationKeepsGoGatesBeforeTheAgent(t *testing.T) {
	ctx := context.Background()
	blocked := newPlanGenerationFixture(t, treatmentAgentRawWithPlan(treatmentPlanBlockV1), &fakeTreatmentBodyState{safetyBlocked: true})
	if _, err := blocked.svc.GeneratePlanProposal(ctx, blocked.userID, TreatmentPlanGenerationInput{IdempotencyKey: "gen-safety"}); !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("safety block: %v", err)
	}
	if blocked.captured.ConfigurationID != "" || len(blocked.plans.inputs) != 0 {
		t.Fatal("a Go safety block must stop before the Agent and write nothing")
	}

	rejected := newPlanGenerationFixture(t, json.RawMessage(`{"governance":{"verdict":"rejected"},"safety_fallback":"x"}`), nil)
	if _, err := rejected.svc.GeneratePlanProposal(ctx, rejected.userID, TreatmentPlanGenerationInput{IdempotencyKey: "gen-governance"}); !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("governance rejection: %v", err)
	}
	if len(rejected.plans.inputs) != 0 {
		t.Fatal("a governance rejection must write nothing")
	}

	invalidKey := newPlanGenerationFixture(t, treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil)
	if _, err := invalidKey.svc.GeneratePlanProposal(ctx, invalidKey.userID, TreatmentPlanGenerationInput{IdempotencyKey: " padded "}); !errors.Is(err, ErrPlanInvalid) {
		t.Fatalf("invalid key: %v", err)
	}

	unwired := newPlanGenerationFixture(t, treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil)
	unwired.svc.plans = nil
	if _, err := unwired.svc.GeneratePlanProposal(ctx, unwired.userID, TreatmentPlanGenerationInput{IdempotencyKey: "gen-unwired"}); !errors.Is(err, ErrTreatmentPlanGenerationUnavailable) {
		t.Fatalf("unwired: %v", err)
	}
	if unwired.captured.ConfigurationID != "" {
		t.Fatal("an unwired proposer must refuse before the Agent runs")
	}

	stale := newPlanGenerationFixture(t, treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil)
	stale.plans.err = ErrPlanConflict
	if _, err := stale.svc.GeneratePlanProposal(ctx, stale.userID, TreatmentPlanGenerationInput{IdempotencyKey: "gen-stale"}); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("stale context must surface as a conflict: %v", err)
	}
}
