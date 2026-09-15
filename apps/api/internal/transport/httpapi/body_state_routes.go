package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type bodyStateRouteService interface {
	GetSnapshot(context.Context, uuid.UUID, int) (*service.BodyStateSnapshot, error)
	CorrectFact(context.Context, uuid.UUID, *int64, uuid.UUID, model.BodyStateFact) (*model.BodyStateFact, *model.BodyStateRevision, error)
	UpdateFactTemporal(context.Context, uuid.UUID, *int64, uuid.UUID, string, string, *time.Time) (*model.BodyStateFact, *model.BodyStateRevision, error)
	ReviewFact(context.Context, uuid.UUID, *int64, uuid.UUID, string) (*model.BodyStateFact, *model.BodyStateRevision, error)
	AddObservation(context.Context, uuid.UUID, *int64, model.BodyStateObservation) (*model.BodyStateObservation, *model.BodyStateRevision, error)
	ReviewObservation(context.Context, uuid.UUID, *int64, uuid.UUID, string) (*model.BodyStateObservation, *model.BodyStateRevision, error)
	AddHypothesis(context.Context, uuid.UUID, *int64, model.BodyStateHypothesis) (*model.BodyStateHypothesis, *model.BodyStateRevision, error)
	UpdateHypothesisLifecycle(context.Context, uuid.UUID, *int64, uuid.UUID, string, json.RawMessage) (*model.BodyStateHypothesis, *model.BodyStateRevision, error)
	ListEvidence(context.Context, uuid.UUID, int) ([]model.BodyStateEvidence, error)
	ResolveSafetyState(context.Context, uuid.UUID, *int64, string, string) (*model.BodyStateRevision, error)
}

func (s *PublicServer) WithBodyStateRoutes(routes bodyStateRouteService) *PublicServer {
	s.bodyStateRoutes = routes
	return s
}

