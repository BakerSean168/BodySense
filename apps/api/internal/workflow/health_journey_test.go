package workflow

import (
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/google/uuid"
)

func TestDetermineStage_Onboarding(t *testing.T) {
	a := dto.JourneyArtifacts{}
	stage, _, actions, missing := determineStage(a)
	if stage != dto.JourneyStageProfileIncomplete {
		t.Errorf("expected profile_incomplete, got %s", stage)
	}
	if len(actions) == 0 {
		t.Error("expected at least one action")
	}
	if len(missing) != 1 || missing[0] != "profile" {
		t.Errorf("expected missing profile, got %v", missing)
	}
}

func TestDetermineStage_DataCollection(t *testing.T) {
	a := dto.JourneyArtifacts{HasProfile: true}
	stage, _, _, _ := determineStage(a)
	if stage != dto.JourneyStageProfileReady {
		t.Errorf("expected profile_ready, got %s", stage)
	}
}

func TestDetermineStage_AssetsUploaded(t *testing.T) {
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true}
	stage, _, _, missing := determineStage(a)
	if stage != dto.JourneyStageAssetsUploaded {
		t.Errorf("expected assets_uploaded, got %s", stage)
	}
	if len(missing) != 1 || missing[0] != "assessment" {
		t.Errorf("expected missing assessment, got %v", missing)
	}
}

func TestDetermineStage_AssessmentReady(t *testing.T) {
	assessmentID := uuid.New()
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, LatestAssessmentID: &assessmentID}
	stage, _, _, _ := determineStage(a)
	if stage != dto.JourneyStageAssessmentReady {
		t.Errorf("expected assessment_ready, got %s", stage)
	}
}

func TestDetermineStage_Consulting(t *testing.T) {
	assessmentID := uuid.New()
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, HasConsultation: true, LatestAssessmentID: &assessmentID}
	stage, _, _, _ := determineStage(a)
	if stage != dto.JourneyStageConsulting {
		t.Errorf("expected consulting, got %s", stage)
	}
}

func TestDetermineStage_DiagnosisReady(t *testing.T) {
	assessmentID := uuid.New()
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, HasConsultation: true, HasDiagnosis: true, LatestAssessmentID: &assessmentID}
	stage, _, _, _ := determineStage(a)
	if stage != dto.JourneyStageDiagnosisReady {
		t.Errorf("expected diagnosis_ready, got %s", stage)
	}
}

func TestDetermineStage_PlanReady(t *testing.T) {
	assessmentID := uuid.New()
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, HasConsultation: true, HasDiagnosis: true, HasTreatment: true, LatestAssessmentID: &assessmentID}
	stage, _, _, _ := determineStage(a)
	if stage != dto.JourneyStagePlanReady {
		t.Errorf("expected plan_ready, got %s", stage)
	}
}

func TestDetermineStage_TrainingActive(t *testing.T) {
	assessmentID := uuid.New()
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, HasConsultation: true, HasDiagnosis: true, HasTreatment: true, HasTraining: true, LatestAssessmentID: &assessmentID}
	stage, _, _, _ := determineStage(a)
	if stage != dto.JourneyStageTrainingActive {
		t.Errorf("expected training_active, got %s", stage)
	}
}

func TestDetermineStage_ReassessmentDue(t *testing.T) {
	assessmentID := uuid.New()
	a := dto.JourneyArtifacts{
		HasProfile:         true,
		HasUpload:          true,
		HasConsultation:    true,
		HasDiagnosis:       true,
		HasTreatment:       true,
		HasTraining:        true,
		NeedsReassessment:  true,
		LatestAssessmentID: &assessmentID,
	}
	stage, _, actions, missing := determineStage(a)
	if stage != dto.JourneyStageReassessmentDue {
		t.Errorf("expected reassessment_due, got %s", stage)
	}
	if len(missing) != 1 || missing[0] != "reassessment" {
		t.Errorf("expected missing reassessment, got %v", missing)
	}
	if len(actions) == 0 || actions[len(actions)-1] != dto.JourneyActionReassess {
		t.Errorf("expected reassess action, got %v", actions)
	}
}

func TestDetermineStage_Completed(t *testing.T) {
	assessmentID := uuid.New()
	a := dto.JourneyArtifacts{
		HasProfile:         true,
		HasUpload:          true,
		HasConsultation:    true,
		HasDiagnosis:       true,
		HasTreatment:       true,
		HasTraining:        true,
		HasReassessment:    true,
		LatestAssessmentID: &assessmentID,
	}
	stage, _, actions, missing := determineStage(a)
	if stage != dto.JourneyStageCompleted {
		t.Errorf("expected completed, got %s", stage)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing requirements, got %v", missing)
	}
	if len(actions) != 1 || actions[0] != dto.JourneyActionReviewSummary {
		t.Errorf("expected review_summary action, got %v", actions)
	}
}

func TestDetermineStage_DerivedFromPopulated(t *testing.T) {
	a := dto.JourneyArtifacts{
		HasProfile:         true,
		HasUpload:          true,
		HasConsultation:    true,
		LatestAssessmentID: ptrUUID(uuid.New()),
		DerivedFrom:        []string{"user_profiles", "user_uploads", "consultation_sessions"},
	}
	stage, _, _, _ := determineStage(a)
	if stage != dto.JourneyStageConsulting {
		t.Errorf("expected consulting, got %s", stage)
	}
}

func TestDetermineStage_ActiveConsultationID(t *testing.T) {
	convID := uuid.New()
	a := dto.JourneyArtifacts{
		HasProfile:           true,
		HasUpload:            true,
		HasConsultation:      true,
		ActiveConsultationID: &convID,
		LatestAssessmentID:   ptrUUID(uuid.New()),
	}
	stage, _, _, _ := determineStage(a)
	if stage != dto.JourneyStageConsulting {
		t.Errorf("expected consulting, got %s", stage)
	}
	if a.ActiveConsultationID == nil || *a.ActiveConsultationID != convID {
		t.Error("expected ActiveConsultationID to be set")
	}
}

func TestDetermineStage_AllComplete(t *testing.T) {
	consultID := uuid.New()
	assessID := uuid.New()
	planID := uuid.New()
	a := dto.JourneyArtifacts{
		HasProfile:           true,
		HasUpload:            true,
		HasConsultation:      true,
		HasDiagnosis:         true,
		HasTreatment:         true,
		HasTraining:          true,
		ActiveConsultationID: &consultID,
		LatestAssessmentID:   &assessID,
		ActiveTrainingPlanID: &planID,
		DerivedFrom:          []string{"user_profiles", "user_uploads", "consultation_sessions", "assessment_reports", "training_plans"},
	}
	stage, _, actions, _ := determineStage(a)
	if stage != dto.JourneyStageTrainingActive {
		t.Errorf("expected training_active, got %s", stage)
	}
	if len(actions) < 2 {
		t.Errorf("expected at least 2 actions, got %d", len(actions))
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}
