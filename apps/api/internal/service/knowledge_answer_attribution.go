package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const ConsultationAnswerAttributionPolicyV1 = "consultation-answer-attribution-v1"

type AnswerAttributionSourceLocator struct {
	LocatorType string `json:"locator_type"`
	Repository  string `json:"repository"`
	GitCommit   string `json:"git_commit"`
	Path        string `json:"path"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
}

type PublishedAnswerEvidenceBinding struct {
	EvidenceRef         string                         `json:"evidence_ref"`
	PublicationID       string                         `json:"publication_id"`
	PublicationKey      string                         `json:"publication_key"`
	PublicationBatchKey string                         `json:"publication_batch_key"`
	PublishedVersion    int                            `json:"published_version"`
	UnitKey             string                         `json:"unit_key"`
	ClaimID             string                         `json:"claim_id"`
	ClaimReviewID       string                         `json:"claim_review_id"`
	ClaimKind           string                         `json:"claim_kind,omitempty"`
	GroundingStatus     string                         `json:"grounding_status"`
	ReasonCodes         []string                       `json:"reason_codes"`
	SourceLocator       AnswerAttributionSourceLocator `json:"source_locator"`
}

type ConsultationAnswerAttribution struct {
	AttributionID   string                           `json:"attribution_id"`
	PolicyRevision  string                           `json:"policy_revision"`
	ClaimText       string                           `json:"claim_text"`
	EvidenceRefs    []string                         `json:"evidence_refs"`
	GroundingStatus string                           `json:"grounding_status"`
	ReasonCodes     []string                         `json:"reason_codes"`
	Bindings        []PublishedAnswerEvidenceBinding `json:"bindings"`
}

type ConsultationAnswerAttributionPayload struct {
	Attribution ConsultationAnswerAttribution `json:"attribution"`
}

func ParseConsultationAnswerAttributionPayload(raw json.RawMessage) (*ConsultationAnswerAttributionPayload, error) {
	var payload ConsultationAnswerAttributionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("answer attribution payload is malformed: %w", err)
	}
	if err := validateConsultationAnswerAttribution(payload.Attribution); err != nil {
		return nil, err
	}
	return &payload, nil
}

func validateConsultationAnswerAttribution(attribution ConsultationAnswerAttribution) error {
	if strings.TrimSpace(attribution.AttributionID) == "" ||
		strings.TrimSpace(attribution.ClaimText) == "" {
		return errors.New("answer attribution identity and claim_text are required")
	}
	if attribution.PolicyRevision != ConsultationAnswerAttributionPolicyV1 {
		return fmt.Errorf("unsupported answer attribution policy %q", attribution.PolicyRevision)
	}
	if len(attribution.EvidenceRefs) == 0 || len(attribution.EvidenceRefs) > 3 {
		return errors.New("answer attribution requires 1-3 evidence refs")
	}
	if !validObservationStatus(attribution.GroundingStatus, "supported", "degraded", "rejected") {
		return fmt.Errorf("invalid answer attribution grounding_status %q", attribution.GroundingStatus)
	}
	if len(attribution.Bindings) != len(attribution.EvidenceRefs) {
		return errors.New("answer attribution bindings must cover every evidence ref")
	}

	refs := make(map[string]struct{}, len(attribution.EvidenceRefs))
	for _, ref := range attribution.EvidenceRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return errors.New("answer attribution evidence_ref must not be empty")
		}
		if _, exists := refs[ref]; exists {
			return errors.New("answer attribution evidence_refs must be unique")
		}
		refs[ref] = struct{}{}
	}
	boundRefs := make(map[string]struct{}, len(attribution.Bindings))
	for _, binding := range attribution.Bindings {
		if err := validatePublishedAnswerEvidenceBinding(binding); err != nil {
			return err
		}
		if !validObservationStatus(binding.GroundingStatus, "supported", "degraded", "rejected") {
			return fmt.Errorf("invalid binding grounding_status %q", binding.GroundingStatus)
		}
		if _, ok := refs[binding.EvidenceRef]; !ok {
			return fmt.Errorf("binding evidence_ref %q not declared by attribution", binding.EvidenceRef)
		}
		if _, exists := boundRefs[binding.EvidenceRef]; exists {
			return fmt.Errorf("duplicate answer attribution binding %q", binding.EvidenceRef)
		}
		boundRefs[binding.EvidenceRef] = struct{}{}
	}
	bindingSupported := false
	bindingDegraded := false
	for _, binding := range attribution.Bindings {
		bindingSupported = bindingSupported || binding.GroundingStatus == "supported"
		bindingDegraded = bindingDegraded || binding.GroundingStatus == "degraded"
	}
	expected := "rejected"
	if bindingSupported {
		expected = "supported"
	} else if bindingDegraded {
		expected = "degraded"
	}
	if attribution.GroundingStatus != expected {
		return errors.New("answer attribution claim grounding does not match binding results")
	}
	return nil
}

func validatePublishedAnswerEvidenceBinding(binding PublishedAnswerEvidenceBinding) error {
	if strings.TrimSpace(binding.EvidenceRef) == "" ||
		strings.TrimSpace(binding.PublicationKey) == "" ||
		strings.TrimSpace(binding.PublicationBatchKey) == "" ||
		strings.TrimSpace(binding.UnitKey) == "" ||
		strings.TrimSpace(binding.ClaimID) == "" ||
		strings.TrimSpace(binding.ClaimReviewID) == "" {
		return errors.New("answer attribution publication binding is incomplete")
	}
	if _, err := uuid.Parse(binding.PublicationID); err != nil {
		return errors.New("answer attribution publication_id is invalid")
	}
	if binding.PublishedVersion <= 0 {
		return errors.New("answer attribution published_version must be positive")
	}
	locator := binding.SourceLocator
	if locator.LocatorType != "markdown_lines" ||
		strings.TrimSpace(locator.Repository) == "" ||
		strings.TrimSpace(locator.GitCommit) == "" ||
		strings.TrimSpace(locator.Path) == "" ||
		locator.LineStart <= 0 || locator.LineEnd < locator.LineStart {
		return errors.New("answer attribution source locator is incomplete")
	}
	return nil
}