func (s *PublicServer) GetBodyState(
	ctx context.Context,
	_ openapiv1.GetBodyStateRequestObject,
) (openapiv1.GetBodyStateResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getBodyStateError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil {
		return getBodyStateError(http.StatusInternalServerError, "INTERNAL_ERROR", "body state service is unavailable"), nil
	}
	snapshot, err := s.bodyStateRoutes.GetSnapshot(ctx, userID, 50)
	if err != nil {
		return getBodyStateError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load body state"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.BodyStateSnapshot]("BodyStateSnapshot", snapshot)
	if err != nil {
		return getBodyStateError(http.StatusInternalServerError, "INTERNAL_ERROR", "body state violates the public contract"), nil
	}
	return openapiv1.GetBodyState200JSONResponse(response), nil
}

func (s *PublicServer) CorrectBodyStateFact(
	ctx context.Context,
	request openapiv1.CorrectBodyStateFactRequestObject,
) (openapiv1.CorrectBodyStateFactResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return correctBodyStateFactError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil || request.Body == nil {
		return correctBodyStateFactError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	replacement, err := factFromOpenAPI(request.Body.Replacement)
	if err != nil {
		return correctBodyStateFactError(http.StatusBadRequest, "INVALID_REQUEST", "replacement JSON fields are invalid"), nil
	}
	expected := request.Body.ExpectedRevision
	fact, revision, err := s.bodyStateRoutes.CorrectFact(ctx, userID, &expected, request.Id, replacement)
	if err != nil {
		return correctBodyStateFactClassifiedError(err), nil
	}
	response, err := factMutationResponseToOpenAPI(fact, revision)
	if err != nil {
		return correctBodyStateFactError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state response"), nil
	}
	return openapiv1.CorrectBodyStateFact200JSONResponse(response), nil
}

func (s *PublicServer) ReviewBodyStateFact(
	ctx context.Context,
	request openapiv1.ReviewBodyStateFactRequestObject,
) (openapiv1.ReviewBodyStateFactResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return reviewBodyStateFactError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil || request.Body == nil {
		return reviewBodyStateFactError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	expected := request.Body.ExpectedRevision
	fact, revision, err := s.bodyStateRoutes.ReviewFact(ctx, userID, &expected, request.Id, string(request.Body.ReviewState))
	if err != nil {
		return reviewBodyStateFactClassifiedError(err), nil
	}
	response, err := factMutationResponseToOpenAPI(fact, revision)
	if err != nil {
		return reviewBodyStateFactError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state response"), nil
	}
	return openapiv1.ReviewBodyStateFact200JSONResponse(response), nil
}

func (s *PublicServer) UpdateBodyStateFactTemporal(
	ctx context.Context,
	request openapiv1.UpdateBodyStateFactTemporalRequestObject,
) (openapiv1.UpdateBodyStateFactTemporalResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return updateBodyStateFactTemporalError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil || request.Body == nil {
		return updateBodyStateFactTemporalError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	expected := request.Body.ExpectedRevision
	fact, revision, err := s.bodyStateRoutes.UpdateFactTemporal(
		ctx,
		userID,
		&expected,
		request.Id,
		stringValue(request.Body.LifecycleState),
		stringValue(request.Body.Trend),
		request.Body.ValidUntil,
	)
	if err != nil {
		return updateBodyStateFactTemporalClassifiedError(err), nil
	}
	response, err := factMutationResponseToOpenAPI(fact, revision)
	if err != nil {
		return updateBodyStateFactTemporalError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state response"), nil
	}
	return openapiv1.UpdateBodyStateFactTemporal200JSONResponse(response), nil
}

func (s *PublicServer) AddBodyStateObservation(
	ctx context.Context,
	request openapiv1.AddBodyStateObservationRequestObject,
) (openapiv1.AddBodyStateObservationResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return addBodyStateObservationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil || request.Body == nil {
		return addBodyStateObservationError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	observation, err := observationFromOpenAPI(*request.Body)
	if err != nil {
		return addBodyStateObservationError(http.StatusBadRequest, "INVALID_REQUEST", "observation JSON fields are invalid"), nil
	}
	expected := request.Body.ExpectedRevision
	stored, revision, err := s.bodyStateRoutes.AddObservation(ctx, userID, &expected, observation)
	if err != nil {
		return addBodyStateObservationClassifiedError(err), nil
	}
	response, err := observationMutationResponseToOpenAPI(stored, revision)
	if err != nil {
		return addBodyStateObservationError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state response"), nil
	}
	return openapiv1.AddBodyStateObservation200JSONResponse(response), nil
}

func (s *PublicServer) ReviewBodyStateObservation(
	ctx context.Context,
	request openapiv1.ReviewBodyStateObservationRequestObject,
) (openapiv1.ReviewBodyStateObservationResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return reviewBodyStateObservationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil || request.Body == nil {
		return reviewBodyStateObservationError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	expected := request.Body.ExpectedRevision
	stored, revision, err := s.bodyStateRoutes.ReviewObservation(ctx, userID, &expected, request.Id, string(request.Body.ReviewState))
	if err != nil {
		return reviewBodyStateObservationClassifiedError(err), nil
	}
	response, err := observationMutationResponseToOpenAPI(stored, revision)
	if err != nil {
		return reviewBodyStateObservationError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state response"), nil
	}
	return openapiv1.ReviewBodyStateObservation200JSONResponse(response), nil
}

func (s *PublicServer) AddBodyStateHypothesis(
	ctx context.Context,
	request openapiv1.AddBodyStateHypothesisRequestObject,
) (openapiv1.AddBodyStateHypothesisResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return addBodyStateHypothesisError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil || request.Body == nil {
		return addBodyStateHypothesisError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	hypothesis, err := hypothesisFromOpenAPI(*request.Body)
	if err != nil {
		return addBodyStateHypothesisError(http.StatusBadRequest, "INVALID_REQUEST", "hypothesis JSON fields are invalid"), nil
	}
	expected := request.Body.ExpectedRevision
	stored, revision, err := s.bodyStateRoutes.AddHypothesis(ctx, userID, &expected, hypothesis)
	if err != nil {
		return addBodyStateHypothesisClassifiedError(err), nil
	}
	response, err := hypothesisMutationResponseToOpenAPI(stored, revision)
	if err != nil {
		return addBodyStateHypothesisError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state response"), nil
	}
	return openapiv1.AddBodyStateHypothesis201JSONResponse(response), nil
}

func (s *PublicServer) UpdateBodyStateHypothesisLifecycle(
	ctx context.Context,
	request openapiv1.UpdateBodyStateHypothesisLifecycleRequestObject,
) (openapiv1.UpdateBodyStateHypothesisLifecycleResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return updateBodyStateHypothesisLifecycleError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil || request.Body == nil {
		return updateBodyStateHypothesisLifecycleError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	counterevidence, err := stringArrayJSON(request.Body.CounterevidenceIds)
	if err != nil {
		return updateBodyStateHypothesisLifecycleError(http.StatusBadRequest, "INVALID_REQUEST", "counterevidence_ids are invalid"), nil
	}
	expected := request.Body.ExpectedRevision
	stored, revision, err := s.bodyStateRoutes.UpdateHypothesisLifecycle(
		ctx, userID, &expected, request.Id, string(request.Body.LifecycleState), json.RawMessage(counterevidence),
	)
	if err != nil {
		return updateBodyStateHypothesisLifecycleClassifiedError(err), nil
	}
	response, err := hypothesisMutationResponseToOpenAPI(stored, revision)
	if err != nil {
		return updateBodyStateHypothesisLifecycleError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state response"), nil
	}
	return openapiv1.UpdateBodyStateHypothesisLifecycle200JSONResponse(response), nil
}

func (s *PublicServer) ListBodyStateEvidence(
	ctx context.Context,
	_ openapiv1.ListBodyStateEvidenceRequestObject,
) (openapiv1.ListBodyStateEvidenceResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return listBodyStateEvidenceError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil {
		return listBodyStateEvidenceError(http.StatusInternalServerError, "INTERNAL_ERROR", "body state service is unavailable"), nil
	}
	items, err := s.bodyStateRoutes.ListEvidence(ctx, userID, 100)
	if err != nil {
		return listBodyStateEvidenceError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load evidence"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.BodyStateEvidenceListResponse]("BodyStateEvidenceListResponse", struct {
		Evidence []model.BodyStateEvidence `json:"evidence"`
	}{Evidence: items})
	if err != nil {
		return listBodyStateEvidenceError(http.StatusInternalServerError, "INTERNAL_ERROR", "body state evidence violates the public contract"), nil
	}
	return openapiv1.ListBodyStateEvidence200JSONResponse(response), nil
}

func (s *PublicServer) ResolveBodyStateSafety(
	ctx context.Context,
	request openapiv1.ResolveBodyStateSafetyRequestObject,
) (openapiv1.ResolveBodyStateSafetyResponseObject, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return resolveBodyStateSafetyError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyStateRoutes == nil || request.Body == nil {
		return resolveBodyStateSafetyError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	expected := request.Body.ExpectedRevision
	revision, err := s.bodyStateRoutes.ResolveSafetyState(ctx, userID, &expected, string(request.Body.Resolution), stringValue(request.Body.Note))
	if err != nil {
		return resolveBodyStateSafetyClassifiedError(err), nil
	}
	var publicRevision *openapiv1.BodyStateRevision
	if revision != nil {
		converted, err := revisionToOpenAPI(revision)
		if err != nil {
			return resolveBodyStateSafetyError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state response"), nil
		}
		publicRevision = &converted
	}
	return openapiv1.ResolveBodyStateSafety200JSONResponse{Revision: publicRevision}, nil
}

func observationFromOpenAPI(input openapiv1.BodyStateObservationInput) (model.BodyStateObservation, error) {
	value, err := jsonObjectToDatatypes(&input.Value)
	if err != nil {
		return model.BodyStateObservation{}, err
	}
	condition, err := jsonObjectToDatatypes(input.Condition)
	if err != nil {
		return model.BodyStateObservation{}, err
	}
	provenance, err := jsonObjectToDatatypes(input.Provenance)
	if err != nil {
		return model.BodyStateObservation{}, err
	}
	return model.BodyStateObservation{
		ConcernKey: stringValue(input.ConcernKey), Kind: input.Kind, BodyRegion: stringValue(input.BodyRegion),
		BodyRegionID: input.BodyRegionId, Method: stringValue(input.Method), Value: value, Condition: condition,
		SourceKey: stringValue(input.SourceKey), Provenance: provenance, ObservedAt: input.ObservedAt,
		LifecycleState: stringValue(input.LifecycleState),
	}, nil
}

func hypothesisFromOpenAPI(input openapiv1.BodyStateHypothesisInput) (model.BodyStateHypothesis, error) {
	factIDs, err := stringArrayJSON(input.SupportingFactIds)
	if err != nil {
		return model.BodyStateHypothesis{}, err
	}
	observationIDs, err := stringArrayJSON(input.SupportingObservationIds)
	if err != nil {
		return model.BodyStateHypothesis{}, err
	}
	evidenceIDs, err := stringArrayJSON(input.SupportingEvidenceIds)
	if err != nil {
		return model.BodyStateHypothesis{}, err
	}
	counterevidenceIDs, err := stringArrayJSON(input.CounterevidenceIds)
	if err != nil {
		return model.BodyStateHypothesis{}, err
	}
	provenance, err := jsonObjectToDatatypes(input.Provenance)
	if err != nil {
		return model.BodyStateHypothesis{}, err
	}
	var lifecycle string
	if input.LifecycleState != nil {
		lifecycle = string(*input.LifecycleState)
	}
	return model.BodyStateHypothesis{
		ConcernKey: stringValue(input.ConcernKey), Statement: input.Statement, LifecycleState: lifecycle,
		Confidence: input.Confidence, SupportingFactIDs: factIDs, SupportingObservationIDs: observationIDs,
		SupportingEvidenceIDs: evidenceIDs, CounterevidenceIDs: counterevidenceIDs, Provenance: provenance,
	}, nil
}

func stringArrayJSON(value *openapiv1.StringArray) (datatypes.JSON, error) {
	items := []string{}
	if value != nil {
		items = *value
	}
	encoded, err := json.Marshal(items)
	return datatypes.JSON(encoded), err
}

func observationMutationResponseToOpenAPI(
	observation *model.BodyStateObservation,
	revision *model.BodyStateRevision,
) (openapiv1.BodyStateObservationMutationResponse, error) {
	if observation == nil {
		return openapiv1.BodyStateObservationMutationResponse{}, errors.New("missing observation result")
	}
	publicObservation, err := strictOpenAPIConvert[openapiv1.BodyStateObservation]("BodyStateObservation", observation)
	if err != nil {
		return openapiv1.BodyStateObservationMutationResponse{}, err
	}
	publicRevision, err := optionalRevisionToOpenAPI(revision)
	if err != nil {
		return openapiv1.BodyStateObservationMutationResponse{}, err
	}
	return openapiv1.BodyStateObservationMutationResponse{Observation: publicObservation, Revision: publicRevision}, nil
}

func hypothesisMutationResponseToOpenAPI(
	hypothesis *model.BodyStateHypothesis,
	revision *model.BodyStateRevision,
) (openapiv1.BodyStateHypothesisMutationResponse, error) {
	if hypothesis == nil {
		return openapiv1.BodyStateHypothesisMutationResponse{}, errors.New("missing hypothesis result")
	}
	publicHypothesis, err := strictOpenAPIConvert[openapiv1.BodyStateHypothesis]("BodyStateHypothesis", hypothesis)
	if err != nil {
		return openapiv1.BodyStateHypothesisMutationResponse{}, err
	}
	publicRevision, err := optionalRevisionToOpenAPI(revision)
	if err != nil {
		return openapiv1.BodyStateHypothesisMutationResponse{}, err
	}
	return openapiv1.BodyStateHypothesisMutationResponse{Hypothesis: publicHypothesis, Revision: publicRevision}, nil
}

func optionalRevisionToOpenAPI(revision *model.BodyStateRevision) (*openapiv1.BodyStateRevision, error) {
	if revision == nil {
		return nil, nil
	}
	converted, err := revisionToOpenAPI(revision)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

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

func getBodyStateError(status int, code, message string) openapiv1.GetBodyStateResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.GetBodyState401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.GetBodyState500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}

func listBodyStateEvidenceError(status int, code, message string) openapiv1.ListBodyStateEvidenceResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.ListBodyStateEvidence401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.ListBodyStateEvidence500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}

func correctBodyStateFactClassifiedError(err error) openapiv1.CorrectBodyStateFactResponseObject {
	e := classifyBodyStatePublicError(err)
	return correctBodyStateFactError(e.status, e.code, e.message)
}
func correctBodyStateFactError(status int, code, message string) openapiv1.CorrectBodyStateFactResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.CorrectBodyStateFact400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.CorrectBodyStateFact401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.CorrectBodyStateFact404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.CorrectBodyStateFact409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.CorrectBodyStateFact503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.CorrectBodyStateFact500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func reviewBodyStateFactClassifiedError(err error) openapiv1.ReviewBodyStateFactResponseObject {
	e := classifyBodyStatePublicError(err)
	return reviewBodyStateFactError(e.status, e.code, e.message)
}
func reviewBodyStateFactError(status int, code, message string) openapiv1.ReviewBodyStateFactResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ReviewBodyStateFact400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.ReviewBodyStateFact401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.ReviewBodyStateFact404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.ReviewBodyStateFact409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ReviewBodyStateFact503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ReviewBodyStateFact500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func updateBodyStateFactTemporalClassifiedError(err error) openapiv1.UpdateBodyStateFactTemporalResponseObject {
	e := classifyBodyStatePublicError(err)
	return updateBodyStateFactTemporalError(e.status, e.code, e.message)
}
func updateBodyStateFactTemporalError(status int, code, message string) openapiv1.UpdateBodyStateFactTemporalResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UpdateBodyStateFactTemporal400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.UpdateBodyStateFactTemporal401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.UpdateBodyStateFactTemporal404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.UpdateBodyStateFactTemporal409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.UpdateBodyStateFactTemporal503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.UpdateBodyStateFactTemporal500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func addBodyStateObservationClassifiedError(err error) openapiv1.AddBodyStateObservationResponseObject {
	e := classifyBodyStatePublicError(err)
	return addBodyStateObservationError(e.status, e.code, e.message)
}
func addBodyStateObservationError(status int, code, message string) openapiv1.AddBodyStateObservationResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.AddBodyStateObservation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.AddBodyStateObservation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.AddBodyStateObservation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.AddBodyStateObservation409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.AddBodyStateObservation503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.AddBodyStateObservation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func reviewBodyStateObservationClassifiedError(err error) openapiv1.ReviewBodyStateObservationResponseObject {
	e := classifyBodyStatePublicError(err)
	return reviewBodyStateObservationError(e.status, e.code, e.message)
}
func reviewBodyStateObservationError(status int, code, message string) openapiv1.ReviewBodyStateObservationResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ReviewBodyStateObservation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.ReviewBodyStateObservation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.ReviewBodyStateObservation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.ReviewBodyStateObservation409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ReviewBodyStateObservation503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ReviewBodyStateObservation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func addBodyStateHypothesisClassifiedError(err error) openapiv1.AddBodyStateHypothesisResponseObject {
	e := classifyBodyStatePublicError(err)
	return addBodyStateHypothesisError(e.status, e.code, e.message)
}
func addBodyStateHypothesisError(status int, code, message string) openapiv1.AddBodyStateHypothesisResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.AddBodyStateHypothesis400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.AddBodyStateHypothesis401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.AddBodyStateHypothesis404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.AddBodyStateHypothesis409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.AddBodyStateHypothesis503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.AddBodyStateHypothesis500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func updateBodyStateHypothesisLifecycleClassifiedError(err error) openapiv1.UpdateBodyStateHypothesisLifecycleResponseObject {
	e := classifyBodyStatePublicError(err)
	return updateBodyStateHypothesisLifecycleError(e.status, e.code, e.message)
}
func updateBodyStateHypothesisLifecycleError(status int, code, message string) openapiv1.UpdateBodyStateHypothesisLifecycleResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UpdateBodyStateHypothesisLifecycle400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.UpdateBodyStateHypothesisLifecycle401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.UpdateBodyStateHypothesisLifecycle404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.UpdateBodyStateHypothesisLifecycle409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.UpdateBodyStateHypothesisLifecycle503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.UpdateBodyStateHypothesisLifecycle500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func resolveBodyStateSafetyClassifiedError(err error) openapiv1.ResolveBodyStateSafetyResponseObject {
	e := classifyBodyStatePublicError(err)
	return resolveBodyStateSafetyError(e.status, e.code, e.message)
}
func resolveBodyStateSafetyError(status int, code, message string) openapiv1.ResolveBodyStateSafetyResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ResolveBodyStateSafety400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.ResolveBodyStateSafety401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.ResolveBodyStateSafety404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.ResolveBodyStateSafety409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ResolveBodyStateSafety503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ResolveBodyStateSafety500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
