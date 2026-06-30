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
	assessmentsCount := 0
	assessments, _, err := w.assessmentRepo.ListByUserID(ctx, userID, 1, 0)
	if err == nil && len(assessments) > 0 {
		assessmentsCount = len(assessments)
		artifacts.LatestAssessmentID = &assessments[0].ID
		artifacts.DerivedFrom = append(artifacts.DerivedFrom, "assessment_reports")
	}

	// 5. Check training plans
	plans, err := w.trainingRepo.ListPlansByUserID(ctx, userID)
	if err == nil && len(plans) > 0 {
		plan := plans[0]
		artifacts.HasTraining = true
		artifacts.ActiveTrainingPlanID = &plan.ID
		artifacts.DerivedFrom = append(artifacts.DerivedFrom, "training_plans")
		if assessmentsCount > 0 && assessments[0].CreatedAt.After(plan.CreatedAt) {
			artifacts.HasReassessment = true
		}
		if !artifacts.HasReassessment && plan.DurationWeeks > 0 && plan.CurrentWeek >= plan.DurationWeeks {
			artifacts.NeedsReassessment = true
		}
	}

	// Determine stage based on artifacts
	stage, reason, actions, missing := determineStage(artifacts)
	artifacts.MissingRequirements = missing

	return &dto.HealthJourneyState{
		Stage:            stage,
		StageReason:      reason,
		AvailableActions: actions,
		Artifacts:        artifacts,
	}, nil
}

// determineStage derives the journey stage from completed artifacts.
func determineStage(a dto.JourneyArtifacts) (string, string, []string, []string) {
	if !a.HasProfile {
		return dto.JourneyStageProfileIncomplete, "用户尚未填写身体档案", []string{dto.JourneyActionCompleteProfile}, []string{"profile"}
	}

	if !a.HasUpload {
		return dto.JourneyStageProfileReady, "已填写档案，尚未上传资料", []string{dto.JourneyActionUploadReport, dto.JourneyActionUploadPhoto}, []string{"upload"}
	}

	if a.LatestAssessmentID == nil {
		return dto.JourneyStageAssetsUploaded, "资料已上传，尚未完成初始评估", []string{dto.JourneyActionStartAssessment}, []string{"assessment"}
	}

	if !a.HasConsultation {
		return dto.JourneyStageAssessmentReady, "初始评估已完成，可以开始咨询", []string{dto.JourneyActionStartConsultation}, []string{"consultation"}
	}

	if !a.HasDiagnosis {
		return dto.JourneyStageConsulting, "咨询进行中，尚未生成诊断", []string{dto.JourneyActionContinueConsult, dto.JourneyActionRequestAnalysis}, []string{"diagnosis"}
	}

	if !a.HasTreatment {
		return dto.JourneyStageDiagnosisReady, "诊断已生成，可以确认并生成方案", []string{dto.JourneyActionConfirmDiagnosis, dto.JourneyActionGenerateTreatment}, []string{"treatment_plan"}
	}

	if !a.HasTraining {
		return dto.JourneyStagePlanReady, "方案已生成，可以开始训练", []string{dto.JourneyActionViewTreatment, dto.JourneyActionStartTraining}, []string{"training_plan"}
	}

	if a.NeedsReassessment {
		return dto.JourneyStageReassessmentDue, "训练周期已完成，需要复评", []string{dto.JourneyActionViewProgress, dto.JourneyActionReassess}, []string{"reassessment"}
	}

	if a.HasReassessment {
		return dto.JourneyStageCompleted, "训练周期和复评已完成", []string{dto.JourneyActionReviewSummary}, nil
	}

	return dto.JourneyStageTrainingActive, "训练进行中", []string{dto.JourneyActionViewProgress, dto.JourneyActionLogTraining, dto.JourneyActionReassess}, nil
}
