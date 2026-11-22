package consultation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func runtimeKnowledgeObservationIdentity(runID, messageID uuid.UUID, seed string) (string, string) {
	digest := sha256.Sum256([]byte(runID.String() + "|" + messageID.String() + "|" + seed))
	suffix := fmt.Sprintf("%x", digest[:12])
	return "runtime-answer:" + runID.String() + ":" + suffix, "runtime-" + suffix
}

func (r *Runtime) recordKnowledgeRuntimeObservations(
	ctx context.Context,
	runID, messageID uuid.UUID,
	parts []map[string]any,
) {
	if r.knowledgeObservationService == nil {
		return
	}

	publishedCitations := map[string]map[string]any{}
	for _, part := range parts {
		if part["type"] != "source" {
			continue
		}
		providerMetadata, ok := part["providerMetadata"].(map[string]any)
		if !ok {
			continue
		}
		citation, ok := providerMetadata["bodysense"].(map[string]any)
		if !ok || fmt.Sprint(citation["source_type"]) != "thought_forest_note" ||
			fmt.Sprint(citation["lifecycle_status"]) != "published" {
			continue
		}
		publicationID := fmt.Sprint(citation["publication_id"])
		unitKey := fmt.Sprint(citation["unit_key"])
		version, ok := intFromAny(citation["published_version"])
		if !ok || publicationID == "" || unitKey == "" {
			continue
		}
		evidenceRef := fmt.Sprintf("published:%s:v%d:%s", publicationID, version, unitKey)
		publishedCitations[evidenceRef] = citation
	}

	attributedRefs := map[string]struct{}{}
	for _, part := range parts {
		if part["type"] != "data" || part["name"] != "answer_attribution" {
			continue
		}
		data, ok := part["data"].(map[string]any)
		if !ok {
			continue
		}
		raw, err := json.Marshal(data)
		if err != nil {
			continue
		}
		parsed, err := service.ParseConsultationAnswerAttributionPayload(raw)
		if err != nil {
			slog.Default().Warn("skip_invalid_completed_answer_attribution", "run_id", runID, "error", err)
			continue
		}
		attribution := parsed.Attribution
		for _, binding := range attribution.Bindings {
			attributedRefs[binding.EvidenceRef] = struct{}{}
			citationStatus := "invalid"
			reasonOverride := "attribution_without_persisted_citation"
			if citation, ok := publishedCitations[binding.EvidenceRef]; ok {
				if publishedCitationMatchesBinding(citation, binding) {
					citationStatus = "valid"
					reasonOverride = ""
				} else {
					reasonOverride = "citation_attribution_identity_mismatch"
				}
			}
			r.recordKnowledgeRuntimeObservation(
				ctx, runID, messageID, attribution, binding, citationStatus, reasonOverride,
			)
		}
	}

	for evidenceRef, citation := range publishedCitations {
		if _, attributed := attributedRefs[evidenceRef]; attributed {
			continue
		}
		publicationID := fmt.Sprint(citation["publication_id"])
		unitKey := fmt.Sprint(citation["unit_key"])
		version, ok := intFromAny(citation["published_version"])
		if !ok {
			continue
		}
		binding := service.PublishedAnswerEvidenceBinding{
			EvidenceRef:         evidenceRef,
			PublicationID:       publicationID,
			PublicationKey:      fmt.Sprint(citation["publication_key"]),
			PublicationBatchKey: fmt.Sprint(citation["publication_batch_key"]),
			PublishedVersion:    version,
			UnitKey:             unitKey,
			ClaimID:             fmt.Sprint(citation["claim_id"]),
			ClaimReviewID:       fmt.Sprint(citation["claim_review_id"]),
			GroundingStatus:     "degraded",
			ReasonCodes:         []string{"missing_answer_attribution"},
		}
		attribution := service.ConsultationAnswerAttribution{
			AttributionID:   "missing:" + evidenceRef,
			PolicyRevision:  service.ConsultationAnswerAttributionPolicyV1,
			EvidenceRefs:    []string{evidenceRef},
			GroundingStatus: "degraded",
			ReasonCodes:     []string{"missing_answer_attribution"},
			Bindings:        []service.PublishedAnswerEvidenceBinding{binding},
		}
		r.recordKnowledgeRuntimeObservation(
			ctx, runID, messageID, attribution, binding, "valid", "missing_answer_attribution",
		)
	}
}

