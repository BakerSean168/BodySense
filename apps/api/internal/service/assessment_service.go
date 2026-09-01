package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type assessmentRepository interface {
	Create(ctx context.Context, report *model.AssessmentReport) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.AssessmentReport, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.AssessmentReport, int64, error)
}

type assessmentProfileSource interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error)
}

type assessmentUploadSource interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error)
}

type assessmentBodyStateSource interface {
	GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateSnapshot, error)
	AddAssessmentObservation(ctx context.Context, userID uuid.UUID, observation model.BodyStateObservation) (*model.BodyStateObservation, *model.BodyStateRevision, error)
}

type assessmentReasoner interface {
	GenerateAssessment(ctx context.Context, req AssessmentGenerationRequest) (json.RawMessage, error)
}

type AssessmentService struct {
	assessmentRepo assessmentRepository
	profiles       assessmentProfileSource
	uploads        assessmentUploadSource
	bodyState      assessmentBodyStateSource
	reasoner       assessmentReasoner
	unitOfWork     treatmentUnitOfWork
	deployment     *AgentDeploymentPolicy
	rollout        *AssessmentRolloutService
}

func NewAssessmentService(
	assessmentRepo assessmentRepository,
	profiles assessmentProfileSource,
	uploads assessmentUploadSource,
	bodyState assessmentBodyStateSource,
	reasoner assessmentReasoner,
	unitOfWork treatmentUnitOfWork,
) *AssessmentService {
	return &AssessmentService{
		assessmentRepo: assessmentRepo,
		profiles:       profiles,
		uploads:        uploads,
		bodyState:      bodyState,
		reasoner:       reasoner,
		unitOfWork:     unitOfWork,
	}
}

// WithAssessmentDeployment attaches the Go-owned deployment policy so Assessment
// generation resolves its champion Agent configuration through the North-Star
// control plane and records provenance/decision trace on the immutable report.
func (s *AssessmentService) WithAssessmentDeployment(p *AgentDeploymentPolicy) *AssessmentService {
	s.deployment = p
	return s
}

// WithAssessmentRollout attaches the Go-owned rollout observer so served reports
// trigger anonymous shadow observations when a Challenger is active.
func (s *AssessmentService) WithAssessmentRollout(r *AssessmentRolloutService) *AssessmentService {
	s.rollout = r
	return s
}

const (
	assessmentOutputContractV2 = "assessment-output-v2"
	assessmentEvidencePolicyV2 = "assessment-evidence-contract-v2"
	assessmentEvidencePolicyV3 = "assessment-evidence-contract-v3"
)

// ErrAssessmentOutputRejected means an upstream/generated Assessment failed the
// trusted evidence contract. No Assessment report or BodyState projection may be
// persisted when this error is returned.
var ErrAssessmentOutputRejected = errors.New("assessment output rejected by evidence governance")

var assessmentSafetyNotesV2 = []string{
	"本报告只呈现待审核的资料与体态观察，不构成医疗诊断、治疗方案或运动处方。" +
		"如存在持续疼痛、进行性无力、麻木或严重不适，请寻求专业医疗评估。",
}

