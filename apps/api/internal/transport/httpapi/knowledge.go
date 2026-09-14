package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type knowledgeSourceApplication interface {
	Register(context.Context, uuid.UUID, service.RegisterKnowledgeSourceInput) (*model.KnowledgeSource, error)
	List(context.Context, int) ([]model.KnowledgeSource, error)
}

type knowledgeIngestionApplication interface {
	EnqueueVideo(context.Context, uuid.UUID, service.KnowledgeVideoIngestionRequest) (*model.Job, bool, error)
	GetJob(context.Context, uuid.UUID) (*model.Job, error)
}

type knowledgeQueryApplication interface {
	Search(context.Context, service.KnowledgeSearchRequest) (*service.KnowledgeSearchResponse, error)
	Stats(context.Context) (*service.KnowledgeStats, error)
}

func (s *PublicServer) WithKnowledge(registry knowledgeSourceApplication, ingestion knowledgeIngestionApplication, query knowledgeQueryApplication) *PublicServer {
	s.knowledgeSources = registry
	s.knowledgeIngestion = ingestion
	s.knowledgeQuery = query
	return s
}

type knowledgeHTTPErrorResponse struct {
	status  int
	code    string
	message string
}

func (r knowledgeHTTPErrorResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}
func (r knowledgeHTTPErrorResponse) VisitRegisterKnowledgeSourceResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r knowledgeHTTPErrorResponse) VisitListKnowledgeSourcesResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r knowledgeHTTPErrorResponse) VisitEnqueueKnowledgeVideoIngestionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r knowledgeHTTPErrorResponse) VisitGetKnowledgeIngestionJobResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r knowledgeHTTPErrorResponse) VisitSearchKnowledgeResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r knowledgeHTTPErrorResponse) VisitGetKnowledgeStatsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func knowledgeOperatorID(ctx context.Context) (uuid.UUID, error) {
	text, ok := ctx.Value("knowledge_operator_id").(string)
	if !ok || text == "" {
		return uuid.Nil, errors.New("missing knowledge operator identity")
	}
	return uuid.Parse(text)
}

func knowledgeForbidden() knowledgeHTTPErrorResponse {
	return knowledgeHTTPErrorResponse{status: http.StatusForbidden, code: "FORBIDDEN", message: "knowledge operator permission is required"}
}
func knowledgeUnavailable(code, message string) knowledgeHTTPErrorResponse {
	return knowledgeHTTPErrorResponse{status: http.StatusServiceUnavailable, code: code, message: message}
}
func knowledgeInternal(message string) knowledgeHTTPErrorResponse {
	return knowledgeHTTPErrorResponse{status: http.StatusInternalServerError, code: "INTERNAL_ERROR", message: message}
}

func (s *PublicServer) RegisterKnowledgeSource(ctx context.Context, request openapiv1.RegisterKnowledgeSourceRequestObject) (openapiv1.RegisterKnowledgeSourceResponseObject, error) {
	actorID, err := knowledgeOperatorID(ctx)
	if err != nil {
		return knowledgeForbidden(), nil
	}
	if s.knowledgeSources == nil {
		return knowledgeUnavailable("KNOWLEDGE_REGISTRY_UNAVAILABLE", "knowledge source registry is unavailable"), nil
	}
	if request.Body == nil {
		return knowledgeHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "request body is required"}, nil
	}
	metadata := map[string]any{}
	if request.Body.Metadata != nil {
		metadata = map[string]any(*request.Body.Metadata)
	}
	source, err := s.knowledgeSources.Register(ctx, actorID, service.RegisterKnowledgeSourceInput{
		SourceKey: request.Body.SourceKey, SourceType: request.Body.SourceType, Title: request.Body.Title,
		Author: request.Body.Author, ProblemSlug: request.Body.ProblemSlug, ProblemDisplayName: request.Body.ProblemDisplayName,
		OriginalFilePath: request.Body.OriginalFilePath, Language: stringValue(request.Body.Language),
		LicenseStatus: string(request.Body.LicenseStatus), ContentHash: request.Body.ContentHash,
		CanonicalURL: stringValue(request.Body.CanonicalUrl), SourceVersion: stringValue(request.Body.SourceVersion),
		Provenance: map[string]any(request.Body.Provenance), Metadata: metadata,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrKnowledgeSourceExists), errors.Is(err, service.ErrKnowledgeSourceConflict):
			return knowledgeHTTPErrorResponse{status: http.StatusConflict, code: "KNOWLEDGE_SOURCE_EXISTS", message: "knowledge source identity already exists"}, nil
		case errors.Is(err, service.ErrKnowledgeSourceInputInvalid):
			return knowledgeHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_KNOWLEDGE_SOURCE", message: err.Error()}, nil
		default:
			return knowledgeInternal("failed to register knowledge source"), nil
		}
	}
	projected, err := strictKnowledgeSource(source)
	if err != nil {
		return nil, err
	}
	return openapiv1.RegisterKnowledgeSource201JSONResponse(projected), nil
}

