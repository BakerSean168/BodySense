package config

import (
	"testing"
	"time"
)

func envFrom(values map[string]string) Getenv {
	return func(name string) string { return values[name] }
}

func TestLoadDefaultsWithoutEnvironment(t *testing.T) {
	cfg := Load(envFrom(nil))

	if cfg.AI.ServiceURL != "http://localhost:8100" {
		t.Fatalf("AI service URL = %q", cfg.AI.ServiceURL)
	}
	if cfg.AI.RuntimeTimeout != 120*time.Second {
		t.Fatalf("AI runtime timeout = %s", cfg.AI.RuntimeTimeout)
	}
	if cfg.Upload.AIServiceURL != cfg.AI.ServiceURL {
		t.Fatalf("upload AI service URL = %q, want %q", cfg.Upload.AIServiceURL, cfg.AI.ServiceURL)
	}
	if cfg.Upload.HealthDocumentServiceURL != cfg.AI.ServiceURL {
		t.Fatalf("health-document URL = %q, want the AI service URL %q", cfg.Upload.HealthDocumentServiceURL, cfg.AI.ServiceURL)
	}
	const wantConsultationTimeout = 5 * time.Minute
	if DefaultConsultationExecutionTimeout != wantConsultationTimeout {
		t.Fatalf("default consultation execution timeout = %s", DefaultConsultationExecutionTimeout)
	}
	if cfg.Consultation.ExecutionTimeout != wantConsultationTimeout {
		t.Fatalf("consultation execution timeout = %s", cfg.Consultation.ExecutionTimeout)
	}
	if cfg.Episodic.ProposalTimeout != 120*time.Second || cfg.Episodic.PassBudget != 600*time.Second {
		t.Fatalf("episodic budgets = %s/%s", cfg.Episodic.ProposalTimeout, cfg.Episodic.PassBudget)
	}
	if cfg.Deployment.DiagnosisRolloutStage != "" || cfg.HealthDocument.Stage != "" {
		t.Fatalf("deployment pointers must stay unresolved: %#v", cfg)
	}
}

// Unset, non-numeric, zero and negative values keep the safe default, exactly
// as the per-package resolvers did before the read moved here.
func TestLoadResolvesBudgets(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
		want  time.Duration
		got   func(Config) time.Duration
	}{
		{"ai unset", aiRuntimeTimeoutEnv, "", 120 * time.Second, func(c Config) time.Duration { return c.AI.RuntimeTimeout }},
		{"ai override", aiRuntimeTimeoutEnv, "300", 300 * time.Second, func(c Config) time.Duration { return c.AI.RuntimeTimeout }},
		{"ai zero", aiRuntimeTimeoutEnv, "0", 120 * time.Second, func(c Config) time.Duration { return c.AI.RuntimeTimeout }},
		{"ai negative", aiRuntimeTimeoutEnv, "-30", 120 * time.Second, func(c Config) time.Duration { return c.AI.RuntimeTimeout }},
		{"ai non numeric", aiRuntimeTimeoutEnv, "two-minutes", 120 * time.Second, func(c Config) time.Duration { return c.AI.RuntimeTimeout }},

		{"consultation unset", consultationExecutionTimeoutEnv, "", 5 * time.Minute, func(c Config) time.Duration { return c.Consultation.ExecutionTimeout }},
		{"consultation override", consultationExecutionTimeoutEnv, "600", 10 * time.Minute, func(c Config) time.Duration { return c.Consultation.ExecutionTimeout }},
		{"consultation zero", consultationExecutionTimeoutEnv, "0", 5 * time.Minute, func(c Config) time.Duration { return c.Consultation.ExecutionTimeout }},
		{"consultation negative", consultationExecutionTimeoutEnv, "-30", 5 * time.Minute, func(c Config) time.Duration { return c.Consultation.ExecutionTimeout }},
		{"consultation non numeric", consultationExecutionTimeoutEnv, "ten-minutes", 5 * time.Minute, func(c Config) time.Duration { return c.Consultation.ExecutionTimeout }},

		{"proposal unset", episodicProposalTimeoutEnv, "", 120 * time.Second, func(c Config) time.Duration { return c.Episodic.ProposalTimeout }},
		{"proposal override", episodicProposalTimeoutEnv, "180", 180 * time.Second, func(c Config) time.Duration { return c.Episodic.ProposalTimeout }},
		{"proposal zero", episodicProposalTimeoutEnv, "0", 120 * time.Second, func(c Config) time.Duration { return c.Episodic.ProposalTimeout }},
		{"proposal negative", episodicProposalTimeoutEnv, "-30", 120 * time.Second, func(c Config) time.Duration { return c.Episodic.ProposalTimeout }},
		{"proposal non numeric", episodicProposalTimeoutEnv, "two-minutes", 120 * time.Second, func(c Config) time.Duration { return c.Episodic.ProposalTimeout }},

		{"pass budget unset", episodicPassBudgetEnv, "", 600 * time.Second, func(c Config) time.Duration { return c.Episodic.PassBudget }},
		{"pass budget override", episodicPassBudgetEnv, "900", 900 * time.Second, func(c Config) time.Duration { return c.Episodic.PassBudget }},
		{"pass budget zero", episodicPassBudgetEnv, "0", 600 * time.Second, func(c Config) time.Duration { return c.Episodic.PassBudget }},
		{"pass budget negative", episodicPassBudgetEnv, "-30", 600 * time.Second, func(c Config) time.Duration { return c.Episodic.PassBudget }},
		{"pass budget non numeric", episodicPassBudgetEnv, "ten-minutes", 600 * time.Second, func(c Config) time.Duration { return c.Episodic.PassBudget }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got(Load(envFrom(map[string]string{tc.key: tc.value}))); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.key, got, tc.want)
			}
		})
	}
}

