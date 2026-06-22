package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/handler"
	"github.com/bodysense/api/internal/middleware"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
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
	authService := service.NewAuthService(userRepo, jwtConfig)
	profileService := service.NewProfileService(profileRepo)
	uploadService := service.NewUploadService(uploadRepo)
	consultationService := service.NewConsultationService(consultationRepo)
	authHandler := handler.NewAuthHandler(authService)
	profileHandler := handler.NewProfileHandler(profileService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	consultationHandler := handler.NewConsultationHandler(consultationService, profileService)
	knowledgeHandler := handler.NewKnowledgeHandler()

	// HTTP server
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
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
	protected.Use(middleware.AuthMiddleware(jwtConfig))
	{
		protected.GET("/me", authHandler.Me)
		protected.GET("/profile", profileHandler.GetProfile)
		protected.PUT("/profile", profileHandler.CreateOrUpdateProfile)

		// Upload routes
		protected.POST("/uploads", uploadHandler.Upload)
		protected.GET("/uploads", uploadHandler.GetUploads)
		protected.GET("/uploads/:id", uploadHandler.GetUpload)
		protected.DELETE("/uploads/:id", uploadHandler.DeleteUpload)

		// Consultation routes
		protected.POST("/consultation", consultationHandler.CreateSession)
		protected.GET("/consultation", consultationHandler.ListSessions)
		protected.GET("/consultation/:id", consultationHandler.GetSession)
		protected.POST("/consultation/:id/message", consultationHandler.SendMessage)
		protected.PUT("/consultation/:id/extracted-info", consultationHandler.UpdateExtractedInfo)
		protected.PUT("/consultation/:id/confirm", consultationHandler.ConfirmDiagnosis)
	}

	// Knowledge base routes (proxy to AI service)
	knowledgeGroup := r.Group("/api/knowledge")
	{
		knowledgeGroup.POST("/entries", knowledgeHandler.AddEntry)
		knowledgeGroup.POST("/search", knowledgeHandler.SearchKnowledge)
		knowledgeGroup.GET("/entries/:id", knowledgeHandler.GetEntry)
		knowledgeGroup.DELETE("/entries/:id", knowledgeHandler.DeleteEntry)
		knowledgeGroup.GET("/stats", knowledgeHandler.GetStats)
	}

	log.Printf("BodySense API starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}
