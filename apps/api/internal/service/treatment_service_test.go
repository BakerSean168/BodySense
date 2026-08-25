package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeTreatmentRepo struct {
	proposal                        *model.TreatmentRevision
	interventions                   []model.Intervention
	current                         *model.Treatment
	status                          string
	statusReasons                   datatypes.JSON
	statusUpdates                   int
	acceptCalled                    bool
	acceptConflict                  bool
	acceptExpectedBodyStateRevision int64
	acceptDecisionTrace             datatypes.JSON
	storedOutcome                   *model.Outcome
	storedOutcomeCreated            bool
	updateOutcomeRevision           int64
	updateOutcomeCalls              int
}

func (r *fakeTreatmentRepo) CreateProposal(_ context.Context, userID uuid.UUID, revision model.TreatmentRevision, interventions []model.Intervention) (*model.Treatment, *model.TreatmentRevision, error) {
	revision.ID = uuid.New()
	revision.TreatmentID = uuid.New()
	revision.Revision = 1
	revision.AcceptanceState = model.TreatmentAcceptanceProposed
	revision.Interventions = interventions
	r.proposal = &revision
	r.interventions = interventions
	return &model.Treatment{ID: revision.TreatmentID, UserID: userID, CurrentRevision: 0}, &revision, nil
}
func (r *fakeTreatmentRepo) AcceptRevision(_ context.Context, userID, revisionID uuid.UUID, expectedBodyStateRevision int64, acceptanceDecisionTrace datatypes.JSON) (*model.Treatment, *model.TreatmentRevision, bool, error) {
	r.acceptCalled = true
	r.acceptExpectedBodyStateRevision = expectedBodyStateRevision
	r.acceptDecisionTrace = acceptanceDecisionTrace
	if r.acceptConflict {
		return nil, nil, false, nil
	}
	if r.proposal == nil || r.proposal.ID != revisionID {
		return nil, nil, false, errors.New("not found")
	}
	copy := *r.proposal
	copy.AcceptanceState = model.TreatmentAcceptanceAccepted
	copy.LifecycleState = model.TreatmentStatusActive
	copy.AcceptanceDecisionTrace = acceptanceDecisionTrace
	r.proposal.AcceptanceDecisionTrace = acceptanceDecisionTrace
	r.current = &model.Treatment{ID: copy.TreatmentID, UserID: userID, CurrentRevision: copy.Revision, Status: model.TreatmentStatusActive, Current: &copy}
	return r.current, &copy, true, nil
}
func (r *fakeTreatmentRepo) RejectRevision(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *fakeTreatmentRepo) SetStatus(_ context.Context, userID uuid.UUID, status string, reasons datatypes.JSON) (*model.Treatment, error) {
	r.status = status
	r.statusReasons = reasons
	r.statusUpdates++
	if r.current == nil {
		r.current = &model.Treatment{UserID: userID}
	}
	r.current.Status = status
	r.current.StatusReasons = reasons
	return r.current, nil
}
func (r *fakeTreatmentRepo) GetCurrent(context.Context, uuid.UUID) (*model.Treatment, error) {
	return r.current, nil
}
func (r *fakeTreatmentRepo) GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.TreatmentRevision, error) {
	return r.proposal, nil
}
func (r *fakeTreatmentRepo) ListRevisions(context.Context, uuid.UUID, int) ([]model.TreatmentRevision, error) {
	return nil, nil
}
func (r *fakeTreatmentRepo) GetIntervention(context.Context, uuid.UUID, uuid.UUID) (*model.Intervention, error) {
	return nil, nil
}
func (r *fakeTreatmentRepo) CreateOutcome(_ context.Context, outcome *model.Outcome) (*model.Outcome, bool, error) {
	if r.storedOutcome != nil {
		return r.storedOutcome, r.storedOutcomeCreated, nil
	}
	r.storedOutcome = outcome
	r.storedOutcomeCreated = true
	return outcome, true, nil
}
func (r *fakeTreatmentRepo) UpdateOutcomeBodyStateRevision(_ context.Context, _ uuid.UUID, _ uuid.UUID, revision int64) error {
	r.updateOutcomeCalls++
	r.updateOutcomeRevision = revision
	if r.storedOutcome != nil {
		r.storedOutcome.BodyStateRevision = &revision
	}
	return nil
}
func (r *fakeTreatmentRepo) ListOutcomes(context.Context, uuid.UUID, int) ([]model.Outcome, error) {
	return nil, nil
}

type fakeTreatmentDiagnosis struct {
	analysis    *model.DiagnosisAnalysisRecord
	assessments []model.DiagnosisCandidateAssessment
}

func (f *fakeTreatmentDiagnosis) GetByID(context.Context, uuid.UUID, uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	return f.analysis, nil
}
func (f *fakeTreatmentDiagnosis) GetLatest(context.Context, uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	return f.analysis, nil
}
func (f *fakeTreatmentDiagnosis) ListAssessments(context.Context, uuid.UUID, uuid.UUID) ([]model.DiagnosisCandidateAssessment, error) {
	return f.assessments, nil
}
func (f *fakeTreatmentDiagnosis) PublicPayload(analysis *model.DiagnosisAnalysisRecord) map[string]any {
	return map[string]any{"analysis_id": analysis.ID}
}

type fakeTreatmentBodyState struct {
	snapshot           *BodyStateSnapshot
	revisions          []model.BodyStateRevision
	recordOutcomeCalls int
	recordOutcomeErr   error
	recordOutcomeRev   int64
}

