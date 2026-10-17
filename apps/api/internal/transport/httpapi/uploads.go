package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type uploadApplication interface {
	UploadFile(context.Context, uuid.UUID, *multipart.FileHeader, string) (*model.UserUploadView, error)
	GetUploads(context.Context, uuid.UUID) ([]model.UserUploadView, error)
	GetUpload(context.Context, uuid.UUID, uuid.UUID) (*model.UserUploadView, error)
	GetPostureAnalysisSummary(context.Context, uuid.UUID) (service.PostureAnalysisSummary, error)
	RemoveSource(context.Context, uuid.UUID, uuid.UUID) (*model.UserAsset, error)
	OpenOwnedAssetSource(context.Context, uuid.UUID, uuid.UUID) (io.ReadCloser, *model.UserAsset, error)
}

type healthDocumentReviewApplication interface {
	CurrentContext(context.Context, uuid.UUID, uuid.UUID) (*service.HealthDocumentReviewContext, error)
	EnsureAssetOwnsRun(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	ListCandidates(context.Context, uuid.UUID, uuid.UUID) ([]service.DocumentIndicatorReviewProjection, error)
	ApplyReview(context.Context, uuid.UUID, service.ReviewRequest) (*service.DocumentIndicatorReviewRecord, error)
}

func (s *PublicServer) WithUploads(uploads uploadApplication, reviews healthDocumentReviewApplication) *PublicServer {
	s.uploads = uploads
	s.healthDocumentReviews = reviews
	return s
}

type uploadHTTPErrorResponse struct {
	status  int
	code    string
	message string
}

func (r uploadHTTPErrorResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}

func (r uploadHTTPErrorResponse) VisitCreateUploadResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitListUploadsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitGetPostureAnalysisResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitGetUploadResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitDeleteUploadResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitDeleteAssetResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitGetHealthDocumentReviewContextResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitListHealthDocumentReviewCandidatesResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitAppendHealthDocumentReviewResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r uploadHTTPErrorResponse) VisitGetHealthDocumentSourceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func uploadUnauthorized() uploadHTTPErrorResponse {
	return uploadHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "authentication required"}
}

func uploadInternal(message string) uploadHTTPErrorResponse {
	return uploadHTTPErrorResponse{status: http.StatusInternalServerError, code: "INTERNAL_ERROR", message: message}
}

// uploadOwnerMaskedNotFound is the single owner-masked absence response: a
// missing upload/asset and a foreign one are indistinguishable.
func uploadOwnerMaskedNotFound(message string) uploadHTTPErrorResponse {
	return uploadHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: message}
}

func (s *PublicServer) CreateUpload(
	ctx context.Context,
	request openapiv1.CreateUploadRequestObject,
) (openapiv1.CreateUploadResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil || request.Body == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "UPLOAD_DOMAIN_UNAVAILABLE", message: "upload service is not configured"}, nil
	}

	form, err := request.Body.ReadForm(16 << 20)
	if err != nil {
		return uploadHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "invalid multipart upload"}, nil
	}
	defer form.RemoveAll()
	files := form.File["file"]
	fileTypes := form.Value["file_type"]
	if len(files) == 0 || files[0] == nil {
		return uploadHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "file is required"}, nil
	}
	if len(fileTypes) == 0 || strings.TrimSpace(fileTypes[0]) == "" {
		return uploadHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "file_type is required"}, nil
	}

	upload, err := s.uploads.UploadFile(ctx, uid, files[0], strings.TrimSpace(fileTypes[0]))
	if err != nil {
		return uploadHTTPErrorResponse{status: http.StatusBadRequest, code: "UPLOAD_REJECTED", message: err.Error()}, nil
	}
	projected, err := strictUserUpload(upload)
	if err != nil {
		return nil, err
	}
	return openapiv1.CreateUpload201JSONResponse(projected), nil
}

func (s *PublicServer) ListUploads(
	ctx context.Context,
	_ openapiv1.ListUploadsRequestObject,
) (openapiv1.ListUploadsResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "UPLOAD_DOMAIN_UNAVAILABLE", message: "upload service is not configured"}, nil
	}
	items, err := s.uploads.GetUploads(ctx, uid)
	if err != nil {
		return uploadInternal("failed to list uploads"), nil
	}
	projected := make([]openapiv1.UserUpload, 0, len(items))
	for i := range items {
		item, convertErr := strictUserUpload(&items[i])
		if convertErr != nil {
			return nil, convertErr
		}
		projected = append(projected, item)
	}
	return openapiv1.ListUploads200JSONResponse{Uploads: projected}, nil
}

