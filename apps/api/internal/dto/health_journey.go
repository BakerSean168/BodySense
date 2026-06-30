package dto

import "github.com/google/uuid"

// HealthJourneyState represents the user's current health journey stage.
type HealthJourneyState struct {
	Stage            string          `json:"stage"`
	StageReason      string          `json:"stage_reason"`
	AvailableActions []string        `json:"available_actions"`
	Artifacts        JourneyArtifacts `json:"artifacts"`
}

// JourneyArtifacts summarizes what the user has completed in their journey.
type JourneyArtifacts struct {
	HasProfile            bool       `json:"has_profile"`
	HasUpload             bool       `json:"has_upload"`
	HasConsultation       bool       `json:"has_consultation"`
	HasDiagnosis          bool       `json:"has_diagnosis"`
	HasTreatment          bool       `json:"has_treatment"`
	HasTraining           bool       `json:"has_training"`
	ActiveConsultationID  *uuid.UUID `json:"active_consultation_id,omitempty"`
	LatestAssessmentID    *uuid.UUID `json:"latest_assessment_id,omitempty"`
	ActiveTrainingPlanID  *uuid.UUID `json:"active_training_plan_id,omitempty"`
	MissingRequirements   []string   `json:"missing_requirements,omitempty"`
	DerivedFrom           []string   `json:"derived_from,omitempty"`
}
