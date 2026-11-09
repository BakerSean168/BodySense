package httpapi

import (
	"context"
	"errors"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/google/uuid"
)

// PublicServer is the handwritten application adapter behind generated public
// HTTP transport types. Generated OpenAPI models stop here and are translated
// into domain models before service execution.
type PublicServer struct {
	safetyRestrictions           safetyRestrictionApplication
	bodyFacts                    bodyFactApplication
	bodyObservations             bodyObservationApplication
	bodyHypotheses               bodyHypothesisApplication
	episodes                     episodeReadApplication
	episodeCommands              episodeCommandApplication
	healthWorkspace              healthWorkspaceService
	lifestyle                    lifestyleApplication
	bodyMetrics                  bodyMetricsApplication
	healthHistory                healthHistoryApplication
	onboarding                   onboardingContextApplication
	reportedDiscomfort           reportedDiscomfortReader
	profile                      profileApplication
	privacy                      privacyErasureApplication
	assessment                   assessmentApplication
	assessmentReplay             assessmentReplayApplication
	conversations                conversationApplication
	runtimeEvents                runtimeEventApplication
	consultationRuntime          consultationRuntimeApplication
	consultationSessions         consultationSessionApplication
	consultationInteractions     consultationInteractionApplication
	consultationReplay           consultationReplayApplication
	consultationThreads          consultationThreadApplication
	nativeDiagnosisRuns          nativeDiagnosisRunReadApplication
	nativeDiagnosis              nativeDiagnosisReadApplication
	nativeDiagnosisDecisions     nativeDiagnosisDecisionApplication
	nativeDiagnosisSettlements   nativeDiagnosisSettlementApplication
	nativeDiagnosisDirectEdits   nativeDiagnosisDirectEditApplication
	nativeDiagnosisStarter       nativeDiagnosisStartApplication
	nativeDiagnosisRequests      nativeDiagnosisAnalysisRequestApplication
	nativeDiagnosisReviewPrompts nativeDiagnosisReviewPromptApplication
	outcomes                     outcomeApplication
	planGenerator                planGenerationApplication
	trainingPlans                trainingPlanApplication
	trainerSessions              trainerSessionApplication
	uploads                      uploadApplication
	assetErasure                 assetErasureApplication
	statementErasure             statementErasureApplication
	healthDocumentReviews        healthDocumentReviewApplication
	knowledgeSources             knowledgeSourceApplication
	knowledgeIngestion           knowledgeIngestionApplication
	knowledgeQuery               knowledgeQueryApplication
	equipment                    equipmentApplication
	episodicNotes                episodicNoteControlsApplication
	episodicNotesEnabled         bool
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
