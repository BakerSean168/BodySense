package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeAssessmentRepository struct {
	created   *model.AssessmentReport
	createErr error
}

func (r *fakeAssessmentRepository) Create(_ context.Context, report *model.AssessmentReport) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = report
	return nil
}
func (r *fakeAssessmentRepository) GetByID(context.Context, uuid.UUID, uuid.UUID) (*model.AssessmentReport, error) {
	return r.created, nil
}
func (r *fakeAssessmentRepository) ListByUserID(context.Context, uuid.UUID, int, int) ([]model.AssessmentReport, int64, error) {
	if r.created == nil {
		return nil, 0, nil
	}
	return []model.AssessmentReport{*r.created}, 1, nil
}

type fakeAssessmentProfileSource struct{ profile *model.UserProfile }

func (s fakeAssessmentProfileSource) GetProfile(context.Context, uuid.UUID) (*model.UserProfile, error) {
	return s.profile, nil
}

type fakeAssessmentUploadSource struct{ uploads []model.UserUpload }

func (s fakeAssessmentUploadSource) GetByUserID(context.Context, uuid.UUID) ([]model.UserUpload, error) {
	return s.uploads, nil
}

type fakeAssessmentBodyState struct {
	facts             []model.BodyStateFact
	stateObservations []model.BodyStateObservation
	observations      []model.BodyStateObservation
	fail              bool
}

func (s *fakeAssessmentBodyState) GetSnapshot(_ context.Context, userID uuid.UUID, _ int) (*BodyStateSnapshot, error) {
	return &BodyStateSnapshot{
		UserID: userID, SafetyState: json.RawMessage(`{}`),
		Facts: s.facts, Observations: s.stateObservations,
	}, nil
}

func (s *fakeAssessmentBodyState) AddAssessmentObservation(_ context.Context, userID uuid.UUID, observation model.BodyStateObservation) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	if s.fail {
		return nil, nil, errors.New("projection failed")
	}
	observation.ID = uuid.New()
	observation.UserID = userID
	observation.ReviewState = "unverified"
	observation.ExcludedFromReasoning = true
	revision := int64(len(s.observations) + 1)
	observation.CreatedRevision = revision
	observation.UpdatedRevision = revision
	s.observations = append(s.observations, observation)
	return &observation, &model.BodyStateRevision{Revision: revision}, nil
}

type fakeAssessmentReasoner struct{ raw json.RawMessage }

func (r fakeAssessmentReasoner) GenerateAssessment(context.Context, AssessmentGenerationRequest) (json.RawMessage, error) {
	return r.raw, nil
}

func exerciseAssessmentBodyState() *fakeAssessmentBodyState {
	return &fakeAssessmentBodyState{facts: []model.BodyStateFact{{
		Kind: model.BodyStateFactKindLifestyleExercise, Value: "健身；频率：1-2",
	}}}
}

func assessmentExerciseOutput() json.RawMessage {
	return json.RawMessage(`{
		"contract_revision":"assessment-output-v2",
		"status":"completed",
		"evidence_policy_revision":"assessment-evidence-contract-v2",
		"observations":[{
			"kind":"exercise_pattern","body_region":"","label":"当前运动频率记录",
			"description":"当前 BodyState 记录每周健身 1-2 次。","evidence_refs":["body_state:fact:0"]
		}],
		"evidence_coverage":{
			"status":"partial","available_sources":["body_state"],
			"dimensions":{
				"posture":{"status":"missing","evidence_refs":[]},
				"exercise":{"status":"available","evidence_refs":["body_state:fact:0"]},
				"lifestyle":{"status":"missing","evidence_refs":[]},
				"injury_safety":{"status":"missing","evidence_refs":[]}
			}
		},
		"evidence_gaps":[
			{"dimension":"posture","description":"当前未提供已完成的体态分析。","needed_sources":["posture_analysis"]},
			{"dimension":"lifestyle","description":"缺少当前生活方式资料。","needed_sources":["body_state"]},
			{"dimension":"injury_safety","description":"缺少伤病史、症状或相关健康报告资料。","needed_sources":["body_state","report"]}
		],
		"summary":"当前资料支持 1 项待审核观察；1/4 个评估维度具备可用证据，3/4 个维度仍需补充资料。",
		"safety_notes":[],
		"governance":{"verdict":"accepted","policy_revision":"assessment-governance-v2","issues":[]}
	}`)
}

