package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeDiagnosisAnalysisRepository struct {
	createdAnalysis   *model.DiagnosisAnalysisRecord
	createdCandidates []model.DiagnosisCandidateRecord
	byID              *model.DiagnosisAnalysisRecord
	assessments       []model.DiagnosisCandidateAssessment
}

func (r *fakeDiagnosisAnalysisRepository) Create(_ context.Context, analysis *model.DiagnosisAnalysisRecord, candidates []model.DiagnosisCandidateRecord) error {
	r.createdAnalysis = analysis
	r.createdCandidates = append([]model.DiagnosisCandidateRecord(nil), candidates...)
	return nil
}
func (r *fakeDiagnosisAnalysisRepository) ListByUser(context.Context, uuid.UUID, int) ([]model.DiagnosisAnalysisRecord, error) {
	if r.createdAnalysis == nil {
		return nil, nil
	}
	copy := *r.createdAnalysis
	copy.Candidates = append([]model.DiagnosisCandidateRecord(nil), r.createdCandidates...)
	return []model.DiagnosisAnalysisRecord{copy}, nil
}
func (r *fakeDiagnosisAnalysisRepository) GetLatestByUser(context.Context, uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	return r.createdAnalysis, nil
}
func (r *fakeDiagnosisAnalysisRepository) GetByID(context.Context, uuid.UUID, uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	return r.byID, nil
}
func (r *fakeDiagnosisAnalysisRepository) UpsertAssessment(_ context.Context, assessment *model.DiagnosisCandidateAssessment) error {
	r.assessments = append(r.assessments, *assessment)
	return nil
}
func (r *fakeDiagnosisAnalysisRepository) ListAssessments(context.Context, uuid.UUID, uuid.UUID) ([]model.DiagnosisCandidateAssessment, error) {
	return append([]model.DiagnosisCandidateAssessment(nil), r.assessments...), nil
}

func TestPersistDiagnosisAnalysisAllowsEightCandidatesAndPinsRevision(t *testing.T) {
	repo := &fakeDiagnosisAnalysisRepository{}
	svc := NewDiagnosisAnalysisService(repo)
	userID := uuid.New()

	candidates := make([]map[string]any, 0, 8)
	for i := 0; i < 8; i++ {
		candidates = append(candidates, map[string]any{
			"concern_key":    "region:neck",
			"name":           "candidate",
			"confidence":     "中",
			"basis":          "current BodyState evidence",
			"basis_fact_ids": []string{"fact-a"},
		})
	}
	raw, _ := json.Marshal(map[string]any{
		"status":     "completed",
		"scope":      "full_body",
		"summary":    "multi-concern analysis",
		"candidates": candidates,
	})

	analysis, err := svc.PersistAIResult(context.Background(), userID, 42, raw)
	if err != nil {
		t.Fatalf("PersistAIResult returned error: %v", err)
	}
	if analysis.BodyStateRevision != 42 {
		t.Fatalf("expected exact BodyState revision 42, got %d", analysis.BodyStateRevision)
	}
	if len(analysis.Candidates) != 8 {
		t.Fatalf("expected 8 candidates, got %d", len(analysis.Candidates))
	}
	seen := map[uuid.UUID]bool{}
	for _, candidate := range analysis.Candidates {
		if candidate.ID == uuid.Nil {
			t.Fatal("Go application layer must assign durable candidate IDs")
		}
		if seen[candidate.ID] {
			t.Fatalf("candidate ID %s was reused", candidate.ID)
		}
		seen[candidate.ID] = true
		if candidate.AnalysisID != analysis.ID {
			t.Fatalf("candidate must belong to analysis %s, got %s", analysis.ID, candidate.AnalysisID)
		}
	}
}

func TestPersistDiagnosisAnalysisAllowsZeroCandidatesForSafetyBlocked(t *testing.T) {
	repo := &fakeDiagnosisAnalysisRepository{}
	svc := NewDiagnosisAnalysisService(repo)
	raw := json.RawMessage(`{"status":"safety_blocked","scope":"full_body","candidates":[],"safety_summary":{"has_red_flags":true}}`)

	analysis, err := svc.PersistAIResult(context.Background(), uuid.New(), 9, raw)
	if err != nil {
		t.Fatalf("safety-blocked analysis should be durable: %v", err)
	}
	if analysis.Status != "safety_blocked" || len(analysis.Candidates) != 0 {
		t.Fatalf("unexpected safety-blocked analysis: %#v", analysis)
	}
}

func TestPersistDiagnosisAnalysisRejectsUnknownStatus(t *testing.T) {
	svc := NewDiagnosisAnalysisService(&fakeDiagnosisAnalysisRepository{})
	_, err := svc.PersistAIResult(
		context.Background(), uuid.New(), 4,
		json.RawMessage(`{"status":"invented_status","candidates":[]}`),
	)
	if err == nil {
		t.Fatal("expected unknown status to be rejected")
	}
}

func TestAssessCandidatesRejectsCandidateFromAnotherAnalysis(t *testing.T) {
	analysisID := uuid.New()
	validCandidateID := uuid.New()
	repo := &fakeDiagnosisAnalysisRepository{byID: &model.DiagnosisAnalysisRecord{
		ID:         analysisID,
		Candidates: []model.DiagnosisCandidateRecord{{ID: validCandidateID, AnalysisID: analysisID}},
	}}
	svc := NewDiagnosisAnalysisService(repo)

	_, err := svc.AssessCandidates(context.Background(), uuid.New(), analysisID, map[uuid.UUID]string{
		uuid.New(): "confirmed",
	})
	if err == nil {
		t.Fatal("candidate from another analysis must be rejected")
	}
}

func TestAssessCandidatesPersistsIndependentUserState(t *testing.T) {
	analysisID := uuid.New()
	candidateA := uuid.New()
	candidateB := uuid.New()
	userID := uuid.New()
	repo := &fakeDiagnosisAnalysisRepository{byID: &model.DiagnosisAnalysisRecord{
		ID:     analysisID,
		UserID: userID,
		Candidates: []model.DiagnosisCandidateRecord{
			{ID: candidateA, AnalysisID: analysisID},
			{ID: candidateB, AnalysisID: analysisID},
		},
	}}
	svc := NewDiagnosisAnalysisService(repo)

	items, err := svc.AssessCandidates(context.Background(), userID, analysisID, map[uuid.UUID]string{
		candidateA: "confirmed",
		candidateB: "unsure",
	})
	if err != nil {
		t.Fatalf("AssessCandidates returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two independent candidate assessments, got %d", len(items))
	}
	if items[0].AssessedAt.IsZero() || items[1].AssessedAt.IsZero() {
		t.Fatal("candidate assessments must record an audit timestamp")
	}
}

func TestPublicPayloadExposesAgentConfigurationFromImmutableRawOutput(t *testing.T) {
	svc := NewDiagnosisAnalysisService(&fakeDiagnosisAnalysisRepository{})
	raw := datatypes.JSON(`{"status":"completed","agent_configuration":{"id":"diag-config-test","role":"diagnosis"}}`)
	analysis := &model.DiagnosisAnalysisRecord{ID: uuid.New(), RawOutput: raw}
	payload := svc.PublicPayload(analysis)
	configuration, ok := payload["agent_configuration"].(json.RawMessage)
	if !ok {
		t.Fatalf("expected raw agent_configuration provenance, got %#v", payload["agent_configuration"])
	}
	if !json.Valid(configuration) || string(configuration) != `{"id":"diag-config-test","role":"diagnosis"}` {
		t.Fatalf("unexpected Agent configuration provenance: %s", configuration)
	}
}
