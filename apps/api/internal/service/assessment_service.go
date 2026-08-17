package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

	raw, err := s.reasoner.GenerateAssessment(ctx, AssessmentGenerationRequest{
		Profile:         profilePayload,
		Images:          images,
		PostureAnalysis: posturePayload,
		UseCase:         "llm.json",
	})
	if err != nil {
		return nil, fmt.Errorf("generate typed assessment: %w", err)
	}
	payload, err := parseAssessmentAgentPayload(raw)
	if err != nil {
		return nil, err
	}

	report := &model.AssessmentReport{
		ID: uuid.New(), UserID: userID, Status: payload.Status,
		HealthGrade: payload.HealthGrade, Summary: strings.TrimSpace(payload.Summary),
		DimensionScores: jsonRaw(payload.DimensionScores, `{}`),
		InformationGaps: jsonRaw(payload.InformationGaps, `[]`),
		SafetyNotes:     jsonRaw(payload.SafetyNotes, `[]`),
		CreatedAt:       time.Now().UTC(),
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
