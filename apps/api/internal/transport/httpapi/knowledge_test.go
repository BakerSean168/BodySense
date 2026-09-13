package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeKnowledgeSourceApplication struct{ listCalls int }

func (f *fakeKnowledgeSourceApplication) Register(context.Context, uuid.UUID, service.RegisterKnowledgeSourceInput) (*model.KnowledgeSource, error) {
	return nil, nil
}
func (f *fakeKnowledgeSourceApplication) List(context.Context, int) ([]model.KnowledgeSource, error) {
	f.listCalls++
	return []model.KnowledgeSource{}, nil
}

type fakeKnowledgeIngestionApplication struct{}

func (fakeKnowledgeIngestionApplication) EnqueueVideo(context.Context, uuid.UUID, service.KnowledgeVideoIngestionRequest) (*model.Job, bool, error) {
	return nil, false, nil
}
func (fakeKnowledgeIngestionApplication) GetJob(context.Context, uuid.UUID) (*model.Job, error) {
	return nil, service.ErrKnowledgeIngestionNotFound
}

type fakeKnowledgeQueryApplication struct{}

func (fakeKnowledgeQueryApplication) Search(context.Context, service.KnowledgeSearchRequest) (*service.KnowledgeSearchResponse, error) {
	return &service.KnowledgeSearchResponse{Results: []service.KnowledgeSearchResult{}}, nil
}
func (fakeKnowledgeQueryApplication) Stats(context.Context) (*service.KnowledgeStats, error) {
	return &service.KnowledgeStats{}, nil
}

func TestKnowledgeRoutesEnforceOperatorDomainBeforeValidationAndApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sourceApp := &fakeKnowledgeSourceApplication{}
	server := NewPublicServer(nil).WithKnowledge(sourceApp, fakeKnowledgeIngestionApplication{}, fakeKnowledgeQueryApplication{})
	r := gin.New()
	authCalls, operatorCalls, validatorCalls := 0, 0, 0
	RegisterRoutes(r, StrictHandler(server), RouteSecurity{
		Auth: func(c *gin.Context) {
			authCalls++
			c.Set("user_id", uuid.New().String())
			c.Next()
		},
		Operator: func(c *gin.Context) {
			operatorCalls++
			c.AbortWithStatusJSON(http.StatusForbidden, map[string]any{"error": map[string]any{"code": "FORBIDDEN", "message": "operator required"}})
		},
		Validator: func(c *gin.Context) { validatorCalls++; c.Next() },
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/sources", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authCalls != 1 || operatorCalls != 1 || validatorCalls != 0 || sourceApp.listCalls != 0 {
		t.Fatalf("security order auth=%d operator=%d validator=%d app=%d", authCalls, operatorCalls, validatorCalls, sourceApp.listCalls)
	}
}

func TestKnowledgeRoutesAllowOperatorIntoGeneratedBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sourceApp := &fakeKnowledgeSourceApplication{}
	server := NewPublicServer(nil).WithKnowledge(sourceApp, fakeKnowledgeIngestionApplication{}, fakeKnowledgeQueryApplication{})
	r := gin.New()
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	operatorID := uuid.New()
	RegisterRoutes(r, StrictHandler(server), RouteSecurity{
		Auth:      func(c *gin.Context) { c.Set("user_id", operatorID.String()); c.Next() },
		Operator:  func(c *gin.Context) { c.Set("knowledge_operator_id", operatorID.String()); c.Next() },
		Validator: RequestValidator(spec),
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/sources", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sourceApp.listCalls != 1 {
		t.Fatalf("operator did not reach application: calls=%d", sourceApp.listCalls)
	}
}

func TestKnowledgePublicProjectionsRemoveOperatorIdentityJobInputAndClipPath(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	registeredBy := uuid.New()
	hash := strings.Repeat("a", 64)
	source, err := strictKnowledgeSource(&model.KnowledgeSource{
		ID: 1, SourceKey: "source-1", SourceType: "video", Title: "Source", Author: "Author",
		ProblemSlug: "neck", ProblemDisplayName: "Neck", OriginalFilePath: "sources/neck.mp4", Language: "zh",
		IngestStatus: "registered", Metadata: datatypes.JSON(`{}`), LicenseStatus: "owned", ContentHash: &hash,
		SourceVersion: "v1", Provenance: datatypes.JSON(`{"origin":"operator"}`), RegisteredBy: &registeredBy,
		RegisteredAt: &now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("project source: %v", err)
	}
	sourceJSON, _ := json.Marshal(source)
	if strings.Contains(string(sourceJSON), "registered_by") || strings.Contains(string(sourceJSON), registeredBy.String()) {
		t.Fatalf("source leaked operator identity: %s", sourceJSON)
	}

	idem := "private-idempotency-key"
	job, err := strictKnowledgeJob(&model.Job{
		ID: uuid.New(), UserID: registeredBy, JobType: service.KnowledgeIngestVideoJobType, Status: "pending",
		Input: datatypes.JSON(`{"operator_id":"secret","video_path":"private"}`), IdempotencyKey: &idem,
		Attempts: 0, MaxAttempts: 3, CreatedAt: now, UpdatedAt: now, Metadata: datatypes.JSON(`{"private":true}`),
	})
	if err != nil {
		t.Fatalf("project job: %v", err)
	}
	jobJSON, _ := json.Marshal(job)
	for _, forbidden := range []string{"user_id", "input", "idempotency_key", "metadata", "operator_id", registeredBy.String(), idem} {
		if strings.Contains(string(jobJSON), forbidden) {
			t.Fatalf("job leaked %q: %s", forbidden, jobJSON)
		}
	}

	search, err := strictKnowledgeSearchResponse(&service.KnowledgeSearchResponse{Results: []service.KnowledgeSearchResult{{
		ID: 1, ProblemSlug: "neck", Category: "education", UnitType: "concept", Title: "T", Summary: "S", BodyMarkdown: "B",
		Similarity: .9, SourceTitle: "Source", SourceAuthor: "Author", SourceTimestamp: "00:01", Tags: []string{},
		Clips: []service.KnowledgeSearchClip{{ID: 2, ClipKey: "clip", ClipType: "video", Title: "Clip", FilePath: "/srv/private/clip.mp4", SourceTimestamp: "00:01"}},
	}}, Total: 1})
	if err != nil {
		t.Fatalf("project search: %v", err)
	}
	searchJSON, _ := json.Marshal(search)
	if strings.Contains(string(searchJSON), "file_path") || strings.Contains(string(searchJSON), "/srv/private") {
		t.Fatalf("search leaked private clip path: %s", searchJSON)
	}
}
