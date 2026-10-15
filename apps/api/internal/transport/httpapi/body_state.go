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

// PublicServer is the handwritten application adapter behind generated public
// HTTP transport types. Generated OpenAPI models stop here and are translated
// into domain models before service execution.
type PublicServer struct {
	bodyFacts                bodyFactApplication
	bodyObservations         bodyObservationApplication
	bodyHypotheses           bodyHypothesisApplication
	episodes                 episodeReadApplication
	episodeCommands          episodeCommandApplication
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
	diagnosisApplication     diagnosisApplication
	diagnosisAnalyses        diagnosisAnalysisApplication
	diagnosisFreshness       diagnosisFreshnessApplication
	diagnosisReplay          diagnosisReplayApplication
	treatments               treatmentApplication
	treatmentTraining        treatmentTrainingApplication
	treatmentReplay          treatmentReplayApplication
	training                 trainingApplication
	uploads                  uploadApplication
	healthDocumentReviews    healthDocumentReviewApplication
	knowledgeSources         knowledgeSourceApplication
	knowledgeIngestion       knowledgeIngestionApplication
	knowledgeQuery           knowledgeQueryApplication
	equipment                equipmentApplication
}

func NewPublicServer() *PublicServer {
	return &PublicServer{}
}

func authenticatedUserID(ctx context.Context) (uuid.UUID, error) {
	value := ctx.Value("user_id")
	text, ok := value.(string)
	if !ok || text == "" {
		return uuid.Nil, errors.New("missing authenticated user")
	}
	return uuid.Parse(text)
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

func errorEnvelope(code, message string) openapiv1.ErrorEnvelope {
	return openapiv1.ErrorEnvelope{Error: openapiv1.ErrorDetail{Code: code, Message: message}}
}