func (f *fakeTreatmentBodyState) GetSnapshot(context.Context, uuid.UUID, int) (*BodyStateSnapshot, error) {
	return f.snapshot, nil
}
func (f *fakeTreatmentBodyState) ListRevisionsAfter(context.Context, uuid.UUID, int64, int) ([]model.BodyStateRevision, error) {
	return f.revisions, nil
}
func (f *fakeTreatmentBodyState) ListEvidence(context.Context, uuid.UUID, int) ([]model.BodyStateEvidence, error) {
	return nil, nil
}
func (f *fakeTreatmentBodyState) RecordOutcome(context.Context, uuid.UUID, model.Outcome) (*model.BodyStateRevision, error) {
	f.recordOutcomeCalls++
	if f.recordOutcomeErr != nil {
		return nil, f.recordOutcomeErr
	}
	revision := f.recordOutcomeRev
	if revision == 0 {
		revision = 12
	}
	return &model.BodyStateRevision{Revision: revision}, nil
}

type fakeTreatmentFreshness struct{ state string }

func (f fakeTreatmentFreshness) GetOrEvaluate(context.Context, uuid.UUID, *model.DiagnosisAnalysisRecord) (*model.DiagnosisAnalysisFreshness, error) {
	return &model.DiagnosisAnalysisFreshness{State: f.state}, nil
}

type testTreatmentUnitOfWork struct {
	called *bool
}

func (u testTreatmentUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if u.called != nil {
		*u.called = true
	}
	return fn(ctx)
}

type testTreatmentDeploymentPolicy struct {
	configurationID       string
	shadowConfigurationID string
	stage                 string
	subjectBucket         int
	canaryBPS             int
}

func (p testTreatmentDeploymentPolicy) SelectTreatmentRoute(_ string) TreatmentRouteSelection {
	served := p.configurationID
	if served == "" {
		served = defaultTreatmentConfigurationID
	}
	stage := p.stage
	if stage == "" {
		stage = TreatmentRolloutChampion
	}
	champion := defaultTreatmentConfigurationID
	challenger := treatmentEvidenceGapConfigurationID
	if served == treatmentEvidenceGapConfigurationID && p.shadowConfigurationID == defaultTreatmentConfigurationID {
		champion, challenger = defaultTreatmentConfigurationID, treatmentEvidenceGapConfigurationID
	}
	registration := knownTreatmentConfigurations[served]
	route := TreatmentRouteSelection{
		Stage: stage, SubjectBucket: p.subjectBucket, ServedConfigurationID: served,
		ServedDecisionPolicyRevision: registration.DecisionPolicyRevision,
		ShadowConfigurationID:        p.shadowConfigurationID,
		ChampionConfigurationID:      champion, ChallengerConfigurationID: challenger,
		CanaryBPS: p.canaryBPS,
	}
	if route.ShadowConfigurationID != "" {
		route.ShadowDecisionPolicyRevision = knownTreatmentConfigurations[route.ShadowConfigurationID].DecisionPolicyRevision
	}
	return route
}

type fakeTreatmentRolloutObserver struct {
	calls      int
	userID     uuid.UUID
	revisionID uuid.UUID
	route      TreatmentRouteSelection
	err        error
}

func (f *fakeTreatmentRolloutObserver) ObserveProposal(
	_ context.Context,
	userID uuid.UUID,
	route TreatmentRouteSelection,
	revisionID uuid.UUID,
) error {
	f.calls++
	f.userID = userID
	f.route = route
	f.revisionID = revisionID
	return f.err
}

type fakeTreatmentReasoner struct {
	raw     json.RawMessage
	capture *TreatmentRecommendationRequest
}

