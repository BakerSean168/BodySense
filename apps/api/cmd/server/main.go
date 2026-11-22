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

// serverWiring is the composition root's assembled dependency set. main()
// builds it in phases (repositories and infrastructure, core services, the
// consultation runtime, then HTTP serving); each phase only reads fields the
// earlier phases filled.
type serverWiring struct {
	apiConfig        config.Config
	corsOrigins      []string
	supabaseConfig   auth.SupabaseConfig
	supabaseVerifier *auth.SupabaseVerifier

	userRepo             *repository.UserRepository
	profileRepo          *repository.ProfileRepository
	consultationRepo     *repository.ConsultationRepository
	bodyStateRepo        *repository.BodyStateRepository
	conversationRepo     *repository.ConversationRepository
	messageRepo          *repository.MessageRepository
	runRepo              *repository.RunRepository
	runtimeEventRepo     *repository.RuntimeEventRepository
	threadProjectionRepo *repository.ThreadProjectionRepository
	equipmentRepo        *repository.EquipmentAssetRepository
	uploadStorage        *uploadstorage.Registry
	leaseOwner           string

	knowledgeSourceRegistry        *service.KnowledgeSourceRegistry
	privacyErasureService          *service.PrivacyErasureService
	profileService                 *service.ProfileService
	jobRuntime                     *service.JobRuntime
	aiClient                       *service.AIClient
	agentDeploymentPolicy          *service.AgentDeploymentPolicy
	healthDocumentDeploymentPolicy *service.HealthDocumentDeploymentPolicy
	contextRetrievalService        *service.ContextRetrievalService
	runService                     *service.RunService
	runtimeEventService            *service.RuntimeEventService
	conversationService            *service.ConversationService
	consultationService            *service.ConsultationService
	bodyStateService               *service.BodyStateService
	bodyFactService                *service.BodyFactService
	bodyObservationService         *service.BodyObservationService
	bodyHypothesisService          *service.BodyHypothesisService
	episodeReadService             *service.EpisodeReadService
	episodeCommandService          *service.EpisodeCommandService
	lifestyleService               *service.LifestyleService
	bodyMetricsService             *service.BodyMetricsService
	healthHistoryService           *service.HealthHistoryService
	onboardingContextService       *service.OnboardingContextService
	reportedDiscomfortService      *service.ReportedDiscomfortService
	nativeDiagnosisReader          *service.NativeDiagnosisReadService
	reviewedDiagnosisReader        *service.ReviewedNativeDiagnosisReader
	nativeDiagnosisRuns            *service.NativeDiagnosisRunReadService
	treatmentService               *service.TreatmentService
	outcomeService                 *service.OutcomeService
	assessmentReplayService        *service.AssessmentReplayService
	assessmentRolloutService       *service.AssessmentRolloutService
	outputReviewService            *service.OutputReviewService
	uploadService                  *service.UploadService
	assetErasureService            *service.AssetErasureService
	statementErasureService        *service.StatementErasureService
	chatRetentionService           *service.ChatRetentionService
	healthDocumentReviewService    *service.HealthDocumentReviewService
	assessmentService              *service.AssessmentService
	interactionService             *service.AgentInteractionService
	threadProjectionService        *service.ThreadProjectionService
	consultationRuntime            *consultationruntime.Runtime
	consultationReplayService      *service.ConsultationReplayService
	knowledgeIngestionService      *service.KnowledgeIngestionService
	knowledgeQueryService          *service.KnowledgeQueryService
	healthWorkspaceService         *service.HealthWorkspaceService
}

func main() {
	// Load .env file from project root (ignore error in production where env is injected)
	_ = godotenv.Load("../../.env")

	// Structured process logger must be installed before service initialization so startup
	// failures and standard-library log.Printf calls share the same JSON log stream.
	observability.ConfigureLogger()

	w := newServerWiring()
	w.buildCoreServices()
	w.buildConsultationRuntime()
	w.serve()
}