type assessmentObservationDraft struct {
	Kind         string   `json:"kind"`
	BodyRegion   string   `json:"body_region"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type assessmentAgentPayload struct {
	ContractRevision       string                       `json:"contract_revision"`
	Status                 string                       `json:"status"`
	EvidencePolicyRevision string                       `json:"evidence_policy_revision"`
	Observations           []assessmentObservationDraft `json:"observations"`
	EvidenceCoverage       map[string]any               `json:"evidence_coverage"`
	EvidenceGaps           []map[string]any             `json:"evidence_gaps"`
	Summary                string                       `json:"summary"`
	SafetyNotes            []string                     `json:"safety_notes"`
	Governance             map[string]any               `json:"governance"`
}

type assessmentEvidenceProjection struct {
	Status       string
	Observations []assessmentObservationDraft
	Coverage     map[string]any
	Gaps         []map[string]any
	Summary      string
}

func (s *AssessmentService) GenerateAssessment(ctx context.Context, userID uuid.UUID) (*model.AssessmentReport, error) {
	if s.reasoner == nil || s.bodyState == nil || s.unitOfWork == nil {
		return nil, errors.New("assessment domain is not fully configured")
	}
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get assessment profile: %w", err)
	}
	if profile == nil {
		return nil, errors.New("user profile not found")
	}
	profilePayload, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("encode assessment profile: %w", err)
	}
	bodySnapshot, err := s.bodyState.GetSnapshot(ctx, userID, 20)
	if err != nil {
		return nil, fmt.Errorf("get assessment BodyState: %w", err)
	}
	bodyStatePayload, err := json.Marshal(bodySnapshot)
	if err != nil {
		return nil, fmt.Errorf("encode assessment BodyState: %w", err)
	}

	uploads, err := s.uploads.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load assessment uploads: %w", err)
	}
	reportIndicators, completedPosture := assessmentInputsFromUploads(uploads)
	reportIndicatorsPayload, _ := json.Marshal(reportIndicators)
	posturePayload := json.RawMessage(`{}`)
	if len(completedPosture) > 0 {
		posturePayload, _ = json.Marshal(BuildPostureAnalysisSummary(completedPosture))
	}

	configurationID := ""
	policyRevision := ""
	var assessmentRoute *AssessmentRouteSelection
	if s.deployment != nil {
		route := s.deployment.SelectAssessmentRoute(userID.String())
		assessmentRoute = &route
		configurationID = route.ServedConfigurationID
		policyRevision = route.ServedDecisionPolicyRevision
		if configurationID == "" || policyRevision == "" {
			return nil, errors.New("assessment deployment policy returned an invalid route")
		}
	}

	generationRequest := AssessmentGenerationRequest{
		ConfigurationID:  configurationID,
		Profile:          profilePayload,
		BodyState:        bodyStatePayload,
		ReportIndicators: reportIndicatorsPayload,
		PostureAnalysis:  posturePayload,
	}
	raw, err := s.reasoner.GenerateAssessment(ctx, generationRequest)
	if err != nil {
		var upstream *AIServiceHTTPError
		if errors.As(err, &upstream) && upstream.StatusCode == http.StatusUnprocessableEntity {
			return nil, fmt.Errorf("%w: upstream governance rejected generated output", ErrAssessmentOutputRejected)
		}
		return nil, fmt.Errorf("generate typed assessment: %w", err)
	}
	payload, err := parseAssessmentAgentPayload(raw, expectedAssessmentEvidencePolicyRevision(configurationID))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssessmentOutputRejected, err)
	}
	evidenceProjection, err := validateAssessmentEvidencePayload(payload, generationRequest)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssessmentOutputRejected, err)
	}

	provenance, provenanceErr := parseAssessmentProvenance(raw)
	if provenanceErr != nil {
		return nil, provenanceErr
	}
	if configurationID != "" && provenance.AgentConfigurationID != "" {
		if err := validateAssessmentAgentIdentity(provenance, configurationID, policyRevision); err != nil {
			return nil, err
		}
	}

	replayEnvelope, replayErr := encodeAssessmentReplayInput(
		configurationID,
		profilePayload,
		bodyStatePayload,
		reportIndicatorsPayload,
		posturePayload,
		nil,
	)
	if replayErr != nil {
		return nil, replayErr
	}
	replayFingerprint := assessmentReplayInputFingerprintOfRaw(replayEnvelope)
	generationTrace := buildAssessmentGenerationTrace(
		provenance,
		configurationID,
		policyRevision,
		replayFingerprint,
		payload.ContractRevision,
		payload.EvidencePolicyRevision,
	)

	report := &model.AssessmentReport{
		ID:                      uuid.New(),
		UserID:                  userID,
		Status:                  evidenceProjection.Status,
		ContractRevision:        payload.ContractRevision,
		HealthGrade:             nil,
		DimensionScores:         nil,
		EvidenceCoverage:        jsonRaw(evidenceProjection.Coverage, `{}`),
		EvidenceGaps:            jsonRaw(evidenceProjection.Gaps, `[]`),
		Summary:                 evidenceProjection.Summary,
		InformationGaps:         json.RawMessage(`[]`),
		SafetyNotes:             jsonRaw(assessmentSafetyNotesV2, `[]`),
		AgentConfigurationID:    provenance.AgentConfigurationID,
		AgentConfiguration:      datatypes.JSON(normalizeRaw(provenance.AgentConfiguration, `{}`)),
		ExecutionProvenance:     datatypes.JSON(normalizeRaw(provenance.ExecutionProvenance, `{}`)),
		GenerationDecisionTrace: datatypes.JSON(generationTrace),
		ReplayInput:             datatypes.JSON(replayEnvelope),
		CreatedAt:               time.Now().UTC(),
	}
	projected := make([]map[string]any, 0, len(evidenceProjection.Observations))
	err = s.unitOfWork.WithinTransaction(ctx, func(txCtx context.Context) error {
		for index, draft := range evidenceProjection.Observations {
			value := map[string]any{
				"label":       draft.Label,
				"description": draft.Description,
			}
			stored, revision, projectionErr := s.bodyState.AddAssessmentObservation(txCtx, userID, model.BodyStateObservation{
				ConcernKey: bodyStateConcernKey(draft.BodyRegion),
				Kind:       draft.Kind,
				BodyRegion: draft.BodyRegion,
				Method:     "assessment_evidence",
				Value:      datatypes.JSON(jsonRaw(value, `{}`)),
				Condition:  datatypes.JSON(`{}`),
				SourceKey:  fmt.Sprintf("assessment:%s:observation:%d", report.ID, index),
				Provenance: datatypes.JSON(jsonRaw(map[string]any{
					"source_type":          "assessment",
					"assessment_report_id": report.ID,
					"contract_revision":    payload.ContractRevision,
					"evidence_selection": map[string]any{
						"kind": draft.Kind, "evidence_refs": draft.EvidenceRefs,
					},
				}, `{}`)),
				ObservedAt:     &report.CreatedAt,
				LifecycleState: "active",
			})
			if projectionErr != nil {
				return projectionErr
			}
			item := map[string]any{
				"observation_id": stored.ID,
				"review_state":   stored.ReviewState,
				"kind":           draft.Kind,
				"body_region":    draft.BodyRegion,
				"label":          draft.Label,
				"description":    draft.Description,
				"method":         stored.Method,
				"evidence_refs":  draft.EvidenceRefs,
			}
			projected = append(projected, item)
			if revision != nil {
				revisionNumber := revision.Revision
				report.BodyStateRevision = &revisionNumber
			}
		}
		report.Observations = jsonRaw(projected, `[]`)
		return s.assessmentRepo.Create(txCtx, report)
	})
	if err != nil {
		return nil, fmt.Errorf("persist assessment and BodyState observations: %w", err)
	}
	if s.rollout != nil && assessmentRoute != nil {
		if observeErr := s.rollout.ObserveReport(ctx, userID, *assessmentRoute, report.ID); observeErr != nil {
			log.Printf("assessment rollout shadow observation failed: %v", observeErr)
		}
	}
	return report, nil
}

func (s *AssessmentService) GetReport(ctx context.Context, id, userID uuid.UUID) (*model.AssessmentReport, error) {
	return s.assessmentRepo.GetByID(ctx, id, userID)
}

func (s *AssessmentService) ListReports(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.AssessmentReport, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.assessmentRepo.ListByUserID(ctx, userID, limit, offset)
}

func expectedAssessmentEvidencePolicyRevision(configurationID string) string {
	configurationID = strings.TrimSpace(configurationID)
	if configurationID == "" {
		configurationID = defaultAssessmentConfigurationID
	}
	if registration, ok := knownAssessmentConfigurations[configurationID]; ok {
		return registration.EvidencePolicyRevision
	}
	return ""
}

func parseAssessmentAgentPayload(raw json.RawMessage, expectedEvidencePolicyRevision string) (*assessmentAgentPayload, error) {
	var payload assessmentAgentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode assessment output: %w", err)
	}
	if payload.ContractRevision != assessmentOutputContractV2 {
		return nil, fmt.Errorf("unsupported assessment contract revision %q", payload.ContractRevision)
	}
	if payload.Status != "completed" && payload.Status != "insufficient_information" {
		return nil, fmt.Errorf("invalid assessment status %q", payload.Status)
	}
	if expectedEvidencePolicyRevision == "" {
		expectedEvidencePolicyRevision = assessmentEvidencePolicyV3
	}
	if payload.EvidencePolicyRevision != expectedEvidencePolicyRevision {
		return nil, fmt.Errorf(
			"assessment evidence policy mismatch: got %q want %q",
			payload.EvidencePolicyRevision, expectedEvidencePolicyRevision,
		)
	}
	if verdict, _ := payload.Governance["verdict"].(string); verdict != "accepted" {
		return nil, fmt.Errorf("assessment governance verdict must be accepted, got %q", verdict)
	}
	for index, observation := range payload.Observations {
		if strings.TrimSpace(observation.Kind) == "" {
			return nil, fmt.Errorf("assessment observation %d has no kind", index)
		}
		if len(observation.EvidenceRefs) != 1 {
			return nil, fmt.Errorf("assessment observation %d must reference exactly one evidence item", index)
		}
	}
	return &payload, nil
}

func assessmentInputsFromUploads(uploads []model.UserUpload) ([]any, []model.UserUpload) {
	reportIndicators := make([]any, 0)
	completedPosture := make([]model.UserUpload, 0, 3)
	for _, upload := range uploads {
		switch upload.FileType {
		case "photo_front", "photo_side", "photo_back":
			if upload.AnalysisStatus == "completed" && len(upload.AnalysisResult) > 0 {
				completedPosture = append(completedPosture, upload)
			}
		case "report":
			if upload.OCRStatus != "completed" {
				continue
			}
			var ocrResponse map[string]any
			if json.Unmarshal(upload.OCRResult, &ocrResponse) == nil {
				if result, ok := ocrResponse["result"].(map[string]any); ok {
					if indicators, ok := result["indicators"].([]any); ok {
						for indicatorIndex, indicator := range indicators {
							reportIndicators = append(reportIndicators, map[string]any{
								"upload_id":       upload.ID.String(),
								"indicator_index": indicatorIndex,
								"value":           indicator,
							})
						}
					}
				}
			}
		}
	}
	return reportIndicators, completedPosture
}

func jsonRaw(value any, fallback string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(fallback)
	}
	return encoded
}

type assessmentProvenance struct {
	AgentConfigurationID    string          `json:"id"`
	AgentConfiguration      json.RawMessage `json:"agent_configuration"`
	ExecutionProvenance     json.RawMessage `json:"execution_provenance"`
	ExecutionStatus         string
	agentConfigurationText  string
	executionProvenanceText string
}

func parseAssessmentProvenance(raw json.RawMessage) (assessmentProvenance, error) {
	var outer struct {
		AgentConfiguration  map[string]any `json:"agent_configuration"`
		ExecutionProvenance map[string]any `json:"execution_provenance"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return assessmentProvenance{}, fmt.Errorf("decode assessment provenance: %w", err)
	}
	configID, _ := outer.AgentConfiguration["id"].(string)
	executionStatus, _ := outer.ExecutionProvenance["status"].(string)
	configJSON, err := json.Marshal(outer.AgentConfiguration)
	if err != nil {
		return assessmentProvenance{}, fmt.Errorf("encode assessment agent configuration: %w", err)
	}
	execJSON, err := json.Marshal(outer.ExecutionProvenance)
	if err != nil {
		return assessmentProvenance{}, fmt.Errorf("encode assessment execution provenance: %w", err)
	}
	return assessmentProvenance{
		AgentConfigurationID:    configID,
		AgentConfiguration:      configJSON,
		ExecutionProvenance:     execJSON,
		ExecutionStatus:         executionStatus,
		agentConfigurationText:  string(configJSON),
		executionProvenanceText: string(execJSON),
	}, nil
}

