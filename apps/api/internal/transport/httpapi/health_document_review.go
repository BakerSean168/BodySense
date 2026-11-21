package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
)

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
