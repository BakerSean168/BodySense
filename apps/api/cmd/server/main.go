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
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/workflow"
	"github.com/gin-gonic/gin"
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

	// JWT config
	jwtConfig := auth.JWTConfigFromEnv()

	// Initialize dependencies
	userRepo := repository.NewUserRepository(database.DB)
	profileRepo := repository.NewProfileRepository(database.DB)
	uploadRepo := repository.NewUploadRepository(database.DB)
	consultationRepo := repository.NewConsultationRepository(database.DB)
	conversationRepo := repository.NewConversationRepository(database.DB)
	messageRepo := repository.NewMessageRepository(database.DB)
	runRepo := repository.NewRunRepository(database.DB)
	runtimeEventRepo := repository.NewRuntimeEventRepository(database.DB)
	threadProjectionRepo := repository.NewThreadProjectionRepository(database.DB)
	shareRepo := repository.NewConversationShareRepository(database.DB)

	// User session cache (Redis-backed, TTL = 2x access token TTL)
	sessionCache := cache.NewUserSessionCache(database.RedisClient, jwtConfig.AccessTokenTTL*2)

	authService := service.NewAuthService(userRepo, jwtConfig, sessionCache)
	profileService := service.NewProfileService(profileRepo)
	jobRepo := repository.NewJobRepository(database.DB)
	jobRuntime := service.NewJobRuntime(jobRepo)
	uploadService := service.NewUploadService(uploadRepo, jobRuntime)
	uploadService.StartOCRWorker(context.Background(), 10*time.Second, 10*time.Minute)
	aiClient := service.NewAIClient()
	messageService := service.NewMessageService(messageRepo)
	runService := service.NewRunService(runRepo)
	runtimeEventService := service.NewRuntimeEventService(runtimeEventRepo)
	conversationService := service.NewConversationService(conversationRepo, messageRepo, runRepo, shareRepo, aiClient)
	shareService := service.NewShareService(conversationRepo, messageRepo, shareRepo)
	consultationService := service.NewConsultationService(consultationRepo, conversationRepo)
	assessmentRepo := repository.NewAssessmentRepository(database.DB)
	assessmentService := service.NewAssessmentService(assessmentRepo, profileService, uploadRepo)
	authHandler := handler.NewAuthHandler(authService)
	profileHandler := handler.NewProfileHandler(profileService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	agentToolRepo := repository.NewAgentToolCallRepository(database.DB)
	agentToolService := service.NewAgentToolService(agentToolRepo)
	interactionRepo := repository.NewAgentInteractionRepository(database.DB)
	interactionService := service.NewAgentInteractionService(interactionRepo, runRepo)
	threadProjectionService := service.NewThreadProjectionService(conversationRepo, consultationRepo, messageRepo, interactionRepo, runtimeEventService, threadProjectionRepo)
	outputReviewRepo := repository.NewAIOutputReviewRepository(database.DB)
	outputReviewService := service.NewOutputReviewService(outputReviewRepo)
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
	)
	convHandler := handler.NewConversationHandler(conversationService, shareService)
	runtimeEventHandler := handler.NewRuntimeEventHandler(runtimeEventService, conversationService)
	threadProjectionHandler := handler.NewThreadProjectionHandler(threadProjectionService)
	consultationHandler := handler.NewConsultationHandler(
		consultationService,
		interactionService,
		consultationRuntime,
	)
	diagnosisHandler := handler.NewDiagnosisHandler(consultationService, profileService, aiClient)
	trainingRepo := repository.NewTrainingRepository(database.DB)
	trainingService := service.NewTrainingService(trainingRepo, profileService)
	trainingHandler := handler.NewTrainingHandler(trainingService)
	reassessmentHandler := handler.NewReassessmentHandler(trainingService)
	assessmentHandler := handler.NewAssessmentHandler(assessmentService)
	knowledgeHandler := handler.NewKnowledgeHandler()

	// Health journey (read-only workflow)
	journeyWorkflow := workflow.NewHealthJourneyWorkflow(profileRepo, uploadRepo, consultationRepo, assessmentRepo, trainingRepo)
	journeyHandler := handler.NewHealthJourneyHandler(journeyWorkflow)

	// HTTP server
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()

	// CORS middleware
	corsOrigins := parseCORSOrigins()
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", resolveCORSOrigin(c.Request.Header.Get("Origin"), corsOrigins))
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
		protected.GET("/profile", profileHandler.GetProfile)
		protected.PUT("/profile", profileHandler.CreateOrUpdateProfile)

		// Upload routes
		protected.POST("/uploads", uploadHandler.Upload)
		protected.GET("/uploads", uploadHandler.GetUploads)
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
		consultations := protected.Group("/consultations")
		consultations.GET("/:id", consultationHandler.GetConsultation)
		consultations.GET("/:id/thread", threadProjectionHandler.GetConsultationThread)

		consultations.PUT("/:id/extracted-info", consultationHandler.UpdateExtractedInfo)
		consultations.PUT("/:id/confirm", consultationHandler.ConfirmDiagnosis)
		consultations.POST("/:id/diagnosis", diagnosisHandler.AnalyzeDiagnosis)
		consultations.POST("/:id/treatment", diagnosisHandler.GenerateTreatment)
		consultations.POST("/:id/interrupts/:interactionId/answers", consultationHandler.ResumeInteraction)

		// Health journey (read-only)
		protected.GET("/journey", journeyHandler.GetJourneyState)

		// Assessment routes
		protected.POST("/assessment/generate", assessmentHandler.GenerateAssessment)
		protected.GET("/assessment", assessmentHandler.ListReports)
		protected.GET("/assessment/:id", assessmentHandler.GetReport)

		// Training routes
		protected.POST("/training/generate", trainingHandler.GeneratePlan)
		protected.GET("/training", trainingHandler.ListPlans)
		protected.GET("/training/:id", trainingHandler.GetPlan)
		protected.GET("/training/:id/today", trainingHandler.GetTodayTask)
		protected.POST("/training/:id/checkin", trainingHandler.CheckIn)
		protected.PUT("/training/:id/log", trainingHandler.UpdateLog)
		protected.PUT("/training/:id/phases", trainingHandler.UpdatePlanPhases)
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
			return "*"
		}
		if requestOrigin != "" && origin == requestOrigin {
			return requestOrigin
		}
	}
	return allowed[0]
}
