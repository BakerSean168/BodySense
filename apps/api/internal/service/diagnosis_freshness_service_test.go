package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
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
	snapshot  *BodyStateSnapshot
	revisions []model.BodyStateRevision
}

func (s fakeDiagnosisFreshnessBodyState) GetSnapshot(context.Context, uuid.UUID, int) (*BodyStateSnapshot, error) {
	return s.snapshot, nil
}
func (s fakeDiagnosisFreshnessBodyState) ListRevisionsAfter(context.Context, uuid.UUID, int64, int) ([]model.BodyStateRevision, error) {
	return s.revisions, nil
}

func TestDiagnosisFreshnessPreviewDoesNotPersist(t *testing.T) {
	userID := uuid.New()
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	analysis.UserID = userID
	analysis.BodyStateRevision = 4
	store := &fakeDiagnosisFreshnessStore{}
	svc := NewDiagnosisFreshnessService(store, fakeDiagnosisFreshnessBodyState{
		snapshot: &BodyStateSnapshot{UserID: userID, CurrentRevision: 5},
		revisions: []model.BodyStateRevision{{
			Revision: 5, ChangeType: "fact.added",
			Changes: datatypes.JSON(`{"fact":{"concern_key":"region:neck","kind":"discomfort"}}`),
		}},
	})

	preview, err := svc.Preview(context.Background(), userID, analysis)
	if err != nil || preview.State != model.DiagnosisFreshnessPotentiallyStale {
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
}

func TestDiagnosisFreshnessReferencedCorrectionIsStale(t *testing.T) {
	factID := uuid.New().String()
	analysis := freshnessAnalysis("region:neck", factID, "")
	revisions := []model.BodyStateRevision{{
		Revision:   8,
		ChangeType: "fact.corrected",
		Changes:    datatypes.JSON(`{"corrected_fact_id":"` + factID + `","replacement":{"concern_key":"region:neck"}}`),
	}}
	state, reasons := EvaluateDiagnosisFreshnessPolicy(analysis, revisions)
	if state != model.DiagnosisFreshnessStale || len(reasons) != 1 || reasons[0].Code != "referenced_fact_corrected" {
		t.Fatalf("unexpected freshness result: state=%s reasons=%#v", state, reasons)
	}
}

func TestDiagnosisFreshnessRelatedAdditionIsOnlyPotentiallyStale(t *testing.T) {
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	revisions := []model.BodyStateRevision{{
		Revision:   9,
		ChangeType: "fact.added",
		Changes:    datatypes.JSON(`{"fact":{"id":"` + uuid.New().String() + `","concern_key":"region:neck","kind":"discomfort"}}`),
	}}
	state, _ := EvaluateDiagnosisFreshnessPolicy(analysis, revisions)
	if state != model.DiagnosisFreshnessPotentiallyStale {
		t.Fatalf("expected potentially stale, got %s", state)
	}
}

func TestDiagnosisFreshnessUnrelatedLifestyleChangeRemainsFresh(t *testing.T) {
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	revisions := []model.BodyStateRevision{{
		Revision:   10,
		ChangeType: "fact.added",
		Changes:    datatypes.JSON(`{"fact":{"id":"` + uuid.New().String() + `","concern_key":"region:sleep","kind":"lifestyle"}}`),
	}}
	state, reasons := EvaluateDiagnosisFreshnessPolicy(analysis, revisions)
	if state != model.DiagnosisFreshnessFresh || len(reasons) != 0 {
		t.Fatalf("unrelated change must remain fresh: state=%s reasons=%#v", state, reasons)
	}
}

func TestDiagnosisFreshnessSafetyChangeIsStale(t *testing.T) {
	analysis := freshnessAnalysis("region:neck", uuid.New().String(), "")
	state, _ := EvaluateDiagnosisFreshnessPolicy(analysis, []model.BodyStateRevision{{
		Revision:   11,
		ChangeType: "safety.changed",
		Changes:    datatypes.JSON(`{"after":{"has_red_flags":true}}`),
	}})
	if state != model.DiagnosisFreshnessStale {
		t.Fatalf("safety change must make analysis stale, got %s", state)
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