func TestLoadEndpoints(t *testing.T) {
	cfg := Load(envFrom(map[string]string{
		aiServiceURLEnv:             "http://ai.internal:8100",
		healthDocumentServiceURLEnv: "http://documents.internal:8101",
	}))
	if cfg.AI.ServiceURL != "http://ai.internal:8100" || cfg.Upload.AIServiceURL != cfg.AI.ServiceURL {
		t.Fatalf("AI endpoints = %#v", cfg.AI)
	}
	if cfg.Upload.HealthDocumentServiceURL != "http://documents.internal:8101" {
		t.Fatalf("health-document endpoint = %q", cfg.Upload.HealthDocumentServiceURL)
	}
}

// Deployment and health-document pointers are only trimmed here: their
// defaults and validation stay with the policy that owns them.
func TestLoadTrimsDeploymentPointersOnly(t *testing.T) {
	cfg := Load(envFrom(map[string]string{
		diagnosisRolloutStageEnv:               " Canary ",
		diagnosisCanaryBPSEnv:                  " 2500 ",
		consultationChampionConfigurationIDEnv: "  consult-config-2bd9b46735dd693c  ",
		healthDocumentStageEnv:                 " qualification ",
	}))
	if cfg.Deployment.DiagnosisRolloutStage != "Canary" {
		t.Fatalf("stage must stay case-preserving for the policy: %q", cfg.Deployment.DiagnosisRolloutStage)
	}
	if cfg.Deployment.DiagnosisCanaryBPS != "2500" {
		t.Fatalf("canary bps = %q", cfg.Deployment.DiagnosisCanaryBPS)
	}
	if cfg.Deployment.ConsultationChampionConfigurationID != "consult-config-2bd9b46735dd693c" {
		t.Fatalf("consultation champion = %q", cfg.Deployment.ConsultationChampionConfigurationID)
	}
	if cfg.HealthDocument.Stage != "qualification" {
		t.Fatalf("health-document stage = %q", cfg.HealthDocument.Stage)
	}
}

func TestLoadReadsCaptchaSecrets(t *testing.T) {
	cfg := Load(envFrom(map[string]string{
		"ALTCHA_HMAC_KEY":                 "  altcha-key  ",
		"BEFORE_USER_CREATED_HOOK_SECRET": "v1,whsec_c2VjcmV0",
	}))
	if cfg.Captcha.HMACKey != "altcha-key" {
		t.Fatalf("expected trimmed ALTCHA HMAC key, got %q", cfg.Captcha.HMACKey)
	}
	if cfg.Captcha.HookSecret != "v1,whsec_c2VjcmV0" {
		t.Fatalf("expected hook secret passed through, got %q", cfg.Captcha.HookSecret)
	}

	empty := Load(envFrom(map[string]string{}))
	if empty.Captcha.HMACKey != "" || empty.Captcha.HookSecret != "" {
		t.Fatalf("expected captcha secrets to default empty, got %+v", empty.Captcha)
	}
}
