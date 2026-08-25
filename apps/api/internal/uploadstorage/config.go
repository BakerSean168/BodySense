package uploadstorage

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Environment         string
	Backend             string
	LocalRoot           string
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
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Environment:         envOr("APP_ENV", envOr("ENVIRONMENT", "development")),
		Backend:             envOr("UPLOAD_STORAGE_BACKEND", "local"),
		LocalRoot:           envOr("UPLOAD_STORAGE_LOCAL_ROOT", "uploads"),
		OSSRegion:           envOr("UPLOAD_OSS_REGION", "cn-hangzhou"),
		OSSBucket:           strings.TrimSpace(os.Getenv("UPLOAD_OSS_BUCKET")),
		OSSEndpoint:         strings.TrimSpace(os.Getenv("UPLOAD_OSS_ENDPOINT")),
		OSSUseInternal:      envBool("UPLOAD_OSS_USE_INTERNAL_ENDPOINT", true),
		OSSCredentialType:   envOr("UPLOAD_OSS_CREDENTIAL_TYPE", "ecs_ram_role"),
		OSSECSRAMRole:       strings.TrimSpace(os.Getenv("UPLOAD_OSS_ECS_RAM_ROLE")),
		OSSAccessKeyID:      envOr("ALIBABA_CLOUD_ACCESS_KEY_ID", os.Getenv("OSS_ACCESS_KEY_ID")),
		OSSAccessKeySecret:  envOr("ALIBABA_CLOUD_ACCESS_KEY_SECRET", os.Getenv("OSS_ACCESS_KEY_SECRET")),
		OSSSecurityToken:    envOr("ALIBABA_CLOUD_SECURITY_TOKEN", os.Getenv("OSS_SECURITY_TOKEN")),
		OSSPrefix:           strings.Trim(envOr("UPLOAD_OSS_PREFIX", "bodysense/production/uploads"), "/"),
		OSSServerEncryption: envOr("UPLOAD_OSS_SERVER_SIDE_ENCRYPTION", "AES256"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch c.Backend {
	case "local", "oss":
	default:
		return fmt.Errorf("unsupported UPLOAD_STORAGE_BACKEND %q", c.Backend)
	}
	if strings.TrimSpace(c.LocalRoot) == "" {
		return errors.New("UPLOAD_STORAGE_LOCAL_ROOT must not be empty")
	}
	ossConfigured := c.Backend == "oss" || strings.TrimSpace(c.OSSBucket) != ""
	if !ossConfigured {
		return nil
	}
	if strings.TrimSpace(c.OSSBucket) == "" {
		return errors.New("UPLOAD_OSS_BUCKET is required when OSS upload storage is configured")
	}
	if strings.TrimSpace(c.OSSRegion) == "" {
		return errors.New("UPLOAD_OSS_REGION is required when OSS upload storage is configured")
	}
	if strings.TrimSpace(c.OSSPrefix) == "" {
		return errors.New("UPLOAD_OSS_PREFIX must not be empty")
	}
	if strings.EqualFold(c.Environment, "production") && c.OSSCredentialType != "ecs_ram_role" {
		return errors.New("production upload OSS credentials must use ecs_ram_role")
	}
	switch c.OSSCredentialType {
	case "ecs_ram_role", "environment":
	case "static":
		if c.OSSAccessKeyID == "" || c.OSSAccessKeySecret == "" {
			return errors.New("static upload OSS credentials require access key id and secret")
		}
	default:
		return fmt.Errorf("unsupported UPLOAD_OSS_CREDENTIAL_TYPE %q", c.OSSCredentialType)
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
