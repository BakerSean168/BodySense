package service

// Retired configuration identities are retained in tests only so immutable
// historical replay/evaluation fixtures can still prove their provenance. They
// are intentionally absent from production runtime registries.
const (
	diagnosisV1ConfigurationID          = retiredDiagnosisV1ConfigurationID
	diagnosisEvidenceGapConfigurationID = retiredDiagnosisEvidenceGapConfigurationID
	DiagnosisPromotionRecordV1          = "diagnosis_promotion_v1"
	DiagnosisRolloutRollback            = "rollback"
	DiagnosisDecisionPolicyPreEnvelope  = "diagnosis-authority-pre-envelope-v0"

	treatmentV1ConfigurationID = retiredTreatmentV1ConfigurationID
	TreatmentPromotionRecordV1 = "treatment_promotion_v1"
	TreatmentRolloutRollback   = "rollback"

	historicalAssessmentV1ConfigurationID = retiredAssessmentV1ConfigurationID
	historicalAssessmentV2ConfigurationID = retiredAssessmentV2ConfigurationID
	historicalAssessmentV3ConfigurationID = retiredAssessmentV3ConfigurationID
	historicalAssessmentV4ConfigurationID = retiredAssessmentV4ConfigurationID
	AssessmentPromotionRecordV1           = "assessment_promotion_v1"
	AssessmentRolloutRollback             = "rollback"
	AssessmentDecisionPolicyV1            = "assessment-go-generation-v1"

	consultationV1ConfigurationID = retiredConsultationV1ConfigurationID
	ConsultationDecisionPolicyV1  = "consultation-go-runtime-v1"

	historicalPostureV1ConfigurationID = retiredPostureV1ConfigurationID
)
