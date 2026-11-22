package service

import (
	"context"
	"encoding/json"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestAssessmentSourceReadFailurePreventsReportPersistence(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAssessmentRepository{}
	bodyState := exerciseAssessmentBodyState()
	bodyState.fail = true
	svc := NewAssessmentService(
		repo,
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		bodyState,
		fakeAssessmentReasoner{raw: assessmentExerciseOutput()},
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err == nil {
		t.Fatal("BodyState source read failure must fail assessment generation")
	}
	if repo.created != nil {
		t.Fatal("report must not persist after source read failure")
	}
}

func TestAssessmentPersistsAgentProvenanceAndDecisionTrace(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAssessmentRepository{}
	policy, err := NewAgentDeploymentPolicy(testProcessConfig().Deployment)
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}
	svc := NewAssessmentService(
		repo,
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		exerciseAssessmentBodyState(),
		fakeAssessmentReasoner{raw: assessmentOutputWithProvenance(assessmentExerciseOutput())},
	).WithAssessmentDeployment(policy)

	report, err := svc.GenerateAssessment(context.Background(), userID)
	if err != nil {
		t.Fatalf("GenerateAssessment: %v", err)
	}
	if report.AgentConfigurationID != defaultAssessmentConfigurationID {
		t.Fatalf("expected agent configuration id %q, got %q", defaultAssessmentConfigurationID, report.AgentConfigurationID)
	}
	if len(report.AgentConfiguration) == 0 || len(report.ExecutionProvenance) == 0 {
		t.Fatal("report must persist agent configuration and execution provenance")
	}
	trace := string(report.GenerationDecisionTrace)
	if trace == "{}" || !strings.Contains(trace, assessmentOutputContractV2) || !strings.Contains(trace, assessmentEvidencePolicyV4) {
		t.Fatalf("generation trace must freeze current contract/evidence policy: %s", trace)
	}
}

func TestAssessmentRejectsIdentityMismatch(t *testing.T) {
	userID := uuid.New()
	policy, err := NewAgentDeploymentPolicy(testProcessConfig().Deployment)
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}
	wrong := json.RawMessage(strings.Replace(
		string(assessmentOutputWithProvenance(assessmentExerciseOutput())),
		defaultAssessmentConfigurationID,
		"assess-config-0000000000000000",
		1,
	))
	svc := NewAssessmentService(
		&fakeAssessmentRepository{},
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		exerciseAssessmentBodyState(),
		fakeAssessmentReasoner{raw: wrong},
	).WithAssessmentDeployment(policy)

	if _, err := svc.GenerateAssessment(context.Background(), userID); err == nil {
		t.Fatal("identity mismatch must fail closed")
	}
}

type capturingAssessmentReasoner struct {
	raw     json.RawMessage
	request AssessmentGenerationRequest
}

func (r *capturingAssessmentReasoner) GenerateAssessment(_ context.Context, request AssessmentGenerationRequest) (json.RawMessage, error) {
	r.request = request
	return r.raw, nil
}

