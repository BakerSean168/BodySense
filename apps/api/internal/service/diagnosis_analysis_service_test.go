package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

func TestDiagnosisEvidenceAuditUsesFrozenNativeSources(t *testing.T) {
	owner, factID, sourceID, observationID, observationSourceID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	body := BodyStateReadModel{UserID: owner, CurrentRevision: 7,
		Facts:        []model.BodyFact{{ID: factID, UserID: owner, SourceEvidenceID: sourceID, CreatedRevision: 3, AssertionStatus: "asserted"}},
		Observations: []model.BodyObservationRecord{{ID: observationID, UserID: owner, CreatedRevision: 5, AssertionStatus: "asserted", Source: model.BodyObservationSource{ID: observationSourceID, Version: 2}}},
	}
	frozen, err := EncodeDiagnosisReplayInput(7, rawJSON(body, `{}`), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"status":"completed","candidates":[{"name":"hypothesis","confidence":"中"}],"citations":[{"evidence_id":"knowledge-1","summary":"external knowledge"}],"evidence":{"body_state_refs":[{"evidence_id":"forged"}]}}`)
	analysis, err := NewDiagnosisAnalysisService(&fakeDiagnosisAnalysisRepository{}).PersistAIResultWithReplayInput(context.Background(), owner, 7, raw, frozen)
	if err != nil {
		t.Fatal(err)
	}
	var trace struct {
		Evidence struct {
			Authority string `json:"body_state_authority"`
			Refs      []struct {
				ID      uuid.UUID `json:"evidence_id"`
				Kind    string    `json:"evidence_type"`
				Version int64     `json:"version"`
				Source  uuid.UUID `json:"statement_evidence_id"`
			} `json:"body_state_refs"`
			Citations []map[string]any `json:"citations"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(analysis.DecisionTrace, &trace); err != nil {
		t.Fatal(err)
	}
	if trace.Evidence.Authority != "none" || len(trace.Evidence.Refs) != 2 {
		t.Fatalf("missing native audit: %s", analysis.DecisionTrace)
	}
	if got := trace.Evidence.Refs[0]; got.ID != factID || got.Kind != "body_fact" || got.Version != 3 || got.Source != sourceID {
		t.Fatalf("wrong fact reference: %+v", got)
	}
	if got := trace.Evidence.Refs[1]; got.ID != observationID || got.Kind != "body_observation" || got.Version != 5 || got.Source != observationSourceID {
		t.Fatalf("wrong observation reference: %+v", got)
	}
	if len(trace.Evidence.Citations) != 1 || trace.Evidence.Citations[0]["evidence_id"] != "knowledge-1" {
		t.Fatalf("citation identity changed: %s", analysis.DecisionTrace)
	}
}

