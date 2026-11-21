package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/service"
)

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
