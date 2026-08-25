package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type thoughtForestRegistryStoreStub struct {
	byKey map[string]*model.KnowledgeSource
}

func newThoughtForestRegistryStoreStub() *thoughtForestRegistryStoreStub {
	return &thoughtForestRegistryStoreStub{byKey: map[string]*model.KnowledgeSource{}}
}

func (s *thoughtForestRegistryStoreStub) Register(_ context.Context, source *model.KnowledgeSource) (bool, error) {
	if _, exists := s.byKey[source.SourceKey]; exists {
		return false, nil
	}
	copy := *source
	s.byKey[source.SourceKey] = &copy
	return true, nil
}

func (s *thoughtForestRegistryStoreStub) FindByKey(_ context.Context, key string) (*model.KnowledgeSource, error) {
	source := s.byKey[key]
	if source == nil {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *source
	return &copy, nil
}

func (s *thoughtForestRegistryStoreStub) List(_ context.Context, _ int) ([]model.KnowledgeSource, error) {
	return nil, nil
}

func validThoughtForestSnapshotPayload(t *testing.T) []byte {
	t.Helper()
	commit := "8dbe766899da073727336a6f93cb142e34eeb4e8"
	payload := map[string]any{
		"schema_version": "bodysense.health.snapshot.v3",
		"snapshot_id":    "thought-forest:" + commit + ":bb88ac2fc4b6",
		"authority_role": "seed_corpus",
		"repository":     map[string]any{"name": "thought-forest", "git_commit": commit},
		"notes": []any{
			map[string]any{
				"source_key": "thought-forest:z/pain.md", "source_type": "thought_forest_note",
				"path": "z/pain.md", "title": "Pain", "aliases": []string{"Pain"},
				"description": "Pain reference", "tags": []string{"life/health"},
				"note_type": "concept", "status": "growing", "updated": "2026-08-24T00:00:00Z",
				"problem_slug": "pain-science", "knowledge_kinds": []string{"definition"},
				"content_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			map[string]any{
				"source_key": "thought-forest:z/nociception.md", "source_type": "thought_forest_note",
				"path": "z/nociception.md", "title": "Nociception", "aliases": []string{"Nociception"},
				"description": "Nociception reference", "tags": []string{"life/health"},
				"note_type": "concept", "status": "growing", "updated": "2026-08-24T00:00:00Z",
				"problem_slug": "nociception", "knowledge_kinds": []string{"definition"},
				"content_hash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRegisterThoughtForestSnapshotIsExactAndIdempotent(t *testing.T) {
	store := newThoughtForestRegistryStoreStub()
	registry := NewKnowledgeSourceRegistry(store)
	actor := uuid.New()
	payload := validThoughtForestSnapshotPayload(t)

	first, err := RegisterThoughtForestSnapshot(context.Background(), registry, actor, payload)
	if err != nil {
		t.Fatal(err)
	}
	if first.Registered != 2 || first.ExistingValidated != 0 || first.TotalSources != 2 {
		t.Fatalf("first report=%#v", first)
	}
	pain := store.byKey["thought-forest:z/pain.md"]
	if pain == nil || pain.SourceVersion != first.SnapshotID || pain.LicenseStatus != "owned" || pain.ContentHash == nil || *pain.ContentHash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("registered source identity=%#v", pain)
	}

	second, err := RegisterThoughtForestSnapshot(context.Background(), registry, uuid.New(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if second.Registered != 0 || second.ExistingValidated != 2 {
		t.Fatalf("second report=%#v", second)
	}
}

func TestRegisterThoughtForestSnapshotRejectsSourceKeyReuseWithDrift(t *testing.T) {
	store := newThoughtForestRegistryStoreStub()
	registry := NewKnowledgeSourceRegistry(store)
	payload := validThoughtForestSnapshotPayload(t)
	if _, err := RegisterThoughtForestSnapshot(context.Background(), registry, uuid.New(), payload); err != nil {
		t.Fatal(err)
	}
	store.byKey["thought-forest:z/pain.md"].SourceVersion = "thought-forest:other:snapshot"
	_, err := RegisterThoughtForestSnapshot(context.Background(), registry, uuid.New(), payload)
	if !errors.Is(err, ErrKnowledgeSourceConflict) {
		t.Fatalf("expected source identity conflict, got %v", err)
	}
}

func TestRegisterThoughtForestSnapshotRejectsSnapshotCommitMismatch(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal(validThoughtForestSnapshotPayload(t), &payload); err != nil {
		t.Fatal(err)
	}
	payload["snapshot_id"] = "thought-forest:0000000000000000000000000000000000000000:bb88ac2fc4b6"
	raw, _ := json.Marshal(payload)
	_, err := RegisterThoughtForestSnapshot(context.Background(), NewKnowledgeSourceRegistry(newThoughtForestRegistryStoreStub()), uuid.New(), raw)
	if !errors.Is(err, ErrThoughtForestSnapshotInvalid) {
		t.Fatalf("expected invalid snapshot identity, got %v", err)
	}
}