func (s *PublicServer) GetUpload(
	ctx context.Context,
	request openapiv1.GetUploadRequestObject,
) (openapiv1.GetUploadResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "UPLOAD_DOMAIN_UNAVAILABLE", message: "upload service is not configured"}, nil
	}
	upload, err := s.uploads.GetUpload(ctx, uid, request.Id)
	if err != nil {
		return ownedUploadError(err, "failed to load upload"), nil
	}
	projected, err := strictUserUpload(upload)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetUpload200JSONResponse(projected), nil
}

// removeSourceResponse projects the surviving asset state after the
// source-removal command. Repeating the command returns the same shape.
func removeSourceResponse(asset *model.UserAsset) (openapiv1.AssetSourceRemovalResponse, error) {
	if asset == nil {
		return openapiv1.AssetSourceRemovalResponse{}, errors.New("asset is nil")
	}
	response := openapiv1.AssetSourceRemovalResponse{
		Availability: openapiv1.AssetSourceRemovalResponseAvailabilityErased,
		Message:      "asset source removed; adopted evidence history retained",
	}
	if asset.Availability != model.UserAssetErased {
		response.Availability = openapiv1.AssetSourceRemovalResponseAvailabilitySourceRemoved
		response.Message = "asset source removed; adopted evidence history retained"
	}
	response.SourceRemovedAt = asset.SourceRemovedAt
	return response, nil
}

// DeleteUpload is the legacy upload-surface alias of the source-removal
// command (PC-D7). It removes the private original and marks the backing
// asset source_removed; it is idempotent and never deletes evidence.
func (s *PublicServer) DeleteUpload(
	ctx context.Context,
	request openapiv1.DeleteUploadRequestObject,
) (openapiv1.DeleteUploadResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "UPLOAD_DOMAIN_UNAVAILABLE", message: "upload service is not configured"}, nil
	}
	asset, err := s.uploads.RemoveSource(ctx, uid, request.Id)
	if err != nil {
		return ownedUploadError(err, "failed to remove upload source"), nil
	}
	response, err := removeSourceResponse(asset)
	if err != nil {
		return nil, err
	}
	return openapiv1.DeleteUpload200JSONResponse(response), nil
}

// DeleteAsset executes the source-removal command on the sole asset
// identity: DELETE /api/v1/assets/{id}. Owner-masked 404 for missing and
// foreign assets; idempotent for already-removed sources.
func (s *PublicServer) DeleteAsset(
	ctx context.Context,
	request openapiv1.DeleteAssetRequestObject,
) (openapiv1.DeleteAssetResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "UPLOAD_DOMAIN_UNAVAILABLE", message: "upload service is not configured"}, nil
	}
	asset, err := s.uploads.RemoveSource(ctx, uid, request.Id)
	if err != nil {
		return ownedUploadError(err, "failed to remove asset source"), nil
	}
	response, err := removeSourceResponse(asset)
	if err != nil {
		return nil, err
	}
	return openapiv1.DeleteAsset200JSONResponse(response), nil
}

func (s *PublicServer) GetPostureAnalysis(
	ctx context.Context,
	_ openapiv1.GetPostureAnalysisRequestObject,
) (openapiv1.GetPostureAnalysisResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "UPLOAD_DOMAIN_UNAVAILABLE", message: "upload service is not configured"}, nil
	}
	summary, err := s.uploads.GetPostureAnalysisSummary(ctx, uid)
	if err != nil {
		return uploadInternal("failed to load posture analysis"), nil
	}
	projected, err := strictOpenAPIConvert[openapiv1.PostureAnalysisSummary]("PostureAnalysisSummary", summary)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetPostureAnalysis200JSONResponse(projected), nil
}

func (s *PublicServer) GetHealthDocumentReviewContext(
	ctx context.Context,
	request openapiv1.GetHealthDocumentReviewContextRequestObject,
) (openapiv1.GetHealthDocumentReviewContextResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.healthDocumentReviews == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "REVIEW_DOMAIN_UNAVAILABLE", message: "health document review service is not configured"}, nil
	}
	value, err := s.healthDocumentReviews.CurrentContext(ctx, uid, request.Id)
	if err != nil {
		if errors.Is(err, service.ErrReviewContextUnavailable) || errors.Is(err, service.ErrReviewAccessDenied) {
			return uploadOwnerMaskedNotFound("review context unavailable"), nil
		}
		return uploadInternal("failed to load review context"), nil
	}
	projected, err := strictHealthDocumentReviewContext(value)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetHealthDocumentReviewContext200JSONResponse(projected), nil
}

