package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/cache"
	consultationruntime "github.com/bodysense/api/internal/consultation"
	"github.com/bodysense/api/internal/database"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/handler"
	"github.com/bodysense/api/internal/middleware"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/observability"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/transport/httpapi"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/gin-contrib/requestid"
	ginslog "github.com/gin-contrib/slog"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file from project root (ignore error in production where env is injected)
	_ = godotenv.Load("../../.env")

	// Structured process logger must be installed before service initialization so startup
	// failures and legacy log.Printf calls share the same JSON log stream.
	observability.ConfigureLogger()

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
	authSecurity := auth.DefaultSecurityConfig(jwtConfig.RefreshTokenTTL)
	authSecurity.CookieSecure = strings.EqualFold(os.Getenv("APP_ENV"), "production")
	authSecurity.RequireOrigin = authSecurity.CookieSecure
	authSecurity.TrustedOrigins = corsOrigins
	authSecurity.RateLimiter = auth.NewRedisRateLimiter(database.RedisClient, "bodysense:auth:rate")

	// Initialize dependencies
	userRepo := repository.NewUserRepository(database.DB)
	knowledgeSourceRepo := repository.NewKnowledgeSourceRepository(database.DB)
	knowledgeSourceRegistry := service.NewKnowledgeSourceRegistry(knowledgeSourceRepo)
	profileRepo := repository.NewProfileRepository(database.DB)
	uploadRepo := repository.NewUploadRepository(database.DB)
	documentExtractionRunRepo := repository.NewDocumentExtractionRunRepository(database.DB)
	uploadStorage, err := uploadstorage.NewRegistryFromEnv()
	if err != nil {
		log.Fatalf("Upload storage configuration failed: %v", err)
	}
	uploadStorageCtx, uploadStorageCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer uploadStorageCancel()
	if err := uploadStorage.Validate(uploadStorageCtx); err != nil {
		log.Fatalf("Upload storage validation failed: %v", err)
	}
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
		uploadStorage,
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
	healthDocumentDeploymentPolicy, err := service.NewHealthDocumentDeploymentPolicy()
	if err != nil {
		log.Fatalf("failed to configure health-document deployment policy: %v", err)
	}
	messageService := service.NewMessageService(messageRepo)
	contextRetrievalService := service.NewContextRetrievalService(messageContextRepo)
	runService := service.NewRunService(runRepo, leaseOwner)
	runtimeEventService := service.NewRuntimeEventService(runtimeEventRepo)
	conversationService := service.NewConversationService(conversationRepo, messageRepo, runRepo, shareRepo, aiClient, database.NewTransactionManager(database.DB)).WithAgentDeployment(agentDeploymentPolicy)
	shareService := service.NewShareService(conversationRepo, messageRepo, shareRepo)
	consultationService := service.NewConsultationService(consultationRepo, conversationRepo)
	bodyStateService := service.NewBodyStateService(bodyStateRepo).WithBodyRegionIDValidator(
		service.NewCanonicalBodyRegionIDValidator(),
	)
	lifestyleService := service.NewLifestyleService(bodyStateService)
	bodyMetricsService := service.NewBodyMetricsService(bodyStateService)
	healthHistoryService := service.NewHealthHistoryService(bodyStateService)
	onboardingContextService := service.NewOnboardingContextService(
		profileService,
		bodyStateService,
		database.NewTransactionManager(database.DB),
	)
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
	documentIndicatorReviewRepo := repository.NewDocumentIndicatorReviewRepository(database.DB)
	assessmentService := service.NewAssessmentService(
		assessmentRepo,
		profileService,
		uploadRepo,
		bodyStateService,
		aiClient,
		database.NewTransactionManager(database.DB),
	).WithAssessmentDeployment(agentDeploymentPolicy).
		WithAssessmentRollout(assessmentRolloutService).
		WithAssessmentReviews(documentIndicatorReviewRepo)
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
	uploadService := service.NewUploadService(uploadRepo, jobRuntime, outputReviewService, uploadStorage).
		WithDeployment(agentDeploymentPolicy).
		WithHealthDocumentDeployment(healthDocumentDeploymentPolicy).
		WithDocumentExtractionRuns(documentExtractionRunRepo)
	uploadService.StartUploadWorker(context.Background(), 10*time.Second, 10*time.Minute)
	healthDocumentReviewService := service.NewHealthDocumentReviewService(documentExtractionRunRepo, documentIndicatorReviewRepo)
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
	consultationReplayService := service.NewConsultationReplayService(runRepo)
	diagnosisReplayService := service.NewDiagnosisReplayService(diagnosisAnalysisService, aiClient)
	diagnosisRolloutRepo := repository.NewDiagnosisRolloutRepository(database.DB)
	diagnosisRolloutService := service.NewDiagnosisRolloutService(diagnosisRolloutRepo)
	diagnosisApplicationService := service.NewDiagnosisApplicationService(
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
	treatmentReplayService := service.NewTreatmentReplayService(treatmentRepo, aiClient)
	treatmentRolloutRepo := repository.NewTreatmentRolloutRepository(database.DB)
	treatmentRolloutService := service.NewTreatmentRolloutService(treatmentRolloutRepo, treatmentReplayService)
	treatmentService.AttachRolloutObserver(treatmentRolloutService)
	knowledgeIngestionService := service.NewKnowledgeIngestionService(
		knowledgeSourceRegistry,
		jobRuntime,
		agentDeploymentPolicy,
		os.Getenv("AI_SERVICE_URL"),
	)
	knowledgeIngestionService.StartWorker(context.Background(), 10*time.Second, 15*time.Minute)
	knowledgeHandler := handler.NewKnowledgeHandler(agentDeploymentPolicy).
		WithSourceRegistry(knowledgeSourceRegistry).
		WithIngestionService(knowledgeIngestionService)

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

	// HTTP server. Host development defaults to loopback; container runtimes
	// explicitly set API_HOST=0.0.0.0 so the Docker network can reach it.
	host := os.Getenv("API_HOST")
	port := os.Getenv("API_PORT")
	listenAddress := resolveAPIListenAddress(host, port)

	r := gin.New()
	r.Use(requestid.New())
	r.Use(ginslog.SetLogger(
		ginslog.WithLogger(func(c *gin.Context, _ *slog.Logger) *slog.Logger {
			return slog.Default().With("http_request_id", requestid.Get(c))
		}),
		ginslog.WithContext(sanitizeHTTPRequestLogRecord),
		ginslog.WithSkipPath([]string{"/api/health"}),
		ginslog.WithMessage("http request"),
		ginslog.WithUTC(true),
		ginslog.WithRequestHeader(false),
	))
	r.Use(gin.Recovery())
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
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
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

	// Protected routes. Keep one auth middleware instance so handwritten and
	// generated OpenAPI routes share the exact same token/session authority.
	authMiddleware := middleware.AuthMiddleware(jwtConfig, userRepo, sessionCache)
	protected := r.Group("/api/v1")
	protected.Use(authMiddleware)
	{

		// Longitudinal BodyState (ADR 0004)

		// User-facing projections backed exclusively by BodyState.

		// Capability-based continuous health workspace.

		// Assessment routes

	}

	// OpenAPI-authoritative public routes. Authentication, operator authority
	// and request validation remain owned by middleware; request/response
	// transport is generated from packages/contracts/openapi/bodysense.v1.openapi.yaml.
	publicAPISpec, err := openapiv1.GetSwagger()
	if err != nil {
		log.Fatalf("failed to load generated public OpenAPI spec: %v", err)
	}
	httpapi.RegisterRoutes(
		r,
		httpapi.StrictHandler(httpapi.NewPublicServer(bodyStateService).WithBodyStateRoutes(bodyStateService).WithHealthWorkspace(healthWorkspaceService).WithHealthContext(lifestyleService, bodyMetricsService, healthHistoryService, onboardingContextService).WithProfile(profileService).WithPrivacy(privacyErasureService, authSecurity.RefreshCookieName, authSecurity.CookieSecure).WithAssessment(assessmentService, assessmentReplayService).WithAuth(authService, authSecurity).WithConversations(conversationService, shareService, runtimeEventService).WithConsultation(consultationRuntime, consultationService, interactionService, consultationReplayService, threadProjectionService, bodyStateService).WithDiagnosis(diagnosisApplicationService, diagnosisAnalysisService, diagnosisFreshnessService, diagnosisReplayService).WithTreatment(treatmentService, trainingService, treatmentReplayService).WithTraining(trainingService).WithUploads(uploadService, healthDocumentReviewService)),
		httpapi.RouteSecurity{
			Auth:      authMiddleware,
			Operator:  middleware.RequireKnowledgeOperator(userRepo),
			Validator: httpapi.RequestValidator(publicAPISpec),
		},
	)

	// Global Knowledge administration is an explicit operator capability.
	// Product Agents retrieve published Knowledge through the internal AI path;
	// these HTTP surfaces are for governed operator workflows only.
	knowledgeGroup := protected.Group("/knowledge")
	knowledgeGroup.Use(middleware.RequireKnowledgeOperator(userRepo))
	{
		knowledgeGroup.POST("/sources", knowledgeHandler.RegisterSource)
		knowledgeGroup.GET("/sources", knowledgeHandler.ListSources)
		knowledgeGroup.POST("/ingestions/video", knowledgeHandler.IngestVideo)
		knowledgeGroup.GET("/ingestions/:jobID", knowledgeHandler.GetIngestionJob)
		knowledgeGroup.POST("/search", knowledgeHandler.SearchKnowledge)
		knowledgeGroup.GET("/stats", knowledgeHandler.GetStats)
	}

	log.Printf("BodySense API starting on %s", listenAddress)
	if err := r.Run(listenAddress); err != nil {
		log.Fatal(err)
	}
}

func resolveAPIListenAddress(host, port string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	port = strings.TrimSpace(port)
	if port == "" {
		port = "8080"
	}
	return net.JoinHostPort(host, port)
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

// sanitizeHTTPRequestLogRecord keeps gin-contrib/slog's mature request lifecycle
// handling while removing fields that are not appropriate for a health product's
// operational logs. Route/status/latency/request-id are enough for correlation;
// raw query strings, client IPs and referrers are intentionally excluded.
func sanitizeHTTPRequestLogRecord(_ *gin.Context, record *slog.Record) *slog.Record {
	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case "query", "ip", "referer":
			return true
		default:
			clean.AddAttrs(attr)
			return true
		}
	})
	return &clean
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