func (f fakeTreatmentReasoner) RecommendTreatment(_ context.Context, req TreatmentRecommendationRequest) (json.RawMessage, error) {
	if f.capture != nil {
		*f.capture = req
	}
	if len(f.raw) == 0 {
		return f.raw, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(f.raw, &payload); err != nil {
		return f.raw, nil
	}
	if _, exists := payload["agent_configuration"]; !exists {
		payload["agent_configuration"] = map[string]any{
			"id": req.ConfigurationID, "role": "treatment",
			"decision_policy_revision": TreatmentDecisionPolicyV1,
		}
	}
	if _, exists := payload["execution_provenance"]; !exists {
		payload["execution_provenance"] = map[string]any{
			"status": "executed", "runtime": "pydantic-ai",
			"logical_model": treatmentLogicalModelV1,
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}
func jsonBytesEqual(left, right json.RawMessage) bool {
	var leftValue any
	var rightValue any
	return json.Unmarshal(left, &leftValue) == nil &&
		json.Unmarshal(right, &rightValue) == nil &&
		reflect.DeepEqual(leftValue, rightValue)
}

func TestTreatmentGenerateCreatesProposalWithoutMakingItCurrent(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{}
	var captured TreatmentRecommendationRequest
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{
			analysis: analysis,
			assessments: []model.DiagnosisCandidateAssessment{{
				CandidateID: analysis.Candidates[0].ID,
				State:       "confirmed",
			}},
		},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 9, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh},
		nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{
			"status":"proposed","summary":"plan","goal":"reduce load","duration_weeks":4,
			"interventions":[{"kind":"exercise","title":"chin tuck","description":"controlled","prescription":{"sets":3}}],
			"daily_habits":[],"expected_timeline":"4 weeks","warning_signs":[],"review_triggers":[],"safety_notes":[]
		}`), capture: &captured},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)
	revision, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID})
	if err != nil {
		t.Fatalf("GenerateProposal returned error: %v", err)
	}
	if revision.AcceptanceState != model.TreatmentAcceptanceProposed {
		t.Fatalf("AI result must remain proposed, got %s", revision.AcceptanceState)
	}
	if repo.current != nil {
		t.Fatal("creating a proposal must not silently replace current treatment")
	}
	if revision.SourceBodyStateRevision != 9 || revision.SourceDiagnosisAnalysisID != analysis.ID {
		t.Fatalf("proposal did not pin exact inputs: %#v", revision)
	}
	if captured.ConfigurationID != defaultTreatmentConfigurationID {
		t.Fatalf("Treatment request lost config identity: %#v", captured)
	}
	if revision.AgentConfigurationID != defaultTreatmentConfigurationID || len(revision.AgentConfiguration) == 0 || len(revision.ExecutionProvenance) == 0 {
		t.Fatalf("proposal lost Agent provenance: %#v", revision)
	}
	frozen, err := decodeTreatmentReplayInput(json.RawMessage(revision.ReplayInput))
	if err != nil {
		t.Fatalf("decode frozen Treatment replay input: %v", err)
	}
	if frozen.BodyStateRevision != captured.BodyStateRevision ||
		!jsonBytesEqual(frozen.BodyState, captured.BodyState) ||
		!jsonBytesEqual(frozen.DiagnosisAnalysis, captured.DiagnosisAnalysis) ||
		!jsonBytesEqual(frozen.CandidateAssessments, captured.CandidateAssessments) ||
		!jsonBytesEqual(frozen.Profile, captured.Profile) ||
		!jsonBytesEqual(frozen.UserConstraints, captured.UserConstraints) ||
		!jsonBytesEqual(frozen.Evidence, captured.Evidence) {
		t.Fatalf("frozen replay input differs from actual Agent request: frozen=%#v captured=%#v", frozen, captured)
	}
	serialized, err := json.Marshal(revision)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), "replay_input") {
		t.Fatalf("private replay input leaked through TreatmentRevision JSON: %s", serialized)
	}
	if len(revision.Interventions) != 1 {
		t.Fatalf("expected one intervention, got %#v", revision.Interventions)
	}
}

func TestTreatmentGenerationRejectsConfigurationMismatchBeforePersistence(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 7, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh},
		nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{
			"status":"proposed","summary":"plan","goal":"reduce load","duration_weeks":4,
			"interventions":[{"kind":"exercise","title":"chin tuck","description":"controlled","prescription":{}}],
			"expected_timeline":"4 weeks",
			"agent_configuration":{"id":"treat-config-wrong","role":"treatment"},
			"execution_provenance":{"status":"executed","runtime":"pydantic-ai"}
		}`)},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	_, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID})
	if !errors.Is(err, ErrTreatmentConfigurationMismatch) {
		t.Fatalf("expected configuration mismatch, got %v", err)
	}
	if repo.proposal != nil {
		t.Fatal("configuration mismatch must fail before CreateProposal")
	}
}

func TestTreatmentGenerationBlockedBySafetyState(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{ID: uuid.New(), UserID: userID, Status: "completed", Candidates: []model.DiagnosisCandidateRecord{{Name: "x", Confidence: "中"}}}
	svc := NewTreatmentService(
		&fakeTreatmentRepo{}, &fakeTreatmentDiagnosis{analysis: analysis},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{CurrentRevision: 3, SafetyState: json.RawMessage(`{"has_red_flags":true,"status":"requires_review"}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{}`)},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)
	_, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID})
	if !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("expected safety block, got %v", err)
	}
}

func TestTreatmentReviewUnrelatedChangeDoesNotRewritePlan(t *testing.T) {
	analysis := &model.DiagnosisAnalysisRecord{Candidates: []model.DiagnosisCandidateRecord{{ConcernKey: "region:neck", BasisFactIDs: datatypes.JSON(`[]`)}}}
	status, reasons := EvaluateTreatmentReviewPolicy(analysis, []model.BodyStateRevision{{
		Revision: 4, ChangeType: "fact.added",
		Changes: datatypes.JSON(`{"fact":{"concern_key":"region:sleep","kind":"lifestyle"}}`),
	}})
	if status != model.TreatmentStatusActive || len(reasons) != 0 {
		t.Fatalf("unrelated change should keep active plan: status=%s reasons=%#v", status, reasons)
	}
}

func TestTreatmentReviewRelatedChangeRecommendsReview(t *testing.T) {
	analysis := &model.DiagnosisAnalysisRecord{Candidates: []model.DiagnosisCandidateRecord{{ConcernKey: "region:neck", BasisFactIDs: datatypes.JSON(`[]`)}}}
	status, reasons := EvaluateTreatmentReviewPolicy(analysis, []model.BodyStateRevision{{
		Revision: 5, ChangeType: "fact.added",
		Changes: datatypes.JSON(`{"fact":{"concern_key":"region:neck","kind":"discomfort"}}`),
	}})
	if status != model.TreatmentStatusReviewRecommended || len(reasons) != 1 {
		t.Fatalf("related change should recommend review: status=%s reasons=%#v", status, reasons)
	}
}

