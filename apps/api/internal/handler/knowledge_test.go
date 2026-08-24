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

func TestKnowledgeIngestPinsGoSelectedAgentConfigurations(t *testing.T) {
	const splitterID = "knowledge-splitter-config-test"
	const curatorID = "knowledge-curator-config-test"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/knowledge/ingestions/video" {
			http.NotFound(w, r)
			return
		}
		var req IngestVideoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.SplitterConfigurationID != splitterID || req.CuratorConfigurationID != curatorID {
			t.Fatalf("Go did not pin knowledge configs: %#v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"agent_execution": map[string]any{
				"knowledge_splitter": map[string]any{
					"agent_configuration": map[string]any{
						"id": splitterID, "role": "knowledge_splitter",
						"decision_policy_revision": "knowledge-splitter-go-v1", "logical_model": "bodysense-structured",
					},
					"execution_provenance": map[string]any{"status": "executed", "logical_model": "bodysense-structured"},
				},
				"knowledge_curator": map[string]any{
					"agent_configuration": map[string]any{
						"id": curatorID, "role": "knowledge_curator",
						"decision_policy_revision": "knowledge-curator-go-v1", "logical_model": "bodysense-structured",
					},
					"execution_provenance": map[string]any{"status": "executed", "logical_model": "bodysense-structured"},
				},
			},
		})
	}))
	defer upstream.Close()

	h := NewKnowledgeHandler(fakeKnowledgeDeployment{curator: curatorID, splitter: splitterID}).WithSourceRegistry(readyVideoRegistry("source-test", "sources/video.mp4"))
	h.aiServiceURL = upstream.URL
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/knowledge/ingestions/video", h.IngestVideo)

	body := []byte(`{
		"source_key":"source-test",
		"video_path":"sources/video.mp4",
		"problem_slug":"forward-head",
		"problem_display_name":"头前移",
		"author":"tester",
		"splitter_provider":"llm",
		"ai_refine":true,
		"splitter_configuration_id":"caller-must-not-control-this",
		"curator_configuration_id":"caller-must-not-control-this"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/knowledge/ingestions/video", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestValidateKnowledgeAgentExecutionRejectsMismatchedLineage(t *testing.T) {
	req := IngestVideoRequest{
		SplitterProvider:        "llm",
		AIRefine:                true,
		SplitterConfigurationID: "splitter-good",
		CuratorConfigurationID:  "curator-good",
	}
	body := []byte(`{"agent_execution":{"knowledge_splitter":{"agent_configuration":{"id":"wrong","role":"knowledge_splitter","decision_policy_revision":"knowledge-splitter-go-v1","logical_model":"bodysense-structured"},"execution_provenance":{"status":"executed","logical_model":"bodysense-structured"}},"knowledge_curator":{"agent_configuration":{"id":"curator-good","role":"knowledge_curator","decision_policy_revision":"knowledge-curator-go-v1","logical_model":"bodysense-structured"},"execution_provenance":{"status":"executed","logical_model":"bodysense-structured"}}}}`)
	h := NewKnowledgeHandler(fakeKnowledgeDeployment{curator: "curator-good", splitter: "splitter-good"})
	if err := h.validateKnowledgeAgentExecution(body, req); err == nil {
		t.Fatal("expected lineage mismatch")
	}
}
