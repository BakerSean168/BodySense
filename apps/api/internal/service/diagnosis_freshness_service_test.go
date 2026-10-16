package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeDiagnosisFreshnessStore struct {
	upserts int
	stored  *model.DiagnosisAnalysisFreshness
}

func (s *fakeDiagnosisFreshnessStore) Upsert(_ context.Context, item *model.DiagnosisAnalysisFreshness) error {
	s.upserts++
	s.stored = item
	return nil
}
func (s *fakeDiagnosisFreshnessStore) Get(context.Context, uuid.UUID, uuid.UUID) (*model.DiagnosisAnalysisFreshness, error) {
	return s.stored, nil
}

type fakeDiagnosisFreshnessBodyState struct {
	head      int64
	revisions []model.BodyStateRevision
}

func (s fakeDiagnosisFreshnessBodyState) GetBodyStateHead(_ context.Context, owner uuid.UUID) (*model.BodyStateHead, error) {
	return &model.BodyStateHead{UserID: owner, CurrentRevision: s.head}, nil
}
func (s fakeDiagnosisFreshnessBodyState) ListRevisionsAfter(context.Context, uuid.UUID, int64, int) ([]model.BodyStateRevision, error) {
	return s.revisions, nil
}
func (s fakeDiagnosisFreshnessBodyState) ResolveRevisionFacts(_ context.Context, _ uuid.UUID, revisions []model.BodyStateRevision) (BodyStateFactIndex, error) {
	return testRevisionFacts(revisions), nil
}

func evaluateFreshness(analysis *model.DiagnosisAnalysisRecord, revisions ...model.BodyStateRevision) (string, []DiagnosisFreshnessReason) {
	return EvaluateDiagnosisFreshnessPolicy(analysis, revisions, testRevisionFacts(revisions))
}

func TestDiagnosisFreshnessPreviewDoesNotPersist(t *testing.T) {
	userID := uuid.New()
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	analysis.UserID = userID
	analysis.BodyStateRevision = 4
	store := &fakeDiagnosisFreshnessStore{}
	svc := NewDiagnosisFreshnessService(store, fakeDiagnosisFreshnessBodyState{
		head:      5,
		revisions: []model.BodyStateRevision{testFactRecorded(5, "region:neck")},
	})

	preview, err := svc.Preview(context.Background(), userID, analysis)
	if err != nil || preview.State != model.DiagnosisFreshnessPotentiallyStale || preview.EvaluatedAgainstRevision != 5 {
		t.Fatalf("unexpected preview: preview=%#v err=%v", preview, err)
	}
	if store.upserts != 0 {
		t.Fatalf("freshness preview must not persist, upserts=%d", store.upserts)
	}

	if _, err := svc.Evaluate(context.Background(), userID, analysis); err != nil {
		t.Fatalf("explicit freshness evaluation failed: %v", err)
	}
	if store.upserts != 1 {
		t.Fatalf("explicit evaluation should persist once, upserts=%d", store.upserts)
	}
	if _, err := svc.GetOrEvaluate(context.Background(), userID, analysis); err != nil || store.upserts != 1 {
		t.Fatalf("unchanged head must reuse the stored evaluation: upserts=%d err=%v", store.upserts, err)
	}
}

func TestDiagnosisFreshnessPreviewRejectsForeignAnalysis(t *testing.T) {
	analysis := freshnessAnalysis("region:neck", "", "")
	analysis.UserID = uuid.New()
	svc := NewDiagnosisFreshnessService(&fakeDiagnosisFreshnessStore{}, fakeDiagnosisFreshnessBodyState{head: 1})
	if _, err := svc.Preview(context.Background(), uuid.New(), analysis); err == nil {
		t.Fatal("another owner's analysis must not be evaluated")
	}
}

func TestDiagnosisFreshnessReferencedCorrectionIsStale(t *testing.T) {
	factID := testFactID("region:neck").String()
	analysis := freshnessAnalysis("region:neck", factID, "")
	replacement := testFactID("region:颈肩").String()
	state, reasons := evaluateFreshness(analysis, testRevision(8, "facts.corrected", `{"target_fact_id":"`+factID+`","fact_ids":["`+replacement+`"]}`))
	if state != model.DiagnosisFreshnessStale || len(reasons) != 1 || reasons[0].Code != "referenced_fact_corrected" || reasons[0].ItemID != factID {
		t.Fatalf("unexpected freshness result: state=%s reasons=%#v", state, reasons)
	}
}

func TestDiagnosisFreshnessReferencedWithdrawalIsStale(t *testing.T) {
	factID := testFactID("region:neck").String()
	analysis := freshnessAnalysis("region:neck", factID, "")
	state, reasons := evaluateFreshness(analysis, testRevision(8, "facts.withdrawn", `{"target_fact_id":"`+factID+`","fact_ids":[]}`))
	if state != model.DiagnosisFreshnessStale || len(reasons) != 1 || reasons[0].Code != "referenced_fact_withdrawn" {
		t.Fatalf("unexpected freshness result: state=%s reasons=%#v", state, reasons)
	}
}