func TestTreatmentGenerationRequiresCandidateAssessment(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	svc := NewTreatmentService(
		&fakeTreatmentRepo{},
		&fakeTreatmentDiagnosis{analysis: analysis},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 7, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh},
		nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{
			"status":"proposed","summary":"plan","goal":"reduce load","duration_weeks":4,
			"interventions":[{"kind":"exercise","title":"chin tuck","description":"controlled","prescription":{"sets":3}}]
		}`)},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	_, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID})
	if !errors.Is(err, ErrTreatmentCandidateAssessmentRequired) {
		t.Fatalf("expected candidate assessment gate, got %v", err)
	}
}

func TestTreatmentAcceptRevalidatesSafetyState(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{proposal: &model.TreatmentRevision{
		ID: uuid.New(), TreatmentID: uuid.New(), AcceptanceState: model.TreatmentAcceptanceProposed,
		SourceBodyStateRevision: 7, SourceDiagnosisAnalysisID: analysis.ID,
	}}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 8, SafetyState: json.RawMessage(`{"has_red_flags":true,"status":"requires_review"}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil, fakeTreatmentReasoner{},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	_, err := svc.AcceptProposal(context.Background(), userID, repo.proposal.ID)
	if !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("expected safety revalidation to block acceptance, got %v", err)
	}
	if repo.acceptCalled {
		t.Fatal("repository accept must not run after a safety gate failure")
	}
}

func TestTreatmentAcceptRejectsStaleDiagnosis(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{proposal: &model.TreatmentRevision{
		ID: uuid.New(), TreatmentID: uuid.New(), AcceptanceState: model.TreatmentAcceptanceProposed,
		SourceBodyStateRevision: 7, SourceDiagnosisAnalysisID: analysis.ID,
	}}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 8, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessStale}, nil, fakeTreatmentReasoner{},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	_, err := svc.AcceptProposal(context.Background(), userID, repo.proposal.ID)
	if !errors.Is(err, ErrTreatmentAnalysisStale) {
		t.Fatalf("expected stale diagnosis to block acceptance, got %v", err)
	}
	if repo.acceptCalled {
		t.Fatal("repository accept must not run for a stale diagnosis")
	}
}

func TestTreatmentAcceptRejectsRelatedBodyStateChange(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{proposal: &model.TreatmentRevision{
		ID: uuid.New(), TreatmentID: uuid.New(), AcceptanceState: model.TreatmentAcceptanceProposed,
		SourceBodyStateRevision: 7, SourceDiagnosisAnalysisID: analysis.ID,
	}}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		&fakeTreatmentBodyState{
			snapshot:  &BodyStateSnapshot{UserID: userID, CurrentRevision: 8, SafetyState: json.RawMessage(`{}`)},
			revisions: []model.BodyStateRevision{{Revision: 8, ChangeType: "fact.added", Changes: datatypes.JSON(`{"fact":{"concern_key":"region:neck","kind":"discomfort"}}`)}},
		},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil, fakeTreatmentReasoner{},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	_, err := svc.AcceptProposal(context.Background(), userID, repo.proposal.ID)
	if !errors.Is(err, ErrTreatmentProposalOutdated) {
		t.Fatalf("expected related BodyState change to invalidate proposal acceptance, got %v", err)
	}
	if repo.acceptCalled {
		t.Fatal("repository accept must not run for an outdated proposal")
	}
}

func TestTreatmentAcceptAllowsUnrelatedChangeAndPinsCheckedRevision(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{proposal: &model.TreatmentRevision{
		ID: uuid.New(), TreatmentID: uuid.New(), Revision: 1, AcceptanceState: model.TreatmentAcceptanceProposed,
		SourceBodyStateRevision: 7, SourceDiagnosisAnalysisID: analysis.ID,
	}}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "unsure"}}},
		&fakeTreatmentBodyState{
			snapshot:  &BodyStateSnapshot{UserID: userID, CurrentRevision: 8, SafetyState: json.RawMessage(`{}`)},
			revisions: []model.BodyStateRevision{{Revision: 8, ChangeType: "fact.added", Changes: datatypes.JSON(`{"fact":{"concern_key":"region:sleep","kind":"lifestyle"}}`)}},
		},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil, fakeTreatmentReasoner{},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	treatment, err := svc.AcceptProposal(context.Background(), userID, repo.proposal.ID)
	if err != nil {
		t.Fatalf("unrelated change should not block acceptance: %v", err)
	}
	if treatment == nil || treatment.Current == nil {
		t.Fatal("expected accepted treatment")
	}
	if repo.acceptExpectedBodyStateRevision != 8 {
		t.Fatalf("accept transaction must pin checked BodyState revision 8, got %d", repo.acceptExpectedBodyStateRevision)
	}
}

func TestTreatmentAcceptRejectsConcurrentBodyStateChange(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{
		acceptConflict: true,
		proposal: &model.TreatmentRevision{
			ID: uuid.New(), TreatmentID: uuid.New(), Revision: 1, AcceptanceState: model.TreatmentAcceptanceProposed,
			SourceBodyStateRevision: 7, SourceDiagnosisAnalysisID: analysis.ID,
		},
	}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 7, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil, fakeTreatmentReasoner{},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	_, err := svc.AcceptProposal(context.Background(), userID, repo.proposal.ID)
	if !errors.Is(err, ErrTreatmentProposalOutdated) {
		t.Fatalf("expected transactional BodyState conflict, got %v", err)
	}
}

func TestRecordOutcomeUsesOneUnitOfWorkForOutcomeAndBodyState(t *testing.T) {
	called := false
	repo := &fakeTreatmentRepo{}
	bodyState := &fakeTreatmentBodyState{recordOutcomeRev: 21}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{},
		bodyState,
		nil,
		nil,
		fakeTreatmentReasoner{},
		testTreatmentUnitOfWork{called: &called},
		testTreatmentDeploymentPolicy{},
	)

	stored, created, err := svc.RecordOutcome(context.Background(), uuid.New(), model.Outcome{
		SourceType: "web_checkin", SourceKey: "outcome-1", Kind: "symptom_change",
		Value: datatypes.JSON(`{"description":"better","trend":"improving"}`),
	})
	if err != nil {
		t.Fatalf("RecordOutcome returned error: %v", err)
	}
	if !called || !created {
		t.Fatalf("expected transactional creation, called=%v created=%v", called, created)
	}
	if stored.BodyStateRevision == nil || *stored.BodyStateRevision != 21 {
		t.Fatalf("expected linked BodyState revision 21, got %#v", stored.BodyStateRevision)
	}
	if bodyState.recordOutcomeCalls != 1 || repo.updateOutcomeCalls != 1 {
		t.Fatalf("expected one projection and one link update: body=%d update=%d", bodyState.recordOutcomeCalls, repo.updateOutcomeCalls)
	}
}

