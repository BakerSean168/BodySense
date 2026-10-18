package service

// Diagnosis promotion records still pin the rollout comparison policy they were
// evaluated under. The legacy shadow-comparison store and its status command
// were retired with the per-analysis Diagnosis stack (DX-RETIRE-001); these
// revisions remain as immutable identities of the recorded promotions.
const (
	DiagnosisRolloutPolicyV1                    = "diagnosis-rollout-policy-v1"
	DiagnosisRolloutPolicyV2StructuredAuthority = "diagnosis-rollout-policy-v2-structured-authority"
	DiagnosisRolloutPolicyV3StructuredAuthority = "diagnosis-rollout-policy-v3-structured-authority"
)

func validDiagnosisRolloutPolicyRevision(revision string) bool {
	switch revision {
	case DiagnosisRolloutPolicyV1, DiagnosisRolloutPolicyV2StructuredAuthority, DiagnosisRolloutPolicyV3StructuredAuthority:
		return true
	default:
		return false
	}
}
