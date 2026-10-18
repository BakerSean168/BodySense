package service

import (
	"encoding/json"
	"errors"
	"fmt"
)

// TreatmentReplayInput is the exact, frozen Treatment Agent input. Generation
// stores it with the generated PlanProposal so the request can be replayed
// offline.
type TreatmentReplayInput struct {
	BodyStateRevision int64                  `json:"body_state_revision"`
	BodyState         json.RawMessage        `json:"body_state"`
	Diagnosis         json.RawMessage        `json:"diagnosis"`
	Profile           json.RawMessage        `json:"profile"`
	UserConstraints   json.RawMessage        `json:"user_constraints"`
	Evidence          json.RawMessage        `json:"evidence"`
	GenerationFacts   TreatmentDecisionFacts `json:"generation_facts"`
}

func EncodeTreatmentReplayInput(
	bodyStateRevision int64,
	bodyState json.RawMessage,
	diagnosis json.RawMessage,
	profile json.RawMessage,
	userConstraints json.RawMessage,
	evidence json.RawMessage,
	generationFacts TreatmentDecisionFacts,
) (json.RawMessage, error) {
	if bodyStateRevision <= 0 || len(bodyState) == 0 || !json.Valid(bodyState) {
		return nil, errors.New("valid Treatment replay BodyState and revision are required")
	}
	for name, raw := range map[string]json.RawMessage{
		"diagnosis":        diagnosis,
		"profile":          profile,
		"user_constraints": userConstraints,
		"evidence":         evidence,
	} {
		if len(raw) == 0 || !json.Valid(raw) {
			return nil, fmt.Errorf("valid Treatment replay %s is required", name)
		}
	}
	return json.Marshal(TreatmentReplayInput{
		BodyStateRevision: bodyStateRevision,
		BodyState:         bodyState,
		Diagnosis:         diagnosis,
		Profile:           profile,
		UserConstraints:   userConstraints,
		Evidence:          evidence,
		GenerationFacts:   generationFacts,
	})
}
