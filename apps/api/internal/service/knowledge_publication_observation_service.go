package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const KnowledgePublicationMinObservations = 5

type knowledgePublicationLookup interface {
	GetByKey(ctx context.Context, publicationKey string) (*model.KnowledgePublication, error)
}

type knowledgePublicationObservationStore interface {
	Create(ctx context.Context, observation *model.KnowledgePublicationObservation) error
	ListByPublication(ctx context.Context, publicationID uuid.UUID, kind string, limit int) ([]model.KnowledgePublicationObservation, error)
}

type RecordKnowledgePublicationObservationInput struct {
	PublicationKey           string
	ExpectedPublicationID    uuid.UUID
	ExpectedPublishedVersion int
	ObservationKey           string
	ObservationKind          string
	EvaluatorRevision        string
	CaseID                   string
	RetrievalStatus          string
	CitationStatus           string
	GroundingStatus          string
	IdentityStatus           string
	ProvenanceStatus         string
	ExecutionError           string
	Metadata                 datatypes.JSON
}

type KnowledgePublicationObservationSummary struct {
	PublicationID      uuid.UUID `json:"publication_id"`
	PublicationKey     string    `json:"publication_key"`
	PublishedVersion   int       `json:"published_version"`
	PublicationStatus  string    `json:"publication_status"`
	ObservationKind    string    `json:"observation_kind"`
	Samples            int       `json:"samples"`
	PositiveHits       int       `json:"positive_hits"`
	ExpectedEmpty      int       `json:"expected_empty"`
	RetrievalMisses    int       `json:"retrieval_misses"`
	UnexpectedResults  int       `json:"unexpected_results"`
	CitationFailures   int       `json:"citation_failures"`
	GroundingDegraded  int       `json:"grounding_degraded"`
	GroundingRejected  int       `json:"grounding_rejected"`
	IdentityMismatches int       `json:"identity_mismatches"`
	ProvenanceFailures int       `json:"provenance_failures"`
	ExecutionErrors    int       `json:"execution_errors"`
}

type KnowledgePublicationGate struct {
	Action string `json:"action"` // continue | hold | rollback
	Reason string `json:"reason"`
}

type KnowledgePublicationObservationService struct {
	publications knowledgePublicationLookup
	observations knowledgePublicationObservationStore
}

func NewKnowledgePublicationObservationService(
	publications knowledgePublicationLookup,
	observations knowledgePublicationObservationStore,
) *KnowledgePublicationObservationService {
	return &KnowledgePublicationObservationService{
		publications: publications,
		observations: observations,
	}
}