// newServerWiring resolves the process configuration and infrastructure, then
// constructs the repositories every service phase shares.
func newServerWiring() *serverWiring {
	w := &serverWiring{}

	// The composition root is the only place the API reads the process
	// environment; every layer below receives the resolved values.
	w.apiConfig = config.Load(os.Getenv)

	w.corsOrigins, w.supabaseConfig, w.supabaseVerifier = initInfrastructure()

	// Initialize dependencies
	w.userRepo = repository.NewUserRepository(database.DB)
	knowledgeSourceRepo := repository.NewKnowledgeSourceRepository(database.DB)
	w.knowledgeSourceRegistry = service.NewKnowledgeSourceRegistry(knowledgeSourceRepo)
	w.profileRepo = repository.NewProfileRepository(database.DB)
	uploadStorage, err := uploadstorage.NewRegistryFromEnv()
	if err != nil {
		log.Fatalf("Upload storage configuration failed: %v", err)
	}
	w.uploadStorage = uploadStorage
	uploadStorageCtx, uploadStorageCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer uploadStorageCancel()
	if err := uploadStorage.Validate(uploadStorageCtx); err != nil {
		log.Fatalf("Upload storage validation failed: %v", err)
	}
	w.leaseOwner = uuid.NewString()
	w.consultationRepo = repository.NewConsultationRepository(database.DB, w.leaseOwner)
	w.bodyStateRepo = repository.NewBodyStateRepository(database.DB)
	w.conversationRepo = repository.NewConversationRepository(database.DB)
	w.messageRepo = repository.NewMessageRepository(database.DB)
	w.runRepo = repository.NewRunRepository(database.DB)
	w.runtimeEventRepo = repository.NewRuntimeEventRepository(database.DB)
	w.threadProjectionRepo = repository.NewThreadProjectionRepository(database.DB)
	w.equipmentRepo = repository.NewEquipmentAssetRepository(database.DB)
	return w
}

