package service

import (
	"context"
	"encoding/json"
	"errors"
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
func (r *fakeTreatmentRepo) AcceptRevision(_ context.Context, userID, revisionID uuid.UUID, expectedBodyStateRevision int64) (*model.Treatment, *model.TreatmentRevision, bool, error) {
	r.acceptCalled = true
	r.acceptExpectedBodyStateRevision = expectedBodyStateRevision
	if r.acceptConflict {
		return nil, nil, false, nil
	}
	if r.proposal == nil || r.proposal.ID != revisionID {
		return nil, nil, false, errors.New("not found")
	}
	copy := *r.proposal
	copy.AcceptanceState = model.TreatmentAcceptanceAccepted
	copy.LifecycleState = model.TreatmentStatusActive
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

type fakeTreatmentReasoner struct{ raw json.RawMessage }

func (f fakeTreatmentReasoner) RecommendTreatment(context.Context, TreatmentRecommendationRequest) (json.RawMessage, error) {
	return f.raw, nil
}
func TestTreatmentGenerateCreatesProposalWithoutMakingItCurrent(t *testing.T) {
	userID := uuid.New()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: 7,
		Candidates: []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
	}
	repo := &fakeTreatmentRepo{}
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
		}`)},
		testTreatmentUnitOfWork{},
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
	if len(revision.Interventions) != 1 {
		t.Fatalf("expected one intervention, got %#v", revision.Interventions)
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