func (s *PublicServer) ListKnowledgeSources(ctx context.Context, _ openapiv1.ListKnowledgeSourcesRequestObject) (openapiv1.ListKnowledgeSourcesResponseObject, error) {
	if _, err := knowledgeOperatorID(ctx); err != nil {
		return knowledgeForbidden(), nil
	}
	if s.knowledgeSources == nil {
		return knowledgeUnavailable("KNOWLEDGE_REGISTRY_UNAVAILABLE", "knowledge source registry is unavailable"), nil
	}
	sources, err := s.knowledgeSources.List(ctx, 100)
	if err != nil {
		return knowledgeInternal("failed to list knowledge sources"), nil
	}
	projected := make([]openapiv1.KnowledgeSource, 0, len(sources))
	for i := range sources {
		item, convertErr := strictKnowledgeSource(&sources[i])
		if convertErr != nil {
			return nil, convertErr
		}
		projected = append(projected, item)
	}
	return openapiv1.ListKnowledgeSources200JSONResponse{Sources: projected}, nil
}

func (s *PublicServer) EnqueueKnowledgeVideoIngestion(ctx context.Context, request openapiv1.EnqueueKnowledgeVideoIngestionRequestObject) (openapiv1.EnqueueKnowledgeVideoIngestionResponseObject, error) {
	actorID, err := knowledgeOperatorID(ctx)
	if err != nil {
		return knowledgeForbidden(), nil
	}
	if s.knowledgeIngestion == nil {
		return knowledgeUnavailable("KNOWLEDGE_INGESTION_UNAVAILABLE", "knowledge ingestion service is unavailable"), nil
	}
	if request.Body == nil {
		return knowledgeHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "request body is required"}, nil
	}
	exportClips := true
	if request.Body.ExportClips != nil {
		exportClips = *request.Body.ExportClips
	}
	job, existed, err := s.knowledgeIngestion.EnqueueVideo(ctx, actorID, service.KnowledgeVideoIngestionRequest{
		SourceKey: request.Body.SourceKey, VideoPath: stringValue(request.Body.VideoPath),
		TranscriptProvider: stringValue(request.Body.TranscriptProvider), TranscriptModel: stringValue(request.Body.TranscriptModel),
		WhisperModel: stringValue(request.Body.WhisperModel), ForceTranscribe: boolValue(request.Body.ForceTranscribe),
		ExportClips: exportClips, SplitterProvider: stringValueEnum(request.Body.SplitterProvider), AIRefine: boolValue(request.Body.AiRefine),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrKnowledgeSourceNotRegistered):
			return knowledgeHTTPErrorResponse{status: http.StatusConflict, code: "KNOWLEDGE_SOURCE_NOT_REGISTERED", message: "knowledge source must be registered before ingestion"}, nil
		case errors.Is(err, service.ErrKnowledgeSourceNotReady):
			return knowledgeHTTPErrorResponse{status: http.StatusConflict, code: "KNOWLEDGE_SOURCE_NOT_READY", message: "knowledge source is not eligible for ingestion"}, nil
		case errors.Is(err, service.ErrKnowledgeIngestionSourceMismatch):
			return knowledgeHTTPErrorResponse{status: http.StatusConflict, code: "KNOWLEDGE_SOURCE_MISMATCH", message: err.Error()}, nil
		case errors.Is(err, service.ErrKnowledgeIngestionUnsafePath):
			return knowledgeHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_VIDEO_PATH", message: "video_path must be a safe relative path under the knowledge source directory"}, nil
		default:
			return knowledgeInternal("failed to enqueue knowledge ingestion"), nil
		}
	}
	projected, err := strictKnowledgeJob(job)
	if err != nil {
		return nil, err
	}
	return openapiv1.EnqueueKnowledgeVideoIngestion202JSONResponse{Job: projected, IdempotentHit: existed}, nil
}

func (s *PublicServer) GetKnowledgeIngestionJob(ctx context.Context, request openapiv1.GetKnowledgeIngestionJobRequestObject) (openapiv1.GetKnowledgeIngestionJobResponseObject, error) {
	if _, err := knowledgeOperatorID(ctx); err != nil {
		return knowledgeForbidden(), nil
	}
	if s.knowledgeIngestion == nil {
		return knowledgeUnavailable("KNOWLEDGE_INGESTION_UNAVAILABLE", "knowledge ingestion service is unavailable"), nil
	}
	job, err := s.knowledgeIngestion.GetJob(ctx, request.JobID)
	if err != nil {
		if errors.Is(err, service.ErrKnowledgeIngestionNotFound) {
			return knowledgeHTTPErrorResponse{status: http.StatusNotFound, code: "KNOWLEDGE_INGESTION_NOT_FOUND", message: "knowledge ingestion job not found"}, nil
		}
		return knowledgeInternal("failed to load knowledge ingestion job"), nil
	}
	projected, err := strictKnowledgeJob(job)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetKnowledgeIngestionJob200JSONResponse(projected), nil
}

