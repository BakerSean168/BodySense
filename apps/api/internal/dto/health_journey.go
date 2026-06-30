package dto

import "github.com/google/uuid"

const (
	JourneyStageProfileIncomplete = "profile_incomplete"
	JourneyStageProfileReady      = "profile_ready"
	JourneyStageAssetsUploaded    = "assets_uploaded"
	JourneyStageAssessmentReady   = "assessment_ready"
	JourneyStageConsulting        = "consulting"
	JourneyStageDiagnosisReady    = "diagnosis_ready"
	JourneyStagePlanReady         = "plan_ready"
	JourneyStageTrainingActive    = "training_active"
	JourneyStageReassessmentDue   = "reassessment_due"
	JourneyStageCompleted         = "completed"

	JourneyActionCompleteProfile   = "complete_profile"
	JourneyActionUploadReport      = "upload_report"
	JourneyActionUploadPhoto       = "upload_photo"
	JourneyActionStartAssessment   = "start_assessment"
	JourneyActionStartConsultation = "start_consultation"
	JourneyActionContinueConsult   = "continue_consultation"
	JourneyActionRequestAnalysis   = "request_analysis"
	JourneyActionConfirmDiagnosis  = "confirm_diagnosis"
	JourneyActionGenerateTreatment = "generate_treatment"
	JourneyActionViewTreatment     = "view_treatment"
	JourneyActionStartTraining     = "start_training"
	JourneyActionViewProgress      = "view_progress"
	JourneyActionLogTraining       = "log_training"
	JourneyActionReassess          = "reassess"
	JourneyActionReviewSummary     = "review_summary"
)

// HealthJourneyState represents the user's current health journey stage.
type HealthJourneyState struct {
	Stage            string           `json:"stage"`
	StageReason      string           `json:"stage_reason"`
	AvailableActions []string         `json:"available_actions"`
	Artifacts        JourneyArtifacts `json:"artifacts"`
}

// JourneyArtifacts summarizes what the user has completed in their journey.
type JourneyArtifacts struct {
	HasProfile           bool       `json:"has_profile"`
	HasUpload            bool       `json:"has_upload"`
	HasConsultation      bool       `json:"has_consultation"`
	HasDiagnosis         bool       `json:"has_diagnosis"`
	HasTreatment         bool       `json:"has_treatment"`
	HasTraining          bool       `json:"has_training"`
	NeedsReassessment    bool       `json:"needs_reassessment"`
	HasReassessment      bool       `json:"has_reassessment"`
	ActiveConsultationID *uuid.UUID `json:"active_consultation_id,omitempty"`
	LatestAssessmentID   *uuid.UUID `json:"latest_assessment_id,omitempty"`
	ActiveTrainingPlanID *uuid.UUID `json:"active_training_plan_id,omitempty"`
	MissingRequirements  []string   `json:"missing_requirements,omitempty"`
	DerivedFrom          []string   `json:"derived_from,omitempty"`
}