func assessmentPostureOutput(evidenceRef string) json.RawMessage {
	raw := `{
		"contract_revision":"assessment-output-v2",
		"status":"completed",
		"evidence_policy_revision":"assessment-evidence-contract-v2",
		"observations":[{
			"kind":"posture_alignment","body_region":"肩部","label":"肩部对称性待复核",
			"description":"正面视觉资料中右侧肩峰位置略高。","evidence_refs":["__POSTURE_EVIDENCE_REF__"]
		}],
		"evidence_coverage":{
			"status":"partial","available_sources":["posture_analysis"],
			"dimensions":{
				"posture":{"status":"available","evidence_refs":["__POSTURE_EVIDENCE_REF__"]},
				"exercise":{"status":"missing","evidence_refs":[]},
				"lifestyle":{"status":"missing","evidence_refs":[]},
				"injury_safety":{"status":"missing","evidence_refs":[]}
			}
		},
		"evidence_gaps":[
			{"dimension":"exercise","description":"缺少当前运动方式或频率资料。","needed_sources":["body_state"]},
			{"dimension":"lifestyle","description":"缺少当前生活方式资料。","needed_sources":["body_state"]},
			{"dimension":"injury_safety","description":"缺少伤病史、症状或相关健康报告资料。","needed_sources":["body_state","report"]}
		],
		"summary":"当前资料支持 1 项待审核观察；1/4 个评估维度具备可用证据，3/4 个维度仍需补充资料。",
		"safety_notes":[],
		"governance":{"verdict":"accepted","policy_revision":"assessment-governance-v2","issues":[]}
	}`
	return json.RawMessage(strings.ReplaceAll(raw, "__POSTURE_EVIDENCE_REF__", evidenceRef))
}

func assessmentOutputWithProvenance(raw json.RawMessage) json.RawMessage {
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	payload["agent_configuration"] = map[string]any{
		"id":                       defaultAssessmentConfigurationID,
		"role":                     "assessment",
		"decision_policy_revision": AssessmentDecisionPolicyV2,
	}
	payload["execution_provenance"] = map[string]any{
		"status": "executed", "runtime": "pydantic-ai", "logical_model": "bodysense-structured",
	}
	encoded, _ := json.Marshal(payload)
	return encoded
}

func TestAssessmentPersistsReportAndUnverifiedBodyStateObservationsAtomically(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAssessmentRepository{}
	bodyState := exerciseAssessmentBodyState()
	transactionCalled := false
	svc := NewAssessmentService(
		repo,
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		bodyState,
		fakeAssessmentReasoner{raw: assessmentExerciseOutput()},
		testTreatmentUnitOfWork{called: &transactionCalled},
	)

	report, err := svc.GenerateAssessment(context.Background(), userID)
	if err != nil {
		t.Fatalf("GenerateAssessment returned error: %v", err)
	}
	if !transactionCalled || repo.created == nil {
		t.Fatal("report and observations must use the coordinated unit of work")
	}
	if report.ContractRevision != assessmentOutputContractV2 || report.HealthGrade != nil || len(report.DimensionScores) != 0 {
		t.Fatalf("v2 report must retire model-authored grade/scores: %#v", report)
	}
	if len(bodyState.observations) != 1 {
		t.Fatalf("expected one projected observation, got %d", len(bodyState.observations))
	}
	observation := bodyState.observations[0]
	if observation.ReviewState != "unverified" || !observation.ExcludedFromReasoning {
		t.Fatalf("assessment observation must await explicit review: %#v", observation)
	}
	if report.BodyStateRevision == nil || *report.BodyStateRevision != 1 {
		t.Fatalf("report must link its final BodyState revision: %#v", report.BodyStateRevision)
	}
	var reportObservations []map[string]any
	if err := json.Unmarshal(report.Observations, &reportObservations); err != nil {
		t.Fatalf("report observations are invalid JSON: %v", err)
	}
	if reportObservations[0]["observation_id"] == nil || reportObservations[0]["review_state"] != "unverified" {
		t.Fatalf("report must expose review identity/state: %#v", reportObservations)
	}
	refs, _ := reportObservations[0]["evidence_refs"].([]any)
	if len(refs) != 1 || refs[0] != "body_state:fact:0" {
		t.Fatalf("report must persist exact evidence refs: %#v", reportObservations[0])
	}
}