func TestRecordOutcomeRepairsDuplicateWithoutBodyStateLink(t *testing.T) {
	userID := uuid.New()
	stored := &model.Outcome{
		ID: uuid.New(), UserID: userID,
		SourceType: "training_feedback", SourceKey: "log-1", Kind: "symptom_change",
		Value: datatypes.JSON(`{"description":"better","trend":"improving"}`),
	}
	repo := &fakeTreatmentRepo{storedOutcome: stored, storedOutcomeCreated: false}
	bodyState := &fakeTreatmentBodyState{recordOutcomeRev: 22}
	svc := NewTreatmentService(
		repo, &fakeTreatmentDiagnosis{}, bodyState, nil, nil, fakeTreatmentReasoner{}, testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	result, created, err := svc.RecordOutcome(context.Background(), userID, *stored)
	if err != nil {
		t.Fatalf("RecordOutcome returned error: %v", err)
	}
	if created {
		t.Fatal("duplicate outcome must remain created=false")
	}
	if bodyState.recordOutcomeCalls != 1 || repo.updateOutcomeRevision != 22 {
		t.Fatalf("duplicate without link must be projected and linked: calls=%d revision=%d", bodyState.recordOutcomeCalls, repo.updateOutcomeRevision)
	}
	if result.BodyStateRevision == nil || *result.BodyStateRevision != 22 {
		t.Fatalf("expected repaired BodyState link, got %#v", result.BodyStateRevision)
	}
}

func TestRecordOutcomeSkipsAlreadyAppliedDuplicate(t *testing.T) {
	userID := uuid.New()
	revision := int64(12)
	stored := &model.Outcome{
		ID: uuid.New(), UserID: userID,
		SourceType: "training_feedback", SourceKey: "log-1", Kind: "training_adherence",
		Value: datatypes.JSON(`{"checked_in":true}`), BodyStateRevision: &revision,
	}
	repo := &fakeTreatmentRepo{storedOutcome: stored, storedOutcomeCreated: false}
	bodyState := &fakeTreatmentBodyState{}
	svc := NewTreatmentService(
		repo, &fakeTreatmentDiagnosis{}, bodyState, nil, nil, fakeTreatmentReasoner{}, testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	_, created, err := svc.RecordOutcome(context.Background(), userID, *stored)
	if err != nil || created {
		t.Fatalf("applied duplicate should be an idempotent read: created=%v err=%v", created, err)
	}
	if bodyState.recordOutcomeCalls != 0 || repo.updateOutcomeCalls != 0 {
		t.Fatalf("already-linked duplicate must not reproject: body=%d update=%d", bodyState.recordOutcomeCalls, repo.updateOutcomeCalls)
	}
}

func currentTreatmentForReview(userID uuid.UUID) *model.Treatment {
	analysisID := uuid.New()
	return &model.Treatment{
		ID: uuid.New(), UserID: userID, Status: model.TreatmentStatusActive,
		StatusReasons: datatypes.JSON(`[]`),
		Current: &model.TreatmentRevision{
			ID: uuid.New(), AcceptanceState: model.TreatmentAcceptanceAccepted,
			SourceBodyStateRevision: 4, SourceDiagnosisAnalysisID: analysisID,
		},
	}
}

func TestTreatmentGetCurrentIsPure(t *testing.T) {
	userID := uuid.New()
	current := currentTreatmentForReview(userID)
	repo := &fakeTreatmentRepo{current: current}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: &model.DiagnosisAnalysisRecord{
			ID:         current.Current.SourceDiagnosisAnalysisID,
			Candidates: []model.DiagnosisCandidateRecord{{ConcernKey: "region:neck"}},
		}},
		&fakeTreatmentBodyState{
			snapshot: &BodyStateSnapshot{CurrentRevision: 5, SafetyState: json.RawMessage(`{}`)},
			revisions: []model.BodyStateRevision{{
				Revision: 5, ChangeType: "fact.added",
				Changes: datatypes.JSON(`{"fact":{"concern_key":"region:neck","kind":"discomfort"}}`),
			}},
		},
		nil, nil, fakeTreatmentReasoner{}, testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	stored, err := svc.GetCurrent(context.Background(), userID)
	if err != nil || stored.Status != model.TreatmentStatusActive {
		t.Fatalf("pure read returned unexpected result: stored=%#v err=%v", stored, err)
	}
	if repo.statusUpdates != 0 {
		t.Fatalf("GetCurrent must not write status, updates=%d", repo.statusUpdates)
	}
}

func TestTreatmentPreviewDerivesReviewWithoutWriting(t *testing.T) {
	userID := uuid.New()
	current := currentTreatmentForReview(userID)
	repo := &fakeTreatmentRepo{current: current}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: &model.DiagnosisAnalysisRecord{
			ID:         current.Current.SourceDiagnosisAnalysisID,
			Candidates: []model.DiagnosisCandidateRecord{{ConcernKey: "region:neck"}},
		}},
		&fakeTreatmentBodyState{
			snapshot: &BodyStateSnapshot{CurrentRevision: 5, SafetyState: json.RawMessage(`{}`)},
			revisions: []model.BodyStateRevision{{
				Revision: 5, ChangeType: "fact.added",
				Changes: datatypes.JSON(`{"fact":{"concern_key":"region:neck","kind":"discomfort"}}`),
			}},
		},
		nil, nil, fakeTreatmentReasoner{}, testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	projected, err := svc.PreviewCurrentReview(context.Background(), userID)
	if err != nil || projected.Status != model.TreatmentStatusReviewRecommended {
		t.Fatalf("expected derived review status: projected=%#v err=%v", projected, err)
	}
	if repo.statusUpdates != 0 || current.Status != model.TreatmentStatusActive {
		t.Fatalf("preview must not mutate storage: updates=%d current=%s", repo.statusUpdates, current.Status)
	}
}