func TestAssessmentConsumesCompletedPostureAnalysisWithoutRawImageAuthority(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	reasoner := &capturingAssessmentReasoner{raw: assessmentPostureOutput("posture:upload:" + uploadID.String() + ":finding:0")}
	svc := NewAssessmentService(
		&fakeAssessmentRepository{},
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{
			uploads: []model.UserUploadView{{
				ID: uploadID, UserID: userID, FileType: "photo_front", OriginalName: "front.png",
				MimeType: "image/png", AnalysisStatus: "completed",
				AnalysisResult: json.RawMessage(`{
					"view":"front",
					"findings":[{"key":"uneven_shoulders","label":"肩部对称性待复核","evidence":"右侧肩峰位置略高"}]
				}`),
			}},
			posture: PostureAnalysisSummary{
				HasAnalysis: true,
				Views: []PostureAnalysisView{{
					UploadID: uploadID.String(), View: "front", FileType: "photo_front",
					AnalysisStatus: "completed",
					Analysis:       json.RawMessage(`{"view":"front","findings":[{"key":"uneven_shoulders","label":"肩部对称性待复核","evidence":"右侧肩峰位置略高"}]}`),
				}},
				Findings:  []any{map[string]any{"key": "uneven_shoulders"}},
				Summaries: []string{},
			},
		},
		&fakeAssessmentBodyState{},
		reasoner,
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err != nil {
		t.Fatalf("GenerateAssessment: %v", err)
	}
	if len(reasoner.request.Images) != 0 {
		t.Fatalf("assessment-output-v2 must not receive raw images: %#v", reasoner.request.Images)
	}
	var posture map[string]any
	if err := json.Unmarshal(reasoner.request.PostureAnalysis, &posture); err != nil {
		t.Fatalf("posture analysis payload invalid: %v", err)
	}
	if posture["has_analysis"] != true {
		t.Fatalf("expected completed Posture evidence, got %#v", posture)
	}
}

func TestAssessmentDerivesInsufficientStatusWhenOnlyProfileExists(t *testing.T) {
	userID := uuid.New()
	raw := json.RawMessage(`{
		"contract_revision":"assessment-output-v2",
		"status":"insufficient_information",
		"evidence_policy_revision":"assessment-evidence-contract-v4",
		"observations":[],
		"evidence_coverage":{},
		"evidence_gaps":[],
		"summary":"upstream derived copy is non-authoritative",
		"safety_notes":[],
		"governance":{"verdict":"accepted","policy_revision":"assessment-governance-v2","issues":[]}
	}`)
	repo := &fakeAssessmentRepository{}
	gender := "male"
	svc := NewAssessmentService(
		repo,
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID, Gender: &gender}},
		fakeAssessmentUploadSource{},
		&fakeAssessmentBodyState{},
		fakeAssessmentReasoner{raw: raw},
	)

	report, err := svc.GenerateAssessment(context.Background(), userID)
	if err != nil {
		t.Fatalf("GenerateAssessment: %v", err)
	}
	if report.Status != "insufficient_information" {
		t.Fatalf("profile must not manufacture health evidence, got status %q", report.Status)
	}
	if report.Summary != "当前资料支持 0 项待审核观察；0/6 个证据领域已有资料，6/6 个领域当前未提供资料。" {
		t.Fatalf("unexpected deterministic summary: %q", report.Summary)
	}
	if len(report.EvidenceGaps) == 0 || !strings.Contains(string(report.EvidenceGaps), `"required":false`) {
		t.Fatalf("coverage gaps must be non-clinical requirements: %s", report.EvidenceGaps)
	}
}

func TestAssessmentGenerationTraceDistinguishesNoModelDerivation(t *testing.T) {
	raw := json.RawMessage(`{
		"agent_configuration":{"id":"assess-config-e579030c2b8b540c","role":"assessment"},
		"execution_provenance":{"status":"skipped_no_evidence","runtime":"deterministic","usage":{"requests":0}}
	}`)
	prov, err := parseAssessmentProvenance(raw)
	if err != nil {
		t.Fatalf("parseAssessmentProvenance: %v", err)
	}
	traceRaw := buildAssessmentGenerationTrace(
		prov,
		defaultAssessmentConfigurationID,
		AssessmentDecisionPolicyV2,
		"fingerprint",
		assessmentOutputContractV2,
		assessmentEvidencePolicyV4,
	)
	var trace map[string]any
	if err := json.Unmarshal(traceRaw, &trace); err != nil {
		t.Fatalf("decode trace: %v", err)
	}
	if trace["status"] != "derived_without_model" || trace["phase"] != "deterministic_derivation" {
		t.Fatalf("no-model derivation must be explicit: %#v", trace)
	}
	if executed, _ := trace["model_executed"].(bool); executed {
		t.Fatalf("trace must report model_executed=false: %#v", trace)
	}
	if trace["execution_status"] != "skipped_no_evidence" {
		t.Fatalf("unexpected execution status: %#v", trace)
	}
}
