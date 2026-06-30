package workflow

import (
	"context"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// HealthJourneyWorkflow derives a user's current health journey stage
// from existing tables. It is read-only and does not modify any data.
type HealthJourneyWorkflow struct {
	profileRepo      *repository.ProfileRepository
	uploadRepo       *repository.UploadRepository
	consultationRepo *repository.ConsultationRepository
	assessmentRepo   *repository.AssessmentRepository
	trainingRepo     *repository.TrainingRepository
}

// NewHealthJourneyWorkflow creates a new HealthJourneyWorkflow.
func NewHealthJourneyWorkflow(
	profileRepo *repository.ProfileRepository,
	uploadRepo *repository.UploadRepository,
	consultationRepo *repository.ConsultationRepository,
	assessmentRepo *repository.AssessmentRepository,
	trainingRepo *repository.TrainingRepository,
) *HealthJourneyWorkflow {
	return &HealthJourneyWorkflow{
		profileRepo:      profileRepo,
		uploadRepo:       uploadRepo,
		consultationRepo: consultationRepo,
		assessmentRepo:   assessmentRepo,
		trainingRepo:     trainingRepo,
	}
}

// GetJourneyState computes the current journey state for a user.
func (w *HealthJourneyWorkflow) GetJourneyState(
	ctx context.Context,
	userID uuid.UUID,
) (*dto.HealthJourneyState, error) {
	artifacts := dto.JourneyArtifacts{
		DerivedFrom: []string{},
	}

	// 1. Check profile
	profile, err := w.profileRepo.GetByUserID(ctx, userID)
	if err == nil && profile != nil {
		artifacts.HasProfile = true
		artifacts.DerivedFrom = append(artifacts.DerivedFrom, "user_profiles")
	}

	// 2. Check uploads
	uploads, err := w.uploadRepo.GetByUserID(ctx, userID)
	if err == nil && len(uploads) > 0 {
		artifacts.HasUpload = true
		artifacts.DerivedFrom = append(artifacts.DerivedFrom, "user_uploads")
	}

	// 3. Check consultations
	consultations, err := w.consultationRepo.ListByUserID(ctx, userID)
	if err == nil && len(consultations) > 0 {
		artifacts.HasConsultation = true
		artifacts.DerivedFrom = append(artifacts.DerivedFrom, "consultation_sessions")

		// Find the most recent active consultation
		for _, c := range consultations {
			if c.Phase != "completed" {
				artifacts.ActiveConsultationID = &c.ConversationID
				break
			}
		}

		// Check if any consultation has a diagnosis
		for _, c := range consultations {
			if c.Diagnosis != nil && len(c.Diagnosis) > 0 {
				artifacts.HasDiagnosis = true
				break
			}
		}

		// Check if any consultation has a treatment plan
		for _, c := range consultations {
			if c.TreatmentPlan != nil && len(c.TreatmentPlan) > 0 {
				artifacts.HasTreatment = true
				break
			}
		}
	}

	// 4. Check assessments
	assessments, _, err := w.assessmentRepo.ListByUserID(ctx, userID, 1, 0)
	if err == nil && len(assessments) > 0 {
		artifacts.LatestAssessmentID = &assessments[0].ID
		artifacts.DerivedFrom = append(artifacts.DerivedFrom, "assessment_reports")
	}

	// 5. Check training plans
	plans, err := w.trainingRepo.ListPlansByUserID(ctx, userID)
	if err == nil && len(plans) > 0 {
		artifacts.HasTraining = true
		artifacts.ActiveTrainingPlanID = &plans[0].ID
		artifacts.DerivedFrom = append(artifacts.DerivedFrom, "training_plans")
	}

	// Determine stage based on artifacts
	stage, reason, actions := determineStage(artifacts)

	return &dto.HealthJourneyState{
		Stage:            stage,
		StageReason:      reason,
		AvailableActions: actions,
		Artifacts:        artifacts,
	}, nil
}

// determineStage derives the journey stage from completed artifacts.
func determineStage(a dto.JourneyArtifacts) (string, string, []string) {
	if !a.HasProfile {
		return "onboarding", "用户尚未填写身体档案", []string{"complete_profile"}
	}

	if !a.HasUpload {
		return "data_collection", "已填写档案，尚未上传资料", []string{"upload_report", "upload_photo"}
	}

	if !a.HasConsultation {
		return "ready_for_consultation", "资料已上传，可以开始咨询", []string{"start_consultation"}
	}

	if !a.HasDiagnosis {
		return "consultation_in_progress", "咨询进行中，尚未生成诊断", []string{"continue_consultation", "request_analysis"}
	}

	if !a.HasTreatment {
		return "diagnosis_ready", "诊断已生成，可以确认并生成方案", []string{"confirm_diagnosis", "generate_treatment"}
	}

	if !a.HasTraining {
		return "plan_ready", "方案已生成，可以开始训练", []string{"view_treatment", "start_training"}
	}

	return "training_active", "训练进行中", []string{"view_progress", "log_training", "reassess"}
}
