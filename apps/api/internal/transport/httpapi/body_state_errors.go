package httpapi

import (
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"gorm.io/gorm"
)

type bodyStatePublicError struct {
	status  int
	code    string
	message string
}

func classifyBodyStatePublicError(err error) bodyStatePublicError {
	switch {
	case errors.Is(err, repository.ErrBodyStateRevisionConflict):
		return bodyStatePublicError{status: http.StatusConflict, code: "BODY_STATE_REVISION_CONFLICT", message: err.Error()}
	case errors.Is(err, service.ErrUnknownBodyRegionID):
		return bodyStatePublicError{status: http.StatusBadRequest, code: "INVALID_BODY_REGION_ID", message: err.Error()}
	case errors.Is(err, service.ErrBodyRegionIDRequired):
		return bodyStatePublicError{status: http.StatusBadRequest, code: "BODY_REGION_ID_REQUIRED", message: err.Error()}
	case errors.Is(err, service.ErrBodyRegionIDValidationUnavailable):
		return bodyStatePublicError{status: http.StatusServiceUnavailable, code: "BODY_REGION_VALIDATION_UNAVAILABLE", message: err.Error()}
	case errors.Is(err, gorm.ErrRecordNotFound):
		return bodyStatePublicError{status: http.StatusNotFound, code: "NOT_FOUND", message: "body state item not found"}
	default:
		return bodyStatePublicError{status: http.StatusInternalServerError, code: "INTERNAL_ERROR", message: "failed to update body state"}
	}
}
