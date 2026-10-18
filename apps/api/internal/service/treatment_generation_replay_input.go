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
	BodyStateRevision    int64                  `json:"body_state_revision"`
	BodyState            json.RawMessage        `json:"body_state"`
	DiagnosisAnalysis    json.RawMessage        `json:"diagnosis_analysis"`
	CandidateAssessments json.RawMessage        `json:"candidate_assessments"`
	Profile              json.RawMessage        `json:"profile"`
	UserConstraints      json.RawMessage        `json:"user_constraints"`
	Evidence             json.RawMessage        `json:"evidence"`
	GenerationFacts      TreatmentDecisionFacts `json:"generation_facts"`
}

func EncodeTreatmentReplayInput(
	bodyStateRevision int64,
	bodyState json.RawMessage,
	diagnosisAnalysis json.RawMessage,
	candidateAssessments json.RawMessage,
	profile json.RawMessage,
	userConstraints json.RawMessage,
	evidence json.RawMessage,
	generationFacts TreatmentDecisionFacts,
) (json.RawMessage, error) {
	if bodyStateRevision <= 0 || len(bodyState) == 0 || !json.Valid(bodyState) {
		return nil, errors.New("valid Treatment replay BodyState and revision are required")
	}
	for name, raw := range map[string]json.RawMessage{
		"diagnosis_analysis":    diagnosisAnalysis,
		"candidate_assessments": candidateAssessments,
		"profile":               profile,
		"user_constraints":      userConstraints,
		"evidence":              evidence,
	} {
		if len(raw) == 0 || !json.Valid(raw) {
			return nil, fmt.Errorf("valid Treatment replay %s is required", name)
		}
	}
	return json.Marshal(TreatmentReplayInput{
		BodyStateRevision:    bodyStateRevision,
		BodyState:            bodyState,
		DiagnosisAnalysis:    diagnosisAnalysis,
		CandidateAssessments: candidateAssessments,
		Profile:              profile,
		UserConstraints:      userConstraints,
		Evidence:             evidence,
		GenerationFacts:      generationFacts,
	})
}
