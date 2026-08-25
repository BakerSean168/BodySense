package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type registryStoreStub struct {
	source *model.KnowledgeSource
}

func (s *registryStoreStub) Register(_ context.Context, source *model.KnowledgeSource) (bool, error) {
	if s.source != nil {
		return false, nil
	}
	s.source = source
	return true, nil
}
func (s *registryStoreStub) FindByKey(_ context.Context, key string) (*model.KnowledgeSource, error) {
	if s.source == nil || s.source.SourceKey != key {
		return nil, gorm.ErrRecordNotFound
	}
	return s.source, nil
}
func (s *registryStoreStub) List(_ context.Context, _ int) ([]model.KnowledgeSource, error) {
	if s.source == nil {
		return nil, nil
	}
	return []model.KnowledgeSource{*s.source}, nil
}

func validRegisterInput() RegisterKnowledgeSourceInput {
	return RegisterKnowledgeSourceInput{
		SourceKey: "video-forward-head-v1", SourceType: "video", Title: "Forward Head",
		Author: "operator", ProblemSlug: "forward-head", ProblemDisplayName: "Forward Head",
		OriginalFilePath: "sources/forward-head.mp4", Language: "en", LicenseStatus: "owned",
		ContentHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SourceVersion: "v1", Provenance: map[string]any{"origin": "owned recording"},
	}
}

func TestKnowledgeSourceRegistryRequiresReviewedLicenseAndProvenance(t *testing.T) {
	registry := NewKnowledgeSourceRegistry(&registryStoreStub{})
	input := validRegisterInput()
	input.LicenseStatus = "unknown"
	if _, err := registry.Register(context.Background(), uuid.New(), input); !errors.Is(err, ErrKnowledgeSourceInputInvalid) {
		t.Fatalf("expected invalid license error, got %v", err)
	}
	input = validRegisterInput()
	input.Provenance = nil
	if _, err := registry.Register(context.Background(), uuid.New(), input); !errors.Is(err, ErrKnowledgeSourceInputInvalid) {
		t.Fatalf("expected missing provenance error, got %v", err)
	}
}

func TestKnowledgeSourceRegistryRegistersActorAndRequiresRegisteredState(t *testing.T) {
	store := &registryStoreStub{}
	registry := NewKnowledgeSourceRegistry(store)
	actor := uuid.New()
	source, err := registry.Register(context.Background(), actor, validRegisterInput())
	if err != nil {
		t.Fatal(err)
	}
	if source.RegisteredBy == nil || *source.RegisteredBy != actor || source.RegisteredAt == nil {
		t.Fatalf("operator provenance missing: %#v", source)
	}
	if source.IngestStatus != "registered" {
		t.Fatalf("ingest_status=%q", source.IngestStatus)
	}
	if _, err := registry.FindIngestible(context.Background(), source.SourceKey); err != nil {
		t.Fatalf("registered source should be ingestible: %v", err)
	}
	store.source.IngestStatus = "ingested"
	if _, err := registry.FindIngestible(context.Background(), source.SourceKey); !errors.Is(err, ErrKnowledgeSourceNotReady) {
		t.Fatalf("expected not-ready after ingestion, got %v", err)
	}
}