// buildCoreServices constructs the domain services and starts their workers.
func (w *serverWiring) buildCoreServices() {
	userAssetRepo := repository.NewUserAssetRepository(database.DB)
	documentExtractionRunRepo := repository.NewDocumentExtractionRunRepository(database.DB)
	postureAnalysisRunRepo := repository.NewPostureAnalysisRunRepository(database.DB)
	outcomeRepo := repository.NewOutcomeRepository(database.DB)
	messageContextRepo := repository.NewMessageContextRepository(database.DB)

	// Hosted identity revocation is required before a full account erasure
	// can be accepted. The server-only admin key is never exposed to the web.
	authAdmin := auth.NewSupabaseAdminRevoker(w.supabaseConfig, nil)
	privacyErasureRepo := repository.NewPrivacyErasureRepository(database.DB)
	w.privacyErasureService = service.NewPrivacyErasureService(
		privacyErasureRepo,
		w.userRepo,
		authAdmin,
		w.uploadStorage,
		database.NewTransactionManager(database.DB),
	)
	w.privacyErasureService.StartWorker(context.Background(), time.Minute)
	w.profileService = service.NewProfileService(w.profileRepo)
	jobRepo := repository.NewJobRepository(database.DB)
	w.jobRuntime = service.NewJobRuntime(jobRepo)
	w.aiClient = service.NewAIClient(w.apiConfig.AI)
	agentDeploymentPolicy, err := service.NewAgentDeploymentPolicy(w.apiConfig.Deployment)
	if err != nil {
		log.Fatalf("failed to configure Agent deployment policy: %v", err)
	}
	w.agentDeploymentPolicy = agentDeploymentPolicy
	healthDocumentDeploymentPolicy, err := service.NewHealthDocumentDeploymentPolicy(w.apiConfig.HealthDocument)
	if err != nil {
		log.Fatalf("failed to configure health-document deployment policy: %v", err)
	}
	w.healthDocumentDeploymentPolicy = healthDocumentDeploymentPolicy
	w.contextRetrievalService = service.NewContextRetrievalService(messageContextRepo)
	w.runService = service.NewRunService(w.runRepo, w.leaseOwner)
	w.runtimeEventService = service.NewRuntimeEventService(w.runtimeEventRepo)
	w.runService.WithLifecycleEvents(w.runtimeEventService, database.NewTransactionManager(database.DB))
	w.conversationService = service.NewConversationService(w.conversationRepo, w.messageRepo, w.runRepo, database.NewTransactionManager(database.DB))
	w.consultationService = service.NewConsultationService(w.consultationRepo, w.conversationRepo)
	w.bodyStateService = service.NewBodyStateService(w.bodyStateRepo).WithBodyRegionIDValidator(
		service.NewCanonicalBodyRegionIDValidator(),
	).WithAgentContextSources(database.NewTransactionManager(database.DB), w.equipmentRepo)
	w.bodyFactService = service.NewBodyFactService(w.bodyStateRepo, service.NewCanonicalBodyRegionIDValidator())
	w.bodyObservationService = service.NewBodyObservationService(w.bodyStateRepo)
	w.bodyHypothesisService = service.NewBodyHypothesisService(w.bodyStateRepo)
	w.episodeReadService = service.NewEpisodeReadService(w.bodyStateRepo)
	w.episodeCommandService = service.NewEpisodeCommandService(w.bodyStateRepo)
	w.lifestyleService = service.NewLifestyleService(w.bodyStateService)
	w.bodyMetricsService = service.NewBodyMetricsService(w.bodyStateService)
	w.healthHistoryService = service.NewHealthHistoryService(w.bodyStateService)
	reportedDiscomfortRepo := repository.NewReportedDiscomfortRepository(database.DB)
	w.onboardingContextService = service.NewOnboardingContextService(
		w.profileService,
		w.bodyStateService,
		database.NewTransactionManager(database.DB),
	).WithReportedDiscomfort(reportedDiscomfortRepo)
	w.reportedDiscomfortService = service.NewReportedDiscomfortService(reportedDiscomfortRepo)
	w.nativeDiagnosisReader = service.NewNativeDiagnosisReadService(repository.NewDiagnosisRepository(database.DB))
	// DX-003: Condition/run freshness is a derived, read-only review signal.
	// User-facing reads carry it; Agent inputs keep the committed projection.
	diagnosisFreshness := service.NewDiagnosisFreshnessService(database.DB)
	w.reviewedDiagnosisReader = service.NewReviewedNativeDiagnosisReader(w.nativeDiagnosisReader, diagnosisFreshness).
		WithReviewPromptDismissals(repository.NewDiagnosisRepository(database.DB))
	w.nativeDiagnosisRuns = service.NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(database.DB)).WithFreshness(diagnosisFreshness)
	w.treatmentService = service.NewTreatmentService(
		w.nativeDiagnosisReader,
		w.bodyStateService,
		w.profileService,
		w.aiClient,
		w.agentDeploymentPolicy,
	)
	w.treatmentService.AttachDiagnosisFreshness(diagnosisFreshness)
	w.outcomeService = service.NewOutcomeService(outcomeRepo, w.bodyStateService, database.NewTransactionManager(database.DB))
	assessmentRepo := repository.NewAssessmentRepository(database.DB)
	w.assessmentReplayService = service.NewAssessmentReplayService(assessmentRepo, w.aiClient)
	w.assessmentRolloutService = service.NewAssessmentRolloutService(
		repository.NewAssessmentRolloutRepository(database.DB),
		w.assessmentReplayService,
	)
	documentIndicatorReviewRepo := repository.NewDocumentIndicatorReviewRepository(database.DB)
	outputReviewRepo := repository.NewAIOutputReviewRepository(database.DB)
	w.outputReviewService = service.NewOutputReviewService(outputReviewRepo)
	w.uploadService = service.NewUploadService(userAssetRepo, documentExtractionRunRepo, postureAnalysisRunRepo, w.jobRuntime, w.outputReviewService, w.uploadStorage, w.apiConfig.Upload).
		WithDeployment(w.agentDeploymentPolicy).
		WithHealthDocumentDeployment(w.healthDocumentDeploymentPolicy)
	w.uploadService.StartUploadWorker(context.Background(), 10*time.Second, 10*time.Minute)
	// AS-002: per-asset erasure of the source and every controlled copy; the
	// worker resumes interrupted passes and retries transient target failures.
	// The ai-service erases and verifies the consultation runtime copies.
	w.assetErasureService = service.NewAssetErasureService(repository.NewAssetErasureRepository(database.DB), w.uploadService).
		WithRuntimeEraser(w.aiClient)
	w.assetErasureService.StartWorker(context.Background(), time.Minute)
	// AS-002: per-statement erasure of one user statement and the BodyState
	// records derived from it; frozen replay inputs are retained by policy.
	w.statementErasureService = service.NewStatementErasureService(repository.NewStatementErasureRepository(database.DB)).
		WithRuntimeEraser(w.aiClient)
	w.statementErasureService.StartWorker(context.Background(), time.Minute)
	// PC-RET-001: prune jobs run only when something enqueues them; a pass
	// moves no cursor until the ai-service checkpoint cleanup is verified.
	// Slice 3: Run settlement reconciles P and the token-budget C, and
	// enqueues a prune job once C - 1 and P are both past R.
	// S2: the HTTP proposer and the writer admission gate are wired only while
	// the default-off switch is on. With the switch off the trigger keeps its
	// previous inert shape (nil proposer, disabled writer, one log line, no
	// model traffic).
	w.chatRetentionService = service.NewChatRetentionService(repository.NewChatRetentionRepository(database.DB), w.aiClient, w.jobRuntime).
		WithContextTokenBudget(model.ChatContextTokenBudget(os.Getenv)).
		WithEpisodicDistillation(service.NewEpisodicDistillationTriggerFromEnv(
			os.Getenv,
			repository.NewEpisodicDistillationRepository(database.DB),
			episodicShadowWriterFromEnv(os.Getenv, database.DB),
			episodicShadowProposerFromEnv(os.Getenv, w.aiClient),
		))
	w.chatRetentionService.StartPruneWorker(context.Background(), time.Minute, 10*time.Minute)
	w.healthDocumentReviewService = service.NewHealthDocumentReviewService(documentExtractionRunRepo, documentIndicatorReviewRepo, userAssetRepo)
	w.assessmentService = service.NewAssessmentService(
		assessmentRepo,
		w.profileService,
		w.uploadService,
		w.bodyStateService,
		w.aiClient,
	).WithAssessmentDeployment(w.agentDeploymentPolicy).
		WithAssessmentRollout(w.assessmentRolloutService).
		WithAssessmentReviews(documentIndicatorReviewRepo)
}

