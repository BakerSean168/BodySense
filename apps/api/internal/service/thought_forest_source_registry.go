package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const ThoughtForestSnapshotSchemaV3 = "bodysense.health.snapshot.v3"

var ErrThoughtForestSnapshotInvalid = errors.New("thought forest snapshot is invalid")

type thoughtForestRepositoryIdentity struct {
	Name      string `json:"name"`
	GitCommit string `json:"git_commit"`
}

type thoughtForestSnapshotNote struct {
	SourceKey      string   `json:"source_key"`
	SourceType     string   `json:"source_type"`
	Path           string   `json:"path"`
	Title          string   `json:"title"`
	Aliases        []string `json:"aliases"`
	Description    string   `json:"description"`
	Tags           []string `json:"tags"`
	NoteType       string   `json:"note_type"`
	Status         string   `json:"status"`
	Updated        string   `json:"updated"`
	ProblemSlug    string   `json:"problem_slug"`
	KnowledgeKinds []string `json:"knowledge_kinds"`
	ContentHash    string   `json:"content_hash"`
}

type thoughtForestHealthSnapshot struct {
	SchemaVersion string                          `json:"schema_version"`
	SnapshotID    string                          `json:"snapshot_id"`
	AuthorityRole string                          `json:"authority_role"`
	Repository    thoughtForestRepositoryIdentity `json:"repository"`
	Notes         []thoughtForestSnapshotNote     `json:"notes"`
}

type ThoughtForestRegistrationReport struct {
	SnapshotID        string   `json:"snapshot_id"`
	GitCommit         string   `json:"git_commit"`
	TotalSources      int      `json:"total_sources"`
	Registered        int      `json:"registered"`
	ExistingValidated int      `json:"existing_validated"`
	SourceKeys        []string `json:"source_keys"`
}

// RegisterThoughtForestSnapshot binds every allowlisted note to the exact
// exported snapshot identity. The caller should execute this inside one DB
// transaction when atomic batch registration is required.
func RegisterThoughtForestSnapshot(
	ctx context.Context,
	registry *KnowledgeSourceRegistry,
	actorID uuid.UUID,
	payload []byte,
) (*ThoughtForestRegistrationReport, error) {
	if registry == nil || actorID == uuid.Nil {
		return nil, fmt.Errorf("%w: registry/operator unavailable", ErrThoughtForestSnapshotInvalid)
	}
	var snapshot thoughtForestHealthSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return nil, fmt.Errorf("%w: decode JSON: %v", ErrThoughtForestSnapshotInvalid, err)
	}
	if err := validateThoughtForestSnapshot(snapshot); err != nil {
		return nil, err
	}

	report := &ThoughtForestRegistrationReport{
		SnapshotID:   snapshot.SnapshotID,
		GitCommit:    snapshot.Repository.GitCommit,
		TotalSources: len(snapshot.Notes),
		SourceKeys:   make([]string, 0, len(snapshot.Notes)),
	}
	seenKeys := make(map[string]struct{}, len(snapshot.Notes))
	seenPaths := make(map[string]struct{}, len(snapshot.Notes))
	for _, note := range snapshot.Notes {
		if err := validateThoughtForestNote(note); err != nil {
			return nil, err
		}
		if _, exists := seenKeys[note.SourceKey]; exists {
			return nil, fmt.Errorf("%w: duplicate source_key %s", ErrThoughtForestSnapshotInvalid, note.SourceKey)
		}
		if _, exists := seenPaths[note.Path]; exists {
			return nil, fmt.Errorf("%w: duplicate note path %s", ErrThoughtForestSnapshotInvalid, note.Path)
		}
		seenKeys[note.SourceKey] = struct{}{}
		seenPaths[note.Path] = struct{}{}

		_, created, err := registry.RegisterOrValidate(ctx, actorID, RegisterKnowledgeSourceInput{
			SourceKey:          note.SourceKey,
			SourceType:         note.SourceType,
			Title:              note.Title,
			Author:             "Thought Forest",
			ProblemSlug:        note.ProblemSlug,
			ProblemDisplayName: note.Title,
			OriginalFilePath:   note.Path,
			Language:           "zh",
			LicenseStatus:      "owned",
			ContentHash:        note.ContentHash,
			SourceVersion:      snapshot.SnapshotID,
			Provenance: map[string]any{
				"origin":                  "thought_forest_snapshot",
				"snapshot_id":             snapshot.SnapshotID,
				"snapshot_schema_version": snapshot.SchemaVersion,
				"repository":              snapshot.Repository.Name,
				"git_commit":              snapshot.Repository.GitCommit,
				"path":                    note.Path,
				"authority_role":          snapshot.AuthorityRole,
				"note_content_hash":       strings.ToLower(note.ContentHash),
			},
			Metadata: map[string]any{
				"aliases":         note.Aliases,
				"description":     note.Description,
				"tags":            note.Tags,
				"note_type":       note.NoteType,
				"note_status":     note.Status,
				"note_updated":    note.Updated,
				"knowledge_kinds": note.KnowledgeKinds,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("register Thought Forest source %s: %w", note.SourceKey, err)
		}
		if created {
			report.Registered++
		} else {
			report.ExistingValidated++
		}
		report.SourceKeys = append(report.SourceKeys, note.SourceKey)
	}
	return report, nil
}

func validateThoughtForestSnapshot(snapshot thoughtForestHealthSnapshot) error {
	if snapshot.SchemaVersion != ThoughtForestSnapshotSchemaV3 {
		return fmt.Errorf("%w: schema_version must be %s", ErrThoughtForestSnapshotInvalid, ThoughtForestSnapshotSchemaV3)
	}
	if snapshot.Repository.Name != "thought-forest" {
		return fmt.Errorf("%w: repository.name must be thought-forest", ErrThoughtForestSnapshotInvalid)
	}
	commit := strings.ToLower(strings.TrimSpace(snapshot.Repository.GitCommit))
	if len(commit) != 40 {
		return fmt.Errorf("%w: git_commit must be a 40-character SHA-1", ErrThoughtForestSnapshotInvalid)
	}
	if _, err := hex.DecodeString(commit); err != nil {
		return fmt.Errorf("%w: git_commit must be hexadecimal", ErrThoughtForestSnapshotInvalid)
	}
	if snapshot.AuthorityRole == "" || len(snapshot.Notes) == 0 {
		return fmt.Errorf("%w: authority_role and notes are required", ErrThoughtForestSnapshotInvalid)
	}
	expectedPrefix := "thought-forest:" + commit + ":"
	if !strings.HasPrefix(snapshot.SnapshotID, expectedPrefix) || len(snapshot.SnapshotID) <= len(expectedPrefix) {
		return fmt.Errorf("%w: snapshot_id does not bind repository git_commit", ErrThoughtForestSnapshotInvalid)
	}
	return nil
}

func validateThoughtForestNote(note thoughtForestSnapshotNote) error {
	if note.SourceType != "thought_forest_note" || note.SourceKey == "" || note.Path == "" || note.Title == "" || note.ProblemSlug == "" {
		return fmt.Errorf("%w: incomplete Thought Forest note identity", ErrThoughtForestSnapshotInvalid)
	}
	if note.SourceKey != "thought-forest:"+note.Path {
		return fmt.Errorf("%w: source_key/path mismatch for %s", ErrThoughtForestSnapshotInvalid, note.SourceKey)
	}
	if _, ok := normalizeSHA256(note.ContentHash); !ok {
		return fmt.Errorf("%w: invalid note content_hash for %s", ErrThoughtForestSnapshotInvalid, note.SourceKey)
	}
	return nil
}
