package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance.
var DB *gorm.DB

// Config holds database connection parameters.
type Config struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

// ConfigFromEnv reads database config from environment variables.
func ConfigFromEnv() Config {
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		log.Fatal("DB_PASSWORD environment variable is required")
	}

	return Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		Name:     getEnv("DB_NAME", "bodysense"),
		User:     getEnv("DB_USER", "bodysense"),
		Password: password,
		SSLMode:  getEnv("DB_SSLMODE", "require"),
	}
}

// Connect initializes the database connection.
func Connect(cfg Config) (*gorm.DB, error) {
	sslMode := getEnv("DB_SSLMODE", "require")
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, sslMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(databaseLogLevel()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Connection pool settings
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	DB = db
	log.Println("Database connected successfully")
	return db, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func databaseLogLevel() logger.LogLevel {
	if configured := strings.ToLower(strings.TrimSpace(os.Getenv("DB_LOG_LEVEL"))); configured != "" {
		switch configured {
		case "silent":
			return logger.Silent
		case "error":
			return logger.Error
		case "warn", "warning":
			return logger.Warn
		case "info":
			return logger.Info
		}
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "staging", "production":
		// Avoid writing consultation/body-state payloads through GORM's SQL
		// interpolation in shared environments. Structured domain diagnostics
		// remain available separately.
		return logger.Warn
	default:
		return logger.Info
	}
}
