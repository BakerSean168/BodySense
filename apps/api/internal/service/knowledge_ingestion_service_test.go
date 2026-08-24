package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type knowledgeJobRuntimeStub struct {
	jobs          map[uuid.UUID]*model.Job
	capturedInput datatypes.JSON
	capturedKey   string
	capturedMax   int
	transitions   []string
}

func newKnowledgeJobRuntimeStub() *knowledgeJobRuntimeStub {
	return &knowledgeJobRuntimeStub{jobs: map[uuid.UUID]*model.Job{}}
}

func (f *knowledgeJobRuntimeStub) CreateJobWithIdempotencyAttempts(_ context.Context, userID uuid.UUID, jobType string, input datatypes.JSON, key string, maxAttempts int, _, _ *uuid.UUID) (*model.Job, bool, error) {
	for _, existing := range f.jobs {
		if existing.IdempotencyKey != nil && *existing.IdempotencyKey == key {
			copy := *existing
			return &copy, true, nil
		}
	}
	id := uuid.New()
	keyCopy := key
	job := &model.Job{ID: id, UserID: userID, JobType: jobType, Status: "pending", Input: append(datatypes.JSON(nil), input...), IdempotencyKey: &keyCopy, MaxAttempts: maxAttempts}
	f.jobs[id] = job
	f.capturedInput = append(datatypes.JSON(nil), input...)
	f.capturedKey = key
	f.capturedMax = maxAttempts
	copy := *job
	return &copy, false, nil
}
func (f *knowledgeJobRuntimeStub) GetJob(_ context.Context, id uuid.UUID) (*model.Job, error) {
	job := f.jobs[id]
	if job == nil {
		return nil, nil
	}
	copy := *job
	return &copy, nil
}
func (f *knowledgeJobRuntimeStub) ListRecoverable(_ context.Context, jobType string, _ time.Duration, _ int) ([]model.Job, error) {
	var jobs []model.Job
	for _, job := range f.jobs {
		if job.JobType == jobType && (job.Status == "pending" || job.Status == "running") {
			jobs = append(jobs, *job)
		}
	}
	return jobs, nil
}
func (f *knowledgeJobRuntimeStub) ClaimPending(_ context.Context, id uuid.UUID) (*model.Job, bool, error) {
	job := f.jobs[id]
	if job == nil || job.Status != "pending" || job.Attempts >= job.MaxAttempts {
		return nil, false, nil
	}
	job.Status = "running"
	job.Attempts++
	copy := *job
	return &copy, true, nil
}
func (f *knowledgeJobRuntimeStub) UpdateProgress(_ context.Context, id uuid.UUID, progress any) error {
	data, _ := json.Marshal(progress)
	f.jobs[id].Progress = data
	return nil
}
func (f *knowledgeJobRuntimeStub) TransitionTo(_ context.Context, id uuid.UUID, status string, result, errData any) error {
	job := f.jobs[id]
	job.Status = status
	if result != nil {
		job.Result, _ = json.Marshal(result)
	}
	if errData != nil {
		job.Error, _ = json.Marshal(errData)
	}
	f.transitions = append(f.transitions, status)
	return nil
}

func registeredKnowledgeRegistry(t *testing.T) (*KnowledgeSourceRegistry, *model.KnowledgeSource) {
	t.Helper()
	store := &registryStoreStub{}
	registry := NewKnowledgeSourceRegistry(store)
	actor := uuid.New()
	source, err := registry.Register(context.Background(), actor, validRegisterInput())
	if err != nil {
		t.Fatal(err)
	}
	source.ID = 42
	store.source.ID = 42
	return registry, source
}

func knowledgeDeploymentForTest() fakeKnowledgeDeploymentForService {
	return fakeKnowledgeDeploymentForService{}
}

type fakeKnowledgeDeploymentForService struct{}