func TestTreatmentExplicitReviewPersistsDerivedStatus(t *testing.T) {
	userID := uuid.New()
	current := currentTreatmentForReview(userID)
	repo := &fakeTreatmentRepo{current: current}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: &model.DiagnosisAnalysisRecord{
			ID:         current.Current.SourceDiagnosisAnalysisID,
			Candidates: []model.DiagnosisCandidateRecord{{ConcernKey: "region:neck"}},
		}},
		&fakeTreatmentBodyState{
			snapshot: &BodyStateSnapshot{CurrentRevision: 5, SafetyState: json.RawMessage(`{}`)},
			revisions: []model.BodyStateRevision{{
				Revision: 5, ChangeType: "fact.added",
				Changes: datatypes.JSON(`{"fact":{"concern_key":"region:neck","kind":"discomfort"}}`),
			}},
		},
		nil, nil, fakeTreatmentReasoner{}, testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	persisted, err := svc.EvaluateCurrentReview(context.Background(), userID)
	if err != nil || persisted.Status != model.TreatmentStatusReviewRecommended {
		t.Fatalf("explicit review failed: persisted=%#v err=%v", persisted, err)
	}
	if repo.statusUpdates != 1 {
		t.Fatalf("explicit review should persist exactly once, updates=%d", repo.statusUpdates)
	}
}

func TestTreatmentReviewTemporalChangePayloadFromOutcomeRecommendsReview(t *testing.T) {
	factID := "82cb44cb-4648-43bb-a27b-271b189da53e"
	analysis := &model.DiagnosisAnalysisRecord{
		Candidates: []model.DiagnosisCandidateRecord{{
			ConcernKey:   "region:颈肩",
			BasisFactIDs: datatypes.JSON(`[]`),
		}},
	}
	revision := model.BodyStateRevision{
		Revision:   4,
		ChangeType: "fact.temporal_changed",
		Changes: datatypes.JSON(`{
			"fact_id":"` + factID + `",
			"before":{"id":"` + factID + `","concern_key":"region:颈肩","kind":"discomfort","trend":"stable"},
			"after":{"id":"` + factID + `","concern_key":"region:颈肩","kind":"discomfort","trend":"improving"}
		}`),
	}

	status, reasons := EvaluateTreatmentReviewPolicy(analysis, []model.BodyStateRevision{revision})
	if status != model.TreatmentStatusReviewRecommended || len(reasons) != 1 {
		t.Fatalf("temporal outcome change should recommend review: status=%s reasons=%#v", status, reasons)
	}
	if reasons[0].ConcernKey != "region:颈肩" || reasons[0].ChangeType != "fact.temporal_changed" {
		t.Fatalf("unexpected temporal review reason: %#v", reasons[0])
	}
}

func TestRawJSONUsesFallbackForNilContainers(t *testing.T) {
	var constraints map[string]any
	if got := string(rawJSON(constraints, `{}`)); got != `{}` {
		t.Fatalf("nil map must use object fallback, got %s", got)
	}

	var evidence []model.BodyStateEvidence
	if got := string(rawJSON(evidence, `[]`)); got != `[]` {
		t.Fatalf("nil slice must use array fallback, got %s", got)
	}
}

