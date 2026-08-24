package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type fakeKnowledgeSourceStore struct {
	source *model.KnowledgeSource
}

func (f *fakeKnowledgeSourceStore) Register(_ context.Context, source *model.KnowledgeSource) (bool, error) {
	if f.source != nil {
		return false, nil
	}
	f.source = source
	return true, nil
}

func (f *fakeKnowledgeSourceStore) FindByKey(_ context.Context, sourceKey string) (*model.KnowledgeSource, error) {
	if f.source == nil || f.source.SourceKey != sourceKey {
		return nil, gorm.ErrRecordNotFound
	}
	return f.source, nil
}

func (f *fakeKnowledgeSourceStore) List(_ context.Context, _ int) ([]model.KnowledgeSource, error) {
	if f.source == nil {
		return nil, nil
	}
	return []model.KnowledgeSource{*f.source}, nil
}

func readyVideoRegistry(sourceKey, videoPath string) *service.KnowledgeSourceRegistry {
	now := time.Now().UTC()
	actor := uuid.New()
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	return service.NewKnowledgeSourceRegistry(&fakeKnowledgeSourceStore{source: &model.KnowledgeSource{
		SourceKey: sourceKey, SourceType: "video", Title: "Source", Author: "tester",
		ProblemSlug: "forward-head", ProblemDisplayName: "头前移", OriginalFilePath: videoPath,
		Language: "zh", IngestStatus: "registered", LicenseStatus: "owned", ContentHash: &hash,
		Provenance: []byte(`{"origin":"operator"}`), RegisteredBy: &actor, RegisteredAt: &now,
	}})
}

type fakeKnowledgeDeployment struct {
	curator  string
	splitter string
}

func (d fakeKnowledgeDeployment) KnowledgeCuratorConfigurationID() string { return d.curator }
func (d fakeKnowledgeDeployment) KnowledgeCuratorDecisionPolicyRevision() string {
	return "knowledge-curator-go-v1"
}
func (d fakeKnowledgeDeployment) KnowledgeCuratorLogicalModel() string     { return "bodysense-structured" }
func (d fakeKnowledgeDeployment) KnowledgeSplitterConfigurationID() string { return d.splitter }
func (d fakeKnowledgeDeployment) KnowledgeSplitterDecisionPolicyRevision() string {
	return "knowledge-splitter-go-v1"
}
func (d fakeKnowledgeDeployment) KnowledgeSplitterLogicalModel() string {
	return "bodysense-structured"
}

type fakeKnowledgeJobRuntime struct {
	captured json.RawMessage
	job      *model.Job
}

func (f *fakeKnowledgeJobRuntime) CreateJobWithIdempotencyAttempts(_ context.Context, userID uuid.UUID, jobType string, input datatypes.JSON, _ string, maxAttempts int, _, _ *uuid.UUID) (*model.Job, bool, error) {
	f.captured = append(json.RawMessage(nil), input...)
	if f.job == nil {
		f.job = &model.Job{ID: uuid.New(), UserID: userID, JobType: jobType, Status: "pending", MaxAttempts: maxAttempts}
	}
	return f.job, false, nil
}
func (f *fakeKnowledgeJobRuntime) GetJob(_ context.Context, _ uuid.UUID) (*model.Job, error) {
	return f.job, nil
}
func (f *fakeKnowledgeJobRuntime) ListRecoverable(context.Context, string, time.Duration, int) ([]model.Job, error) {
	return nil, nil
}
func (f *fakeKnowledgeJobRuntime) ClaimPending(context.Context, uuid.UUID) (*model.Job, bool, error) {
	return nil, false, nil
}
func (f *fakeKnowledgeJobRuntime) UpdateProgress(context.Context, uuid.UUID, any) error { return nil }
func (f *fakeKnowledgeJobRuntime) TransitionTo(context.Context, uuid.UUID, string, any, any) error {
	return nil
}

func TestValidateVideoPathUsesSharedRelativeDataRootContract(t *testing.T) {
	for _, path := range []string{"sources/video.mp4", "nested/source/video.mp4"} {
		if !validateVideoPath(path) {
			t.Fatalf("expected relative path %q to be valid", path)
		}
	}
	for _, path := range []string{"", ".", "../video.mp4", "nested/../video.mp4", `nested\..\video.mp4`, "/tmp/video.mp4"} {
		if validateVideoPath(path) {
			t.Fatalf("expected path %q to be rejected", path)
		}
	}
}

func TestKnowledgeIngestEnqueuesPinnedAgentConfigurations(t *testing.T) {
	const splitterID = "knowledge-splitter-config-test"
	const curatorID = "knowledge-curator-config-test"
	deployment := fakeKnowledgeDeployment{curator: curatorID, splitter: splitterID}
	registry := readyVideoRegistry("source-test", "sources/video.mp4")
	jobs := &fakeKnowledgeJobRuntime{}
	ingestion := service.NewKnowledgeIngestionService(registry, jobs, deployment, "http://unused")
	h := NewKnowledgeHandler(deployment).WithSourceRegistry(registry).WithIngestionService(ingestion)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/knowledge/ingestions/video", func(c *gin.Context) {
		c.Set("knowledge_operator_id", uuid.New().String())
		c.Next()
	}, h.IngestVideo)

	body := []byte(`{
		"source_key":"source-test",
		"video_path":"sources/video.mp4",
		"splitter_provider":"llm",
		"ai_refine":true,
		"splitter_configuration_id":"caller-must-not-control-this",
		"curator_configuration_id":"caller-must-not-control-this"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/knowledge/ingestions/video", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	var input map[string]any
	if err := json.Unmarshal(jobs.captured, &input); err != nil {
		t.Fatal(err)
	}
	if input["splitter_configuration_id"] != splitterID || input["curator_configuration_id"] != curatorID {
		t.Fatalf("Go did not pin immutable Knowledge Agent configs: %#v", input)
	}
	if input["source_key"] != "source-test" || input["content_hash"] == "" || input["operator_id"] == "" {
		t.Fatalf("durable job input is missing source/operator identity: %#v", input)
	}
	if input["export_clips"] != true {
		t.Fatalf("omitted export_clips must preserve the historical default=true: %#v", input)
	}
}
