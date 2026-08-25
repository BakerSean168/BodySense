package uploadstorage

import "testing"

func TestConfigProductionOSSRequiresECSRAMRole(t *testing.T) {
	base := Config{
		Environment: "production",
		Backend:     "oss",
		LocalRoot:   "uploads",
		OSSRegion:   "cn-hangzhou",
		OSSBucket:   "private-bucket",
		OSSPrefix:   "bodysense/production/uploads",
	}
	for _, credentialType := range []string{"static", "environment"} {
		cfg := base
		cfg.OSSCredentialType = credentialType
		cfg.OSSAccessKeyID = "id"
		cfg.OSSAccessKeySecret = "secret"
		if err := cfg.Validate(); err == nil {
			t.Fatalf("production credential type %q should be rejected", credentialType)
		}
	}
	cfg := base
	cfg.OSSCredentialType = "ecs_ram_role"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("ecs ram role should be accepted: %v", err)
	}
}

func TestConfigLocalBackendDoesNotRequireOSSProvisioning(t *testing.T) {
	cfg := Config{Environment: "production", Backend: "local", LocalRoot: "uploads"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local compatibility backend should remain available before cutover: %v", err)
	}
}