func TestDiagnosisEvidenceAuditRejectsInvalidFrozenSourcesBeforePersistence(t *testing.T) {
	owner := uuid.New()
	for _, name := range []string{"head owner", "fact owner", "observation owner", "head revision", "future fact", "missing source"} {
		t.Run(name, func(t *testing.T) {
			body := BodyStateReadModel{UserID: owner, CurrentRevision: 7,
				Facts:        []model.BodyFact{{ID: uuid.New(), UserID: owner, SourceEvidenceID: uuid.New(), CreatedRevision: 3}},
				Observations: []model.BodyObservationRecord{{ID: uuid.New(), UserID: owner, CreatedRevision: 5, Source: model.BodyObservationSource{ID: uuid.New(), Version: 1}}},
			}
			switch name {
			case "head owner":
				body.UserID = uuid.New()
			case "fact owner":
				body.Facts[0].UserID = uuid.New()
			case "observation owner":
				body.Observations[0].UserID = uuid.New()
			case "head revision":
				body.CurrentRevision = 8
			case "future fact":
				body.Facts[0].CreatedRevision = 8
			case "missing source":
				body.Facts[0].SourceEvidenceID = uuid.Nil
			}
			frozen, err := EncodeDiagnosisReplayInput(7, rawJSON(body, `{}`), nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			repo := &fakeDiagnosisAnalysisRepository{}
			_, err = NewDiagnosisAnalysisService(repo).PersistAIResultWithReplayInput(context.Background(), owner, 7, json.RawMessage(`{"status":"insufficient_information"}`), frozen)
			if err == nil || repo.createdAnalysis != nil {
				t.Fatalf("invalid frozen source persisted: err=%v", err)
			}
		})
	}
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

	analysis, err := svc.PersistAIResult(context.Background(), uuid.New(), 9, json.RawMessage(raw))
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

func TestPersistAndPublicPayloadUseDedicatedDiagnosisProvenance(t *testing.T) {
	repo := &fakeDiagnosisAnalysisRepository{}
	svc := NewDiagnosisAnalysisService(repo)
	raw := datatypes.JSON(`{
		"status":"completed",
		"scope":"full_body",
		"summary":"qualified result",
		"candidates":[{"name":"candidate","confidence":"中"}],
		"governance":{"kind":"diagnosis","verdict":"accepted"},
		"agent_configuration":{"id":"diag-config-test","role":"diagnosis","decision_policy_revision":"diagnosis-decision-policy-v1"},
		"execution_provenance":{"status":"executed","runtime":"pydantic-ai","gateway_reported_model":"test-model","provider_adapter":"test","agent_run_id":"run-1","usage":{"requests":1,"input_tokens":10,"output_tokens":5,"total_tokens":15}},
		"evidence_acquisition":{"policy_revision":"diagnosis-evidence-gap-v2","budget":{"used_searches":1},"attempts":[{"gap":{"gap_id":"g1"},"evidence_ids":["e1"]}],"unresolved_critical_gaps":[]},
		"rollout_provenance":{"stage":"canary","subject_bucket":42,"canary_bps":500,"served_configuration_id":"diag-config-test","champion_configuration_id":"diag-config-champion","challenger_configuration_id":"diag-config-test","promotion_record":"diagnosis_promotion_v1"},
		"decision_authority":{"policy_revision":"diagnosis-decision-policy-v1","outcome":"allow-normal","reasons":[]}
	}`)
	analysis, err := svc.PersistAIResult(context.Background(), uuid.New(), 9, json.RawMessage(raw))
	if err != nil {
		t.Fatalf("PersistAIResult: %v", err)
	}
	if analysis.AgentConfigurationID != "diag-config-test" {
		t.Fatalf("expected dedicated configuration id, got %q", analysis.AgentConfigurationID)
	}
	if string(analysis.AgentConfiguration) == "{}" || string(analysis.ExecutionProvenance) == "{}" {
		t.Fatalf("expected dedicated configuration/execution provenance: %#v", analysis)
	}
	if string(analysis.EvidenceAcquisitionTrace) == "{}" || string(analysis.DecisionTrace) == "{}" {
		t.Fatalf("expected evidence + decision trace columns: %#v", analysis)
	}
	var trace map[string]any
	if err := json.Unmarshal(analysis.DecisionTrace, &trace); err != nil {
		t.Fatalf("decode DecisionTrace: %v", err)
	}
	if trace["trace_revision"] != "diagnosis-decision-trace-v1" || trace["authority_mode"] != "go-decision-policy" {
		t.Fatalf("unexpected DecisionTrace identity: %#v", trace)
	}
	rollout, ok := trace["rollout_provenance"].(map[string]any)
	if !ok || rollout["stage"] != "canary" || rollout["promotion_record"] != DiagnosisPromotionRecordV1 {
		t.Fatalf("DecisionTrace lost rollout provenance: %#v", trace)
	}
	repo.byID = analysis
	reloaded, err := svc.GetByID(context.Background(), analysis.ID, analysis.UserID)
	if err != nil || reloaded == nil {
		t.Fatalf("reload Diagnosis provenance: analysis=%#v err=%v", reloaded, err)
	}
	if reloaded.AgentConfigurationID != "diag-config-test" || string(reloaded.DecisionTrace) == "{}" {
		t.Fatalf("reload lost durable provenance: %#v", reloaded)
	}
	payload := svc.PublicPayload(reloaded)
	configuration, ok := payload["agent_configuration"].(json.RawMessage)
	if !ok || !json.Valid(configuration) {
		t.Fatalf("expected dedicated agent_configuration provenance, got %#v", payload["agent_configuration"])
	}
	decisionTrace, ok := payload["decision_trace"].(json.RawMessage)
	if !ok || !json.Valid(decisionTrace) {
		t.Fatalf("expected durable DecisionTrace, got %#v", payload["decision_trace"])
	}
	decision, ok := payload["decision_authority"].(json.RawMessage)
	if !ok || !json.Valid(decision) {
		t.Fatalf("expected public decision authority, got %#v", payload["decision_authority"])
	}
}

func TestPersistDiagnosisAnalysisFreezesReplayInputWithoutExposingItOnNormalReadModel(t *testing.T) {
	repo := &fakeDiagnosisAnalysisRepository{}
	svc := NewDiagnosisAnalysisService(repo)
	owner := uuid.New()
	input := replayTestInput(t, 12, owner)
	raw := replayTestRaw(diagnosisDecisionAuthorityConfigID, DiagnosisDecisionPolicyV1, "region:neck")

	analysis, err := svc.PersistAIResultWithReplayInput(context.Background(), owner, 12, raw, input)
	if err != nil {
		t.Fatalf("PersistAIResultWithReplayInput: %v", err)
	}
	if string(analysis.ReplayInput) != string(input) {
		t.Fatalf("replay input must be frozen exactly: %s", analysis.ReplayInput)
	}
	payload := svc.PublicPayload(analysis)
	if _, exposed := payload["replay_input"]; exposed {
		t.Fatal("normal Diagnosis read model must not expose frozen replay input")
	}
}

func TestPersistHistoricalEvidenceGapDiagnosisRequiresConsistentAvailabilityTrace(t *testing.T) {
	repo := &fakeDiagnosisAnalysisRepository{}
	svc := NewDiagnosisAnalysisService(repo)
	valid := json.RawMessage(`{
		"status":"completed",
		"scope":"full_body",
		"summary":"evidence-aware analysis",
		"candidates":[{"name":"candidate","confidence":"中"}],
		"agent_configuration":{"id":"` + diagnosisDecisionAuthorityConfigID + `","role":"diagnosis"},
		"evidence_acquisition":{
			"trace_revision":"evidence-acquisition-trace-v2",
			"policy_revision":"diagnosis-evidence-gap-v2",
			"external_evidence_status":"unresolved",
			"attempts":[{
				"gap":{"kind":"external_knowledge"},
				"status":"unresolved",
				"stop_reason":"published_corpus_empty",
				"search_performed":true,
				"retrieval_status":"published_corpus_empty"
			}]
		}
	}`)
	analysis, err := svc.PersistAIResult(context.Background(), uuid.New(), 17, valid)
	if err != nil {
		t.Fatalf("valid evidence availability trace should persist: %v", err)
	}
	if analysis.AgentConfigurationID != diagnosisDecisionAuthorityConfigID {
		t.Fatalf("configuration id=%q", analysis.AgentConfigurationID)
	}

	invalid := json.RawMessage(`{
		"status":"completed",
		"scope":"full_body",
		"summary":"self-reported drift",
		"candidates":[{"name":"candidate","confidence":"中"}],
		"agent_configuration":{"id":"` + diagnosisDecisionAuthorityConfigID + `","role":"diagnosis"},
		"evidence_acquisition":{
			"trace_revision":"evidence-acquisition-trace-v2",
			"policy_revision":"diagnosis-evidence-gap-v2",
			"external_evidence_status":"available",
			"attempts":[{
				"gap":{"kind":"external_knowledge"},
				"status":"unresolved",
				"stop_reason":"search_unavailable",
				"search_performed":true,
				"retrieval_status":"search_unavailable"
			}]
		}
	}`)
	if _, err := svc.PersistAIResult(context.Background(), uuid.New(), 18, invalid); !errors.Is(err, ErrEvidenceAvailabilityTraceInvalid) {
		t.Fatalf("expected Go trust boundary to reject status drift, got %v", err)
	}
}

func TestPersistDiagnosisAnalysisKeepsAgentHypothesesInAuditOnly(t *testing.T) {
	repo := &fakeDiagnosisAnalysisRepository{}
	svc := NewDiagnosisAnalysisService(repo)
	raw := json.RawMessage(`{"status":"completed","scope":"full_body","summary":"s","candidates":[` +
		`{"concern_key":"region:neck","name":"颈部肌筋膜紧张","confidence":"中","basis":"b","basis_fact_ids":["fact-a"],"counterevidence_ids":["ev-x"],"reasoning_summary":"r1"},` +
		`{"name":"姿势相关","confidence":"低","basis":"b"}]}`)

	analysis, err := svc.PersistAIResult(context.Background(), uuid.New(), 7, raw)
	if err != nil {
		t.Fatalf("PersistAIResult: %v", err)
	}
	var trace struct {
		AgentHypotheses []map[string]any `json:"agent_hypotheses"`
	}
	if err := json.Unmarshal(repo.createdAnalysis.DecisionTrace, &trace); err != nil {
		t.Fatalf("decode decision trace: %v", err)
	}
	if len(trace.AgentHypotheses) != 2 {
		t.Fatalf("expected 2 agent hypotheses in audit, got %d", len(trace.AgentHypotheses))
	}
	first := trace.AgentHypotheses[0]
	if first["candidate_id"] != analysis.Candidates[0].ID.String() || first["statement"] != "颈部肌筋膜紧张" ||
		first["confidence"] != "中" || first["source_actor"] != "diagnosis_agent" || first["body_state_authority"] != "none" {
		t.Fatalf("unexpected agent hypothesis snapshot: %#v", first)
	}
	if facts, ok := first["basis_fact_ids"].([]any); !ok || len(facts) != 1 || facts[0] != "fact-a" {
		t.Fatalf("basis_fact_ids not preserved: %#v", first["basis_fact_ids"])
	}
	if second, ok := trace.AgentHypotheses[1]["counterevidence_ids"].([]any); !ok || len(second) != 0 {
		t.Fatalf("missing id lists must default to []: %#v", trace.AgentHypotheses[1])
	}

	empty, err := NewDiagnosisAnalysisService(&fakeDiagnosisAnalysisRepository{}).PersistAIResult(
		context.Background(), uuid.New(), 8,
		json.RawMessage(`{"status":"safety_blocked","scope":"full_body","candidates":[]}`),
	)
	if err != nil {
		t.Fatalf("PersistAIResult safety_blocked: %v", err)
	}
	if !json.Valid(empty.DecisionTrace) || !strings.Contains(string(empty.DecisionTrace), `"agent_hypotheses":[]`) {
		t.Fatalf("blocked analysis must record an empty agent_hypotheses snapshot: %s", empty.DecisionTrace)
	}
}
