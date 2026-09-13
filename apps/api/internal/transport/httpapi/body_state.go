package httpapi

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bodysense/api/internal/auth"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// authConfig mirrors the auth edge policy without exposing the limiter
// implementation beyond the transport package.
type authConfig = auth.SecurityConfig

type bodyStateFactService interface {
	UpsertFact(
		ctx context.Context,
		userID uuid.UUID,
		expectedRevision *int64,
		fact model.BodyStateFact,
	) (*model.BodyStateFact, *model.BodyStateRevision, error)
}

// PublicServer is the handwritten application adapter behind generated public
// HTTP transport types. Generated OpenAPI models stop here and are translated
// into domain models before service execution.
type PublicServer struct {
	bodyState                bodyStateFactService
	bodyStateRoutes          bodyStateRouteService
	healthWorkspace          healthWorkspaceService
	lifestyle                lifestyleApplication
	bodyMetrics              bodyMetricsApplication
	healthHistory            healthHistoryApplication
	onboarding               onboardingContextApplication
	profile                  profileApplication
	privacy                  privacyErasureApplication
	privacyCookie            privacyRefreshCookiePolicy
	assessment               assessmentApplication
	assessmentReplay         assessmentReplayApplication
	accounts                 authAccountApplication
	authSecurity             authConfig
	conversations            conversationApplication
	shares                   conversationShareApplication
	runtimeEvents            runtimeEventApplication
	consultationRuntime      consultationRuntimeApplication
	consultationSessions     consultationSessionApplication
	consultationInteractions consultationInteractionApplication
	consultationReplay       consultationReplayApplication
	consultationThreads      consultationThreadApplication
	consultationBodyState    consultationBodyStateApplication
	diagnosisApplication     diagnosisApplication
	diagnosisAnalyses        diagnosisAnalysisApplication
	diagnosisFreshness       diagnosisFreshnessApplication
	diagnosisReplay          diagnosisReplayApplication
	treatments               treatmentApplication
	treatmentTraining        treatmentTrainingApplication
	treatmentReplay          treatmentReplayApplication
	training                 trainingApplication
}

func NewPublicServer(bodyState bodyStateFactService) *PublicServer {
	return &PublicServer{bodyState: bodyState}
}

func (s *PublicServer) AddBodyStateFact(
	ctx context.Context,
	request openapiv1.AddBodyStateFactRequestObject,
) (openapiv1.AddBodyStateFactResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return addFactError401("UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return addFactError400("INVALID_REQUEST", "request body is required"), nil
	}

	fact, err := factFromOpenAPI(request.Body.Fact)
	if err != nil {
		return addFactError400("INVALID_REQUEST", "fact JSON fields are invalid"), nil
	}
	expectedRevision := request.Body.ExpectedRevision
	created, revision, err := s.bodyState.UpsertFact(ctx, userID, &expectedRevision, fact)
	if err != nil {
		return bodyStateFactErrorResponse(err), nil
	}
	if created == nil {
		return addFactError500("INTERNAL_ERROR", "failed to update body state"), nil
	}

	response, err := factMutationResponseToOpenAPI(created, revision)
	if err != nil {
		return addFactError500("INTERNAL_ERROR", "failed to encode body state response"), nil
	}
	return openapiv1.AddBodyStateFact200JSONResponse(response), nil
}

func authenticatedUserID(ctx context.Context) (uuid.UUID, error) {
	value := ctx.Value("user_id")
	text, ok := value.(string)
	if !ok || text == "" {
		return uuid.Nil, errors.New("missing authenticated user")
	}
	return uuid.Parse(text)
}

func factFromOpenAPI(input openapiv1.BodyStateFactInput) (model.BodyStateFact, error) {
	details, err := jsonObjectToDatatypes(input.Details)
	if err != nil {
		return model.BodyStateFact{}, err
	}
	provenance, err := jsonObjectToDatatypes(input.Provenance)
	if err != nil {
		return model.BodyStateFact{}, err
	}
	return model.BodyStateFact{
		ConcernKey:     stringValue(input.ConcernKey),
		Kind:           input.Kind,
		BodyRegion:     stringValue(input.BodyRegion),
		BodyRegionID:   input.BodyRegionId,
		Value:          input.Value,
		Details:        details,
		Origin:         stringValue(input.Origin),
		ReviewState:    stringValue(input.ReviewState),
		LifecycleState: stringValue(input.LifecycleState),
		Trend:          stringValue(input.Trend),
		SourceKey:      stringValue(input.SourceKey),
		Provenance:     provenance,
		ObservedAt:     input.ObservedAt,
		ValidFrom:      input.ValidFrom,
		ValidUntil:     input.ValidUntil,
	}, nil
}