func (fakeKnowledgeDeploymentForService) KnowledgeCuratorConfigurationID() string {
	return "curator-v1"
}
func (fakeKnowledgeDeploymentForService) KnowledgeCuratorDecisionPolicyRevision() string {
	return "curator-policy-v1"
}
func (fakeKnowledgeDeploymentForService) KnowledgeCuratorLogicalModel() string { return "structured" }
func (fakeKnowledgeDeploymentForService) KnowledgeSplitterConfigurationID() string {
	return "splitter-v1"
}
func (fakeKnowledgeDeploymentForService) KnowledgeSplitterDecisionPolicyRevision() string {
	return "splitter-policy-v1"
}
func (fakeKnowledgeDeploymentForService) KnowledgeSplitterLogicalModel() string { return "structured" }

func TestKnowledgeIngestionEnqueuePinsImmutableInputAndIsIdempotent(t *testing.T) {
	registry, source := registeredKnowledgeRegistry(t)
	jobs := newKnowledgeJobRuntimeStub()
	service := NewKnowledgeIngestionService(registry, jobs, knowledgeDeploymentForTest(), "http://unused")
	operator := uuid.New()
	req := KnowledgeVideoIngestionRequest{
		SourceKey: source.SourceKey, VideoPath: source.OriginalFilePath,
		SplitterProvider: "llm", AIRefine: true, ExportClips: true,
	}
	first, existed, err := service.EnqueueVideo(context.Background(), operator, req)
	if err != nil || existed {
		t.Fatalf("first enqueue existed=%v err=%v", existed, err)
	}
	second, existed, err := service.EnqueueVideo(context.Background(), uuid.New(), req)
	if err != nil || !existed || second.ID != first.ID {
		t.Fatalf("idempotent enqueue job=%v existed=%v err=%v", second, existed, err)
	}
	if jobs.capturedMax != 3 {
		t.Fatalf("max_attempts=%d", jobs.capturedMax)
	}
	var input knowledgeVideoJobInput
	if err := json.Unmarshal(jobs.capturedInput, &input); err != nil {
		t.Fatal(err)
	}
	if input.SourceID != 42 || input.ContentHash != *source.ContentHash || input.SourceVersion != source.SourceVersion || input.OperatorID != operator.String() {
		t.Fatalf("immutable source/operator input not pinned: %#v", input)
	}
	if input.SplitterConfigurationID != "splitter-v1" || input.CuratorConfigurationID != "curator-v1" {
		t.Fatalf("Agent identities not pinned: %#v", input)
	}
}

