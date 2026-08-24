package dr

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment         string
	Driver              string
	FilesystemRoot      string
	OSSRegion           string
	OSSBucket           string
	OSSEndpoint         string
	OSSUseInternal      bool
	OSSCredentialType   string
	OSSECSRAMRole       string
	OSSAccessKeyID      string
	OSSAccessKeySecret  string
	OSSSecurityToken    string
	OSSPrefix           string
	OSSServerEncryption string
	DBHost              string
	DBPort              string
	DBName              string
	DBUser              string
	DBPassword          string
	DBSSLMode           string
	ReleaseRevision     string
	TempRoot            string
	DomainValidatorPath string
	MaxBackupAge        time.Duration
}

func ConfigFromEnv() (Config, error) {
	maxAgeHours, err := strconv.Atoi(envOr("DR_MAX_BACKUP_AGE_HOURS", "30"))
	if err != nil || maxAgeHours <= 0 {
		return Config{}, errors.New("DR_MAX_BACKUP_AGE_HOURS must be a positive integer")
	}
	cfg := Config{
		Environment:         envOr("APP_ENV", envOr("ENVIRONMENT", "development")),
		Driver:              envOr("DR_OBJECT_STORE_DRIVER", "oss"),
		FilesystemRoot:      os.Getenv("DR_FILESYSTEM_ROOT"),
		OSSRegion:           envOr("DR_OSS_REGION", "cn-hangzhou"),
		OSSBucket:           os.Getenv("DR_OSS_BUCKET"),
		OSSEndpoint:         os.Getenv("DR_OSS_ENDPOINT"),
		OSSUseInternal:      envBool("DR_OSS_USE_INTERNAL_ENDPOINT", true),
		OSSCredentialType:   envOr("DR_OSS_CREDENTIAL_TYPE", "ecs_ram_role"),
		OSSECSRAMRole:       os.Getenv("DR_OSS_ECS_RAM_ROLE"),
		OSSAccessKeyID:      envOr("ALIBABA_CLOUD_ACCESS_KEY_ID", os.Getenv("OSS_ACCESS_KEY_ID")),
		OSSAccessKeySecret:  envOr("ALIBABA_CLOUD_ACCESS_KEY_SECRET", os.Getenv("OSS_ACCESS_KEY_SECRET")),
		OSSSecurityToken:    envOr("ALIBABA_CLOUD_SECURITY_TOKEN", os.Getenv("OSS_SECURITY_TOKEN")),
		OSSPrefix:           strings.Trim(envOr("DR_OSS_PREFIX", "bodysense/production/postgres"), "/"),
		OSSServerEncryption: envOr("DR_OSS_SERVER_SIDE_ENCRYPTION", "AES256"),
		DBHost:              envOr("DB_HOST", "127.0.0.1"),
		DBPort:              envOr("DB_PORT", "5432"),
		DBName:              envOr("DB_NAME", "bodysense"),
		DBUser:              envOr("DB_USER", "bodysense"),
		DBPassword:          os.Getenv("DB_PASSWORD"),
		DBSSLMode:           envOr("DB_SSLMODE", "disable"),
		ReleaseRevision:     os.Getenv("DR_RELEASE_REVISION"),
		TempRoot:            envOr("DR_TEMP_ROOT", os.TempDir()),
		DomainValidatorPath: envOr("DR_DOMAIN_VALIDATOR_PATH", "/app/domain-validator"),
		MaxBackupAge:        time.Duration(maxAgeHours) * time.Hour,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.DBPassword == "" {
		return errors.New("DB_PASSWORD is required")
	}
	if c.ReleaseRevision == "" {
		return errors.New("DR_RELEASE_REVISION is required")
	}
	if c.OSSPrefix == "" {
		return errors.New("DR_OSS_PREFIX must not be empty")
	}
	switch c.Driver {
	case "filesystem":
		if strings.EqualFold(c.Environment, "production") {
			return errors.New("filesystem DR object store is forbidden in production")
		}
		if c.FilesystemRoot == "" {
			return errors.New("DR_FILESYSTEM_ROOT is required for filesystem driver")
		}
		abs, err := filepath.Abs(c.FilesystemRoot)
		if err != nil {
			return fmt.Errorf("resolve DR_FILESYSTEM_ROOT: %w", err)
		}
		if abs == string(filepath.Separator) {
			return errors.New("DR_FILESYSTEM_ROOT may not be filesystem root")
		}
	case "oss":
		if strings.EqualFold(c.Environment, "production") && c.OSSCredentialType != "ecs_ram_role" {
			return errors.New("production OSS credentials must use ecs_ram_role")
		}
		if c.OSSBucket == "" {
			return errors.New("DR_OSS_BUCKET is required for oss driver")
		}
		if c.OSSRegion == "" {
			return errors.New("DR_OSS_REGION is required for oss driver")
		}
		switch c.OSSCredentialType {
		case "ecs_ram_role", "environment":
		case "static":
			if c.OSSAccessKeyID == "" || c.OSSAccessKeySecret == "" {
				return errors.New("static OSS credentials require access key id and secret")
			}
		default:
			return fmt.Errorf("unsupported DR_OSS_CREDENTIAL_TYPE %q", c.OSSCredentialType)
		}
	default:
		return fmt.Errorf("unsupported DR_OBJECT_STORE_DRIVER %q", c.Driver)
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