func publishedCitationMatchesBinding(
	citation map[string]any,
	binding service.PublishedAnswerEvidenceBinding,
) bool {
	if fmt.Sprint(citation["publication_id"]) != binding.PublicationID ||
		fmt.Sprint(citation["publication_key"]) != binding.PublicationKey ||
		fmt.Sprint(citation["publication_batch_key"]) != binding.PublicationBatchKey ||
		fmt.Sprint(citation["unit_key"]) != binding.UnitKey ||
		fmt.Sprint(citation["claim_id"]) != binding.ClaimID ||
		fmt.Sprint(citation["claim_review_id"]) != binding.ClaimReviewID {
		return false
	}
	version, ok := intFromAny(citation["published_version"])
	if !ok || version != binding.PublishedVersion {
		return false
	}
	locator, ok := citation["source_locator"].(map[string]any)
	if !ok {
		return false
	}
	return fmt.Sprint(locator["locator_type"]) == binding.SourceLocator.LocatorType &&
		fmt.Sprint(locator["repository"]) == binding.SourceLocator.Repository &&
		fmt.Sprint(locator["git_commit"]) == binding.SourceLocator.GitCommit &&
		fmt.Sprint(locator["path"]) == binding.SourceLocator.Path &&
		intValueEquals(locator["line_start"], binding.SourceLocator.LineStart) &&
		intValueEquals(locator["line_end"], binding.SourceLocator.LineEnd)
}

func intValueEquals(value any, expected int) bool {
	actual, ok := intFromAny(value)
	return ok && actual == expected
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	}
	return 0, false
}

func (r *Runtime) recordKnowledgeRuntimeObservation(
	ctx context.Context,
	runID, messageID uuid.UUID,
	attribution service.ConsultationAnswerAttribution,
	binding service.PublishedAnswerEvidenceBinding,
	citationStatus string,
	reasonOverride string,
) {
	publicationID, err := uuid.Parse(binding.PublicationID)
	if err != nil {
		slog.Default().Warn("skip_runtime_knowledge_observation_invalid_publication_id", "error", err)
		return
	}
	seed := attribution.AttributionID + "|" + binding.EvidenceRef
	observationKey, caseID := runtimeKnowledgeObservationIdentity(runID, messageID, seed)
	reasons := binding.ReasonCodes
	if reasonOverride != "" {
		reasons = []string{reasonOverride}
	}
	observationSource := "source.answer_attribution.added"
	if reasonOverride == "missing_answer_attribution" {
		observationSource = "source.citation.added"
	}
	metadata, _ := json.Marshal(map[string]any{
		"run_id":             runID.String(),
		"message_id":         messageID.String(),
		"attribution_id":     attribution.AttributionID,
		"claim_text":         attribution.ClaimText,
		"evidence_ref":       binding.EvidenceRef,
		"unit_key":           binding.UnitKey,
		"claim_id":           binding.ClaimID,
		"claim_review_id":    binding.ClaimReviewID,
		"reason_codes":       reasons,
		"observation_source": observationSource,
	})
	if err := r.knowledgeObservationService.Record(ctx, service.RecordKnowledgePublicationObservationInput{
		PublicationKey:           binding.PublicationKey,
		ExpectedPublicationID:    publicationID,
		ExpectedPublishedVersion: binding.PublishedVersion,
		ObservationKey:           observationKey,
		ObservationKind:          "runtime_answer",
		EvaluatorRevision:        service.ConsultationAnswerAttributionPolicyV1,
		CaseID:                   caseID,
		RetrievalStatus:          "hit",
		CitationStatus:           citationStatus,
		GroundingStatus:          binding.GroundingStatus,
		IdentityStatus:           "match",
		ProvenanceStatus:         "valid",
		Metadata:                 datatypes.JSON(metadata),
	}); err != nil {
		slog.Default().Error("record_runtime_knowledge_observation_failed", "run_id", runID, "error", err)
	}
}

func citationPart(payload json.RawMessage) map[string]any {
	var citation map[string]any
	_ = json.Unmarshal(payload, &struct {
		Citation *map[string]any `json:"citation"`
	}{Citation: &citation})

	title, _ := citation["title"].(string)
	return map[string]any{
		"type":       "source",
		"title":      title,
		"sourceType": "document",
		"providerMetadata": map[string]any{
			"bodysense": citation,
		},
	}
}

func dataPart(name string, payload json.RawMessage) map[string]any {
	var data map[string]any
	_ = json.Unmarshal(payload, &data)
	return map[string]any{
		"type": "data",
		"name": name,
		"data": data,
	}
}

func toolResultIsError(result json.RawMessage) bool {
	var parsed struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return false
	}
	return parsed.Status == "error" || parsed.Error != ""
}

func httpErr(status int, code ConsultationErrorCode, message string) *HTTPError {
	return &HTTPError{Status: status, Code: code, Message: message}
}

// AttachRolloutService attaches the anonymous rollout observer (North-Star).
func (r *Runtime) AttachRolloutService(rollout *service.ConsultationRolloutService) {
	r.rolloutService = rollout
}

// AttachKnowledgeObservationService records publication-version answer observations after message completion.
func (r *Runtime) AttachKnowledgeObservationService(observer runtimeKnowledgeObservationService) {
	r.knowledgeObservationService = observer
}
