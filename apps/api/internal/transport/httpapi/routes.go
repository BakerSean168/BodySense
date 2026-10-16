package httpapi

import (
	"net/http"

	"github.com/bodysense/api/internal/dto"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/gin-gonic/gin"
)

// RouteSecurity names the middleware authorities the generated public routes
// are partitioned across. Each field stays owned by main so token/session
// authority is never duplicated inside the generated boundary.
type RouteSecurity struct {
	// Auth authenticates the browser user (sets "user_id").
	Auth gin.HandlerFunc
	// Operator upgrades an authenticated user to knowledge operator authority.
	Operator gin.HandlerFunc
	// Validator enforces the generated OpenAPI request schemas.
	Validator gin.HandlerFunc
}

// routeErrorHandler replaces the generated wrapper's default error writer so
// parameter-binding failures keep the public error envelope.
func routeErrorHandler(c *gin.Context, err error, statusCode int) {
	if statusCode == http.StatusBadRequest {
		c.JSON(statusCode, dto.NewErrorResponse("INVALID_REQUEST", err.Error()))
		return
	}
	c.JSON(statusCode, dto.NewErrorResponse("INTERNAL_ERROR", "request processing failed"))
}

// RegisterRoutes registers every operation from the generated public OpenAPI
// contract exactly once, partitioned by security domain:
//
//   - auth public:     account session establishment without authentication
//   - knowledge:       authenticated knowledge operator authority
//   - protected:       authenticated browser user
//
// The per-group RequestValidator keeps unknown routes outside the generated
// contract unvalidated so operational endpoints such as /api/health stay
// reachable.
func RegisterRoutes(router gin.IRouter, si openapiv1.ServerInterface, security RouteSecurity) {
	wrapper := &openapiv1.ServerInterfaceWrapper{
		Handler:      si,
		ErrorHandler: routeErrorHandler,
	}

	authPublic := router.Group("/api/v1/auth", security.Validator)
	authPublic.POST("/register", wrapper.RegisterAccount)
	authPublic.POST("/login", wrapper.LoginAccount)
	authPublic.POST("/refresh", wrapper.RefreshAccountSession)
	authPublic.POST("/logout", wrapper.LogoutAccount)

	// Knowledge administration is a distinct authorization domain. Authenticate
	// first, resolve durable operator role second, then validate the request.
	// Unauthorized members therefore never reach either request binding or the
	// generated Knowledge adapter.
	knowledge := router.Group("/api/v1/knowledge", security.Auth, security.Operator, security.Validator)
	registerKnowledgeRoutes(knowledge, wrapper)

	protected := router.Group("/api/v1", security.Auth, security.Validator)
	registerProtectedRoutes(protected, wrapper)
}

func registerKnowledgeRoutes(knowledge gin.IRoutes, wrapper *openapiv1.ServerInterfaceWrapper) {
	knowledge.POST("/sources", wrapper.RegisterKnowledgeSource)
	knowledge.GET("/sources", wrapper.ListKnowledgeSources)
	knowledge.POST("/ingestions/video", wrapper.EnqueueKnowledgeVideoIngestion)
	knowledge.GET("/ingestions/:jobID", wrapper.GetKnowledgeIngestionJob)
	knowledge.POST("/search", wrapper.SearchKnowledge)
	knowledge.GET("/stats", wrapper.GetKnowledgeStats)
}

