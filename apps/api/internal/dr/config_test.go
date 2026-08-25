package dr

import (
	"strings"
	"testing"
)

func TestConfigRejectsFilesystemDriverInProduction(t *testing.T) {
	cfg := Config{
		Environment: "production", Driver: "filesystem", FilesystemRoot: t.TempDir(),
		OSSPrefix: "bodysense/production/postgres", DBPassword: "secret", ReleaseRevision: strings.Repeat("a", 40),
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "forbidden in production") {
		t.Fatalf("expected production filesystem rejection, got %v", err)
	}
}

func TestConfigRequiresExplicitOSSBucket(t *testing.T) {
	cfg := Config{
		Environment: "production", Driver: "oss", OSSRegion: "cn-hangzhou",
		OSSCredentialType: "ecs_ram_role", OSSPrefix: "bodysense/production/postgres",
		DBPassword: "secret", ReleaseRevision: strings.Repeat("b", 40),
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "DR_OSS_BUCKET") {
		t.Fatalf("expected bucket requirement, got %v", err)
	}
}

func TestConfigRequiresECSRAMRoleInProduction(t *testing.T) {
	cfg := Config{
		Environment: "production", Driver: "oss", OSSRegion: "cn-hangzhou", OSSBucket: "private-dr",
		OSSCredentialType: "static", OSSPrefix: "bodysense/production/postgres",
		OSSAccessKeyID: "id", OSSAccessKeySecret: "secret", DBPassword: "secret", ReleaseRevision: strings.Repeat("d", 40),
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "ecs_ram_role") {
		t.Fatalf("expected ECS RAM role requirement, got %v", err)
	}
}