func TestKnowledgeIngestionWorkerCompletesOnlyAfterAgentIdentityValidation(t *testing.T) {
	registry, source := registeredKnowledgeRegistry(t)
	jobs := newKnowledgeJobRuntimeStub()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["source_key"] != source.SourceKey || request["expected_content_hash"] != *source.ContentHash {
			t.Fatalf("source identity not forwarded: %#v", request)
		}
		if request["splitter_configuration_id"] != "splitter-v1" || request["curator_configuration_id"] != "curator-v1" {
			t.Fatalf("Agent identity not forwarded: %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"source_id": 42, "source_key": source.SourceKey, "status": "ingested",
			"artifact_dir": "knowledge_sources/test", "transcript_segments": 2,
			"knowledge_units": 1, "clips": 0,
			"agent_execution": map[string]any{
				"knowledge_splitter": map[string]any{
					"agent_configuration":  map[string]any{"id": "splitter-v1", "role": "knowledge_splitter", "decision_policy_revision": "splitter-policy-v1", "logical_model": "structured"},
					"execution_provenance": map[string]any{"status": "executed", "logical_model": "structured"},
				},
				"knowledge_curator": map[string]any{
					"agent_configuration":  map[string]any{"id": "curator-v1", "role": "knowledge_curator", "decision_policy_revision": "curator-policy-v1", "logical_model": "structured"},
					"execution_provenance": map[string]any{"status": "executed", "logical_model": "structured"},
				},
			},
		})
	}))
	defer server.Close()
	service := NewKnowledgeIngestionService(registry, jobs, knowledgeDeploymentForTest(), server.URL)
	job, _, err := service.EnqueueVideo(context.Background(), uuid.New(), KnowledgeVideoIngestionRequest{
		SourceKey: source.SourceKey, VideoPath: source.OriginalFilePath, SplitterProvider: "llm", AIRefine: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.processPending(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if jobs.jobs[job.ID].Status != "completed" || jobs.jobs[job.ID].Attempts != 1 {
		t.Fatalf("job=%#v", jobs.jobs[job.ID])
	}
}

func TestKnowledgeIngestionWorkerRetriesTransientFailureAndFailsClosedOnIdentityMismatch(t *testing.T) {
	registry, source := registeredKnowledgeRegistry(t)
	jobs := newKnowledgeJobRuntimeStub()
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	service := NewKnowledgeIngestionService(registry, jobs, knowledgeDeploymentForTest(), failServer.URL)
	job, _, err := service.EnqueueVideo(context.Background(), uuid.New(), KnowledgeVideoIngestionRequest{SourceKey: source.SourceKey, VideoPath: source.OriginalFilePath})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.processPending(context.Background(), job.ID); err == nil {
		t.Fatal("expected transient failure")
	}
	if jobs.jobs[job.ID].Status != "pending" || jobs.jobs[job.ID].Attempts != 1 {
		t.Fatalf("transient failure should requeue within attempt budget: %#v", jobs.jobs[job.ID])
	}
	failServer.Close()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"source_id": 42, "source_key": source.SourceKey, "status": "ingested",
			"agent_execution": map[string]any{
				"knowledge_splitter": map[string]any{
					"agent_configuration":  map[string]any{"id": "wrong", "role": "knowledge_splitter", "decision_policy_revision": "splitter-policy-v1", "logical_model": "structured"},
					"execution_provenance": map[string]any{"status": "executed", "logical_model": "structured"},
				},
			},
		})
	}))
	defer badServer.Close()
	jobs2 := newKnowledgeJobRuntimeStub()
	service2 := NewKnowledgeIngestionService(registry, jobs2, knowledgeDeploymentForTest(), badServer.URL)
	job2, _, err := service2.EnqueueVideo(context.Background(), uuid.New(), KnowledgeVideoIngestionRequest{SourceKey: source.SourceKey, VideoPath: source.OriginalFilePath, SplitterProvider: "llm"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service2.processPending(context.Background(), job2.ID); err == nil {
		t.Fatal("expected identity mismatch")
	}
	if jobs2.jobs[job2.ID].Status != "failed" {
		t.Fatalf("identity mismatch must fail closed: %#v", jobs2.jobs[job2.ID])
	}
}

func TestKnowledgeIngestionStaleRunningRequeuesThenTimesOutAtBudget(t *testing.T) {
	registry, source := registeredKnowledgeRegistry(t)
	jobs := newKnowledgeJobRuntimeStub()
	service := NewKnowledgeIngestionService(registry, jobs, knowledgeDeploymentForTest(), "http://unused")
	job, _, err := service.EnqueueVideo(context.Background(), uuid.New(), KnowledgeVideoIngestionRequest{SourceKey: source.SourceKey, VideoPath: source.OriginalFilePath})
	if err != nil {
		t.Fatal(err)
	}
	stored := jobs.jobs[job.ID]
	stored.Status = "running"
	stored.Attempts = 1
	if _, err := service.RecoverJobs(context.Background(), 10, time.Minute); err != nil {
		t.Fatal(err)
	}
	if stored.Status != "pending" {
		t.Fatalf("stale running job should be requeued, got %s", stored.Status)
	}
	stored.Status = "running"
	stored.Attempts = stored.MaxAttempts
	if _, err := service.RecoverJobs(context.Background(), 10, time.Minute); err != nil {
		t.Fatal(err)
	}
	if stored.Status != "timed_out" {
		t.Fatalf("exhausted stale job should time out, got %s", stored.Status)
	}
}