func registerProtectedRoutes(protected gin.IRoutes, wrapper *openapiv1.ServerInterfaceWrapper) {
	// Approved assertion ledger, protected by the same authenticated owner boundary.
	protected.POST("/body-state/records", wrapper.RecordBodyFacts)
	protected.GET("/body-state/records", wrapper.ListBodyFactRecords)
	protected.POST("/body-state/records/:id/correct", wrapper.CorrectBodyFactRecord)
	protected.POST("/body-state/records/:id/withdraw", wrapper.WithdrawBodyFactRecord)
	// New source-backed BodyObservation ledger, distinct from legacy observations.
	protected.POST("/body-state/observation-records", wrapper.RecordBodyObservation)
	protected.GET("/body-state/observation-records", wrapper.ListBodyObservationRecords)
	protected.POST("/body-state/observation-records/:id/correct", wrapper.CorrectBodyObservationRecord)
	protected.POST("/body-state/observation-records/:id/withdraw", wrapper.WithdrawBodyObservationRecord)
	// V2 user-authored BodyHypothesis (direct edit only; no AI write path) and
	// the derived, non-authoritative ConcernGroup read projection.
	protected.POST("/body-state/hypothesis-records", wrapper.RecordBodyHypothesis)
	protected.GET("/body-state/hypothesis-records", wrapper.ListBodyHypothesisRecords)
	protected.POST("/body-state/hypothesis-records/:id/withdraw", wrapper.WithdrawBodyHypothesisRecord)
	protected.POST("/body-state/hypothesis-records/:id/correct", wrapper.CorrectBodyHypothesisRecord)
	protected.GET("/body-state/concern-groups", wrapper.ListDerivedConcernGroups)
	// Owner-authorized explicit Episode mutations and source-backed reads;
	// AI-driven high-confidence association has no public bypass route.
	protected.POST("/body-state/episodes", wrapper.CreateBodyEpisode)
	protected.POST("/body-state/episodes/:id/facts", wrapper.LinkBodyEpisodeFact)
	protected.POST("/body-state/episodes/:id/status", wrapper.AdmitBodyEpisodeStatus)
	protected.POST("/body-state/episodes/:id/recurrences", wrapper.ReportBodyEpisodeRecurrence)
	protected.GET("/body-state/episodes/:id/relations", wrapper.ListBodyEpisodeRelations)
	protected.GET("/body-state/episodes", wrapper.ListBodyEpisodes)
	protected.GET("/body-state/episodes/:id", wrapper.GetBodyEpisode)

	// Native owner-scoped Safety restrictions and per-restriction resolutions.
	protected.GET("/body-state/safety/restrictions", wrapper.ListSafetyRestrictions)
	protected.POST("/body-state/safety/restrictions/:id/resolve", wrapper.ResolveSafetyRestriction)

	// Owner-scoped EquipmentAsset inventory (PC-D2); not BodyState.
	protected.GET("/equipment", wrapper.ListEquipmentAssets)
	protected.POST("/equipment", wrapper.CreateEquipmentAsset)
	protected.PUT("/equipment/:id", wrapper.UpdateEquipmentAsset)

	// Continuous health workspace.
	protected.GET("/health-workspace", wrapper.GetHealthWorkspace)

	// Health context commands/read models.
	protected.GET("/lifestyle", wrapper.GetLifestyle)
	protected.PUT("/lifestyle", wrapper.UpdateLifestyle)
	protected.GET("/body-metrics", wrapper.GetBodyMetrics)
	protected.PUT("/body-metrics", wrapper.UpdateBodyMetrics)
	protected.GET("/health-history/injury", wrapper.GetInjuryHistory)
	protected.PUT("/health-history/injury", wrapper.UpdateInjuryHistory)
	protected.PUT("/onboarding/context", wrapper.SubmitOnboardingContext)

	// Identity and profile.
	protected.GET("/me", wrapper.GetCurrentUser)
	protected.GET("/profile", wrapper.GetUserProfile)
	protected.PUT("/profile", wrapper.UpdateUserProfile)

	// Privacy.
	protected.GET("/privacy/erasure-plan", wrapper.GetPrivacyErasurePlan)
	protected.POST("/privacy/erasure", wrapper.RequestPrivacyErasure)

	// Client diagnostics.
	protected.POST("/client-diagnostics", wrapper.RecordClientDiagnostic)

	// Assessments.
	protected.POST("/assessment/generate", wrapper.GenerateAssessment)
	protected.GET("/assessment", wrapper.ListAssessments)
	protected.GET("/assessment/:id", wrapper.GetAssessment)
	protected.POST("/assessment/:id/replay", wrapper.ReplayAssessment)
	protected.GET("/assessment/:id/regression-export", wrapper.ExportAssessmentRegressionCase)

	// The user's single Conversation (get-or-create), its runs and durable events.
	// There is no public sharing path (PC-D8).
	protected.GET("/conversation", wrapper.GetConversation)
	protected.GET("/conversations/:id/runs", wrapper.ListConversationRuns)
	protected.GET("/conversations/:id/runs/:runId/events", wrapper.ListRunEvents)

	// Consultation runtime, durable thread and HITL surfaces.
	protected.POST("/consultation-runs", wrapper.StartConsultationRun)
	protected.POST("/consultation-runs/:id/cancel", wrapper.CancelConsultationRun)
	protected.POST("/consultation-runs/:id/replay", wrapper.ReplayConsultationRun)
	protected.POST("/consultation-runs/:id/replay/counterfactual", wrapper.ReplayConsultationRunCounterfactual)
	protected.GET("/consultations/:id", wrapper.GetConsultation)
	protected.GET("/consultations/:id/thread", wrapper.GetConsultationThread)
	protected.POST("/consultations/:id/interrupts/:interactionId/answers", wrapper.ResumeConsultationInteraction)
	protected.GET("/consultations/:id/interaction-metrics", wrapper.GetConsultationInteractionMetrics)

	// Diagnosis analysis, assessment and replay surfaces.
	protected.POST("/consultations/:id/diagnosis", wrapper.AnalyzeDiagnosis)
	protected.GET("/diagnosis-analyses", wrapper.ListDiagnosisAnalyses)
	protected.GET("/diagnosis-analyses/:analysisId", wrapper.GetDiagnosisAnalysis)
	protected.PUT("/diagnosis-analyses/:analysisId/assessment", wrapper.AssessDiagnosisCandidates)
	protected.POST("/diagnosis-analyses/:analysisId/replay", wrapper.ReplayDiagnosisAnalysis)
	protected.GET("/diagnosis-analyses/:analysisId/regression-export", wrapper.ExportDiagnosisRegressionCase)

	// Treatment, revision replay and Outcome feedback surfaces.
	protected.POST("/treatments/proposals", wrapper.GenerateTreatmentProposal)
	protected.GET("/treatments/current", wrapper.GetCurrentTreatment)
	protected.POST("/treatments/current/review", wrapper.ReviewCurrentTreatment)
	protected.GET("/treatments/revisions", wrapper.ListTreatmentRevisions)
	protected.GET("/treatments/revisions/:revisionId", wrapper.GetTreatmentRevision)
	protected.POST("/treatments/revisions/:revisionId/replay", wrapper.ReplayTreatmentRevision)
	protected.GET("/treatments/revisions/:revisionId/regression-export", wrapper.ExportTreatmentRegressionCase)
	protected.POST("/treatments/revisions/:revisionId/accept", wrapper.AcceptTreatmentRevision)
	protected.POST("/treatments/revisions/:revisionId/reject", wrapper.RejectTreatmentRevision)
	protected.POST("/outcomes", wrapper.RecordOutcome)
	protected.GET("/outcomes", wrapper.ListOutcomes)

	// Private uploads, posture analysis and health-document review surfaces.
	protected.POST("/uploads", wrapper.CreateUpload)
	protected.GET("/uploads", wrapper.ListUploads)
	protected.GET("/uploads/posture-analysis", wrapper.GetPostureAnalysis)
	protected.GET("/uploads/:id", wrapper.GetUpload)
	protected.DELETE("/uploads/:id", wrapper.DeleteUpload)
	protected.GET("/uploads/:id/health-document-review", wrapper.GetHealthDocumentReviewContext)
	protected.GET("/uploads/:id/extractions/:runId/reviews", wrapper.ListHealthDocumentReviewCandidates)
	protected.POST("/uploads/:id/extractions/:runId/reviews", wrapper.AppendHealthDocumentReview)
	protected.GET("/uploads/:id/extractions/:runId/source", wrapper.GetHealthDocumentSource)

	// Training execution, adherence and reassessment surfaces.
	protected.GET("/training", wrapper.ListTrainingPlans)
	protected.GET("/training/:id", wrapper.GetTrainingPlan)
	protected.GET("/training/:id/today", wrapper.GetTrainingTodayTask)
	protected.POST("/training/:id/checkin", wrapper.CheckInTrainingPlan)
	protected.PUT("/training/:id/log", wrapper.UpdateTrainingLog)
	protected.GET("/training/:id/progress", wrapper.GetTrainingProgress)
	protected.POST("/training/:id/reassess", wrapper.ReassessTrainingPlan)
}
