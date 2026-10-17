package trainingplan

// PlanStatus is the user-managed TrainingPlan status (training.md §4.2). It is
// not a Safety gate and has no review_recommended value: review hints and
// Safety execution blocks are read-time projections that never write status.
type PlanStatus string

const (
	PlanActive    PlanStatus = "active"
	PlanPaused    PlanStatus = "paused"
	PlanCompleted PlanStatus = "completed"
)

// Valid reports whether status is one of the three user-managed values.
func (s PlanStatus) Valid() bool {
	return s == PlanActive || s == PlanPaused || s == PlanCompleted
}

// StatusTransitionAllowed is the complete explicit-user transition table:
// active<->paused, active/paused->completed, completed->active (reopen the
// same planId). Same-status and completed->paused are not transitions.
// Callers must still prove an explicit user command; this function never
// authorizes an automatic change from Safety, BodyState, Diagnosis or feedback.
func StatusTransitionAllowed(from, to PlanStatus) bool {
	switch from {
	case PlanActive:
		return to == PlanPaused || to == PlanCompleted
	case PlanPaused:
		return to == PlanActive || to == PlanCompleted
	case PlanCompleted:
		return to == PlanActive
	default:
		return false
	}
}
