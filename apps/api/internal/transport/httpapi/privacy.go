package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/auth"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

const privacyErasureAcceptedMessage = "data deletion has been accepted and will continue even if this browser disconnects"

type privacyErasureApplication interface {
	Plan(context.Context, uuid.UUID) (*service.PrivacyErasurePlan, error)
	Request(context.Context, uuid.UUID, string) (*model.PrivacyErasureRequest, error)
}

type privacyRefreshCookiePolicy struct {
	name   string
	secure bool
}

func (s *PublicServer) WithPrivacy(
	privacy privacyErasureApplication,
	refreshCookieName string,
	refreshCookieSecure bool,
) *PublicServer {
	s.privacy = privacy
	s.privacyCookie = privacyRefreshCookiePolicy{name: refreshCookieName, secure: refreshCookieSecure}
	return s
}

func (s *PublicServer) GetPrivacyErasurePlan(
	ctx context.Context,
	_ openapiv1.GetPrivacyErasurePlanRequestObject,
) (openapiv1.GetPrivacyErasurePlanResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getPrivacyErasurePlanError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.privacy == nil {
		return getPrivacyErasurePlanError(http.StatusInternalServerError, "PRIVACY_PLAN_FAILED", "unable to prepare data deletion plan"), nil
	}
	plan, err := s.privacy.Plan(ctx, userID)
	if err != nil {
		return getPrivacyErasurePlanError(http.StatusInternalServerError, "PRIVACY_PLAN_FAILED", "unable to prepare data deletion plan"), nil
	}
	response, err := strictJSONConvert[openapiv1.PrivacyErasurePlan](plan)
	if err != nil {
		return getPrivacyErasurePlanError(http.StatusInternalServerError, "INTERNAL_ERROR", "privacy erasure plan violates the public contract"), nil
	}
	noStore := "no-store"
	return openapiv1.GetPrivacyErasurePlan200JSONResponse{
		Body: response,
		Headers: openapiv1.GetPrivacyErasurePlan200ResponseHeaders{
			CacheControl: &noStore,
		},
	}, nil
}

func (s *PublicServer) RequestPrivacyErasure(
	ctx context.Context,
	request openapiv1.RequestPrivacyErasureRequestObject,
) (openapiv1.RequestPrivacyErasureResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return requestPrivacyErasureError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.privacy == nil || request.Body == nil {
		return requestPrivacyErasureError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	confirmation := string(request.Body.Confirmation)
	erasure, err := s.privacy.Request(ctx, userID, confirmation)
	if err != nil {
		if errors.Is(err, service.ErrPrivacyErasureConfirmation) {
			return requestPrivacyErasureError(http.StatusBadRequest, "CONFIRMATION_MISMATCH", "confirmation phrase does not match"), nil
		}
		return requestPrivacyErasureError(http.StatusInternalServerError, "PRIVACY_ERASURE_FAILED", "unable to persist data deletion request"), nil
	}
	if erasure == nil {
		return requestPrivacyErasureError(http.StatusInternalServerError, "PRIVACY_ERASURE_FAILED", "unable to persist data deletion request"), nil
	}
	status := openapiv1.PrivacyErasureAcceptedStatus(erasure.Status)
	if !status.Valid() {
		return requestPrivacyErasureError(http.StatusInternalServerError, "INTERNAL_ERROR", "privacy erasure status violates the public contract"), nil
	}

	noStore := "no-store"
	clearCookie := auth.NewClearedRefreshCookie(s.privacyCookie.name, s.privacyCookie.secure).String()
	return openapiv1.RequestPrivacyErasure202JSONResponse{
		Body: openapiv1.PrivacyErasureAccepted{
			RequestId: erasure.ID,
			Status:    status,
			Message:   privacyErasureAcceptedMessage,
		},
		Headers: openapiv1.RequestPrivacyErasure202ResponseHeaders{
			CacheControl: &noStore,
			SetCookie:    &clearCookie,
		},
	}, nil
}

func getPrivacyErasurePlanError(status int, code, message string) openapiv1.GetPrivacyErasurePlanResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.GetPrivacyErasurePlan401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.GetPrivacyErasurePlan500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}

func requestPrivacyErasureError(status int, code, message string) openapiv1.RequestPrivacyErasureResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.RequestPrivacyErasure400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.RequestPrivacyErasure401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	default:
		return openapiv1.RequestPrivacyErasure500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
