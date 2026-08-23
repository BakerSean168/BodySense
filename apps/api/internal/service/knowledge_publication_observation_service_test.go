package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakePublicationLookup struct {
	publication *model.KnowledgePublication
}

func (f *fakePublicationLookup) GetByKey(_ context.Context, _ string) (*model.KnowledgePublication, error) {
	return f.publication, nil
}

type fakePublicationObservationStore struct {
	created      []model.KnowledgePublicationObservation
	observations []model.KnowledgePublicationObservation
}

func (f *fakePublicationObservationStore) Create(_ context.Context, observation *model.KnowledgePublicationObservation) error {
	f.created = append(f.created, *observation)
	return nil
}

func (f *fakePublicationObservationStore) ListByPublication(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	_ int,
) ([]model.KnowledgePublicationObservation, error) {
	return f.observations, nil
}

func cleanPublishedObservation(publicationID uuid.UUID, key string, positive bool) model.KnowledgePublicationObservation {
	observation := model.KnowledgePublicationObservation{
		ID:                uuid.New(),
		PublicationID:     publicationID,
		ObservationKey:    key,
		ObservationKind:   "predeploy_eval",
		EvaluatorRevision: "published-knowledge-eval-v1",
		CaseID:            key,
	}
	if positive {
		observation.RetrievalStatus = "hit"
		observation.CitationStatus = "valid"
		observation.GroundingStatus = "supported"
		observation.IdentityStatus = "match"
		observation.ProvenanceStatus = "valid"
	} else {
		observation.RetrievalStatus = "expected_empty"
		observation.CitationStatus = "not_applicable"
		observation.GroundingStatus = "not_applicable"
		observation.IdentityStatus = "not_applicable"
		observation.ProvenanceStatus = "not_applicable"
	}
	return observation
}

func TestKnowledgePublicationGateContinuesOnCleanQualifiedWindow(t *testing.T) {
	publicationID := uuid.New()
	observations := make([]model.KnowledgePublicationObservation, 0, 6)
	for i := 0; i < 3; i++ {
		observations = append(observations, cleanPublishedObservation(publicationID, "positive", true))
		observations = append(observations, cleanPublishedObservation(publicationID, "negative", false))
	}
	service := NewKnowledgePublicationObservationService(
		&fakePublicationLookup{publication: &model.KnowledgePublication{
			ID: publicationID, PublicationKey: "pub-v1", PublishedVersion: 1, Status: "published",
		}},
		&fakePublicationObservationStore{observations: observations},
	)

	summary, err := service.Summary(context.Background(), "pub-v1", "predeploy_eval")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	gate := EvaluateKnowledgePublicationGate(summary)
	if gate.Action != "continue" {
		t.Fatalf("gate = %+v, want continue", gate)
	}
	if summary.Samples != 6 || summary.PositiveHits != 3 || summary.ExpectedEmpty != 3 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestKnowledgePublicationGateRollsBackOnHardSignals(t *testing.T) {
	base := KnowledgePublicationObservationSummary{
		PublicationStatus: "published",
		Samples:           6,
		PositiveHits:      3,
	}
	cases := []struct {
		name   string
		mutate func(*KnowledgePublicationObservationSummary)
	}{
		{"identity", func(s *KnowledgePublicationObservationSummary) { s.IdentityMismatches = 1 }},
		{"provenance", func(s *KnowledgePublicationObservationSummary) { s.ProvenanceFailures = 1 }},
		{"citation", func(s *KnowledgePublicationObservationSummary) { s.CitationFailures = 1 }},
		{"grounding", func(s *KnowledgePublicationObservationSummary) { s.GroundingRejected = 1 }},
		{"unexpected-result", func(s *KnowledgePublicationObservationSummary) { s.UnexpectedResults = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := base
			tc.mutate(&summary)
			if gate := EvaluateKnowledgePublicationGate(&summary); gate.Action != "rollback" {
				t.Fatalf("gate = %+v, want rollback", gate)
			}
		})
	}
}

func TestKnowledgePublicationGateHoldsOnSoftSignalsOrInsufficientEvidence(t *testing.T) {
	cases := []KnowledgePublicationObservationSummary{
		{PublicationStatus: "published", Samples: 6, PositiveHits: 3, RetrievalMisses: 1},
		{PublicationStatus: "published", Samples: 6, PositiveHits: 3, GroundingDegraded: 1},
		{PublicationStatus: "published", Samples: 6, PositiveHits: 3, ExecutionErrors: 1},
		{PublicationStatus: "published", Samples: 4, PositiveHits: 2},
		{PublicationStatus: "published", Samples: 6, PositiveHits: 0},
	}
	for _, summary := range cases {
		if gate := EvaluateKnowledgePublicationGate(&summary); gate.Action != "hold" {
			t.Fatalf("summary=%+v gate=%+v, want hold", summary, gate)
		}
	}
}

func TestRecordObservationPinsExactPublicationIdentityAndVersion(t *testing.T) {
	publicationID := uuid.New()
	lookup := &fakePublicationLookup{publication: &model.KnowledgePublication{
		ID: publicationID, PublicationKey: "pub-v3", PublishedVersion: 3, Status: "published",
	}}
	store := &fakePublicationObservationStore{}
	service := NewKnowledgePublicationObservationService(lookup, store)
	input := RecordKnowledgePublicationObservationInput{
		PublicationKey: "pub-v3", ExpectedPublicationID: publicationID, ExpectedPublishedVersion: 3,
		ObservationKey: "run-1:case-1", ObservationKind: "predeploy_eval",
		EvaluatorRevision: "published-knowledge-eval-v1", CaseID: "case-1",
		RetrievalStatus: "hit", CitationStatus: "valid", GroundingStatus: "supported",
		IdentityStatus: "match", ProvenanceStatus: "valid",
	}
	if err := service.Record(context.Background(), input); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(store.created) != 1 || store.created[0].PublicationID != publicationID {
		t.Fatalf("unexpected stored observations: %+v", store.created)
	}

	input.ExpectedPublishedVersion = 2
	if err := service.Record(context.Background(), input); err == nil {
		t.Fatal("expected publication version mismatch")
	}
}
