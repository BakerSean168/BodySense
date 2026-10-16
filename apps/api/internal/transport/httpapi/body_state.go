package httpapi

import (
	"context"
	"errors"

	"github.com/bodysense/api/internal/auth"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/google/uuid"
)

// authConfig mirrors the auth edge policy without exposing the limiter
// implementation beyond the transport package.
type authConfig = auth.SecurityConfig

// PublicServer is the handwritten application adapter behind generated public
// HTTP transport types. Generated OpenAPI models stop here and are translated
// into domain models before service execution.
type PublicServer struct {
	safetyRestrictions       safetyRestrictionApplication
	bodyFacts                bodyFactApplication
	bodyObservations         bodyObservationApplication
	bodyHypotheses           bodyHypothesisApplication
	episodes                 episodeReadApplication
	episodeCommands          episodeCommandApplication
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

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func errorEnvelope(code, message string) openapiv1.ErrorEnvelope {
	return openapiv1.ErrorEnvelope{Error: openapiv1.ErrorDetail{Code: code, Message: message}}
}
