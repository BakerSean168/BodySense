package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/config"
	consultationruntime "github.com/bodysense/api/internal/consultation"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/episodicrecall"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/middleware"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/observability"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/transport/httpapi"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file from project root (ignore error in production where env is injected)
	_ = godotenv.Load("../../.env")

	// Structured process logger must be installed before service initialization so startup
	// failures and standard-library log.Printf calls share the same JSON log stream.
	observability.ConfigureLogger()

	// The composition root is the only place the API reads the process
	// environment; every layer below receives the resolved values.
	apiConfig := config.Load(os.Getenv)

	corsOrigins, supabaseConfig, supabaseVerifier := initInfrastructure()

	// Initialize dependencies
	userRepo := repository.NewUserRepository(database.DB)
	knowledgeSourceRepo := repository.NewKnowledgeSourceRepository(database.DB)
	knowledgeSourceRegistry := service.NewKnowledgeSourceRegistry(knowledgeSourceRepo)
	profileRepo := repository.NewProfileRepository(database.DB)
	userAssetRepo := repository.NewUserAssetRepository(database.DB)
	documentExtractionRunRepo := repository.NewDocumentExtractionRunRepository(database.DB)
	postureAnalysisRunRepo := repository.NewPostureAnalysisRunRepository(database.DB)
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
	outcomeRepo := repository.NewOutcomeRepository(database.DB)
	conversationRepo := repository.NewConversationRepository(database.DB)
	messageRepo := repository.NewMessageRepository(database.DB)
	messageContextRepo := repository.NewMessageContextRepository(database.DB)
	runRepo := repository.NewRunRepository(database.DB)
	runtimeEventRepo := repository.NewRuntimeEventRepository(database.DB)
	threadProjectionRepo := repository.NewThreadProjectionRepository(database.DB)

	// Hosted identity revocation is required before a full account erasure
	// can be accepted. The server-only admin key is never exposed to the web.
	authAdmin := auth.NewSupabaseAdminRevoker(supabaseConfig, nil)
	privacyErasureRepo := repository.NewPrivacyErasureRepository(database.DB)
	privacyErasureService := service.NewPrivacyErasureService(
		privacyErasureRepo,
		userRepo,
		authAdmin,
		uploadStorage,
		database.NewTransactionManager(database.DB),
	)
	privacyErasureService.StartWorker(context.Background(), time.Minute)
	profileService := service.NewProfileService(profileRepo)
	jobRepo := repository.NewJobRepository(database.DB)
	jobRuntime := service.NewJobRuntime(jobRepo)
	aiClient := service.NewAIClient(apiConfig.AI)
	agentDeploymentPolicy, err := service.NewAgentDeploymentPolicy(apiConfig.Deployment)
	if err != nil {
		log.Fatalf("failed to configure Agent deployment policy: %v", err)
	}
	healthDocumentDeploymentPolicy, err := service.NewHealthDocumentDeploymentPolicy(apiConfig.HealthDocument)
	if err != nil {
		log.Fatalf("failed to configure health-document deployment policy: %v", err)
	}
	messageService := service.NewMessageService(messageRepo)
	contextRetrievalService := service.NewContextRetrievalService(messageContextRepo)
	runService := service.NewRunService(runRepo, leaseOwner)
	runtimeEventService := service.NewRuntimeEventService(runtimeEventRepo)
	runService.WithLifecycleEvents(runtimeEventService, database.NewTransactionManager(database.DB))
	conversationService := service.NewConversationService(conversationRepo, messageRepo, runRepo, database.NewTransactionManager(database.DB))
	consultationService := service.NewConsultationService(consultationRepo, conversationRepo)
	equipmentRepo := repository.NewEquipmentAssetRepository(database.DB)
	bodyStateService := service.NewBodyStateService(bodyStateRepo).WithBodyRegionIDValidator(
		service.NewCanonicalBodyRegionIDValidator(),
	).WithAgentContextSources(database.NewTransactionManager(database.DB), equipmentRepo)
	bodyFactService := service.NewBodyFactService(bodyStateRepo, service.NewCanonicalBodyRegionIDValidator())
	bodyObservationService := service.NewBodyObservationService(bodyStateRepo)
	bodyHypothesisService := service.NewBodyHypothesisService(bodyStateRepo)
	episodeReadService := service.NewEpisodeReadService(bodyStateRepo)
	episodeCommandService := service.NewEpisodeCommandService(bodyStateRepo)
	lifestyleService := service.NewLifestyleService(bodyStateService)
	bodyMetricsService := service.NewBodyMetricsService(bodyStateService)
	healthHistoryService := service.NewHealthHistoryService(bodyStateService)
	reportedDiscomfortRepo := repository.NewReportedDiscomfortRepository(database.DB)
	onboardingContextService := service.NewOnboardingContextService(
		profileService,
		bodyStateService,
		database.NewTransactionManager(database.DB),
	).WithReportedDiscomfort(reportedDiscomfortRepo)
	reportedDiscomfortService := service.NewReportedDiscomfortService(reportedDiscomfortRepo)
	nativeDiagnosisReader := service.NewNativeDiagnosisReadService(repository.NewDiagnosisRepository(database.DB))
	// DX-003: Condition/run freshness is a derived, read-only review signal.
	// User-facing reads carry it; Agent inputs keep the committed projection.
	diagnosisFreshness := service.NewDiagnosisFreshnessService(database.DB)
	reviewedDiagnosisReader := service.NewReviewedNativeDiagnosisReader(nativeDiagnosisReader, diagnosisFreshness).
		WithReviewPromptDismissals(repository.NewDiagnosisRepository(database.DB))
	nativeDiagnosisRuns := service.NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(database.DB)).WithFreshness(diagnosisFreshness)
	treatmentService := service.NewTreatmentService(
		nativeDiagnosisReader,
		bodyStateService,
		profileService,
		aiClient,
		agentDeploymentPolicy,
	)
	treatmentService.AttachDiagnosisFreshness(diagnosisFreshness)
	outcomeService := service.NewOutcomeService(outcomeRepo, bodyStateService, database.NewTransactionManager(database.DB))
	assessmentRepo := repository.NewAssessmentRepository(database.DB)
	assessmentReplayService := service.NewAssessmentReplayService(assessmentRepo, aiClient)
	assessmentRolloutService := service.NewAssessmentRolloutService(
		repository.NewAssessmentRolloutRepository(database.DB),
		assessmentReplayService,
	)
	documentIndicatorReviewRepo := repository.NewDocumentIndicatorReviewRepository(database.DB)
	outputReviewRepo := repository.NewAIOutputReviewRepository(database.DB)
	outputReviewService := service.NewOutputReviewService(outputReviewRepo)
	uploadService := service.NewUploadService(userAssetRepo, documentExtractionRunRepo, postureAnalysisRunRepo, jobRuntime, outputReviewService, uploadStorage, apiConfig.Upload).
		WithDeployment(agentDeploymentPolicy).
		WithHealthDocumentDeployment(healthDocumentDeploymentPolicy)
	uploadService.StartUploadWorker(context.Background(), 10*time.Second, 10*time.Minute)
	// AS-002: per-asset erasure of the source and every controlled copy; the
	// worker resumes interrupted passes and retries transient target failures.
	// The ai-service erases and verifies the consultation runtime copies.
	assetErasureService := service.NewAssetErasureService(repository.NewAssetErasureRepository(database.DB), uploadService).
		WithRuntimeEraser(aiClient)
	assetErasureService.StartWorker(context.Background(), time.Minute)
	// AS-002: per-statement erasure of one user statement and the BodyState
	// records derived from it; frozen replay inputs are retained by policy.
	statementErasureService := service.NewStatementErasureService(repository.NewStatementErasureRepository(database.DB)).
		WithRuntimeEraser(aiClient)
	statementErasureService.StartWorker(context.Background(), time.Minute)
	// PC-RET-001: prune jobs run only when something enqueues them; a pass
	// moves no cursor until the ai-service checkpoint cleanup is verified.
	// Slice 3: Run settlement reconciles P and the token-budget C, and
	// enqueues a prune job once C - 1 and P are both past R.
	// S2: the HTTP proposer and the writer admission gate are wired only while
	// the default-off switch is on. With the switch off the trigger keeps its
	// previous inert shape (nil proposer, disabled writer, one log line, no
	// model traffic).
	chatRetentionService := service.NewChatRetentionService(repository.NewChatRetentionRepository(database.DB), aiClient, jobRuntime).
		WithContextTokenBudget(model.ChatContextTokenBudget(os.Getenv)).
		WithEpisodicDistillation(service.NewEpisodicDistillationTriggerFromEnv(
			os.Getenv,
			repository.NewEpisodicDistillationRepository(database.DB),
			episodicShadowWriterFromEnv(os.Getenv, database.DB),
			episodicShadowProposerFromEnv(os.Getenv, aiClient),
		))
	chatRetentionService.StartPruneWorker(context.Background(), time.Minute, 10*time.Minute)
	healthDocumentReviewService := service.NewHealthDocumentReviewService(documentExtractionRunRepo, documentIndicatorReviewRepo, userAssetRepo)
	assessmentService := service.NewAssessmentService(
		assessmentRepo,
		profileService,
		uploadService,
		bodyStateService,
		aiClient,
	).WithAssessmentDeployment(agentDeploymentPolicy).
		WithAssessmentRollout(assessmentRolloutService).
		WithAssessmentReviews(documentIndicatorReviewRepo)
	agentToolRepo := repository.NewAgentToolCallRepository(database.DB)
	agentToolService := service.NewAgentToolService(agentToolRepo)
	interactionRepo := repository.NewAgentInteractionRepository(database.DB)
	// An expired or cancelled Ask User settles its Run, so the expiry worker
	// fires the same retention settle trigger as every Run-ending path.
	interactionService := service.NewAgentInteractionService(interactionRepo, runService, conversationRepo, database.NewTransactionManager(database.DB)).
		WithLifecycleEvents(runtimeEventService).
		WithRetentionSettle(chatRetentionService)
	interactionService.StartInteractionExpiryWorker(context.Background(), time.Minute, nil)
	threadProjectionService := service.NewThreadProjectionService(conversationRepo, consultationRepo, messageRepo, interactionRepo, runtimeEventService, threadProjectionRepo)
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
		apiConfig.Consultation,
		bodyStateService,
	)
	consultationRuntime.AttachBodyFactService(bodyFactService)
	consultationRuntime.AttachChatRetention(chatRetentionService)
	consultationRuntime.AttachInteractionResumeService(service.NewInteractionResumeService(consultationRepo, database.NewTransactionManager(database.DB), runtimeEventService).WithStructuredIntake(bodyStateService).WithBodyStateSuggestions(bodyStateService))
	consultationRuntime.AttachBodyStateSuggestionSources(bodyStateRepo)
	consultationRuntime.AttachLongitudinalContextServices(
		contextRetrievalService,
		nativeDiagnosisReader,
		outcomeService,
	)
	// S4: the recall path is attached only when the default-off switch is on.
	// With it off the Runtime holds a nil recaller: no query, no payload field.
	if episodicrecall.EpisodicRecallEnabledFromEnv(os.Getenv) {
		consultationRuntime.AttachEpisodicRecall(
			episodicrecall.NewRecaller(repository.NewEpisodicRecallRepository(database.DB)),
		)
	}
	consultationRolloutRepo := repository.NewConsultationRolloutRepository(database.DB)
	consultationRuntime.AttachRolloutService(
		service.NewConsultationRolloutService(consultationRolloutRepo),
	)
	startRunLeaseReconciler(runService, conversationService, runtimeEventService, chatRetentionService)
	knowledgePublicationRepo := repository.NewKnowledgePublicationRepository(database.DB)
	knowledgeObservationRepo := repository.NewKnowledgePublicationObservationRepository(database.DB)
	consultationRuntime.AttachKnowledgeObservationService(
		service.NewKnowledgePublicationObservationService(
			knowledgePublicationRepo,
			knowledgeObservationRepo,
		),
	)
	consultationReplayService := service.NewConsultationReplayService(runRepo)
	trainingRepo := repository.NewTrainingRepository(database.DB)
	treatmentService.AttachPlanProposer(service.NewIndependentTrainingPlanService(database.DB))
	knowledgeIngestionService := service.NewKnowledgeIngestionService(
		knowledgeSourceRegistry,
		jobRuntime,
		agentDeploymentPolicy,
		apiConfig.AI.ServiceURL,
	)
	knowledgeIngestionService.StartWorker(context.Background(), 10*time.Second, 15*time.Minute)
	knowledgeQueryService := service.NewKnowledgeQueryService(apiConfig.AI.ServiceURL)

	// Continuous health workspace is the single capability/read model for the product loop.
	healthWorkspaceService := service.NewHealthWorkspaceService(
		profileService,
		consultationService,
		bodyStateService,
		reviewedDiagnosisReader,
		nativeDiagnosisRuns,
		outcomeService,
		trainingRepo,
	)

	// HTTP server. Host development defaults to loopback; container runtimes
	// explicitly set API_HOST=0.0.0.0 so the Docker network can reach it.
	host := os.Getenv("API_HOST")
	port := os.Getenv("API_PORT")
	listenAddress := resolveAPIListenAddress(host, port)

	r := newRouter(corsOrigins)

	// Protected routes. Keep one auth middleware instance so handwritten and
	// generated OpenAPI routes share the exact same token/session authority.
	authMiddleware := middleware.AuthMiddleware(supabaseVerifier, userRepo)
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
	publicAPISpec, err := openapiv1.GetSpec()
	if err != nil {
		log.Fatalf("failed to load generated public OpenAPI spec: %v", err)
	}
	httpapi.RegisterRoutes(
		r,
		httpapi.StrictHandler(httpapi.NewPublicServer().WithSafetyRestrictions(bodyStateRepo).WithEquipment(equipmentRepo).WithBodyFacts(bodyFactService).WithBodyObservations(bodyObservationService).WithBodyHypotheses(bodyHypothesisService).WithEpisodes(episodeReadService).WithEpisodeCommands(episodeCommandService).WithHealthWorkspace(healthWorkspaceService).WithHealthContext(lifestyleService, bodyMetricsService, healthHistoryService, onboardingContextService).WithReportedDiscomfort(reportedDiscomfortService).WithProfile(profileService).WithPrivacy(privacyErasureService).WithEpisodicNotesControls(repository.NewEpisodicNoteControlsRepository(database.DB), httpapi.EpisodicNotesControlsEnabledFromEnv(os.Getenv)).WithAssessment(assessmentService, assessmentReplayService).WithConversations(conversationService, runtimeEventService).WithConsultation(consultationRuntime, consultationService, interactionService, consultationReplayService, threadProjectionService).WithNativeDiagnosis(reviewedDiagnosisReader).WithNativeDiagnosisRuns(nativeDiagnosisRuns).WithNativeDiagnosisAnalysis(service.NewDiagnosisFormalAnalysisService(database.DB, profileService, aiClient, agentDeploymentPolicy), nativeDiagnosisRuns).WithNativeDiagnosisCommands(service.NewDiagnosisProposalDecisionService(database.DB), service.NewDiagnosisSettlementService(database.DB)).WithNativeDiagnosisDirectEdits(service.NewDiagnosisDirectEditService(database.DB)).WithNativeDiagnosisReview(service.NewDiagnosisAnalysisRequestService(database.DB), service.NewDiagnosisReviewPromptService(database.DB, reviewedDiagnosisReader)).WithOutcomes(outcomeService).WithPlanGeneration(treatmentService).WithTrainingPlans(service.NewIndependentTrainingPlanService(database.DB)).WithTrainerSessions(service.NewTrainerSessionService(database.DB)).WithUploads(uploadService, healthDocumentReviewService).WithAssetErasure(assetErasureService).WithStatementErasure(statementErasureService).WithKnowledge(knowledgeSourceRegistry, knowledgeIngestionService, knowledgeQueryService)),
		httpapi.RouteSecurity{
			Auth:      authMiddleware,
			Operator:  middleware.RequireKnowledgeOperator(userRepo),
			Validator: httpapi.RequestValidator(publicAPISpec),
		},
	)

	log.Printf("BodySense API starting on %s", listenAddress)
	if err := r.Run(listenAddress); err != nil {
		log.Fatal(err)
	}
}
