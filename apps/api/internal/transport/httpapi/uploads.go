package httpapi

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
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
		// An erased asset is gone for its owner: only its content-free
		// tombstone (and its erasure receipt) remains, so it is not listed.
		if items[i].Availability == model.UserAssetErased {
			continue
		}
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