func TestTreatmentAgentIdentityRejectsPolicyAndRuntimeDrift(t *testing.T) {
	validConfig := json.RawMessage(`{
		"id":"treat-config-85718f8e90ac9d80",
		"role":"treatment",
		"decision_policy_revision":"treatment-go-acceptance-v1"
	}`)
	validExecution := json.RawMessage(`{
		"status":"executed",
		"runtime":"pydantic-ai",
		"logical_model":"bodysense-structured"
	}`)

	cases := []struct {
		name      string
		config    json.RawMessage
		execution json.RawMessage
	}{
		{
			name: "decision-policy-drift",
			config: json.RawMessage(`{
				"id":"treat-config-85718f8e90ac9d80",
				"role":"treatment",
				"decision_policy_revision":"treatment-go-acceptance-v2"
			}`),
			execution: validExecution,
		},
		{
			name:   "logical-model-drift",
			config: validConfig,
			execution: json.RawMessage(`{
				"status":"executed",
				"runtime":"pydantic-ai",
				"logical_model":"unexpected-model"
			}`),
		},
		{
			name:   "runtime-drift",
			config: validConfig,
			execution: json.RawMessage(`{
				"status":"executed",
				"runtime":"direct-provider",
				"logical_model":"bodysense-structured"
			}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := &treatmentAgentPayload{
				AgentConfiguration:  tc.config,
				ExecutionProvenance: tc.execution,
			}
			if err := validateTreatmentAgentIdentity(payload, defaultTreatmentConfigurationID); !errors.Is(err, ErrTreatmentConfigurationMismatch) {
				t.Fatalf("expected fail-closed identity mismatch, got %v", err)
			}
		})
	}
}

func TestTreatmentEvidenceGapChallengerPersistsAcquisitionTrace(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 12,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:shoulder", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{}
	trace := `{
		"trace_revision":"evidence-acquisition-trace-v2",
		"policy_revision":"treatment-evidence-gap-v2",
		"external_evidence_status":"available",
		"budget":{"max_searches":2,"max_results_per_search":5,"used_searches":1,"remaining_searches":1},
		"attempts":[{"gap":{"gap_id":"dose","kind":"external_knowledge","description":"dose evidence","rationale":"changes dose","critical":false,"query":"dose evidence"},"status":"evidence_returned","stop_reason":"evidence_returned","search_performed":true,"retrieval_status":"results_returned","query":"dose evidence","requested_top_k":5,"evidence_ids":["evidence-dose"]}],
		"unresolved_critical_gaps":[]
	}`
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{
			analysis: analysis,
			assessments: []model.DiagnosisCandidateAssessment{{
				CandidateID: analysis.Candidates[0].ID,
				State:       "confirmed",
			}},
		},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 12, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh},
		nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{
			"status":"proposed","summary":"plan","goal":"improve tolerance","duration_weeks":4,
			"interventions":[{"kind":"exercise","title":"graded load","description":"controlled","prescription":{"sets":2}}],
			"daily_habits":[],"expected_timeline":"4 weeks","warning_signs":[],"review_triggers":["worsening"],"safety_notes":[],
			"evidence_ids":["evidence-dose"],"evidence_acquisition":` + trace + `
		}`)},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{configurationID: treatmentEvidenceGapConfigurationID},
	)

	revision, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID})
	if err != nil {
		t.Fatalf("GenerateProposal returned error: %v", err)
	}
	if revision.AgentConfigurationID != treatmentEvidenceGapConfigurationID {
		t.Fatalf("unexpected Challenger identity: %q", revision.AgentConfigurationID)
	}
	if string(revision.EvidenceAcquisitionTrace) == "{}" || !json.Valid(revision.EvidenceAcquisitionTrace) {
		t.Fatalf("EvidenceGap trace was not persisted: %s", revision.EvidenceAcquisitionTrace)
	}
	var persisted map[string]any
	if err := json.Unmarshal(revision.EvidenceAcquisitionTrace, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted["policy_revision"] != "treatment-evidence-gap-v2" {
		t.Fatalf("unexpected persisted policy trace: %#v", persisted)
	}
}

func TestTreatmentProposalPersistsGenerationDecisionTrace(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 14,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 14, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh},
		nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{
			"status":"proposed","summary":"plan","goal":"reduce load","duration_weeks":4,
			"interventions":[{"kind":"exercise","title":"chin tuck","description":"controlled","prescription":{"sets":2}}],
			"daily_habits":[],"expected_timeline":"4 weeks","warning_signs":[],"review_triggers":["worsening"],"safety_notes":[]
		}`)},
		testTreatmentUnitOfWork{},
		testTreatmentDeploymentPolicy{},
	)

	revision, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID})
	if err != nil {
		t.Fatalf("GenerateProposal returned error: %v", err)
	}
	var trace map[string]any
	if err := json.Unmarshal(revision.GenerationDecisionTrace, &trace); err != nil {
		t.Fatalf("decode generation DecisionTrace: %v", err)
	}
	if trace["trace_revision"] != TreatmentDecisionTraceV1 || trace["phase"] != TreatmentDecisionGeneration || trace["outcome"] != string(TreatmentAllowProposal) {
		t.Fatalf("unexpected generation DecisionTrace: %#v", trace)
	}
	if trace["diagnosis_analysis_id"] != analysis.ID.String() || trace["agent_configuration_id"] != defaultTreatmentConfigurationID {
		t.Fatalf("generation DecisionTrace lost durable identities: %#v", trace)
	}
	facts, ok := trace["facts"].(map[string]any)
	if !ok || facts["current_body_state_revision"] != float64(14) || facts["candidate_assessment_ready"] != true {
		t.Fatalf("generation DecisionTrace lost authority facts: %#v", trace)
	}
}

func TestTreatmentAcceptancePersistsDecisionTraceWithCheckedRevision(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 15,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{proposal: &model.TreatmentRevision{
		ID: uuid.New(), TreatmentID: uuid.New(), Revision: 1,
		AcceptanceState:         model.TreatmentAcceptanceProposed,
		SourceBodyStateRevision: 15, SourceDiagnosisAnalysisID: analysis.ID,
		AgentConfigurationID: defaultTreatmentConfigurationID,
	}}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "unsure"}}},
		&fakeTreatmentBodyState{
			snapshot:  &BodyStateSnapshot{UserID: userID, CurrentRevision: 16, SafetyState: json.RawMessage(`{}`)},
			revisions: []model.BodyStateRevision{{Revision: 16, ChangeType: "fact.added", Changes: datatypes.JSON(`{"fact":{"concern_key":"region:sleep","kind":"lifestyle"}}`)}},
		},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil, fakeTreatmentReasoner{},
		testTreatmentUnitOfWork{}, testTreatmentDeploymentPolicy{},
	)

	treatment, err := svc.AcceptProposal(context.Background(), userID, repo.proposal.ID)
	if err != nil {
		t.Fatalf("AcceptProposal returned error: %v", err)
	}
	if treatment == nil || treatment.Current == nil {
		t.Fatal("expected accepted Treatment")
	}
	var trace map[string]any
	if err := json.Unmarshal(repo.acceptDecisionTrace, &trace); err != nil {
		t.Fatalf("decode acceptance DecisionTrace: %v", err)
	}
	if trace["phase"] != TreatmentDecisionAcceptance || trace["outcome"] != string(TreatmentAllowAcceptance) {
		t.Fatalf("unexpected acceptance DecisionTrace: %#v", trace)
	}
	facts, ok := trace["facts"].(map[string]any)
	if !ok || facts["source_body_state_revision"] != float64(15) || facts["current_body_state_revision"] != float64(16) {
		t.Fatalf("acceptance DecisionTrace lost checked revisions: %#v", trace)
	}
	if string(repo.proposal.AcceptanceDecisionTrace) == "{}" || len(repo.proposal.AcceptanceDecisionTrace) == 0 {
		t.Fatalf("acceptance trace was not persisted by repository seam: %s", repo.proposal.AcceptanceDecisionTrace)
	}
}

func TestTreatmentMalformedSafetyStateFailsClosedBeforeAgentOrAcceptance(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 20,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	bodyState := &fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{
		UserID: userID, CurrentRevision: 20,
		SafetyState: json.RawMessage(`{"has_red_flags":"yes","status":"requires_review"}`),
	}}
	repo := &fakeTreatmentRepo{}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		bodyState, fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{"status":"proposed"}`)}, testTreatmentUnitOfWork{}, testTreatmentDeploymentPolicy{},
	)
	if _, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID}); !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("malformed safety must block generation: %v", err)
	}
	if repo.proposal != nil {
		t.Fatal("malformed safety must block before proposal persistence")
	}

	repo.proposal = &model.TreatmentRevision{
		ID: uuid.New(), TreatmentID: uuid.New(), Revision: 1,
		AcceptanceState:         model.TreatmentAcceptanceProposed,
		SourceBodyStateRevision: 20, SourceDiagnosisAnalysisID: analysis.ID,
	}
	if _, err := svc.AcceptProposal(context.Background(), userID, repo.proposal.ID); !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("malformed safety must block acceptance: %v", err)
	}
	if repo.acceptCalled {
		t.Fatal("malformed safety must block before repository acceptance")
	}
}