func factMutationResponseToOpenAPI(
	fact *model.BodyStateFact,
	revision *model.BodyStateRevision,
) (openapiv1.BodyStateFactMutationResponse, error) {
	details, err := datatypesToJSONObject(fact.Details)
	if err != nil {
		return openapiv1.BodyStateFactMutationResponse{}, err
	}
	provenance, err := datatypesToJSONObject(fact.Provenance)
	if err != nil {
		return openapiv1.BodyStateFactMutationResponse{}, err
	}
	var publicRevision *openapiv1.BodyStateRevision
	if revision != nil {
		converted, err := revisionToOpenAPI(revision)
		if err != nil {
			return openapiv1.BodyStateFactMutationResponse{}, err
		}
		publicRevision = &converted
	}

	return openapiv1.BodyStateFactMutationResponse{
		Fact: openapiv1.BodyStateFact{
			Id:                    fact.ID,
			UserId:                fact.UserID,
			ConcernKey:            optionalString(fact.ConcernKey),
			Kind:                  fact.Kind,
			BodyRegion:            optionalString(fact.BodyRegion),
			BodyRegionId:          fact.BodyRegionID,
			Value:                 fact.Value,
			Details:               details,
			Origin:                fact.Origin,
			ReviewState:           fact.ReviewState,
			LifecycleState:        fact.LifecycleState,
			Trend:                 fact.Trend,
			SourceKey:             optionalString(fact.SourceKey),
			Provenance:            provenance,
			ObservedAt:            fact.ObservedAt,
			ValidFrom:             fact.ValidFrom,
			ValidUntil:            fact.ValidUntil,
			SupersedesFactId:      fact.SupersedesFactID,
			ExcludedFromReasoning: fact.ExcludedFromReasoning,
			CreatedRevision:       fact.CreatedRevision,
			UpdatedRevision:       fact.UpdatedRevision,
			CreatedAt:             fact.CreatedAt,
			UpdatedAt:             fact.UpdatedAt,
		},
		Revision: publicRevision,
	}, nil
}

func revisionToOpenAPI(revision *model.BodyStateRevision) (openapiv1.BodyStateRevision, error) {
	changes, err := datatypesToJSONObject(revision.Changes)
	if err != nil {
		return openapiv1.BodyStateRevision{}, err
	}
	return openapiv1.BodyStateRevision{
		Id: revision.ID, UserId: revision.UserID, Revision: revision.Revision,
		ChangeType: revision.ChangeType, Source: revision.Source, Changes: changes, CreatedAt: revision.CreatedAt,
	}, nil
}

func jsonObjectToDatatypes(value *openapiv1.JsonObject) (datatypes.JSON, error) {
	if value == nil {
		return datatypes.JSON(`{}`), nil
	}
	encoded, err := json.Marshal(*value)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(encoded), nil
}

func datatypesToJSONObject(value datatypes.JSON) (openapiv1.JsonObject, error) {
	if len(value) == 0 || string(value) == "null" {
		return openapiv1.JsonObject{}, nil
	}
	var decoded openapiv1.JsonObject
	if err := json.Unmarshal(value, &decoded); err != nil {
		return nil, err
	}
	if decoded == nil {
		decoded = openapiv1.JsonObject{}
	}
	return decoded, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func bodyStateFactErrorResponse(err error) openapiv1.AddBodyStateFactResponseObject {
	classified := classifyBodyStatePublicError(err)
	switch classified.status {
	case 400:
		return addFactError400(classified.code, classified.message)
	case 401:
		return addFactError401(classified.code, classified.message)
	case 404:
		return addFactError404(classified.code, classified.message)
	case 409:
		return addFactError409(classified.code, classified.message)
	case 503:
		return addFactError503(classified.code, classified.message)
	default:
		return addFactError500(classified.code, classified.message)
	}
}

func errorEnvelope(code, message string) openapiv1.ErrorEnvelope {
	return openapiv1.ErrorEnvelope{Error: openapiv1.ErrorDetail{Code: code, Message: message}}
}

func addFactError400(code, message string) openapiv1.AddBodyStateFact400JSONResponse {
	return openapiv1.AddBodyStateFact400JSONResponse{
		InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message)),
	}
}

func addFactError401(code, message string) openapiv1.AddBodyStateFact401JSONResponse {
	return openapiv1.AddBodyStateFact401JSONResponse{
		UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message)),
	}
}

func addFactError404(code, message string) openapiv1.AddBodyStateFact404JSONResponse {
	return openapiv1.AddBodyStateFact404JSONResponse{
		NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message)),
	}
}

func addFactError409(code, message string) openapiv1.AddBodyStateFact409JSONResponse {
	return openapiv1.AddBodyStateFact409JSONResponse{
		RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(errorEnvelope(code, message)),
	}
}

func addFactError500(code, message string) openapiv1.AddBodyStateFact500JSONResponse {
	return openapiv1.AddBodyStateFact500JSONResponse{
		InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message)),
	}
}

func addFactError503(code, message string) openapiv1.AddBodyStateFact503JSONResponse {
	return openapiv1.AddBodyStateFact503JSONResponse{
		ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(errorEnvelope(code, message)),
	}
}
