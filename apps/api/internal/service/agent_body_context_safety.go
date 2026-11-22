package service

import (
	"encoding/json"
	"github.com/google/uuid"
	"sort"
	"strings"
)

// applyAgentSafety projects the Go Safety envelope over the pinned inputs and
// stamps each symptom fact. Any projection failure, or symptoms that could not
// all be read, keeps the view fail-closed (unscreened / incomplete).
func applyAgentSafety(view *AgentBodyContext, input safetyProjectionInput, allSymptomsRead bool) {
	summary := AgentSafetySummary{PolicyRevision: SafetyEnvelopePolicyV1, ActiveBlockers: []string{}, UnscreenedFactIDs: []uuid.UUID{}}
	defer func() { view.Safety = summary }()
	if input.CurrentRevision <= 0 && len(input.Symptoms) == 0 && len(input.Screens) == 0 {
		summary.Status = AgentSafetyNoCurrentSymptoms
		return
	}
	envelope, err := projectSafetyEnvelope(input)
	if err != nil {
		summary.Status = AgentSafetyUnavailable
		summary.RequiresReview = true
		for _, symptom := range input.Symptoms {
			summary.UnscreenedFactIDs = append(summary.UnscreenedFactIDs, symptom.ID)
		}
		sortUUIDs(summary.UnscreenedFactIDs)
		return
	}
	view.SafetyEnvelope = &envelope
	covered := map[uuid.UUID]bool{}
	for _, ref := range envelope.Coverage.CoveredSourceRefs {
		if id, parseErr := uuid.Parse(strings.TrimPrefix(ref, "body-state:fact:")); parseErr == nil {
			covered[id] = true
		}
	}
	for _, ref := range envelope.Coverage.IncompleteSourceRefs {
		if id, parseErr := uuid.Parse(strings.TrimPrefix(ref, "body-state:fact:")); parseErr == nil {
			summary.UnscreenedFactIDs = append(summary.UnscreenedFactIDs, id)
		}
	}
	sortUUIDs(summary.UnscreenedFactIDs)
	for index := range view.Facts {
		if view.Facts[index].SafetyScreen == AgentSymptomUnscreened && covered[view.Facts[index].ID] {
			view.Facts[index].SafetyScreen = AgentSymptomScreened
		}
	}
	concepts := map[string]bool{}
	for _, blocker := range envelope.ActiveBlockers {
		concepts[string(blocker.Concept)] = true
	}
	for concept := range concepts {
		summary.ActiveBlockers = append(summary.ActiveBlockers, concept)
	}
	sort.Strings(summary.ActiveBlockers)
	summary.RequiresReview = envelope.RequiresReview
	summary.CoverageComplete = envelope.Coverage.Complete && allSymptomsRead
	switch {
	case summary.RequiresReview:
		summary.Status = AgentSafetyRequiresReview
	case len(summary.UnscreenedFactIDs) > 0 || (!allSymptomsRead && len(input.Symptoms) > 0):
		summary.Status = AgentSafetyScreeningIncomplete
	case len(input.Symptoms) == 0:
		summary.Status = AgentSafetyNoCurrentSymptoms
	default:
		summary.Status = AgentSafetyScreened
	}
}

func agentJSONOr(raw []byte, fallback string) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(append([]byte(nil), raw...))
}

func sortUUIDs(ids []uuid.UUID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
}