func TestTreatmentGenerationPersistsRolloutRouteAndShadowFailureDoesNotFailServedProposal(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 30,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{}
	observer := &fakeTreatmentRolloutObserver{err: errors.New("shadow storage unavailable")}
	deployment := testTreatmentDeploymentPolicy{
		configurationID:       defaultTreatmentConfigurationID,
		shadowConfigurationID: treatmentEvidenceGapConfigurationID,
		stage:                 TreatmentRolloutShadow,
		subjectBucket:         2468,
		canaryBPS:             500,
	}
	svc := NewTreatmentService(
		repo,
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 30, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{
			"status":"proposed","summary":"served champion","goal":"reduce load","duration_weeks":4,
			"interventions":[{"kind":"exercise","title":"graded load","description":"controlled","prescription":{"sets":2}}],
			"daily_habits":[],"expected_timeline":"4 weeks","warning_signs":[],"review_triggers":[],"safety_notes":[]
		}`)},
		testTreatmentUnitOfWork{}, deployment,
	)
	svc.AttachRolloutObserver(observer)

	revision, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID})
	if err != nil {
		t.Fatalf("served Treatment proposal must survive shadow observation failure: %v", err)
	}
	if revision.AgentConfigurationID != defaultTreatmentConfigurationID {
		t.Fatalf("shadow must not replace served Champion: %#v", revision)
	}
	var provenance TreatmentRouteSelection
	if err := json.Unmarshal(revision.RolloutProvenance, &provenance); err != nil {
		t.Fatalf("decode rollout provenance: %v", err)
	}
	if provenance.Stage != TreatmentRolloutShadow || provenance.SubjectBucket != 2468 ||
		provenance.ServedConfigurationID != defaultTreatmentConfigurationID ||
		provenance.ShadowConfigurationID != treatmentEvidenceGapConfigurationID {
		t.Fatalf("unexpected durable Treatment rollout provenance: %#v", provenance)
	}
	if observer.calls != 1 || observer.userID != userID || observer.revisionID != revision.ID ||
		observer.route.ShadowConfigurationID != treatmentEvidenceGapConfigurationID {
		t.Fatalf("served proposal was not paired exactly once: %#v", observer)
	}
}

func TestTreatmentChampionGenerationDoesNotInvokeRolloutObserver(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 31,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	observer := &fakeTreatmentRolloutObserver{}
	svc := NewTreatmentService(
		&fakeTreatmentRepo{},
		&fakeTreatmentDiagnosis{analysis: analysis, assessments: []model.DiagnosisCandidateAssessment{{CandidateID: analysis.Candidates[0].ID, State: "confirmed"}}},
		&fakeTreatmentBodyState{snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 31, SafetyState: json.RawMessage(`{}`)}},
		fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil,
		fakeTreatmentReasoner{raw: json.RawMessage(`{
			"status":"proposed","summary":"champion","goal":"reduce load","duration_weeks":4,
			"interventions":[{"kind":"exercise","title":"graded load","description":"controlled","prescription":{}}],
			"daily_habits":[],"expected_timeline":"4 weeks","warning_signs":[],"review_triggers":[],"safety_notes":[]
		}`)},
		testTreatmentUnitOfWork{}, testTreatmentDeploymentPolicy{},
	)
	svc.AttachRolloutObserver(observer)
	if _, err := svc.GenerateProposal(context.Background(), userID, TreatmentProposalInput{DiagnosisAnalysisID: analysis.ID}); err != nil {
		t.Fatal(err)
	}
	if observer.calls != 0 {
		t.Fatalf("Champion-only stage must not execute a paired shadow, calls=%d", observer.calls)
	}
}
