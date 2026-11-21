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
	if s.reasoner == nil || s.bodyState == nil {
		return nil, errors.New("assessment domain is not fully configured")
	}
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get assessment profile: %w", err)
	}
	if profile == nil {
		return nil, errors.New("user profile not found")
	}
	profilePayload, err := json.Marshal(profile.AgentContext())
	if err != nil {
		return nil, fmt.Errorf("encode assessment profile: %w", err)
	}
	bodySnapshot, err := s.bodyState.GetAgentBodyContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get assessment BodyState: %w", err)
	}
	if bodySnapshot == nil || bodySnapshot.UserID != userID {
		return nil, errors.New("assessment BodyState owner mismatch or unavailable")
	}
	bodyStatePayload, err := json.Marshal(bodySnapshot)
	if err != nil {
		return nil, fmt.Errorf("encode assessment BodyState: %w", err)
	}

	uploads, err := s.uploads.GetUploads(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load assessment uploads: %w", err)
	}
	reportIndicators := assessmentInputsFromUploads(uploads)
	reportIndicatorsPayload, _ := json.Marshal(reportIndicators)
	reviewedEvidencePayload, err := s.collectReviewedReportEvidence(ctx, userID, uploads)
	if err != nil {
		return nil, err
	}
	posturePayload := json.RawMessage(`{}`)
	postureSummary, err := s.uploads.GetPostureAnalysisSummary(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load assessment posture analysis: %w", err)
	}
	if postureSummary.HasAnalysis {
		posturePayload, _ = json.Marshal(postureSummary)
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
		ConfigurationID:        configurationID,
		Profile:                profilePayload,
		BodyState:              bodyStatePayload,
		ReportIndicators:       reportIndicatorsPayload,
		PostureAnalysis:        posturePayload,
		ReviewedReportEvidence: reviewedEvidencePayload,
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
		reviewedEvidencePayload,
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

	reportStatus, ok := model.ParseAssessmentReportStatus(evidenceProjection.Status)
	if !ok {
		return nil, fmt.Errorf("invalid evidence-derived assessment status %q", evidenceProjection.Status)
	}
	report := &model.AssessmentReport{
		ID:                      uuid.New(),
		UserID:                  userID,
		Status:                  reportStatus,
		ContractRevision:        payload.ContractRevision,
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
	// Generated selections stay in the report (BS-D9). They are not admitted
	// BodyObservations and never create a BodyState revision.
	report.BodyStateRevision = &bodySnapshot.CurrentRevision
	observations := make([]map[string]any, 0, len(evidenceProjection.Observations))
	for _, draft := range evidenceProjection.Observations {
		observations = append(observations, map[string]any{
			"kind": draft.Kind, "body_region": draft.BodyRegion, "label": draft.Label,
			"description": draft.Description, "method": "assessment_evidence", "evidence_refs": draft.EvidenceRefs,
		})
	}
	report.Observations = jsonRaw(observations, `[]`)
	if err := s.assessmentRepo.Create(ctx, report); err != nil {
		return nil, fmt.Errorf("persist assessment report: %w", err)
	}

	if s.rollout != nil && assessmentRoute != nil {
		if observeErr := s.rollout.ObserveReport(ctx, userID, *assessmentRoute, report.ID); observeErr != nil {
			log.Printf("assessment rollout shadow observation failed: %v", observeErr)
		}
	}
	return report, nil
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
		return nil, errors.New("Assessment configuration has no current evidence policy")
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

func jsonRaw(value any, fallback string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(fallback)
	}
	return encoded
}
