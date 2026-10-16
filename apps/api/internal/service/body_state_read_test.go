package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestNativeBodyStateKeepsV2StatementAndRejectsForeignRows(t *testing.T) {
	owner := uuid.New()
	fact := agentTestFact(owner, "symptom", "我的膝盖疼", 2, `{"presence":"present"}`, `[]`)
	svc, repo := agentTestService(owner, 2, []model.BodyFact{fact})
	runner := &countingSnapshotRunner{}
	svc.WithAgentContextSources(runner, nil)
	view, err := svc.GetBodyState(context.Background(), owner, 20)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 || len(view.Facts) != 1 || view.Facts[0].Kind != "symptom" || view.Facts[0].Statement != fact.Statement || view.Facts[0].AssertionStatus != "asserted" {
		t.Fatalf("native read: %+v", view)
	}
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), "lifecycle_state") || strings.Contains(string(raw), "review_state") || view.Episodes == nil || view.Hypotheses == nil {
		t.Fatalf("native contract: %s", raw)
	}
	repo.facts[0].UserID = uuid.New()
	if _, err := svc.GetBodyState(context.Background(), owner, 20); err == nil {
		t.Fatal("foreign row must fail, not disappear")
	}
}

func TestAssessmentNativeV2EvidenceKeepsSourceText(t *testing.T) {
	factID, observationID := uuid.NewString(), uuid.NewString()
	catalog := buildAssessmentEvidenceCatalog(AssessmentGenerationRequest{BodyState: json.RawMessage(`{"schema":"agent-body-context-v1","facts":[{"id":"` + factID + `","kind":"lifestyle","statement":"每周跑步两次","details":{"habit_type":"exercise"},"authorship":"user_reported"}],"observations":[{"id":"` + observationID + `","kind":"weight","statement":"我体重70公斤","raw_value":"70","raw_unit":"kg","authorship":"user_reported"}]}`)})
	fact := catalog["body_state:fact:"+factID]
	observation := catalog["body_state:observation:"+observationID]
	if !assessmentBodyStateEvidenceKindAllowed("exercise_pattern", fact.Kind) || !assessmentBodyStateEvidenceKindAllowed("anthropometry", observation.Kind) {
		t.Fatalf("V2 evidence classified incorrectly: %+v", catalog)
	}
	rendered := renderAssessmentObservation("exercise_pattern", "body_state:fact:"+factID, fact)
	if !strings.Contains(rendered.Description, "每周跑步两次") {
		t.Fatalf("original statement lost: %+v", rendered)
	}
}

func TestNativeSafetyScreenCannotCoverAnotherSymptom(t *testing.T) {
	owner := uuid.New()
	first := agentTestFact(owner, "symptom", "左膝痛", 2, `{"presence":"present"}`, `[]`)
	second := agentTestFact(owner, "symptom", "右肩痛", 2, `{"presence":"present"}`, `[]`)
	snapshot := &BodyStateReadModel{UserID: owner, CurrentRevision: 3, Facts: []model.BodyFact{first, second}, SafetyScreens: []model.SafetyScreen{{ID: uuid.New(), UserID: owner, BodyFactID: first.ID, BodyStateRevision: 3}}}
	envelope, err := ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Coverage.Complete || len(envelope.Coverage.IncompleteSourceRefs) != 1 || envelope.Coverage.IncompleteSourceRefs[0] != "body-state:fact:"+second.ID.String() {
		t.Fatalf("unrelated symptom was screened: %+v", envelope.Coverage)
	}
}
