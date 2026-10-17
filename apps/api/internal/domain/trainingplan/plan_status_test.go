package trainingplan

import "testing"

func TestStatusTransitionAllowedIsTheExplicitTable(t *testing.T) {
	all := []PlanStatus{PlanActive, PlanPaused, PlanCompleted, "review_recommended", ""}
	allowed := map[[2]PlanStatus]bool{
		{PlanActive, PlanPaused}: true, {PlanPaused, PlanActive}: true,
		{PlanActive, PlanCompleted}: true, {PlanPaused, PlanCompleted}: true,
		{PlanCompleted, PlanActive}: true,
	}
	for _, from := range all {
		for _, to := range all {
			if got := StatusTransitionAllowed(from, to); got != allowed[[2]PlanStatus{from, to}] {
				t.Fatalf("%q -> %q = %v", from, to, got)
			}
		}
	}
	if PlanStatus("review_recommended").Valid() || !PlanPaused.Valid() {
		t.Fatal("review_recommended is a projection, never a plan status")
	}
}
