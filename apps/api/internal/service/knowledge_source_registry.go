package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
	ErrKnowledgeSourceConflict     = errors.New("knowledge source registration conflicts with existing identity")
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

func normalizeKnowledgeSourceRegistration(input RegisterKnowledgeSourceInput) (RegisterKnowledgeSourceInput, string, datatypes.JSON, datatypes.JSON, error) {
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
		return input, "", nil, nil, fmt.Errorf("%w: source identity fields are required", ErrKnowledgeSourceInputInvalid)
	}
	if _, ok := allowedKnowledgeLicenseStatuses[input.LicenseStatus]; !ok {
		return input, "", nil, nil, fmt.Errorf("%w: license_status must be an explicitly reviewed value", ErrKnowledgeSourceInputInvalid)
	}
	contentHash, ok := normalizeSHA256(input.ContentHash)
	if !ok {
		return input, "", nil, nil, fmt.Errorf("%w: content_hash must be a SHA-256 hex digest", ErrKnowledgeSourceInputInvalid)
	}
	if len(input.Provenance) == 0 {
		return input, "", nil, nil, fmt.Errorf("%w: provenance is required", ErrKnowledgeSourceInputInvalid)
	}
	provenance, err := json.Marshal(input.Provenance)
	if err != nil {
		return input, "", nil, nil, fmt.Errorf("%w: encode provenance: %v", ErrKnowledgeSourceInputInvalid, err)
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return input, "", nil, nil, fmt.Errorf("%w: encode metadata: %v", ErrKnowledgeSourceInputInvalid, err)
	}
	input.ContentHash = contentHash
	return input, contentHash, datatypes.JSON(provenance), datatypes.JSON(metadata), nil
}

func knowledgeSourceFromRegistration(actorID uuid.UUID, input RegisterKnowledgeSourceInput, contentHash string, provenance, metadata datatypes.JSON) *model.KnowledgeSource {
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
		Metadata:           metadata,
		LicenseStatus:      input.LicenseStatus,
		ContentHash:        &contentHash,
		SourceVersion:      input.SourceVersion,
		Provenance:         provenance,
		RegisteredBy:       &actorID,
		RegisteredAt:       &now,
	}
	if input.CanonicalURL != "" {
		source.CanonicalURL = &input.CanonicalURL
	}
	return source
}

func (r *KnowledgeSourceRegistry) register(ctx context.Context, actorID uuid.UUID, input RegisterKnowledgeSourceInput) (*model.KnowledgeSource, bool, error) {
	if r == nil || r.store == nil || actorID == uuid.Nil {
		return nil, false, fmt.Errorf("%w: registry unavailable", ErrKnowledgeSourceInputInvalid)
	}
	normalized, contentHash, provenance, metadata, err := normalizeKnowledgeSourceRegistration(input)
	if err != nil {
		return nil, false, err
	}
	source := knowledgeSourceFromRegistration(actorID, normalized, contentHash, provenance, metadata)
	created, err := r.store.Register(ctx, source)
	if err != nil {
		return nil, false, err
	}
	if created {
		return source, true, nil
	}
	existing, err := r.store.FindByKey(ctx, normalized.SourceKey)
	if err != nil {
		return nil, false, err
	}
	if !sameKnowledgeSourceRegistration(existing, source) {
		return nil, false, fmt.Errorf("%w: source_key=%s", ErrKnowledgeSourceConflict, normalized.SourceKey)
	}
	return existing, false, nil
}

func (r *KnowledgeSourceRegistry) Register(ctx context.Context, actorID uuid.UUID, input RegisterKnowledgeSourceInput) (*model.KnowledgeSource, error) {
	source, created, err := r.register(ctx, actorID, input)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, ErrKnowledgeSourceExists
	}
	return source, nil
}

// RegisterOrValidate is the idempotent operator/import path. The same immutable
// source identity is a no-op; reusing a source_key for a different hash/version/
// provenance fails closed instead of mutating the registered source.
func (r *KnowledgeSourceRegistry) RegisterOrValidate(ctx context.Context, actorID uuid.UUID, input RegisterKnowledgeSourceInput) (*model.KnowledgeSource, bool, error) {
	return r.register(ctx, actorID, input)
}

func sameKnowledgeSourceRegistration(existing, candidate *model.KnowledgeSource) bool {
	if existing == nil || candidate == nil || existing.ContentHash == nil || candidate.ContentHash == nil {
		return false
	}
	if existing.SourceKey != candidate.SourceKey || existing.SourceType != candidate.SourceType || existing.Title != candidate.Title || existing.Author != candidate.Author || existing.ProblemSlug != candidate.ProblemSlug || existing.ProblemDisplayName != candidate.ProblemDisplayName || existing.OriginalFilePath != candidate.OriginalFilePath || existing.Language != candidate.Language || existing.LicenseStatus != candidate.LicenseStatus || !strings.EqualFold(*existing.ContentHash, *candidate.ContentHash) || existing.SourceVersion != candidate.SourceVersion {
		return false
	}
	if stringPtrValue(existing.CanonicalURL) != stringPtrValue(candidate.CanonicalURL) {
		return false
	}
	return semanticJSONEqual(existing.Provenance, candidate.Provenance)
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func semanticJSONEqual(left, right []byte) bool {
	var l, r any
	if json.Unmarshal(left, &l) != nil || json.Unmarshal(right, &r) != nil {
		return false
	}
	return reflect.DeepEqual(l, r)
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

func (r *KnowledgeSourceRegistry) FindByKey(ctx context.Context, sourceKey string) (*model.KnowledgeSource, error) {
	if r == nil || r.store == nil {
		return nil, ErrKnowledgeSourceNotReady
	}
	return r.store.FindByKey(ctx, strings.TrimSpace(sourceKey))
}

func (r *KnowledgeSourceRegistry) List(ctx context.Context, limit int) ([]model.KnowledgeSource, error) {
	return r.store.List(ctx, limit)
}