func validObservationStatus(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func validateKnowledgePublicationObservation(input RecordKnowledgePublicationObservationInput) error {
	if input.PublicationKey == "" || input.ObservationKey == "" || input.ObservationKind == "" ||
		input.EvaluatorRevision == "" || input.CaseID == "" {
		return errors.New("publication_key, observation_key/kind, evaluator_revision and case_id are required")
	}
	if !validObservationStatus(input.RetrievalStatus, "hit", "expected_empty", "miss", "unexpected_result") {
		return fmt.Errorf("invalid retrieval_status %q", input.RetrievalStatus)
	}
	if !validObservationStatus(input.CitationStatus, "valid", "invalid", "not_applicable") {
		return fmt.Errorf("invalid citation_status %q", input.CitationStatus)
	}
	if !validObservationStatus(input.GroundingStatus, "supported", "degraded", "rejected", "not_applicable") {
		return fmt.Errorf("invalid grounding_status %q", input.GroundingStatus)
	}
	if !validObservationStatus(input.IdentityStatus, "match", "mismatch", "not_applicable") {
		return fmt.Errorf("invalid identity_status %q", input.IdentityStatus)
	}
	if !validObservationStatus(input.ProvenanceStatus, "valid", "invalid", "not_applicable") {
		return fmt.Errorf("invalid provenance_status %q", input.ProvenanceStatus)
	}
	if input.RetrievalStatus == "hit" && (input.CitationStatus == "not_applicable" ||
		input.GroundingStatus == "not_applicable" || input.IdentityStatus == "not_applicable" ||
		input.ProvenanceStatus == "not_applicable") {
		return errors.New("hit observation requires citation, grounding, identity and provenance statuses")
	}
	return nil
}

func (s *KnowledgePublicationObservationService) Record(
	ctx context.Context,
	input RecordKnowledgePublicationObservationInput,
) error {
	if err := validateKnowledgePublicationObservation(input); err != nil {
		return err
	}
	publication, err := s.publications.GetByKey(ctx, input.PublicationKey)
	if err != nil {
		return err
	}
	if publication == nil {
		return fmt.Errorf("publication %s not found", input.PublicationKey)
	}
	if publication.Status != "published" {
		return fmt.Errorf("publication %s status=%s, want published", input.PublicationKey, publication.Status)
	}
	if input.ExpectedPublicationID != uuid.Nil && publication.ID != input.ExpectedPublicationID {
		return errors.New("publication identity mismatch")
	}
	if input.ExpectedPublishedVersion > 0 && publication.PublishedVersion != input.ExpectedPublishedVersion {
		return errors.New("publication version mismatch")
	}
	metadata := input.Metadata
	if len(metadata) == 0 {
		metadata = datatypes.JSON(`{}`)
	}
	return s.observations.Create(ctx, &model.KnowledgePublicationObservation{
		ID: uuid.New(), PublicationID: publication.ID,
		ObservationKey: input.ObservationKey, ObservationKind: input.ObservationKind,
		EvaluatorRevision: input.EvaluatorRevision, CaseID: input.CaseID,
		RetrievalStatus: input.RetrievalStatus, CitationStatus: input.CitationStatus,
		GroundingStatus: input.GroundingStatus, IdentityStatus: input.IdentityStatus,
		ProvenanceStatus: input.ProvenanceStatus, ExecutionError: input.ExecutionError,
		Metadata: metadata,
	})
}

func (s *KnowledgePublicationObservationService) Summary(
	ctx context.Context,
	publicationKey string,
	kind string,
) (*KnowledgePublicationObservationSummary, error) {
	publication, err := s.publications.GetByKey(ctx, publicationKey)
	if err != nil {
		return nil, err
	}
	if publication == nil {
		return nil, fmt.Errorf("publication %s not found", publicationKey)
	}
	observations, err := s.observations.ListByPublication(ctx, publication.ID, kind, 500)
	if err != nil {
		return nil, err
	}
	summary := &KnowledgePublicationObservationSummary{
		PublicationID: publication.ID, PublicationKey: publication.PublicationKey,
		PublishedVersion: publication.PublishedVersion, PublicationStatus: publication.Status,
		ObservationKind: kind,
	}
	for _, observation := range observations {
		summary.Samples++
		switch observation.RetrievalStatus {
		case "hit":
			summary.PositiveHits++
		case "expected_empty":
			summary.ExpectedEmpty++
		case "miss":
			summary.RetrievalMisses++
		case "unexpected_result":
			summary.UnexpectedResults++
		}
		if observation.CitationStatus == "invalid" {
			summary.CitationFailures++
		}
		if observation.GroundingStatus == "degraded" {
			summary.GroundingDegraded++
		}
		if observation.GroundingStatus == "rejected" {
			summary.GroundingRejected++
		}
		if observation.IdentityStatus == "mismatch" {
			summary.IdentityMismatches++
		}
		if observation.ProvenanceStatus == "invalid" {
			summary.ProvenanceFailures++
		}
		if observation.ExecutionError != "" {
			summary.ExecutionErrors++
		}
	}
	return summary, nil
}

func EvaluateKnowledgePublicationGate(summary *KnowledgePublicationObservationSummary) KnowledgePublicationGate {
	if summary == nil {
		return KnowledgePublicationGate{Action: "hold", Reason: "no observation summary"}
	}
	if summary.PublicationStatus != "published" {
		return KnowledgePublicationGate{Action: "hold", Reason: "publication is not active"}
	}
	if summary.IdentityMismatches > 0 || summary.ProvenanceFailures > 0 ||
		summary.CitationFailures > 0 || summary.GroundingRejected > 0 || summary.UnexpectedResults > 0 {
		return KnowledgePublicationGate{
			Action: "rollback",
			Reason: "blocking publication identity, citation, grounding, provenance or relevance signal",
		}
	}
	if summary.ExecutionErrors > 0 || summary.RetrievalMisses > 0 || summary.GroundingDegraded > 0 {
		return KnowledgePublicationGate{
			Action: "hold",
			Reason: "non-blocking retrieval, grounding or execution regression observed",
		}
	}
	if summary.Samples < KnowledgePublicationMinObservations || summary.PositiveHits == 0 {
		return KnowledgePublicationGate{Action: "hold", Reason: "insufficient qualified observations"}
	}
	return KnowledgePublicationGate{Action: "continue", Reason: "clean publication observation window"}
}
