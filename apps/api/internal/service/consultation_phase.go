package service

// ConsultationPhaseRank orders the only phases owned by Consultation.
// Diagnosis assessment, Treatment and longitudinal monitoring are separate
// durable domains and must not be encoded as later consultation phases.
var ConsultationPhaseRank = map[string]int{
	"":                   -1,
	"collecting":         0,
	"ready_for_analysis": 1,
	"analysis_ready":     2,
}

// ShouldAdvancePhase returns true when transitioning from current to next
// is a valid forward (or same-rank) phase change.
func ShouldAdvancePhase(current string, next string) bool {
	currentRank, ok := ConsultationPhaseRank[current]
	if !ok {
		currentRank = 0
	}
	nextRank, ok := ConsultationPhaseRank[next]
	if !ok {
		return false
	}
	return nextRank >= currentRank
}
