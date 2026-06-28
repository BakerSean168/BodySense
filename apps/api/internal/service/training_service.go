package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// TrainingService handles training business logic.
type TrainingService struct {
	trainingRepo    *repository.TrainingRepository
	profileService  *ProfileService
	aiServiceURL    string
}

func NewTrainingService(
	trainingRepo *repository.TrainingRepository,
	profileService *ProfileService,
) *TrainingService {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}
	return &TrainingService{
		trainingRepo:   trainingRepo,
		profileService: profileService,
		aiServiceURL:   aiServiceURL,
	}
}

// GeneratePlan generates a training plan via AI service.
func (s *TrainingService) GeneratePlan(
	ctx context.Context,
	userID uuid.UUID,
	consultationID *uuid.UUID,
	diagnosis map[string]any,
	preferences map[string]any,
) (*model.TrainingPlan, error) {
	// Get user profile
	profile, _ := s.profileService.GetProfile(ctx, userID)
	profileMap := map[string]any{}
	if profile != nil {
		pj, _ := json.Marshal(profile)
		_ = json.Unmarshal(pj, &profileMap)
	}

	// Call AI service
	aiReq := map[string]any{
		"diagnosis":   diagnosis,
		"profile":     profileMap,
		"preferences": preferences,
	}
	aiReqBody, _ := json.Marshal(aiReq)

	resp, err := http.Post(
		s.aiServiceURL+"/api/training/generate",
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

	var result struct {
		Goal          string         `json:"goal"`
		DurationWeeks int            `json:"duration_weeks"`
		Phases        []map[string]any `json:"phases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode AI response: %w", err)
	}

	phasesJSON, _ := json.Marshal(result.Phases)

	plan := &model.TrainingPlan{
		ID:            uuid.New(),
		UserID:        userID,
		ConsultationID: consultationID,
		Goal:          result.Goal,
		DurationWeeks: result.DurationWeeks,
		CurrentWeek:   1,
		Phases:        phasesJSON,
	}

	if err := s.trainingRepo.CreatePlan(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to save plan: %w", err)
	}

	return plan, nil
}

// GetPlan retrieves a training plan by ID.
func (s *TrainingService) GetPlan(ctx context.Context, id, userID uuid.UUID) (*model.TrainingPlan, error) {
	return s.trainingRepo.GetPlanByID(ctx, id, userID)
}

// ListPlans lists all training plans for a user.
func (s *TrainingService) ListPlans(ctx context.Context, userID uuid.UUID) ([]model.TrainingPlan, error) {
	return s.trainingRepo.ListPlansByUserID(ctx, userID)
}

// GetTodayTask gets today's training task for a plan.
func (s *TrainingService) GetTodayTask(ctx context.Context, planID, userID uuid.UUID) (*model.TrainingLog, error) {
	// Verify plan belongs to user
	plan, err := s.trainingRepo.GetPlanByID(ctx, planID, userID)
	if err != nil || plan == nil {
		return nil, fmt.Errorf("plan not found")
	}

	today := time.Now().Truncate(24 * time.Hour)
	log, err := s.trainingRepo.GetLogByDate(ctx, planID, today)
	if err != nil {
		return nil, err
	}

	// Create empty log if none exists
	if log == nil {
		log = &model.TrainingLog{
			ID:        uuid.New(),
			UserID:    userID,
			PlanID:    planID,
			Date:      today,
			Exercises: []byte("[]"),
		}
	}

	return log, nil
}

// CheckIn records a check-in for today.
func (s *TrainingService) CheckIn(ctx context.Context, planID, userID uuid.UUID) error {
	// Verify plan belongs to user
	plan, err := s.trainingRepo.GetPlanByID(ctx, planID, userID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan not found")
	}

	today := time.Now().Truncate(24 * time.Hour)
	return s.trainingRepo.CheckIn(ctx, planID, userID, today)
}

// UpdateLog updates a training log.
func (s *TrainingService) UpdateLog(ctx context.Context, planID, userID uuid.UUID, notes string, exercises any) error {
	// Verify plan belongs to user
	plan, err := s.trainingRepo.GetPlanByID(ctx, planID, userID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan not found")
	}

	today := time.Now().Truncate(24 * time.Hour)
	exercisesJSON, _ := json.Marshal(exercises)

	return s.trainingRepo.CreateOrUpdateLog(ctx, &model.TrainingLog{
		ID:        uuid.New(),
		UserID:    userID,
		PlanID:    planID,
		Date:      today,
		Exercises: exercisesJSON,
		Notes:     &notes,
	})
}

// GetLogsByPlanID retrieves all training logs for a plan.
func (s *TrainingService) GetLogsByPlanID(ctx context.Context, planID uuid.UUID) ([]model.TrainingLog, error) {
	return s.trainingRepo.GetLogsByPlanID(ctx, planID)
}

// GetProgress returns progress statistics for a plan.
func (s *TrainingService) GetProgress(ctx context.Context, planID, userID uuid.UUID) (map[string]any, error) {
	plan, err := s.trainingRepo.GetPlanByID(ctx, planID, userID)
	if err != nil || plan == nil {
		return nil, fmt.Errorf("plan not found")
	}

	logs, err := s.trainingRepo.GetLogsByPlanID(ctx, planID)
	if err != nil {
		return nil, err
	}

	consecutive, _ := s.trainingRepo.GetConsecutiveCheckInDays(ctx, planID)

	totalCheckIns := 0
	for _, log := range logs {
		if log.IsCheckedIn {
			totalCheckIns++
		}
	}

	return map[string]any{
		"consecutive_days": consecutive,
		"total_checkins":   totalCheckIns,
		"current_week":     plan.CurrentWeek,
		"total_weeks":      plan.DurationWeeks,
	}, nil
}
