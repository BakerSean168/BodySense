package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrKnowledgeSourceExists       = errors.New("knowledge source already exists")
	ErrKnowledgeSourceNotReady     = errors.New("knowledge source is not registered for ingestion")
	ErrKnowledgeSourceInputInvalid = errors.New("knowledge source registration is invalid")
)

type knowledgeSourceStore interface {
	Register(context.Context, *model.KnowledgeSource) (bool, error)
	FindByKey(context.Context, string) (*model.KnowledgeSource, error)
	List(context.Context, int) ([]model.KnowledgeSource, error)
}

type KnowledgeSourceRegistry struct {
	store knowledgeSourceStore
}

func NewKnowledgeSourceRegistry(store knowledgeSourceStore) *KnowledgeSourceRegistry {
	return &KnowledgeSourceRegistry{store: store}
}

type RegisterKnowledgeSourceInput struct {
	SourceKey          string
	SourceType         string
	Title              string
	Author             string
	ProblemSlug        string
	ProblemDisplayName string
	OriginalFilePath   string
	Language           string
	LicenseStatus      string
	ContentHash        string
	CanonicalURL       string
	SourceVersion      string
	Provenance         map[string]any
	Metadata           map[string]any
}

var allowedKnowledgeLicenseStatuses = map[string]struct{}{
	"verified_reuse": {},
	"citation_only":  {},
	"owned":          {},
	"public_domain":  {},
}

func normalizeSHA256(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return "", false
	}
	_, err := hex.DecodeString(value)
	return value, err == nil
}

func (r *KnowledgeSourceRegistry) Register(ctx context.Context, actorID uuid.UUID, input RegisterKnowledgeSourceInput) (*model.KnowledgeSource, error) {
	if r == nil || r.store == nil || actorID == uuid.Nil {
		return nil, fmt.Errorf("%w: registry unavailable", ErrKnowledgeSourceInputInvalid)
	}
	input.SourceKey = strings.TrimSpace(input.SourceKey)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.Title = strings.TrimSpace(input.Title)
	input.Author = strings.TrimSpace(input.Author)
	input.ProblemSlug = strings.TrimSpace(input.ProblemSlug)
	input.ProblemDisplayName = strings.TrimSpace(input.ProblemDisplayName)
	input.OriginalFilePath = strings.TrimSpace(input.OriginalFilePath)
	input.Language = strings.TrimSpace(input.Language)
	input.LicenseStatus = strings.TrimSpace(input.LicenseStatus)
	input.CanonicalURL = strings.TrimSpace(input.CanonicalURL)
	input.SourceVersion = strings.TrimSpace(input.SourceVersion)
	if input.Language == "" {
		input.Language = "zh"
	}
	if input.SourceVersion == "" {
		input.SourceVersion = "v1"
	}
	if input.SourceKey == "" || len(input.SourceKey) > 200 || input.SourceType == "" || input.Title == "" || input.Author == "" || input.ProblemSlug == "" || input.ProblemDisplayName == "" || input.OriginalFilePath == "" {
		return nil, fmt.Errorf("%w: source identity fields are required", ErrKnowledgeSourceInputInvalid)
	}
	if _, ok := allowedKnowledgeLicenseStatuses[input.LicenseStatus]; !ok {
		return nil, fmt.Errorf("%w: license_status must be an explicitly reviewed value", ErrKnowledgeSourceInputInvalid)
	}
	contentHash, ok := normalizeSHA256(input.ContentHash)
	if !ok {
		return nil, fmt.Errorf("%w: content_hash must be a SHA-256 hex digest", ErrKnowledgeSourceInputInvalid)
	}
	if len(input.Provenance) == 0 {
		return nil, fmt.Errorf("%w: provenance is required", ErrKnowledgeSourceInputInvalid)
	}
	provenance, err := json.Marshal(input.Provenance)
	if err != nil {
		return nil, fmt.Errorf("%w: encode provenance: %v", ErrKnowledgeSourceInputInvalid, err)
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("%w: encode metadata: %v", ErrKnowledgeSourceInputInvalid, err)
	}
	now := time.Now().UTC()
	source := &model.KnowledgeSource{
		SourceKey:          input.SourceKey,
		SourceType:         input.SourceType,
		Title:              input.Title,
		Author:             input.Author,
		ProblemSlug:        input.ProblemSlug,
		ProblemDisplayName: input.ProblemDisplayName,
		OriginalFilePath:   input.OriginalFilePath,
		Language:           input.Language,
		IngestStatus:       "registered",
		Metadata:           datatypes.JSON(metadata),
		LicenseStatus:      input.LicenseStatus,
		ContentHash:        &contentHash,
		SourceVersion:      input.SourceVersion,
		Provenance:         datatypes.JSON(provenance),
		RegisteredBy:       &actorID,
		RegisteredAt:       &now,
	}
	if input.CanonicalURL != "" {
		source.CanonicalURL = &input.CanonicalURL
	}
	created, err := r.store.Register(ctx, source)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, ErrKnowledgeSourceExists
	}
	return source, nil
}

func (r *KnowledgeSourceRegistry) FindIngestible(ctx context.Context, sourceKey string) (*model.KnowledgeSource, error) {
	if r == nil || r.store == nil {
		return nil, ErrKnowledgeSourceNotReady
	}
	source, err := r.store.FindByKey(ctx, strings.TrimSpace(sourceKey))
	if err != nil {
		return nil, err
	}
	if source.IngestStatus != "registered" || source.RegisteredAt == nil || source.RegisteredBy == nil || source.ContentHash == nil || strings.TrimSpace(*source.ContentHash) == "" || len(source.Provenance) == 0 {
		return nil, ErrKnowledgeSourceNotReady
	}
	if _, ok := allowedKnowledgeLicenseStatuses[source.LicenseStatus]; !ok {
		return nil, ErrKnowledgeSourceNotReady
	}
	return source, nil
}

func (r *KnowledgeSourceRegistry) List(ctx context.Context, limit int) ([]model.KnowledgeSource, error) {
	return r.store.List(ctx, limit)
}
