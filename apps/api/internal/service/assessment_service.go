package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// AssessmentService handles assessment business logic.
type AssessmentService struct {
	assessmentRepo *repository.AssessmentRepository
	profileService *ProfileService
	aiServiceURL   string
}

// NewAssessmentService creates a new AssessmentService.
func NewAssessmentService(
	assessmentRepo *repository.AssessmentRepository,
	profileService *ProfileService,
) *AssessmentService {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}
	return &AssessmentService{
		assessmentRepo: assessmentRepo,
		profileService: profileService,
		aiServiceURL:   aiServiceURL,
	}
}

// GenerateAssessment generates a new assessment report for a user.
func (s *AssessmentService) GenerateAssessment(ctx context.Context, userID uuid.UUID) (*model.AssessmentReport, error) {
	// Get user profile
	profile, err := s.profileService.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}
	if profile == nil {
		return nil, fmt.Errorf("user profile not found")
	}

	// Convert profile to map for AI service
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal profile: %w", err)
	}

	var profileMap map[string]any
	if err := json.Unmarshal(profileJSON, &profileMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile: %w", err)
	}

	// Call AI service
	aiReq := map[string]any{
		"profile":     profileMap,
		"rag_context": "",
	}

	aiReqBody, err := json.Marshal(aiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AI request: %w", err)
	}

	resp, err := http.Post(
		s.aiServiceURL+"/api/assessment/generate",
		"application/json",
		bytes.NewBuffer(aiReqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse AI response
	var aiResult struct {
		HealthGrade        string         `json:"health_grade"`
		DimensionScores    map[string]any `json:"dimension_scores"`
		IdentifiedIssues   []any          `json:"identified_issues"`
		ImprovementSummary map[string]any `json:"improvement_summary"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&aiResult); err != nil {
		return nil, fmt.Errorf("failed to decode AI response: %w", err)
	}

	// Create report
	report := &model.AssessmentReport{
		ID:          uuid.New(),
		UserID:      userID,
		HealthGrade: aiResult.HealthGrade,
	}

	// Marshal JSONB fields
	if scoresJSON, err := json.Marshal(aiResult.DimensionScores); err == nil {
		report.DimensionScores = scoresJSON
	}
	if issuesJSON, err := json.Marshal(aiResult.IdentifiedIssues); err == nil {
		report.IdentifiedIssues = issuesJSON
	}
	if summaryJSON, err := json.Marshal(aiResult.ImprovementSummary); err == nil {
		report.ImprovementSummary = summaryJSON
	}

	// Save to database
	if err := s.assessmentRepo.Create(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to save report: %w", err)
	}

	return report, nil
}

// GetReport retrieves an assessment report by ID.
func (s *AssessmentService) GetReport(ctx context.Context, id, userID uuid.UUID) (*model.AssessmentReport, error) {
	return s.assessmentRepo.GetByID(ctx, id, userID)
}

// ListReports retrieves assessment reports for a user with pagination.
func (s *AssessmentService) ListReports(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.AssessmentReport, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.assessmentRepo.ListByUserID(ctx, userID, limit, offset)
}
