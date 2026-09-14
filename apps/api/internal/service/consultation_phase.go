package service

import "github.com/bodysense/api/internal/model"

// ShouldAdvancePhase enforces the complete Consultation state machine. The only
// forward transition is collecting -> ready_for_analysis; repeating the current
// phase is idempotent. Diagnosis/Treatment states are not Consultation phases.
func ShouldAdvancePhase(current, next model.ConsultationPhase) bool {
	if current == next {
		return true
	}
	return current == model.ConsultationPhaseCollecting && next == model.ConsultationPhaseReadyForAnalysis
}