func TestDiagnosisFreshnessRelatedAmendmentsArePotentiallyStale(t *testing.T) {
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	neck, sleep := testFactID("region:neck").String(), testFactID("lifestyle:sleep").String()
	for name, revision := range map[string]model.BodyStateRevision{
		"related withdrawn":          testRevision(8, "facts.withdrawn", `{"target_fact_id":"`+neck+`","fact_ids":[]}`),
		"related corrected":          testRevision(9, "facts.corrected", `{"target_fact_id":"`+neck+`","fact_ids":["`+sleep+`"]}`),
		"correction into the region": testRevision(10, "facts.corrected", `{"target_fact_id":"`+sleep+`","fact_ids":["`+neck+`"]}`),
		"related episode change":     testRevision(11, "episode.status", `{"fact_id":"`+neck+`"}`),
	} {
		if state, reasons := evaluateFreshness(analysis, revision); state != model.DiagnosisFreshnessPotentiallyStale || len(reasons) != 1 {
			t.Fatalf("%s: state=%s reasons=%#v", name, state, reasons)
		}
	}
}

func TestDiagnosisFreshnessReferencedObservationAmendmentIsStale(t *testing.T) {
	observationID := uuid.New().String()
	analysis := freshnessAnalysis("region:neck", "", observationID)
	state, reasons := evaluateFreshness(analysis, testRevision(8, "observation.corrected", `{"target_observation_id":"`+observationID+`"}`))
	if state != model.DiagnosisFreshnessStale || len(reasons) != 1 || reasons[0].Code != "referenced_observation_corrected" {
		t.Fatalf("unexpected freshness result: state=%s reasons=%#v", state, reasons)
	}
	if state, _ := evaluateFreshness(analysis, testRevision(9, "observation.withdrawn", `{"target_observation_id":"`+uuid.New().String()+`"}`)); state != model.DiagnosisFreshnessFresh {
		t.Fatalf("an uncited observation must not invalidate the analysis, got %s", state)
	}
}

func TestDiagnosisFreshnessRelatedAdditionIsOnlyPotentiallyStale(t *testing.T) {
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	state, reasons := evaluateFreshness(analysis, testFactRecorded(9, "region:neck"))
	if state != model.DiagnosisFreshnessPotentiallyStale || len(reasons) != 1 || reasons[0].Code != "new_related_fact" || reasons[0].ConcernKey != "region:neck" {
		t.Fatalf("expected potentially stale, got %s %#v", state, reasons)
	}
}

func TestDiagnosisFreshnessUnrelatedChangesRemainFresh(t *testing.T) {
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	neck := testFactID("region:neck").String()
	state, reasons := evaluateFreshness(analysis,
		testFactRecorded(10, "lifestyle:sleep"),
		testFactRecorded(11, "general"),
		testRevision(12, "observation.recorded", `{"observation_id":"`+uuid.New().String()+`"}`),
		testRevision(13, "hypothesis.recorded", `{"related_fact_ids":["`+neck+`"]}`),
		testRevision(14, "fact.added", `{"fact":{"concern_key":"region:neck","kind":"discomfort"}}`),
		testRevision(15, "facts.recorded", `{"fact_ids":["`+uuid.New().String()+`"]}`),
	)
	if state != model.DiagnosisFreshnessFresh || len(reasons) != 0 {
		t.Fatalf("unrelated change must remain fresh: state=%s reasons=%#v", state, reasons)
	}
}

func TestDiagnosisFreshnessSafetyChangeIsStale(t *testing.T) {
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	for _, changeType := range []string{"safety.screened", "safety.updated"} {
		state, reasons := evaluateFreshness(analysis, testRevision(11, changeType, `{}`))
		if state != model.DiagnosisFreshnessStale || reasons[0].Code != "safety_state_changed" {
			t.Fatalf("%s must make analysis stale, got %s", changeType, state)
		}
	}
}

func freshnessAnalysis(concern, factID, observationID string) *model.DiagnosisAnalysisRecord {
	facts := []byte(`[]`)
	if factID != "" {
		facts = []byte(`["` + factID + `"]`)
	}
	observations := []byte(`[]`)
	if observationID != "" {
		observations = []byte(`["` + observationID + `"]`)
	}
	return &model.DiagnosisAnalysisRecord{
		ID: uuid.New(),
		Candidates: []model.DiagnosisCandidateRecord{{
			ConcernKey:          concern,
			BasisFactIDs:        facts,
			BasisObservationIDs: observations,
		}},
	}
}
