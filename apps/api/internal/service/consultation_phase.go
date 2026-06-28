package service

// ConsultationPhaseRank defines the ordering of consultation workflow phases.
// Higher rank means a later phase. Phase transitions are only allowed to
// equal or higher rank (no regression).
//
// Phase definitions (must match frontend ConsultationPhase type):
//   - collecting:           symptom information gathering
//   - ready_for_analysis:   enough info collected, can request diagnosis
//   - analysis_ready:       diagnosis generated, awaiting user confirmation
//   - diagnosis_confirmed:  user confirmed a diagnosis candidate
//   - plan_ready:           treatment plan generated
//   - completed:            session ended
var ConsultationPhaseRank = map[string]int{
	"":                    -1,
	"collecting":          0,
	"ready_for_analysis":  1,
	"analysis_ready":      2,
	"diagnosis_confirmed": 3,
	"plan_ready":          4,
	"completed":           5,
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
