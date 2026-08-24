package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/cache"
	consultationruntime "github.com/bodysense/api/internal/consultation"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/handler"
	"github.com/bodysense/api/internal/middleware"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file from project root (ignore error in production where env is injected)
	_ = godotenv.Load("../../.env")

	// Database connection
	dbCfg := database.ConfigFromEnv()
	db, err := database.Connect(dbCfg)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	_ = db

	// Run migrations
	if err := database.RunMigrations(dbCfg); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	// Redis connection
	redisCfg := database.RedisConfigFromEnv()
	_, err = database.ConnectRedis(redisCfg)
	if err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}

	// JWT + browser auth security configuration.
	jwtConfig := auth.JWTConfigFromEnv()
	corsOrigins := parseCORSOrigins()
	authSecurity := handler.DefaultAuthSecurityConfig(jwtConfig.RefreshTokenTTL)
	authSecurity.CookieSecure = strings.EqualFold(os.Getenv("APP_ENV"), "production")
	authSecurity.RequireOrigin = authSecurity.CookieSecure
	authSecurity.TrustedOrigins = corsOrigins
	authSecurity.RateLimiter = auth.NewRedisRateLimiter(database.RedisClient, "bodysense:auth:rate")

	// Initialize dependencies
	userRepo := repository.NewUserRepository(database.DB)
	profileRepo := repository.NewProfileRepository(database.DB)
	uploadRepo := repository.NewUploadRepository(database.DB)
	leaseOwner := uuid.NewString()
	consultationRepo := repository.NewConsultationRepository(database.DB, leaseOwner)
	bodyStateRepo := repository.NewBodyStateRepository(database.DB)
	diagnosisAnalysisRepo := repository.NewDiagnosisAnalysisRepository(database.DB)
	diagnosisFreshnessRepo := repository.NewDiagnosisFreshnessRepository(database.DB)
	treatmentRepo := repository.NewTreatmentRepository(database.DB)
	conversationRepo := repository.NewConversationRepository(database.DB)
	messageRepo := repository.NewMessageRepository(database.DB)
	messageContextRepo := repository.NewMessageContextRepository(database.DB)
	runRepo := repository.NewRunRepository(database.DB)
	runtimeEventRepo := repository.NewRuntimeEventRepository(database.DB)
	threadProjectionRepo := repository.NewThreadProjectionRepository(database.DB)
	shareRepo := repository.NewConversationShareRepository(database.DB)

	// User session cache (Redis-backed). Sessions live as long as their refresh
	// token, so the session TTL matches the refresh-token TTL and outlives the
	// short-lived access token.
	sessionCache := cache.NewUserSessionCache(database.RedisClient, jwtConfig.RefreshTokenTTL)

	authService := service.NewAuthService(userRepo, jwtConfig, sessionCache)
	privacyErasureRepo := repository.NewPrivacyErasureRepository(database.DB)
	privacyErasureService := service.NewPrivacyErasureService(
		privacyErasureRepo,
		userRepo,
		authService,
		service.NewLocalUserObjectCleaner(service.UploadDir),
		database.NewTransactionManager(database.DB),
	)
	privacyErasureService.StartWorker(context.Background(), time.Minute)
	profileService := service.NewProfileService(profileRepo)
	jobRepo := repository.NewJobRepository(database.DB)
	jobRuntime := service.NewJobRuntime(jobRepo)
	aiClient := service.NewAIClient()
	agentDeploymentPolicy, err := service.NewAgentDeploymentPolicy()
	if err != nil {
		log.Fatalf("failed to configure Agent deployment policy: %v", err)
	}
	messageService := service.NewMessageService(messageRepo)
	contextRetrievalService := service.NewContextRetrievalService(messageContextRepo)
	runService := service.NewRunService(runRepo, leaseOwner)
	runtimeEventService := service.NewRuntimeEventService(runtimeEventRepo)
	conversationService := service.NewConversationService(conversationRepo, messageRepo, runRepo, shareRepo, aiClient, database.NewTransactionManager(database.DB)).WithAgentDeployment(agentDeploymentPolicy)
	shareService := service.NewShareService(conversationRepo, messageRepo, shareRepo)
	consultationService := service.NewConsultationService(consultationRepo, conversationRepo)
	bodyStateService := service.NewBodyStateService(bodyStateRepo)
	diagnosisAnalysisService := service.NewDiagnosisAnalysisService(diagnosisAnalysisRepo)
	diagnosisFreshnessService := service.NewDiagnosisFreshnessService(diagnosisFreshnessRepo, bodyStateService)
	treatmentService := service.NewTreatmentService(
		treatmentRepo,
		diagnosisAnalysisService,
		bodyStateService,
		diagnosisFreshnessService,
		profileService,
		aiClient,
		database.NewTransactionManager(database.DB),
		agentDeploymentPolicy,
	)
	assessmentRepo := repository.NewAssessmentRepository(database.DB)
	assessmentReplayService := service.NewAssessmentReplayService(assessmentRepo, aiClient)
	assessmentRolloutService := service.NewAssessmentRolloutService(
		repository.NewAssessmentRolloutRepository(database.DB),
		assessmentReplayService,
	)
	assessmentService := service.NewAssessmentService(
		assessmentRepo,
		profileService,
		uploadRepo,
		bodyStateService,
		aiClient,
		database.NewTransactionManager(database.DB),
	).WithAssessmentDeployment(agentDeploymentPolicy).
		WithAssessmentRollout(assessmentRolloutService)
	authHandler := handler.NewAuthHandler(authService, authSecurity)
	privacyHandler := handler.NewPrivacyHandler(privacyErasureService, authHandler)
	profileHandler := handler.NewProfileHandler(profileService)
	agentToolRepo := repository.NewAgentToolCallRepository(database.DB)
	agentToolService := service.NewAgentToolService(agentToolRepo)
	interactionRepo := repository.NewAgentInteractionRepository(database.DB)
	interactionService := service.NewAgentInteractionService(interactionRepo, runRepo, conversationRepo)
	interactionService.StartInteractionExpiryWorker(
		context.Background(),
		time.Minute,
		func(ctx context.Context, interaction model.AgentInteraction) {
			if err := runtimeEventService.RecordInteractionExpired(ctx, &interaction); err != nil {
				log.Printf("record interaction expired event %s: %v", interaction.ID, err)
			}
		},
	)
	threadProjectionService := service.NewThreadProjectionService(conversationRepo, consultationRepo, messageRepo, interactionRepo, runtimeEventService, threadProjectionRepo)
	outputReviewRepo := repository.NewAIOutputReviewRepository(database.DB)
	outputReviewService := service.NewOutputReviewService(outputReviewRepo)
	uploadService := service.NewUploadService(uploadRepo, jobRuntime, outputReviewService).
		WithDeployment(agentDeploymentPolicy)
	uploadService.StartUploadWorker(context.Background(), 10*time.Second, 10*time.Minute)
	uploadHandler := handler.NewUploadHandler(uploadService)
	consultationRuntime := consultationruntime.NewRuntime(
		conversationService,
		consultationService,
		profileService,
		messageService,
		runService,
		aiClient,
		agentToolService,
		interactionService,
		outputReviewService,
		threadProjectionService,
		runtimeEventService,
		uploadService,
		agentDeploymentPolicy,
		bodyStateService,
	)
	consultationRuntime.AttachLongitudinalContextServices(
		contextRetrievalService,
		diagnosisAnalysisService,
		diagnosisFreshnessService,
		treatmentService,
	)
	consultationRolloutRepo := repository.NewConsultationRolloutRepository(database.DB)
	consultationRuntime.AttachRolloutService(
		service.NewConsultationRolloutService(consultationRolloutRepo),
	)
	startRunLeaseReconciler(runService, conversationService, runtimeEventService)
	knowledgePublicationRepo := repository.NewKnowledgePublicationRepository(database.DB)
	knowledgeObservationRepo := repository.NewKnowledgePublicationObservationRepository(database.DB)
	consultationRuntime.AttachKnowledgeObservationService(
		service.NewKnowledgePublicationObservationService(
			knowledgePublicationRepo,
			knowledgeObservationRepo,
		),
	)
	convHandler := handler.NewConversationHandler(conversationService, shareService)
	runtimeEventHandler := handler.NewRuntimeEventHandler(runtimeEventService, conversationService)
	threadProjectionHandler := handler.NewThreadProjectionHandler(threadProjectionService, bodyStateService)
	bodyStateHandler := handler.NewBodyStateHandler(bodyStateService)
	consultationHandler := handler.NewConsultationHandler(
		consultationService,
		interactionService,
		consultationRuntime,
		bodyStateService,
	).WithReplayService(service.NewConsultationReplayService(runRepo))
	diagnosisReplayService := service.NewDiagnosisReplayService(diagnosisAnalysisService, aiClient)
	diagnosisRolloutRepo := repository.NewDiagnosisRolloutRepository(database.DB)
	diagnosisRolloutService := service.NewDiagnosisRolloutService(diagnosisRolloutRepo)
	diagnosisHandler := handler.NewDiagnosisHandler(
		consultationService,
		profileService,
		aiClient,
		outputReviewService,
		bodyStateService,
		diagnosisAnalysisService,
		diagnosisFreshnessService,
		agentDeploymentPolicy,
		diagnosisReplayService,
		diagnosisRolloutService,
	)
	trainingRepo := repository.NewTrainingRepository(database.DB)
	trainingService := service.NewTrainingService(
		trainingRepo,
		treatmentService,
		database.NewTransactionManager(database.DB),
	)
	trainingHandler := handler.NewTrainingHandler(trainingService)
	treatmentReplayService := service.NewTreatmentReplayService(treatmentRepo, aiClient)
	treatmentRolloutRepo := repository.NewTreatmentRolloutRepository(database.DB)
	treatmentRolloutService := service.NewTreatmentRolloutService(treatmentRolloutRepo, treatmentReplayService)
	treatmentService.AttachRolloutObserver(treatmentRolloutService)
	treatmentHandler := handler.NewTreatmentHandler(treatmentService, trainingService, treatmentReplayService)
	reassessmentHandler := handler.NewReassessmentHandler(trainingService)
	assessmentHandler := handler.NewAssessmentHandler(assessmentService).
		WithAssessmentReplay(assessmentReplayService)
	knowledgeHandler := handler.NewKnowledgeHandler(agentDeploymentPolicy)

	// Continuous health workspace is the single capability/read model for the product loop.
	healthWorkspaceService := service.NewHealthWorkspaceService(
		profileService,
		consultationService,
		bodyStateService,
		diagnosisAnalysisService,
		diagnosisFreshnessService,
		treatmentService,
		trainingService,
	)
	healthWorkspaceHandler := handler.NewHealthWorkspaceHandler(healthWorkspaceService)

	// HTTP server
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()
	if err := r.SetTrustedProxies(parseTrustedProxies()); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES configuration: %v", err)
	}

	// CORS middleware. Production is same-origin, but explicit credentialed
	// CORS remains available for bounded development/test topologies.
	r.Use(func(c *gin.Context) {
		origin := resolveCORSOrigin(c.Request.Header.Get("Origin"), corsOrigins)
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Add("Vary", "Origin")
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// Health check
	r.GET("/api/health", func(c *gin.Context) {
		// Check DB connectivity
		sqlDB, err := database.DB.DB()
		dbStatus := "ok"
		if err != nil || sqlDB.Ping() != nil {
			dbStatus = "unreachable"
		}

		// Check Redis connectivity
		redisStatus := "ok"
		if err := database.RedisClient.Ping(c.Request.Context()).Err(); err != nil {
			redisStatus = "unreachable"
		}

		status := http.StatusOK
		if dbStatus != "ok" || redisStatus != "ok" {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"status":  "ok",
			"service": "bodysense-api",
			"db":      dbStatus,
			"redis":   redisStatus,
		})
	})

	// Auth routes
	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/logout", authHandler.Logout)
	}

	// Protected routes
	protected := r.Group("/api/v1")
	protected.Use(middleware.AuthMiddleware(jwtConfig, userRepo, sessionCache))
	{
		protected.GET("/me", authHandler.Me)
		protected.GET("/privacy/erasure-plan", privacyHandler.PlanErasure)
		protected.POST("/privacy/erasure", privacyHandler.RequestErasure)
		protected.GET("/profile", profileHandler.GetProfile)
		protected.PUT("/profile", profileHandler.CreateOrUpdateProfile)

		// Upload routes
		protected.POST("/uploads", uploadHandler.Upload)
		protected.GET("/uploads", uploadHandler.GetUploads)
		// Static path must be registered before /uploads/:id so Gin does not
		// treat "posture-analysis" as an upload id.
		protected.GET("/uploads/posture-analysis", uploadHandler.GetPostureAnalysis)
		protected.GET("/uploads/:id", uploadHandler.GetUpload)
		protected.DELETE("/uploads/:id", uploadHandler.DeleteUpload)

		// Conversation API
		conversations := protected.Group("/conversations")
		conversations.GET("", convHandler.ListConversations)
		conversations.GET("/:id", convHandler.GetConversation)
		conversations.PATCH("/:id", convHandler.UpdateConversation)
		conversations.DELETE("/:id", convHandler.DeleteConversation)
		conversations.PATCH("/:id/pin", convHandler.PinConversation)
		conversations.GET("/:id/runs", convHandler.ListRuns)
		conversations.GET("/:id/runs/:runId/events", runtimeEventHandler.ListRunEvents)
		conversations.POST("/:id/title", convHandler.GenerateTitle)
		conversations.PUT("/:id/title", convHandler.RenameTitle)
		conversations.POST("/:id/share", convHandler.ShareConversation)
		conversations.DELETE("/:id/share", convHandler.UnshareConversation)

		// Consultation domain
		protected.POST("/consultation-runs", consultationHandler.StartRun)
		protected.POST("/consultation-runs/:id/cancel", consultationHandler.CancelRun)
		consultations := protected.Group("/consultations")
		consultations.GET("/:id", consultationHandler.GetConsultation)
		consultations.GET("/:id/thread", threadProjectionHandler.GetConsultationThread)

		protected.POST("/consultation-runs/:id/replay", consultationHandler.ReplayRun)
		protected.POST("/consultation-runs/:id/replay/counterfactual", consultationHandler.ReplayRunCounterfactual)

		consultations.POST("/:id/diagnosis", diagnosisHandler.AnalyzeDiagnosis)

		// Diagnosis history is user-scoped and pinned to BodyState revisions.
		protected.GET("/diagnosis-analyses", diagnosisHandler.ListDiagnosisHistory)
		protected.GET("/diagnosis-analyses/:analysisId", diagnosisHandler.GetDiagnosisAnalysis)
		protected.PUT("/diagnosis-analyses/:analysisId/assessment", diagnosisHandler.AssessDiagnosisCandidates)
		protected.POST("/diagnosis-analyses/:analysisId/replay", diagnosisHandler.ReplayDiagnosisAnalysis)
		protected.GET("/diagnosis-analyses/:analysisId/regression-export", diagnosisHandler.ExportDiagnosisRegressionCase)

		// Revisioned Treatment / Intervention / Outcome loop.
		protected.POST("/treatments/proposals", treatmentHandler.GenerateProposal)
		protected.GET("/treatments/current", treatmentHandler.GetCurrent)
		protected.POST("/treatments/current/review", treatmentHandler.ReviewCurrent)
		protected.GET("/treatments/revisions", treatmentHandler.ListRevisions)
		protected.GET("/treatments/revisions/:revisionId", treatmentHandler.GetRevision)
		protected.POST("/treatments/revisions/:revisionId/replay", treatmentHandler.ReplayRevision)
		protected.GET("/treatments/revisions/:revisionId/regression-export", treatmentHandler.ExportRegressionCase)
		protected.POST("/treatments/revisions/:revisionId/accept", treatmentHandler.AcceptRevision)
		protected.POST("/treatments/revisions/:revisionId/reject", treatmentHandler.RejectRevision)
		protected.POST("/outcomes", treatmentHandler.RecordOutcome)
		protected.GET("/outcomes", treatmentHandler.ListOutcomes)

		consultations.POST("/:id/interrupts/:interactionId/answers", consultationHandler.ResumeInteraction)
		consultations.GET("/:id/interaction-metrics", consultationHandler.GetInteractionMetrics)

		// Longitudinal BodyState (ADR 0004)
		protected.GET("/body-state", bodyStateHandler.GetCurrent)
		protected.POST("/body-state/facts", bodyStateHandler.UpsertFact)
		protected.POST("/body-state/facts/:id/correct", bodyStateHandler.CorrectFact)
		protected.PATCH("/body-state/facts/:id/temporal", bodyStateHandler.UpdateFactTemporal)
		protected.PATCH("/body-state/facts/:id/review", bodyStateHandler.ReviewFact)
		protected.POST("/body-state/observations", bodyStateHandler.AddObservation)
		protected.PATCH("/body-state/observations/:id/review", bodyStateHandler.ReviewObservation)
		protected.POST("/body-state/hypotheses", bodyStateHandler.AddHypothesis)
		protected.PATCH("/body-state/hypotheses/:id/lifecycle", bodyStateHandler.UpdateHypothesisLifecycle)
		protected.GET("/body-state/evidence", bodyStateHandler.ListEvidence)
		protected.POST("/body-state/safety/resolve", bodyStateHandler.ResolveSafety)

		// Capability-based continuous health workspace.
		protected.GET("/health-workspace", healthWorkspaceHandler.Get)

		// Assessment routes
		protected.POST("/assessment/generate", assessmentHandler.GenerateAssessment)
		protected.GET("/assessment", assessmentHandler.ListReports)
		protected.GET("/assessment/:id", assessmentHandler.GetReport)
		protected.POST("/assessment/:id/replay", assessmentHandler.ReplayAssessment)
		protected.GET("/assessment/:id/regression-export", assessmentHandler.ExportAssessmentRegressionCase)

		// Training routes
		protected.GET("/training", trainingHandler.ListPlans)
		protected.GET("/training/:id", trainingHandler.GetPlan)
		protected.GET("/training/:id/today", trainingHandler.GetTodayTask)
		protected.POST("/training/:id/checkin", trainingHandler.CheckIn)
		protected.PUT("/training/:id/log", trainingHandler.UpdateLog)
		protected.GET("/training/:id/progress", trainingHandler.GetProgress)
		protected.POST("/training/:id/reassess", reassessmentHandler.SubmitReassessment)
	}

	// Public share routes (no auth)
	public := r.Group("/api/v1")
	public.GET("/conversations/share/:token", convHandler.GetSharedConversation)

	// Knowledge base routes (proxy to AI service, require auth)
	knowledgeGroup := protected.Group("/knowledge")
	{
		knowledgeGroup.POST("/ingestions/video", knowledgeHandler.IngestVideo)
		knowledgeGroup.POST("/search", knowledgeHandler.SearchKnowledge)
		knowledgeGroup.GET("/sources", knowledgeHandler.ListSources)
		knowledgeGroup.GET("/stats", knowledgeHandler.GetStats)
	}

	log.Printf("BodySense API starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}

func startRunLeaseReconciler(
	runService *service.RunService,
	conversationService *service.ConversationService,
	runtimeEventService *service.RuntimeEventService,
) {
	reconcile := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		runs, err := runService.ReconcileExpiredRuns(ctx, 100)
		if err != nil {
			log.Printf("run lease reconciliation failed: %v", err)
			return
		}
		for i := range runs {
			run := &runs[i]
			failedMessage, err := conversationService.FinalizeExecutionLostProjection(ctx, run)
			if err != nil {
				log.Printf("finalize execution-lost projection for run %s: %v", run.ID, err)
			}
			if err := runtimeEventService.RecordRunExecutionLost(ctx, run); err != nil {
				log.Printf("record execution-lost event for run %s: %v", run.ID, err)
			}
			if err := runtimeEventService.RecordMessageExecutionLost(ctx, run, failedMessage); err != nil {
				log.Printf("record execution-lost message event for run %s: %v", run.ID, err)
			}
		}
	}

	go func() {
		reconcile()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			reconcile()
		}
	}()
}

func parseCORSOrigins() []string {
	raw := os.Getenv("CORS_ORIGINS")
	if raw == "" {
		raw = os.Getenv("CORS_ORIGIN")
	}
	if raw == "" {
		return []string{"*"}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
}

func resolveCORSOrigin(requestOrigin string, allowed []string) string {
	for _, origin := range allowed {
		if origin == "*" {
			// Credentialed CORS cannot use a wildcard response. Reflect the
			// requesting origin only in explicitly wildcard development mode.
			return requestOrigin
		}
		if requestOrigin != "" && origin == requestOrigin {
			return requestOrigin
		}
	}
	return ""
}

func parseTrustedProxies() []string {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if raw == "" {
		return []string{"127.0.0.1", "::1"}
	}
	parts := strings.Split(raw, ",")
	proxies := make([]string, 0, len(parts))
	for _, part := range parts {
		if proxy := strings.TrimSpace(part); proxy != "" {
			proxies = append(proxies, proxy)
		}
	}
	return proxies
}