// reviewTargetGuard maps the ownership guard of the extraction endpoints to
// owner-masked responses: neither a foreign run nor a wrong asset binding may
// reveal whether the target exists.
func reviewTargetGuard(err error) uploadHTTPErrorResponse {
	switch {
	case errors.Is(err, service.ErrReviewAccessDenied):
		return uploadOwnerMaskedNotFound("review target not found")
	case errors.Is(err, service.ErrReviewCandidateMismatch):
		return uploadOwnerMaskedNotFound("review target not found")
	default:
		return uploadInternal("failed to verify review target")
	}
}

func (s *PublicServer) ListHealthDocumentReviewCandidates(
	ctx context.Context,
	request openapiv1.ListHealthDocumentReviewCandidatesRequestObject,
) (openapiv1.ListHealthDocumentReviewCandidatesResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.healthDocumentReviews == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "REVIEW_DOMAIN_UNAVAILABLE", message: "health document review service is not configured"}, nil
	}
	if err := s.healthDocumentReviews.EnsureAssetOwnsRun(ctx, uid, request.Id, request.RunId); err != nil {
		return reviewTargetGuard(err), nil
	}
	items, err := s.healthDocumentReviews.ListCandidates(ctx, uid, request.RunId)
	if err != nil {
		if errors.Is(err, service.ErrReviewAccessDenied) {
			return uploadOwnerMaskedNotFound("extraction run not found"), nil
		}
		return uploadInternal("failed to load review candidates"), nil
	}
	projected, err := strictReviewProjectionList(items)
	if err != nil {
		return nil, err
	}
	return openapiv1.ListHealthDocumentReviewCandidates200JSONResponse{ReviewCandidates: projected}, nil
}

func (s *PublicServer) AppendHealthDocumentReview(
	ctx context.Context,
	request openapiv1.AppendHealthDocumentReviewRequestObject,
) (openapiv1.AppendHealthDocumentReviewResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.healthDocumentReviews == nil || request.Body == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "REVIEW_DOMAIN_UNAVAILABLE", message: "health document review service is not configured"}, nil
	}
	if err := s.healthDocumentReviews.EnsureAssetOwnsRun(ctx, uid, request.Id, request.RunId); err != nil {
		return reviewTargetGuard(err), nil
	}
	var reviewedPayload json.RawMessage
	if request.Body.ReviewedPayload != nil {
		reviewedPayload, err = json.Marshal(*request.Body.ReviewedPayload)
		if err != nil {
			return uploadHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "reviewed_payload is invalid"}, nil
		}
	}
	record, err := s.healthDocumentReviews.ApplyReview(ctx, uid, service.ReviewRequest{
		ExtractionRunID: request.RunId,
		IndicatorIndex:  request.Body.IndicatorIndex,
		IndicatorID:     request.Body.IndicatorId,
		Action:          string(request.Body.Action),
		ReviewedPayload: reviewedPayload,
		SourceRefs:      append([]string(nil), request.Body.SourceRefs...),
		IdempotencyKey:  request.Body.IdempotencyKey,
		Note:            stringValue(request.Body.Note),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrReviewAccessDenied):
			return uploadOwnerMaskedNotFound("review target not found"), nil
		case errors.Is(err, service.ErrReviewCandidateMismatch):
			return uploadHTTPErrorResponse{status: http.StatusConflict, code: "REVIEW_CANDIDATE_STALE", message: "review candidate is stale; reload the extraction run"}, nil
		case errors.Is(err, service.ErrReviewDuplicateConflict):
			return uploadHTTPErrorResponse{status: http.StatusConflict, code: "IDEMPOTENCY_CONFLICT", message: "idempotency key reused with different review content"}, nil
		case errors.Is(err, service.ErrReviewValidation):
			return uploadHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: err.Error()}, nil
		default:
			return uploadInternal("failed to append review"), nil
		}
	}
	projected, err := strictReviewRecord(record)
	if err != nil {
		return nil, err
	}
	return openapiv1.AppendHealthDocumentReview201JSONResponse(projected), nil
}

type healthDocumentSourceResponse struct {
	body   io.ReadCloser
	length int64
	mime   string
}

func (r healthDocumentSourceResponse) VisitGetHealthDocumentSourceResponse(w http.ResponseWriter) error {
	if r.body == nil {
		return errors.New("upload source stream is unavailable")
	}
	defer r.body.Close()
	mimeType := strings.TrimSpace(r.mime)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.length > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(r.length, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, err := io.Copy(w, r.body)
	return err
}

// GetHealthDocumentSource streams the authenticated private source document
// for one exact owned extraction run. After source removal the original is
// not readable anymore (CD-D4): the endpoint fails closed with 404 while the
// review context keeps serving the historical evidence with its
// source-unavailable marker.
func (s *PublicServer) GetHealthDocumentSource(
	ctx context.Context,
	request openapiv1.GetHealthDocumentSourceRequestObject,
) (openapiv1.GetHealthDocumentSourceResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil || s.healthDocumentReviews == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "REVIEW_DOMAIN_UNAVAILABLE", message: "health document source service is not configured"}, nil
	}
	if err := s.healthDocumentReviews.EnsureAssetOwnsRun(ctx, uid, request.Id, request.RunId); err != nil {
		return reviewTargetGuard(err), nil
	}
	reader, asset, err := s.uploads.OpenOwnedAssetSource(ctx, uid, request.Id)
	if err != nil {
		return ownedUploadError(err, "failed to open upload source"), nil
	}
	return healthDocumentSourceResponse{body: reader, length: asset.ByteSize, mime: asset.MimeType}, nil
}

