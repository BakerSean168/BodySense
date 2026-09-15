package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type lifestyleApplication interface {
	Get(context.Context, uuid.UUID) (*service.LifestyleSnapshot, error)
	Update(context.Context, uuid.UUID, service.UpdateLifestyleRequest) (*service.LifestyleSnapshot, error)
	AcceptCandidate(context.Context, uuid.UUID, *int64, uuid.UUID) (*service.LifestyleSnapshot, error)
	RejectCandidate(context.Context, uuid.UUID, *int64, uuid.UUID) (*service.LifestyleSnapshot, error)
}

type bodyMetricsApplication interface {
	Get(context.Context, uuid.UUID) (*service.BodyMetricsSnapshot, error)
	Update(context.Context, uuid.UUID, service.UpdateBodyMetricsRequest) (*service.BodyMetricsSnapshot, error)
}

type healthHistoryApplication interface {
	GetInjuryHistory(context.Context, uuid.UUID) (*service.InjuryHistorySnapshot, error)
	UpdateInjuryHistory(context.Context, uuid.UUID, service.UpdateInjuryHistoryRequest) (*service.InjuryHistorySnapshot, error)
}

type onboardingContextApplication interface {
	Submit(context.Context, uuid.UUID, service.OnboardingContextRequest) (*service.OnboardingContextResult, error)
}

func (s *PublicServer) WithHealthContext(
	lifestyle lifestyleApplication,
	bodyMetrics bodyMetricsApplication,
	healthHistory healthHistoryApplication,
	onboarding onboardingContextApplication,
) *PublicServer {
	s.lifestyle = lifestyle
	s.bodyMetrics = bodyMetrics
	s.healthHistory = healthHistory
	s.onboarding = onboarding
	return s
}