// buildConsultationRuntime assembles the interaction service, the thread
// projection and the consultation runtime with all of its attachments.
func (w *serverWiring) buildConsultationRuntime() {
	agentToolRepo := repository.NewAgentToolCallRepository(database.DB)
	agentToolService := service.NewAgentToolService(agentToolRepo)
	messageService := service.NewMessageService(w.messageRepo)
	interactionRepo := repository.NewAgentInteractionRepository(database.DB)
	// An expired or cancelled Ask User settles its Run, so the expiry worker
	// fires the same retention settle trigger as every Run-ending path.
	w.interactionService = service.NewAgentInteractionService(interactionRepo, w.runService, w.conversationRepo, database.NewTransactionManager(database.DB)).
		WithLifecycleEvents(w.runtimeEventService).
		WithRetentionSettle(w.chatRetentionService)
	w.interactionService.StartInteractionExpiryWorker(context.Background(), time.Minute, nil)
	w.threadProjectionService = service.NewThreadProjectionService(w.conversationRepo, w.consultationRepo, w.messageRepo, interactionRepo, w.runtimeEventService, w.threadProjectionRepo)
	w.consultationRuntime = consultationruntime.NewRuntime(
		w.conversationService,
		w.consultationService,
		w.profileService,
		messageService,
		w.runService,
		w.aiClient,
		agentToolService,
		w.interactionService,
		w.outputReviewService,
		w.threadProjectionService,
		w.runtimeEventService,
		w.uploadService,
		w.agentDeploymentPolicy,
		w.apiConfig.Consultation,
		w.bodyStateService,
	)
	w.consultationRuntime.AttachBodyFactService(w.bodyFactService)
	w.consultationRuntime.AttachChatRetention(w.chatRetentionService)
	w.consultationRuntime.AttachInteractionResumeService(service.NewInteractionResumeService(w.consultationRepo, database.NewTransactionManager(database.DB), w.runtimeEventService).WithStructuredIntake(w.bodyStateService).WithBodyStateSuggestions(w.bodyStateService))
	w.consultationRuntime.AttachBodyStateSuggestionSources(w.bodyStateRepo)
	w.consultationRuntime.AttachLongitudinalContextServices(
		w.contextRetrievalService,
		w.nativeDiagnosisReader,
		w.outcomeService,
	)
	// S4: the recall path is attached only when the default-off switch is on.
	// With it off the Runtime holds a nil recaller: no query, no payload field.
	if episodicrecall.EpisodicRecallEnabledFromEnv(os.Getenv) {
		w.consultationRuntime.AttachEpisodicRecall(
			episodicrecall.NewRecaller(repository.NewEpisodicRecallRepository(database.DB)),
		)
	}
	consultationRolloutRepo := repository.NewConsultationRolloutRepository(database.DB)
	w.consultationRuntime.AttachRolloutService(
		service.NewConsultationRolloutService(consultationRolloutRepo),
	)
	startRunLeaseReconciler(w.runService, w.conversationService, w.runtimeEventService, w.chatRetentionService)
	knowledgePublicationRepo := repository.NewKnowledgePublicationRepository(database.DB)
	knowledgeObservationRepo := repository.NewKnowledgePublicationObservationRepository(database.DB)
	w.consultationRuntime.AttachKnowledgeObservationService(
		service.NewKnowledgePublicationObservationService(
			knowledgePublicationRepo,
			knowledgeObservationRepo,
		),
	)
	w.consultationReplayService = service.NewConsultationReplayService(w.runRepo)
	trainingRepo := repository.NewTrainingRepository(database.DB)
	w.treatmentService.AttachPlanProposer(service.NewIndependentTrainingPlanService(database.DB))
	w.knowledgeIngestionService = service.NewKnowledgeIngestionService(
		w.knowledgeSourceRegistry,
		w.jobRuntime,
		w.agentDeploymentPolicy,
		w.apiConfig.AI.ServiceURL,
	)
	w.knowledgeIngestionService.StartWorker(context.Background(), 10*time.Second, 15*time.Minute)
	w.knowledgeQueryService = service.NewKnowledgeQueryService(w.apiConfig.AI.ServiceURL)

	// Continuous health workspace is the single capability/read model for the product loop.
	w.healthWorkspaceService = service.NewHealthWorkspaceService(
		w.profileService,
		w.consultationService,
		w.bodyStateService,
		w.reviewedDiagnosisReader,
		w.nativeDiagnosisRuns,
		w.outcomeService,
		trainingRepo,
	)
}

