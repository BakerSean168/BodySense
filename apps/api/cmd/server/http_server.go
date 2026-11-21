package main

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/bodysense/api/internal/database"
	"github.com/gin-contrib/requestid"
	ginslog "github.com/gin-contrib/slog"
	"github.com/gin-gonic/gin"
)

// newRouter assembles the base gin engine: request id, structured request
// logging, recovery, trusted-proxy validation, the credentialed CORS policy
// and the health probe. Route groups are registered by the caller.
func newRouter(corsOrigins []string) *gin.Engine {
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

	return r
}
