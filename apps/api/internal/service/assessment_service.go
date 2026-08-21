package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
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

type assessmentObservationDraft struct {
	Kind        string         `json:"kind"`
	BodyRegion  string         `json:"body_region"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Severity    string         `json:"severity"`
	Confidence  string         `json:"confidence"`
	Method      string         `json:"method"`
	Condition   map[string]any `json:"condition"`
}

type assessmentAgentPayload struct {
	Status          string                       `json:"status"`
	HealthGrade     string                       `json:"health_grade"`
	DimensionScores map[string]float64           `json:"dimension_scores"`
	Observations    []assessmentObservationDraft `json:"observations"`
	Summary         string                       `json:"summary"`
	InformationGaps []string                     `json:"information_gaps"`
	SafetyNotes     []string                     `json:"safety_notes"`
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
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("encode assessment profile: %w", err)
	}
	var profileMap map[string]any
	if err := json.Unmarshal(profileJSON, &profileMap); err != nil {
		return nil, fmt.Errorf("normalize assessment profile: %w", err)
	}

	uploads, err := s.uploads.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load assessment uploads: %w", err)
	}
	images, reportIndicators, completedPosture, err := assessmentInputsFromUploads(uploads)
	if err != nil {
		return nil, err
	}
	profileMap["health_report_indicators"] = reportIndicators
	profilePayload, _ := json.Marshal(profileMap)
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

	raw, err := s.reasoner.GenerateAssessment(ctx, AssessmentGenerationRequest{
		ConfigurationID: configurationID,
		Profile:         profilePayload,
		Images:          images,
		PostureAnalysis: posturePayload,
	})
	if err != nil {
		return nil, fmt.Errorf("generate typed assessment: %w", err)
	}
	payload, err := parseAssessmentAgentPayload(raw)
	if err != nil {
		return nil, err
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
		posturePayload,
		images,
	)
	if replayErr != nil {
		return nil, replayErr
	}
	replayFingerprint := assessmentReplayInputFingerprintOfRaw(replayEnvelope)

	generationTrace := buildAssessmentGenerationTrace(provenance, configurationID, policyRevision, replayFingerprint)

	report := &model.AssessmentReport{
		ID: uuid.New(), UserID: userID, Status: payload.Status,
		HealthGrade: payload.HealthGrade, Summary: strings.TrimSpace(payload.Summary),
		DimensionScores:         jsonRaw(payload.DimensionScores, `{}`),
		InformationGaps:         jsonRaw(payload.InformationGaps, `[]`),
		SafetyNotes:             jsonRaw(payload.SafetyNotes, `[]`),
		AgentConfigurationID:    provenance.AgentConfigurationID,
		AgentConfiguration:      datatypes.JSON(normalizeRaw(provenance.AgentConfiguration, `{}`)),
		ExecutionProvenance:     datatypes.JSON(normalizeRaw(provenance.ExecutionProvenance, `{}`)),
		GenerationDecisionTrace: datatypes.JSON(generationTrace),
		ReplayInput:             datatypes.JSON(replayEnvelope),
		CreatedAt:               time.Now().UTC(),
	}
	projected := make([]map[string]any, 0, len(payload.Observations))
	err = s.unitOfWork.WithinTransaction(ctx, func(txCtx context.Context) error {
		for index, draft := range payload.Observations {
			value := map[string]any{
				"label":       draft.Label,
				"description": draft.Description,
				"severity":    draft.Severity,
				"confidence":  draft.Confidence,
			}
			stored, revision, projectionErr := s.bodyState.AddAssessmentObservation(txCtx, userID, model.BodyStateObservation{
				ConcernKey: bodyStateConcernKey(draft.BodyRegion),
				Kind:       draft.Kind,
				BodyRegion: draft.BodyRegion,
				Method:     firstNonEmpty(draft.Method, "assessment"),
				Value:      datatypes.JSON(jsonRaw(value, `{}`)),
				Condition:  datatypes.JSON(jsonRaw(draft.Condition, `{}`)),
				SourceKey:  fmt.Sprintf("assessment:%s:observation:%d", report.ID, index),
				Provenance: datatypes.JSON(jsonRaw(map[string]any{
					"source_type":          "assessment",
					"assessment_report_id": report.ID,
					"agent_output":         draft,
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
				"severity":       draft.Severity,
				"confidence":     draft.Confidence,
				"method":         stored.Method,
				"condition":      draft.Condition,
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
		// Shadow observation is non-blocking evidence collection; it never
		// changes the served report. Log failures so operators can trace.
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

func parseAssessmentAgentPayload(raw json.RawMessage) (*assessmentAgentPayload, error) {
	var payload assessmentAgentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode assessment output: %w", err)
	}
	if payload.Status != "completed" && payload.Status != "insufficient_information" {
		return nil, fmt.Errorf("invalid assessment status %q", payload.Status)
	}
	if payload.HealthGrade != "A" && payload.HealthGrade != "B" && payload.HealthGrade != "C" && payload.HealthGrade != "D" {
		return nil, fmt.Errorf("invalid assessment health grade %q", payload.HealthGrade)
	}
	for _, key := range []string{"posture", "exercise", "lifestyle", "injury_risk", "overall"} {
		score, ok := payload.DimensionScores[key]
		if !ok || score < 0 || score > 100 {
			return nil, fmt.Errorf("invalid assessment score %q", key)
		}
	}
	if payload.Status == "completed" && len(payload.Observations) == 0 {
		return nil, errors.New("completed assessment requires at least one observation")
	}
	for index, observation := range payload.Observations {
		if strings.TrimSpace(observation.Kind) == "" || strings.TrimSpace(observation.Label) == "" || strings.TrimSpace(observation.Description) == "" {
			return nil, fmt.Errorf("assessment observation %d is incomplete", index)
		}
	}
	return &payload, nil
}

func assessmentInputsFromUploads(uploads []model.UserUpload) ([]string, []any, []model.UserUpload, error) {
	images := make([]string, 0, 3)
	reportIndicators := make([]any, 0)
	completedPosture := make([]model.UserUpload, 0, 3)
	for _, upload := range uploads {
		switch upload.FileType {
		case "photo_front", "photo_side", "photo_back":
			if upload.AnalysisStatus == "completed" && len(upload.AnalysisResult) > 0 {
				completedPosture = append(completedPosture, upload)
				continue
			}
			imageBytes, err := os.ReadFile(upload.FilePath)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("read assessment image %s: %w", upload.ID, err)
			}
			images = append(images, fmt.Sprintf("data:%s;base64,%s", upload.MimeType, base64.StdEncoding.EncodeToString(imageBytes)))
		case "report":
			if upload.OCRStatus != "completed" {
				continue
			}
			var ocrResponse map[string]any
			if json.Unmarshal(upload.OCRResult, &ocrResponse) == nil {
				if result, ok := ocrResponse["result"].(map[string]any); ok {
					if indicators, ok := result["indicators"].([]any); ok {
						reportIndicators = append(reportIndicators, indicators...)
					}
				}
			}
		}
	}
	return images, reportIndicators, completedPosture, nil
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
) json.RawMessage {
	trace := map[string]any{
		"status":                   "generated",
		"phase":                    "generation",
		"agent_configuration_id":   configurationID,
		"decision_policy_revision": policyRevision,
		"evaluated":                prov.AgentConfigurationID != "",
		"outcome":                  "accepted",
		"replay_input_fingerprint": replayFingerprint,
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
	ConfigurationID string            `json:"configuration_id"`
	Profile         json.RawMessage   `json:"profile"`
	PostureAnalysis json.RawMessage   `json:"posture_analysis"`
	Images          []imageDescriptor `json:"images"`
}

type imageDescriptor struct {
	MediaType string `json:"media_type"`
}

func encodeAssessmentReplayInput(
	configurationID string,
	profile json.RawMessage,
	posture json.RawMessage,
	images []string,
) (json.RawMessage, error) {
	if len(profile) == 0 || !json.Valid(profile) {
		profile = json.RawMessage(`{}`)
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
		ConfigurationID: configurationID,
		Profile:         profile,
		PostureAnalysis: posture,
		Images:          descriptors,
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
