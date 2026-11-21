package main

import (
	"log"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/database"
)

// initInfrastructure connects the process to its external dependencies in
// startup order: database, migrations, Redis and the hosted identity verifier.
// It also resolves the CORS allowlist and identity configuration the transport
// and erasure layers need.
func initInfrastructure() ([]string, auth.SupabaseConfig, *auth.SupabaseVerifier) {
	// Reject invalid privacy-gate configuration before DB/Redis connections,
	// migrations, background workers or external services are initialized.
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
	if _, err := database.ConnectRedis(redisCfg); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}

	corsOrigins := parseCORSOrigins()
	// Hosted Supabase is the only identity authority. Go verifies its ES256
	// access tokens and keeps the existing local health-data ownership boundary.
	supabaseConfig, err := auth.SupabaseConfigFromEnv()
	if err != nil {
		log.Fatalf("Supabase Auth configuration failed: %v", err)
	}
	supabaseVerifier, err := auth.NewSupabaseVerifier(supabaseConfig.URL, nil)
	if err != nil {
		log.Fatalf("Supabase JWT verifier failed: %v", err)
	}

	return corsOrigins, supabaseConfig, supabaseVerifier
}