func (s *PublicServer) GetLifestyle(
	ctx context.Context,
	_ openapiv1.GetLifestyleRequestObject,
) (openapiv1.GetLifestyleResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getLifestyleError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.lifestyle == nil {
		return getLifestyleError(http.StatusInternalServerError, "INTERNAL_ERROR", "lifestyle service is unavailable"), nil
	}
	result, err := s.lifestyle.Get(ctx, userID)
	if err != nil {
		return getLifestyleError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load lifestyle context"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.LifestyleSnapshot]("LifestyleSnapshot", result)
	if err != nil {
		return getLifestyleError(http.StatusInternalServerError, "INTERNAL_ERROR", "lifestyle projection violates the public contract"), nil
	}
	return openapiv1.GetLifestyle200JSONResponse(response), nil
}

func (s *PublicServer) UpdateLifestyle(
	ctx context.Context,
	request openapiv1.UpdateLifestyleRequestObject,
) (openapiv1.UpdateLifestyleResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return updateLifestyleError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.lifestyle == nil || request.Body == nil {
		return updateLifestyleError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	command, err := strictJSONConvert[service.UpdateLifestyleRequest](request.Body)
	if err != nil {
		return updateLifestyleError(http.StatusBadRequest, "INVALID_REQUEST", "invalid lifestyle request"), nil
	}
	result, err := s.lifestyle.Update(ctx, userID, command)
	if err != nil {
		classified := classifyHealthContextPublicError(err)
		return updateLifestyleError(classified.status, classified.code, classified.message), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.LifestyleSnapshot]("LifestyleSnapshot", result)
	if err != nil {
		return updateLifestyleError(http.StatusInternalServerError, "INTERNAL_ERROR", "lifestyle projection violates the public contract"), nil
	}
	return openapiv1.UpdateLifestyle200JSONResponse(response), nil
}

func (s *PublicServer) AcceptLifestyleCandidate(
	ctx context.Context,
	request openapiv1.AcceptLifestyleCandidateRequestObject,
) (openapiv1.AcceptLifestyleCandidateResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return acceptLifestyleCandidateError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.lifestyle == nil || request.Body == nil {
		return acceptLifestyleCandidateError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	expected := request.Body.ExpectedRevision
	result, err := s.lifestyle.AcceptCandidate(ctx, userID, &expected, request.Id)
	if err != nil {
		classified := classifyHealthContextPublicError(err)
		return acceptLifestyleCandidateError(classified.status, classified.code, classified.message), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.LifestyleSnapshot]("LifestyleSnapshot", result)
	if err != nil {
		return acceptLifestyleCandidateError(http.StatusInternalServerError, "INTERNAL_ERROR", "lifestyle projection violates the public contract"), nil
	}
	return openapiv1.AcceptLifestyleCandidate200JSONResponse(response), nil
}

func (s *PublicServer) RejectLifestyleCandidate(
	ctx context.Context,
	request openapiv1.RejectLifestyleCandidateRequestObject,
) (openapiv1.RejectLifestyleCandidateResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return rejectLifestyleCandidateError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.lifestyle == nil || request.Body == nil {
		return rejectLifestyleCandidateError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	expected := request.Body.ExpectedRevision
	result, err := s.lifestyle.RejectCandidate(ctx, userID, &expected, request.Id)
	if err != nil {
		classified := classifyHealthContextPublicError(err)
		return rejectLifestyleCandidateError(classified.status, classified.code, classified.message), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.LifestyleSnapshot]("LifestyleSnapshot", result)
	if err != nil {
		return rejectLifestyleCandidateError(http.StatusInternalServerError, "INTERNAL_ERROR", "lifestyle projection violates the public contract"), nil
	}
	return openapiv1.RejectLifestyleCandidate200JSONResponse(response), nil
}

func (s *PublicServer) GetBodyMetrics(
	ctx context.Context,
	_ openapiv1.GetBodyMetricsRequestObject,
) (openapiv1.GetBodyMetricsResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getBodyMetricsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyMetrics == nil {
		return getBodyMetricsError(http.StatusInternalServerError, "INTERNAL_ERROR", "body metrics service is unavailable"), nil
	}
	result, err := s.bodyMetrics.Get(ctx, userID)
	if err != nil {
		return getBodyMetricsError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load body metrics"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.BodyMetricsSnapshot]("BodyMetricsSnapshot", result)
	if err != nil {
		return getBodyMetricsError(http.StatusInternalServerError, "INTERNAL_ERROR", "body metrics projection violates the public contract"), nil
	}
	return openapiv1.GetBodyMetrics200JSONResponse(response), nil
}

func (s *PublicServer) UpdateBodyMetrics(
	ctx context.Context,
	request openapiv1.UpdateBodyMetricsRequestObject,
) (openapiv1.UpdateBodyMetricsResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return updateBodyMetricsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyMetrics == nil || request.Body == nil {
		return updateBodyMetricsError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	command, err := strictJSONConvert[service.UpdateBodyMetricsRequest](request.Body)
	if err != nil {
		return updateBodyMetricsError(http.StatusBadRequest, "INVALID_REQUEST", "invalid body metrics request"), nil
	}
	result, err := s.bodyMetrics.Update(ctx, userID, command)
	if err != nil {
		classified := classifyHealthContextPublicError(err)
		return updateBodyMetricsError(classified.status, classified.code, classified.message), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.BodyMetricsSnapshot]("BodyMetricsSnapshot", result)
	if err != nil {
		return updateBodyMetricsError(http.StatusInternalServerError, "INTERNAL_ERROR", "body metrics projection violates the public contract"), nil
	}
	return openapiv1.UpdateBodyMetrics200JSONResponse(response), nil
}

func (s *PublicServer) GetInjuryHistory(
	ctx context.Context,
	_ openapiv1.GetInjuryHistoryRequestObject,
) (openapiv1.GetInjuryHistoryResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getInjuryHistoryError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.healthHistory == nil {
		return getInjuryHistoryError(http.StatusInternalServerError, "INTERNAL_ERROR", "health history service is unavailable"), nil
	}
	result, err := s.healthHistory.GetInjuryHistory(ctx, userID)
	if err != nil {
		return getInjuryHistoryError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load injury history"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.InjuryHistorySnapshot]("InjuryHistorySnapshot", result)
	if err != nil {
		return getInjuryHistoryError(http.StatusInternalServerError, "INTERNAL_ERROR", "injury history projection violates the public contract"), nil
	}
	return openapiv1.GetInjuryHistory200JSONResponse(response), nil
}

func (s *PublicServer) UpdateInjuryHistory(
	ctx context.Context,
	request openapiv1.UpdateInjuryHistoryRequestObject,
) (openapiv1.UpdateInjuryHistoryResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return updateInjuryHistoryError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.healthHistory == nil || request.Body == nil {
		return updateInjuryHistoryError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	command, err := strictJSONConvert[service.UpdateInjuryHistoryRequest](request.Body)
	if err != nil {
		return updateInjuryHistoryError(http.StatusBadRequest, "INVALID_REQUEST", "invalid injury history request"), nil
	}
	result, err := s.healthHistory.UpdateInjuryHistory(ctx, userID, command)
	if err != nil {
		classified := classifyHealthContextPublicError(err)
		return updateInjuryHistoryError(classified.status, classified.code, classified.message), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.InjuryHistorySnapshot]("InjuryHistorySnapshot", result)
	if err != nil {
		return updateInjuryHistoryError(http.StatusInternalServerError, "INTERNAL_ERROR", "injury history projection violates the public contract"), nil
	}
	return openapiv1.UpdateInjuryHistory200JSONResponse(response), nil
}

func (s *PublicServer) SubmitOnboardingContext(
	ctx context.Context,
	request openapiv1.SubmitOnboardingContextRequestObject,
) (openapiv1.SubmitOnboardingContextResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return submitOnboardingContextError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.onboarding == nil || request.Body == nil {
		return submitOnboardingContextError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	command, err := strictJSONConvert[service.OnboardingContextRequest](request.Body)
	if err != nil {
		return submitOnboardingContextError(http.StatusBadRequest, "INVALID_REQUEST", "invalid onboarding context"), nil
	}
	result, err := s.onboarding.Submit(ctx, userID, command)
	if err != nil {
		classified := classifyHealthContextPublicError(err)
		return submitOnboardingContextError(classified.status, classified.code, classified.message), nil
	}
	if result == nil || result.BodyStateRevision == nil {
		return submitOnboardingContextError(http.StatusInternalServerError, "INTERNAL_ERROR", "onboarding did not create a BodyState revision"), nil
	}
	return openapiv1.SubmitOnboardingContext200JSONResponse{
		BodyStateRevision: *result.BodyStateRevision,
	}, nil
}

func classifyHealthContextPublicError(err error) bodyStatePublicError {
	switch {
	case errors.Is(err, service.ErrInvalidLifestyleCandidate):
		return bodyStatePublicError{status: http.StatusNotFound, code: "LIFESTYLE_CANDIDATE_NOT_FOUND", message: "lifestyle candidate is not reviewable"}
	case errors.Is(err, service.ErrInvalidBodyMetric):
		return bodyStatePublicError{status: http.StatusBadRequest, code: "INVALID_BODY_METRIC", message: "height or weight is outside the supported range"}
	case errors.Is(err, service.ErrInvalidOnboardingContext):
		return bodyStatePublicError{status: http.StatusBadRequest, code: "INVALID_ONBOARDING_CONTEXT", message: err.Error()}
	default:
		return classifyBodyStatePublicError(err)
	}
}

func getLifestyleError(status int, code, message string) openapiv1.GetLifestyleResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.GetLifestyle401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.GetLifestyle500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}
func updateLifestyleError(status int, code, message string) openapiv1.UpdateLifestyleResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UpdateLifestyle400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.UpdateLifestyle401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.UpdateLifestyle409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	default:
		return openapiv1.UpdateLifestyle500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
func acceptLifestyleCandidateError(status int, code, message string) openapiv1.AcceptLifestyleCandidateResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.AcceptLifestyleCandidate400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.AcceptLifestyleCandidate401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.AcceptLifestyleCandidate404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.AcceptLifestyleCandidate409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	default:
		return openapiv1.AcceptLifestyleCandidate500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
func rejectLifestyleCandidateError(status int, code, message string) openapiv1.RejectLifestyleCandidateResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.RejectLifestyleCandidate400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.RejectLifestyleCandidate401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.RejectLifestyleCandidate404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.RejectLifestyleCandidate409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	default:
		return openapiv1.RejectLifestyleCandidate500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
func getBodyMetricsError(status int, code, message string) openapiv1.GetBodyMetricsResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.GetBodyMetrics401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.GetBodyMetrics500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}
func updateBodyMetricsError(status int, code, message string) openapiv1.UpdateBodyMetricsResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UpdateBodyMetrics400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.UpdateBodyMetrics401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.UpdateBodyMetrics409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	default:
		return openapiv1.UpdateBodyMetrics500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
func getInjuryHistoryError(status int, code, message string) openapiv1.GetInjuryHistoryResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.GetInjuryHistory401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.GetInjuryHistory500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}
func updateInjuryHistoryError(status int, code, message string) openapiv1.UpdateInjuryHistoryResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UpdateInjuryHistory400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.UpdateInjuryHistory401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.UpdateInjuryHistory409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	default:
		return openapiv1.UpdateInjuryHistory500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
func submitOnboardingContextError(status int, code, message string) openapiv1.SubmitOnboardingContextResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.SubmitOnboardingContext400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.SubmitOnboardingContext401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.SubmitOnboardingContext409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	default:
		return openapiv1.SubmitOnboardingContext500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