func TestAssessmentRejectsTreatmentLikeLegacyPayload(t *testing.T) {
	userID := uuid.New()
	svc := NewAssessmentService(
		&fakeAssessmentRepository{},
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		&fakeAssessmentBodyState{},
		fakeAssessmentReasoner{raw: json.RawMessage(`{
			"health_grade":"B",
			"dimension_scores":{"posture":70},
			"observations":[],"summary":{"exercise":"do squats"}
		}`)},
		testTreatmentUnitOfWork{},
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err == nil {
		t.Fatal("legacy grade/score/advice payload must not cross the v2 assessment contract")
	}
}

func TestAssessmentDurableProjectionDoesNotTrustUpstreamDerivedCopy(t *testing.T) {
	userID := uuid.New()
	raw := strings.ReplaceAll(
		string(assessmentExerciseOutput()),
		"当前资料支持 1 项待审核观察；1/4 个评估维度具备可用证据，3/4 个维度仍需补充资料。",
		"upstream copy may change without becoming durable truth",
	)
	raw = strings.ReplaceAll(raw, "当前未提供已完成的体态分析。", "upstream localized posture gap")
	repo := &fakeAssessmentRepository{}
	svc := NewAssessmentService(
		repo,
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		exerciseAssessmentBodyState(),
		fakeAssessmentReasoner{raw: json.RawMessage(raw)},
		testTreatmentUnitOfWork{},
	)

	report, err := svc.GenerateAssessment(context.Background(), userID)
	if err != nil {
		t.Fatalf("derived upstream copy must not be a cross-service security invariant: %v", err)
	}
	if report.Summary != "当前资料支持 1 项待审核观察；1/6 个证据领域已有资料，5/6 个领域当前未提供资料。" {
		t.Fatalf("Go must persist its own derived summary, got %q", report.Summary)
	}
	if strings.Contains(string(report.EvidenceGaps), "upstream localized posture gap") {
		t.Fatalf("Go must persist its own derived gaps, got %s", report.EvidenceGaps)
	}
	if !strings.Contains(string(report.EvidenceGaps), "当前未提供已完成的体态分析。") {
		t.Fatalf("Go-derived posture gap missing: %s", report.EvidenceGaps)
	}
}

func TestAssessmentRejectsUnsupportedPostureObservationWithoutVisualEvidence(t *testing.T) {
	userID := uuid.New()
	svc := NewAssessmentService(
		&fakeAssessmentRepository{},
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		&fakeAssessmentBodyState{},
		fakeAssessmentReasoner{raw: assessmentPostureOutput("posture:upload:missing:finding:0")},
		testTreatmentUnitOfWork{},
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err == nil || !strings.Contains(err.Error(), "unavailable evidence") {
		t.Fatalf("unsupported posture claim must fail closed, got %v", err)
	}
}

func TestAssessmentDoesNotReuseUnverifiedBodyStateAsEvidence(t *testing.T) {
	userID := uuid.New()
	bodyState := &fakeAssessmentBodyState{facts: []model.BodyStateFact{{
		Kind:                  model.BodyStateFactKindLifestyleExercise,
		Value:                 "健身；频率：1-2",
		ReviewState:           "unverified",
		LifecycleState:        "active",
		ExcludedFromReasoning: true,
	}}}
	svc := NewAssessmentService(
		&fakeAssessmentRepository{},
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		bodyState,
		fakeAssessmentReasoner{raw: assessmentExerciseOutput()},
		testTreatmentUnitOfWork{},
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err == nil || !strings.Contains(err.Error(), "unavailable evidence") {
		t.Fatalf("unverified BodyState must not bootstrap new Assessment claims, got %v", err)
	}
}

func TestAssessmentDurableBoundaryIgnoresUpstreamObservationProse(t *testing.T) {
	userID := uuid.New()
	raw := strings.Replace(
		string(assessmentExerciseOutput()),
		"当前 BodyState 记录每周健身 1-2 次。",
		"当前 BodyState 记录每周健身 1-2 次，建议增加运动频率。",
		1,
	)
	bodyState := exerciseAssessmentBodyState()
	svc := NewAssessmentService(
		&fakeAssessmentRepository{},
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		bodyState,
		fakeAssessmentReasoner{raw: json.RawMessage(raw)},
		testTreatmentUnitOfWork{},
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err != nil {
		t.Fatalf("upstream prose must be non-authoritative, got %v", err)
	}
	if len(bodyState.observations) != 1 {
		t.Fatalf("expected one rendered observation, got %d", len(bodyState.observations))
	}
	var value map[string]any
	if err := json.Unmarshal(bodyState.observations[0].Value, &value); err != nil {
		t.Fatalf("decode rendered observation value: %v", err)
	}
	description, _ := value["description"].(string)
	if description != "来源记录：健身；频率：1-2。" || strings.Contains(description, "建议") {
		t.Fatalf("Go must deterministically render source-only prose, got %q", description)
	}
}

func TestAssessmentProjectionFailurePreventsReportPersistence(t *testing.T) {
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
		testTreatmentUnitOfWork{},
	)
	if _, err := svc.GenerateAssessment(context.Background(), userID); err == nil {
		t.Fatal("BodyState projection failure must fail the assessment write")
	}
	if repo.created != nil {
		t.Fatal("report must not persist after observation projection failure")
	}
}

func TestAssessmentPersistsAgentProvenanceAndDecisionTrace(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAssessmentRepository{}
	policy, err := NewAgentDeploymentPolicy()
	if err != nil {
		t.Fatalf("NewAgentDeploymentPolicy: %v", err)
	}
	svc := NewAssessmentService(
		repo,
		fakeAssessmentProfileSource{profile: &model.UserProfile{UserID: userID}},
		fakeAssessmentUploadSource{},
		exerciseAssessmentBodyState(),
		fakeAssessmentReasoner{raw: assessmentOutputWithProvenance(assessmentExerciseOutput())},
		testTreatmentUnitOfWork{},
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
	if trace == "{}" || !strings.Contains(trace, assessmentOutputContractV2) || !strings.Contains(trace, assessmentEvidencePolicyV2) {
		t.Fatalf("generation trace must freeze v2 contract/evidence policy: %s", trace)
	}
}

func TestAssessmentRejectsIdentityMismatch(t *testing.T) {
	userID := uuid.New()
	policy, err := NewAgentDeploymentPolicy()
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
		testTreatmentUnitOfWork{},
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
		fakeAssessmentUploadSource{uploads: []model.UserUpload{{
			ID: uploadID, UserID: userID, FileType: "photo_front", OriginalName: "front.png",
			MimeType: "image/png", AnalysisStatus: "completed",
			AnalysisResult: json.RawMessage(`{
				"view":"front",
				"findings":[{"key":"uneven_shoulders","label":"肩部对称性待复核","evidence":"右侧肩峰位置略高"}]
			}`),
		}}},
		&fakeAssessmentBodyState{},
		reasoner,
		testTreatmentUnitOfWork{},
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
		"evidence_policy_revision":"assessment-evidence-contract-v2",
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
		testTreatmentUnitOfWork{},
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
		"agent_configuration":{"id":"assess-config-c6cfff22aa362fff","role":"assessment"},
		"execution_provenance":{"status":"skipped_no_evidence","runtime":"deterministic","usage":{"requests":0}}
	}`)
	prov, err := parseAssessmentProvenance(raw)
	if err != nil {
		t.Fatalf("parseAssessmentProvenance: %v", err)
	}
	traceRaw := buildAssessmentGenerationTrace(
		prov,
		"assess-config-c6cfff22aa362fff",
		AssessmentDecisionPolicyV2,
		"fingerprint",
		assessmentOutputContractV2,
		assessmentEvidencePolicyV2,
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