// ownedUploadError maps service-layer absence/unavailability errors to
// owner-masked responses. Only ErrUploadNotFound and
// ErrUploadSourceUnavailable are 404; anything else is an internal failure.
func ownedUploadError(err error, operation string) uploadHTTPErrorResponse {
	if err == nil {
		return uploadInternal(operation)
	}
	switch {
	case errors.Is(err, service.ErrUploadNotFound):
		return uploadOwnerMaskedNotFound("upload not found")
	case errors.Is(err, service.ErrUploadSourceUnavailable):
		return uploadHTTPErrorResponse{status: http.StatusNotFound, code: "SOURCE_UNAVAILABLE", message: "upload source is unavailable"}
	default:
		return uploadInternal(operation)
	}
}

func strictUserUpload(upload *model.UserUploadView) (openapiv1.UserUpload, error) {
	if upload == nil {
		return openapiv1.UserUpload{}, errors.New("upload is nil")
	}
	encoded, err := json.Marshal(upload)
	if err != nil {
		return openapiv1.UserUpload{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.UserUpload{}, err
	}
	// The view already hides user_id, availability and storage identity;
	// the deletes keep the guard explicit against future field additions.
	delete(projected, "user_id")
	delete(projected, "availability")
	delete(projected, "agent_configuration_id")
	delete(projected, "storage_backend")
	delete(projected, "storage_key")
	return strictOpenAPIConvert[openapiv1.UserUpload]("UserUpload", projected)
}

func strictReviewRecord(record *service.DocumentIndicatorReviewRecord) (openapiv1.DocumentIndicatorReviewRecord, error) {
	if record == nil {
		return openapiv1.DocumentIndicatorReviewRecord{}, errors.New("review record is nil")
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return openapiv1.DocumentIndicatorReviewRecord{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.DocumentIndicatorReviewRecord{}, err
	}
	delete(projected, "reviewer_user_id")
	return strictOpenAPIConvert[openapiv1.DocumentIndicatorReviewRecord]("DocumentIndicatorReviewRecord", projected)
}

func strictReviewProjectionList(items []service.DocumentIndicatorReviewProjection) ([]openapiv1.DocumentIndicatorReviewProjection, error) {
	projected := make([]openapiv1.DocumentIndicatorReviewProjection, 0, len(items))
	for i := range items {
		item, err := strictReviewProjection(&items[i])
		if err != nil {
			return nil, err
		}
		projected = append(projected, item)
	}
	return projected, nil
}

func strictReviewProjection(value *service.DocumentIndicatorReviewProjection) (openapiv1.DocumentIndicatorReviewProjection, error) {
	if value == nil {
		return openapiv1.DocumentIndicatorReviewProjection{}, errors.New("review projection is nil")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return openapiv1.DocumentIndicatorReviewProjection{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.DocumentIndicatorReviewProjection{}, err
	}
	stripReviewRecordIdentity(projected["effective_review"])
	if history, ok := projected["history"].([]any); ok {
		for _, item := range history {
			stripReviewRecordIdentity(item)
		}
	}
	return strictOpenAPIConvert[openapiv1.DocumentIndicatorReviewProjection]("DocumentIndicatorReviewProjection", projected)
}

func strictHealthDocumentReviewContext(value *service.HealthDocumentReviewContext) (openapiv1.HealthDocumentReviewContext, error) {
	if value == nil {
		return openapiv1.HealthDocumentReviewContext{}, errors.New("review context is nil")
	}
	projected, err := strictReviewProjectionList(value.ReviewCandidates)
	if err != nil {
		return openapiv1.HealthDocumentReviewContext{}, err
	}
	return openapiv1.HealthDocumentReviewContext{
		ExtractionRunId:    value.ExtractionRunID,
		UploadId:           value.AssetID,
		SourceAvailability: openapiv1.HealthDocumentReviewContextSourceAvailability(value.SourceAvailability),
		ReviewCandidates:   projected,
	}, nil
}

func stripReviewRecordIdentity(value any) {
	if record, ok := value.(map[string]any); ok {
		delete(record, "reviewer_user_id")
	}
}
