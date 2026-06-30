package workflow

import (
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/google/uuid"
)

func TestDetermineStage_Onboarding(t *testing.T) {
	a := dto.JourneyArtifacts{}
	stage, _, actions := determineStage(a)
	if stage != "onboarding" {
		t.Errorf("expected onboarding, got %s", stage)
	}
	if len(actions) == 0 {
		t.Error("expected at least one action")
	}
}

func TestDetermineStage_DataCollection(t *testing.T) {
	a := dto.JourneyArtifacts{HasProfile: true}
	stage, _, _ := determineStage(a)
	if stage != "data_collection" {
		t.Errorf("expected data_collection, got %s", stage)
	}
}

func TestDetermineStage_ReadyForConsultation(t *testing.T) {
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true}
	stage, _, _ := determineStage(a)
	if stage != "ready_for_consultation" {
		t.Errorf("expected ready_for_consultation, got %s", stage)
	}
}

func TestDetermineStage_ConsultationInProgress(t *testing.T) {
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, HasConsultation: true}
	stage, _, _ := determineStage(a)
	if stage != "consultation_in_progress" {
		t.Errorf("expected consultation_in_progress, got %s", stage)
	}
}

func TestDetermineStage_DiagnosisReady(t *testing.T) {
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, HasConsultation: true, HasDiagnosis: true}
	stage, _, _ := determineStage(a)
	if stage != "diagnosis_ready" {
		t.Errorf("expected diagnosis_ready, got %s", stage)
	}
}

func TestDetermineStage_PlanReady(t *testing.T) {
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, HasConsultation: true, HasDiagnosis: true, HasTreatment: true}
	stage, _, _ := determineStage(a)
	if stage != "plan_ready" {
		t.Errorf("expected plan_ready, got %s", stage)
	}
}

func TestDetermineStage_TrainingActive(t *testing.T) {
	a := dto.JourneyArtifacts{HasProfile: true, HasUpload: true, HasConsultation: true, HasDiagnosis: true, HasTreatment: true, HasTraining: true}
	stage, _, _ := determineStage(a)
	if stage != "training_active" {
		t.Errorf("expected training_active, got %s", stage)
	}
}

func TestDetermineStage_DerivedFromPopulated(t *testing.T) {
	a := dto.JourneyArtifacts{
		HasProfile:      true,
		HasUpload:       true,
		HasConsultation: true,
		DerivedFrom:     []string{"user_profiles", "user_uploads", "consultation_sessions"},
	}
	stage, _, _ := determineStage(a)
	if stage != "consultation_in_progress" {
		t.Errorf("expected consultation_in_progress, got %s", stage)
	}
}

func TestDetermineStage_ActiveConsultationID(t *testing.T) {
	convID := uuid.New()
	a := dto.JourneyArtifacts{
		HasProfile:           true,
		HasUpload:            true,
		HasConsultation:      true,
		ActiveConsultationID: &convID,
	}
	stage, _, _ := determineStage(a)
	if stage != "consultation_in_progress" {
		t.Errorf("expected consultation_in_progress, got %s", stage)
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
	stage, _, actions := determineStage(a)
	if stage != "training_active" {
		t.Errorf("expected training_active, got %s", stage)
	}
	if len(actions) < 2 {
		t.Errorf("expected at least 2 actions, got %d", len(actions))
	}
}