// serve registers the HTTP routes on the wired services and runs the server.
func (w *serverWiring) serve() {
	// HTTP server. Host development defaults to loopback; container runtimes
	// explicitly set API_HOST=0.0.0.0 so the Docker network can reach it.
	host := os.Getenv("API_HOST")
	port := os.Getenv("API_PORT")
	listenAddress := resolveAPIListenAddress(host, port)

	r := newRouter(w.corsOrigins)

	// Protected routes. Keep one auth middleware instance so handwritten and
	// generated OpenAPI routes share the exact same token/session authority.
	authMiddleware := middleware.AuthMiddleware(w.supabaseVerifier, w.userRepo)
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
		httpapi.StrictHandler(httpapi.NewPublicServer().WithSafetyRestrictions(w.bodyStateRepo).WithEquipment(w.equipmentRepo).WithBodyFacts(w.bodyFactService).WithBodyObservations(w.bodyObservationService).WithBodyHypotheses(w.bodyHypothesisService).WithEpisodes(w.episodeReadService).WithEpisodeCommands(w.episodeCommandService).WithHealthWorkspace(w.healthWorkspaceService).WithHealthContext(w.lifestyleService, w.bodyMetricsService, w.healthHistoryService, w.onboardingContextService).WithReportedDiscomfort(w.reportedDiscomfortService).WithProfile(w.profileService).WithPrivacy(w.privacyErasureService).WithEpisodicNotesControls(repository.NewEpisodicNoteControlsRepository(database.DB), httpapi.EpisodicNotesControlsEnabledFromEnv(os.Getenv)).WithAssessment(w.assessmentService, w.assessmentReplayService).WithConversations(w.conversationService, w.runtimeEventService).WithConsultation(w.consultationRuntime, w.consultationService, w.interactionService, w.consultationReplayService, w.threadProjectionService).WithNativeDiagnosis(w.reviewedDiagnosisReader).WithNativeDiagnosisRuns(w.nativeDiagnosisRuns).WithNativeDiagnosisAnalysis(service.NewDiagnosisFormalAnalysisService(database.DB, w.profileService, w.aiClient, w.agentDeploymentPolicy), w.nativeDiagnosisRuns).WithNativeDiagnosisCommands(service.NewDiagnosisProposalDecisionService(database.DB), service.NewDiagnosisSettlementService(database.DB)).WithNativeDiagnosisDirectEdits(service.NewDiagnosisDirectEditService(database.DB)).WithNativeDiagnosisReview(service.NewDiagnosisAnalysisRequestService(database.DB), service.NewDiagnosisReviewPromptService(database.DB, w.reviewedDiagnosisReader)).WithOutcomes(w.outcomeService).WithPlanGeneration(w.treatmentService).WithTrainingPlans(service.NewIndependentTrainingPlanService(database.DB)).WithTrainerSessions(service.NewTrainerSessionService(database.DB)).WithUploads(w.uploadService, w.healthDocumentReviewService).WithAssetErasure(w.assetErasureService).WithStatementErasure(w.statementErasureService).WithKnowledge(w.knowledgeSourceRegistry, w.knowledgeIngestionService, w.knowledgeQueryService)),
		httpapi.RouteSecurity{
			Auth:      authMiddleware,
			Operator:  middleware.RequireKnowledgeOperator(w.userRepo),
			Validator: httpapi.RequestValidator(publicAPISpec),
		},
	)

	log.Printf("BodySense API starting on %s", listenAddress)
	if err := r.Run(listenAddress); err != nil {
		log.Fatal(err)
	}
}
