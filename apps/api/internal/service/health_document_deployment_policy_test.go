package service

import "testing"

func clearHealthDocumentDeploymentEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HEALTH_DOCUMENT_STAGE", "")
	t.Setenv("HEALTH_DOCUMENT_CHAMPION_CONFIGURATION_ID", "")
	t.Setenv("HEALTH_DOCUMENT_QUALIFICATION_CONFIGURATION_ID", "")
	t.Setenv("HEALTH_DOCUMENT_ROLLBACK_CONFIGURATION_ID", "")
}

func TestHealthDocumentDeploymentDefaultsToFrozenTesseractChampion(t *testing.T) {
	clearHealthDocumentDeploymentEnv(t)
	policy, err := NewHealthDocumentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.ConfigurationID(); got != legacyTesseractConfigurationID {
		t.Fatalf("configuration = %q", got)
	}
	if policy.Stage() != HealthDocumentStageChampion {
		t.Fatalf("stage = %q", policy.Stage())
	}
}

func TestHealthDocumentQualificationStageUsesBenchmarkGatedCandidate(t *testing.T) {
	clearHealthDocumentDeploymentEnv(t)
	t.Setenv("HEALTH_DOCUMENT_STAGE", HealthDocumentStageQualification)
	policy, err := NewHealthDocumentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.ConfigurationID(); got != healthDocumentCandidateConfigurationID {
		t.Fatalf("qualification configuration = %q", got)
	}
}

func TestHealthDocumentRollbackUsesFrozenTesseract(t *testing.T) {
	clearHealthDocumentDeploymentEnv(t)
	t.Setenv("HEALTH_DOCUMENT_STAGE", HealthDocumentStageRollback)
	policy, err := NewHealthDocumentDeploymentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.ConfigurationID(); got != legacyTesseractConfigurationID {
		t.Fatalf("rollback configuration = %q", got)
	}
}

func TestHealthDocumentDeploymentRejectsInvalidStageAndPointerRoles(t *testing.T) {
	t.Run("stage", func(t *testing.T) {
		clearHealthDocumentDeploymentEnv(t)
		t.Setenv("HEALTH_DOCUMENT_STAGE", "canary")
		if _, err := NewHealthDocumentDeploymentPolicy(); err == nil {
			t.Fatal("expected invalid stage")
		}
	})
	t.Run("unqualified candidate cannot masquerade as champion", func(t *testing.T) {
		clearHealthDocumentDeploymentEnv(t)
		t.Setenv("HEALTH_DOCUMENT_CHAMPION_CONFIGURATION_ID", healthDocumentCandidateConfigurationID)
		if _, err := NewHealthDocumentDeploymentPolicy(); err == nil {
			t.Fatal("qualification candidate must not masquerade as champion")
		}
	})
	t.Run("champion cannot masquerade as qualification candidate", func(t *testing.T) {
		clearHealthDocumentDeploymentEnv(t)
		t.Setenv("HEALTH_DOCUMENT_QUALIFICATION_CONFIGURATION_ID", legacyTesseractConfigurationID)
		if _, err := NewHealthDocumentDeploymentPolicy(); err == nil {
			t.Fatal("current champion must not masquerade as qualification candidate")
		}
	})
}