func validateAssessmentAgentIdentity(
	prov assessmentProvenance,
	expectedConfigurationID string,
	expectedPolicyRevision string,
) error {
	if prov.AgentConfigurationID != expectedConfigurationID {
		return fmt.Errorf(
			"Assessment agent configuration mismatch: got %q want %q",
			prov.AgentConfigurationID,
			expectedConfigurationID,
		)
	}
	if strings.TrimSpace(prov.agentConfigurationText) == "" || strings.TrimSpace(prov.executionProvenanceText) == "" {
		return errors.New("Assessment response missing agent configuration or execution provenance")
	}
	if expectedPolicyRevision != "" {
		registration, ok := knownAssessmentConfigurations[expectedConfigurationID]
		if !ok || registration.DecisionPolicyRevision != expectedPolicyRevision {
			return errors.New("Assessment deployment policy revision not registered for configuration")
		}
	}
	return nil
}

func buildAssessmentGenerationTrace(
	prov assessmentProvenance,
	configurationID string,
	policyRevision string,
	replayFingerprint string,
	contractRevision string,
	evidencePolicyRevision string,
) json.RawMessage {
	modelExecuted := prov.ExecutionStatus == "executed"
	status := "generated"
	phase := "generation"
	if !modelExecuted && prov.ExecutionStatus == "skipped_no_evidence" {
		status = "derived_without_model"
		phase = "deterministic_derivation"
	}
	trace := map[string]any{
		"status":                   status,
		"phase":                    phase,
		"agent_configuration_id":   configurationID,
		"decision_policy_revision": policyRevision,
		"evaluated":                prov.AgentConfigurationID != "",
		"model_executed":           modelExecuted,
		"execution_status":         prov.ExecutionStatus,
		"outcome":                  "accepted",
		"replay_input_fingerprint": replayFingerprint,
		"contract_revision":        contractRevision,
		"evidence_policy_revision": evidencePolicyRevision,
	}
	encoded, err := json.Marshal(trace)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

// AssessmentReplayInput freezes the durable inputs that produced one assessment
// report so historical/counterfactual replay can reproduce the report without
// substituting today's BodyState. Images are stored as sanitized descriptors
// (media-type + count), never raw base64, to keep the replay envelope private
// and lightweight.
type AssessmentReplayInput struct {
	ConfigurationID  string            `json:"configuration_id"`
	Profile          json.RawMessage   `json:"profile"`
	BodyState        json.RawMessage   `json:"body_state"`
	ReportIndicators json.RawMessage   `json:"report_indicators"`
	PostureAnalysis  json.RawMessage   `json:"posture_analysis"`
	Images           []imageDescriptor `json:"images"`
}

type imageDescriptor struct {
	MediaType string `json:"media_type"`
}

func encodeAssessmentReplayInput(
	configurationID string,
	profile json.RawMessage,
	bodyState json.RawMessage,
	reportIndicators json.RawMessage,
	posture json.RawMessage,
	images []string,
) (json.RawMessage, error) {
	if len(profile) == 0 || !json.Valid(profile) {
		profile = json.RawMessage(`{}`)
	}
	if len(bodyState) == 0 || !json.Valid(bodyState) {
		bodyState = json.RawMessage(`{}`)
	}
	if len(reportIndicators) == 0 || !json.Valid(reportIndicators) {
		reportIndicators = json.RawMessage(`[]`)
	}
	if len(posture) == 0 || !json.Valid(posture) {
		posture = json.RawMessage(`{}`)
	}
	descriptors := make([]imageDescriptor, 0, len(images))
	for _, image := range images {
		mediaType := "image/*"
		if start := strings.Index(image, "data:"); start == 0 {
			if semi := strings.Index(image[start:], ";"); semi > 0 {
				mediaType = image[start+5 : start+semi]
			}
		}
		descriptors = append(descriptors, imageDescriptor{MediaType: mediaType})
	}
	return json.Marshal(AssessmentReplayInput{
		ConfigurationID:  configurationID,
		Profile:          profile,
		BodyState:        bodyState,
		ReportIndicators: reportIndicators,
		PostureAnalysis:  posture,
		Images:           descriptors,
	})
}

func decodeAssessmentReplayInput(raw json.RawMessage) (AssessmentReplayInput, error) {
	var input AssessmentReplayInput
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return input, ErrAssessmentReplayUnavailable
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return input, fmt.Errorf("decode Assessment replay input: %w", err)
	}
	if len(input.Profile) == 0 || !json.Valid(input.Profile) {
		return input, ErrAssessmentReplayUnavailable
	}
	return input, nil
}
