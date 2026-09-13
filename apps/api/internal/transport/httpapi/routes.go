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
//   - share public:    token-addressed conversation shares without authentication
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

	protected := router.Group("/api/v1", security.Auth, security.Validator)
	registerProtectedRoutes(protected, wrapper)
}

func registerProtectedRoutes(protected gin.IRoutes, wrapper *openapiv1.ServerInterfaceWrapper) {
	// Longitudinal BodyState (ADR 0004).
	protected.POST("/body-state/facts", wrapper.AddBodyStateFact)
	protected.GET("/body-state", wrapper.GetBodyState)
	protected.POST("/body-state/facts/:id/correct", wrapper.CorrectBodyStateFact)
	protected.PATCH("/body-state/facts/:id/temporal", wrapper.UpdateBodyStateFactTemporal)
	protected.PATCH("/body-state/facts/:id/review", wrapper.ReviewBodyStateFact)
	protected.POST("/body-state/observations", wrapper.AddBodyStateObservation)
	protected.PATCH("/body-state/observations/:id/review", wrapper.ReviewBodyStateObservation)
	protected.POST("/body-state/hypotheses", wrapper.AddBodyStateHypothesis)
	protected.PATCH("/body-state/hypotheses/:id/lifecycle", wrapper.UpdateBodyStateHypothesisLifecycle)
	protected.GET("/body-state/evidence", wrapper.ListBodyStateEvidence)
	protected.POST("/body-state/safety/resolve", wrapper.ResolveBodyStateSafety)

	// Continuous health workspace.
	protected.GET("/health-workspace", wrapper.GetHealthWorkspace)

	// Health context commands/read models.
	protected.GET("/lifestyle", wrapper.GetLifestyle)
	protected.PUT("/lifestyle", wrapper.UpdateLifestyle)
	protected.POST("/lifestyle/candidates/:id/accept", wrapper.AcceptLifestyleCandidate)
	protected.POST("/lifestyle/candidates/:id/reject", wrapper.RejectLifestyleCandidate)
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
}