func (s *PublicServer) SearchKnowledge(ctx context.Context, request openapiv1.SearchKnowledgeRequestObject) (openapiv1.SearchKnowledgeResponseObject, error) {
	if _, err := knowledgeOperatorID(ctx); err != nil {
		return knowledgeForbidden(), nil
	}
	if s.knowledgeQuery == nil {
		return knowledgeUnavailable("KNOWLEDGE_QUERY_UNAVAILABLE", "knowledge query service is unavailable"), nil
	}
	if request.Body == nil {
		return knowledgeHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "request body is required"}, nil
	}
	topK := 5
	if request.Body.TopK != nil {
		topK = *request.Body.TopK
	}
	response, err := s.knowledgeQuery.Search(ctx, service.KnowledgeSearchRequest{
		Query: request.Body.Query, TopK: topK, ProblemSlug: request.Body.ProblemSlug, UnitType: request.Body.UnitType,
	})
	if err != nil {
		if errors.Is(err, service.ErrKnowledgeQueryUpstream) {
			return knowledgeHTTPErrorResponse{status: http.StatusBadGateway, code: "AI_SERVICE_ERROR", message: "knowledge search service is unavailable"}, nil
		}
		return knowledgeHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: err.Error()}, nil
	}
	projected, err := strictKnowledgeSearchResponse(response)
	if err != nil {
		return nil, err
	}
	return openapiv1.SearchKnowledge200JSONResponse(projected), nil
}

func (s *PublicServer) GetKnowledgeStats(ctx context.Context, _ openapiv1.GetKnowledgeStatsRequestObject) (openapiv1.GetKnowledgeStatsResponseObject, error) {
	if _, err := knowledgeOperatorID(ctx); err != nil {
		return knowledgeForbidden(), nil
	}
	if s.knowledgeQuery == nil {
		return knowledgeUnavailable("KNOWLEDGE_QUERY_UNAVAILABLE", "knowledge query service is unavailable"), nil
	}
	stats, err := s.knowledgeQuery.Stats(ctx)
	if err != nil {
		return knowledgeHTTPErrorResponse{status: http.StatusBadGateway, code: "AI_SERVICE_ERROR", message: "knowledge statistics service is unavailable"}, nil
	}
	projected, err := strictOpenAPIConvert[openapiv1.KnowledgeStats]("KnowledgeStats", stats)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetKnowledgeStats200JSONResponse(projected), nil
}

func strictKnowledgeSource(source *model.KnowledgeSource) (openapiv1.KnowledgeSource, error) {
	if source == nil {
		return openapiv1.KnowledgeSource{}, errors.New("knowledge source is nil")
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return openapiv1.KnowledgeSource{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.KnowledgeSource{}, err
	}
	delete(projected, "registered_by")
	return strictOpenAPIConvert[openapiv1.KnowledgeSource]("KnowledgeSource", projected)
}

func strictKnowledgeJob(job *model.Job) (openapiv1.KnowledgeIngestionJob, error) {
	if job == nil {
		return openapiv1.KnowledgeIngestionJob{}, errors.New("knowledge ingestion job is nil")
	}
	encoded, err := json.Marshal(job)
	if err != nil {
		return openapiv1.KnowledgeIngestionJob{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.KnowledgeIngestionJob{}, err
	}
	for _, key := range []string{"user_id", "run_id", "conversation_id", "input", "idempotency_key", "metadata"} {
		delete(projected, key)
	}
	return strictOpenAPIConvert[openapiv1.KnowledgeIngestionJob]("KnowledgeIngestionJob", projected)
}

func strictKnowledgeSearchResponse(response *service.KnowledgeSearchResponse) (openapiv1.KnowledgeSearchResponse, error) {
	if response == nil {
		return openapiv1.KnowledgeSearchResponse{}, errors.New("knowledge search response is nil")
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return openapiv1.KnowledgeSearchResponse{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.KnowledgeSearchResponse{}, err
	}
	if results, ok := projected["results"].([]any); ok {
		for _, rawResult := range results {
			result, _ := rawResult.(map[string]any)
			clips, _ := result["clips"].([]any)
			for _, rawClip := range clips {
				if clip, ok := rawClip.(map[string]any); ok {
					delete(clip, "file_path")
				}
			}
		}
	}
	return strictOpenAPIConvert[openapiv1.KnowledgeSearchResponse]("KnowledgeSearchResponse", projected)
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func stringValueEnum[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
