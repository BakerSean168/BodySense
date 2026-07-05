package database

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

// RedisClient is the global Redis client instance.
var RedisClient *redis.Client

// RedisConfig holds Redis connection parameters.
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// RedisConfigFromEnv reads Redis config from environment variables.
func RedisConfigFromEnv() RedisConfig {
	return RedisConfig{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     getEnv("REDIS_PORT", "6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	}
}

// ConnectRedis initializes the Redis connection.
func ConnectRedis(cfg RedisConfig) (*redis.Client, error) {
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	// Enable TLS for managed Redis/Valkey (e.g. DigitalOcean)
	if os.Getenv("REDIS_TLS") == "true" {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	RedisClient = client
	log.Println("Redis connected successfully")
	return client, nil
}
