package model

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestSafetyResolutionCommandsRequireExactBasisAndVersions(t *testing.T) {
	valid := ResolveSafetyRestrictionCommand{IdempotencyKey: "expiry", ExpectedRevision: 3, ExpectedRestrictionVersion: 1, BasisKind: "expiry"}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ResolveSafetyRestrictionCommand){
		func(c *ResolveSafetyRestrictionCommand) { c.ExpectedRestrictionVersion = 0 },
		func(c *ResolveSafetyRestrictionCommand) { c.BasisKind = "ai_clearance" },
		func(c *ResolveSafetyRestrictionCommand) { c.SourceMessageID = uuid.New() },
		func(c *ResolveSafetyRestrictionCommand) { c.CheckInteractionID = uuid.New() },
		func(c *ResolveSafetyRestrictionCommand) { c.SourceVersion = 1 },
	} {
		bad := valid
		mutate(&bad)
		if bad.Validate() == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

func TestSafetyReverificationQuestionBindsTargetVersionAndEveryCondition(t *testing.T) {
	r := SafetyRestriction{ID: uuid.New(), UserID: uuid.New(), CreatedRevision: 7, LiftPolicy: "user_reverification", Reason: "test risk", ScopeKind: "all_training", LiftConditions: []string{"condition A", "condition B"}}
	q, err := SafetyReverificationQuestion(r)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if json.Unmarshal(q, &parsed) != nil || parsed["restriction_id"] != r.ID.String() || parsed["restriction_version"] != float64(7) {
		t.Fatalf("unbound %s", q)
	}
	answer := []byte(`{"fields":{"condition_0":"是","condition_1":"是"}}`)
	if !SafetyReverificationSatisfied(r, q, answer) {
		t.Fatal("explicit all-condition answer rejected")
	}
	for _, raw := range []string{`{"fields":{"condition_0":"是","condition_1":"否"}}`, `{"fields":{"condition_0":"是","condition_1":"不确定"}}`, `{"text":"我没事了"}`, `{"fields":{"condition_0":"是"}}`} {
		if SafetyReverificationSatisfied(r, q, []byte(raw)) {
			t.Fatal("accepted incomplete/negative answer")
		}
	}
	r.CreatedRevision++
	if SafetyReverificationSatisfied(r, q, answer) {
		t.Fatal("stale question accepted")
	}
	r.LiftConditions = nil
	if _, err := SafetyReverificationQuestion(r); err == nil {
		t.Fatal("missing conditions accepted")
	}
}
