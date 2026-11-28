// Package config reads the process environment once, at the composition root,
// into the structures the API layers receive by injection. Services and the
// consultation runtime never consult the environment themselves.
//
// Only reading lives here. Values a deployment owns (AI endpoints, execution
// budgets) are resolved with their documented defaults; values a business
// policy owns (Agent deployment pointers, health-document stage) are handed
// over as trimmed strings so the policy keeps its defaults, validation and
// fail-closed messages in the package that owns the behaviour.
package config

import (
	"strconv"
	"strings"
	"time"
)

// Getenv reads one configuration value. The composition root passes os.Getenv.
type Getenv func(string) string

const (
	aiServiceURLEnv                 = "AI_SERVICE_URL"
	aiRuntimeTimeoutEnv             = "AI_RUNTIME_TIMEOUT_SECONDS"
	healthDocumentServiceURLEnv     = "HEALTH_DOCUMENT_SERVICE_URL"
	consultationExecutionTimeoutEnv = "CONSULTATION_EXECUTION_TIMEOUT_SECONDS"
	episodicProposalTimeoutEnv      = "EPISODIC_PROPOSAL_TIMEOUT_SECONDS"
	episodicPassBudgetEnv           = "EPISODIC_PASS_BUDGET_SECONDS"

	diagnosisChampionConfigurationIDEnv   = "DIAGNOSIS_CHAMPION_CONFIGURATION_ID"
	diagnosisChallengerConfigurationIDEnv = "DIAGNOSIS_CHALLENGER_CONFIGURATION_ID"
	diagnosisRolloutStageEnv              = "DIAGNOSIS_ROLLOUT_STAGE"
	diagnosisCanaryBPSEnv                 = "DIAGNOSIS_CANARY_BPS"
	diagnosisRolloutSaltEnv               = "DIAGNOSIS_ROLLOUT_SALT"
	diagnosisPromotionRecordEnv           = "DIAGNOSIS_PROMOTION_RECORD"

	treatmentChampionConfigurationIDEnv   = "TREATMENT_CHAMPION_CONFIGURATION_ID"
	treatmentChallengerConfigurationIDEnv = "TREATMENT_CHALLENGER_CONFIGURATION_ID"
	treatmentRolloutStageEnv              = "TREATMENT_ROLLOUT_STAGE"
	treatmentCanaryBPSEnv                 = "TREATMENT_CANARY_BPS"
	treatmentRolloutSaltEnv               = "TREATMENT_ROLLOUT_SALT"
	treatmentPromotionRecordEnv           = "TREATMENT_PROMOTION_RECORD"

	assessmentChampionConfigurationIDEnv   = "ASSESSMENT_CHAMPION_CONFIGURATION_ID"
	assessmentChallengerConfigurationIDEnv = "ASSESSMENT_CHALLENGER_CONFIGURATION_ID"
	assessmentRolloutStageEnv              = "ASSESSMENT_ROLLOUT_STAGE"
	assessmentCanaryBPSEnv                 = "ASSESSMENT_CANARY_BPS"
	assessmentRolloutSaltEnv               = "ASSESSMENT_ROLLOUT_SALT"
	assessmentPromotionRecordEnv           = "ASSESSMENT_PROMOTION_RECORD"

	consultationChampionConfigurationIDEnv = "CONSULTATION_CHAMPION_CONFIGURATION_ID"
	postureChampionConfigurationIDEnv      = "POSTURE_CHAMPION_CONFIGURATION_ID"
	knowledgeCuratorConfigurationIDEnv     = "KNOWLEDGE_CURATOR_CONFIGURATION_ID"
	knowledgeSplitterConfigurationIDEnv    = "KNOWLEDGE_SPLITTER_CONFIGURATION_ID"

	healthDocumentStageEnv                        = "HEALTH_DOCUMENT_STAGE"
	healthDocumentChampionConfigurationIDEnv      = "HEALTH_DOCUMENT_CHAMPION_CONFIGURATION_ID"
	healthDocumentQualificationConfigurationIDEnv = "HEALTH_DOCUMENT_QUALIFICATION_CONFIGURATION_ID"
	healthDocumentRollbackConfigurationIDEnv      = "HEALTH_DOCUMENT_ROLLBACK_CONFIGURATION_ID"

	altchaHMACKeyEnv               = "ALTCHA_HMAC_KEY"
	beforeUserCreatedHookSecretEnv = "BEFORE_USER_CREATED_HOOK_SECRET"
)

const (
	defaultAIServiceURL     = "http://localhost:8100"
	defaultAIRuntimeTimeout = 120 * time.Second
	defaultEpisodicProposal = 120 * time.Second
	defaultEpisodicPass     = 600 * time.Second
)

// DefaultConsultationExecutionTimeout is the durable execution budget used
// when no configuration was injected (a zero-value consultation runtime).
const DefaultConsultationExecutionTimeout = 5 * time.Minute

// AI holds the AI runtime client endpoint and stream budget.
type AI struct {
	ServiceURL string
	// RuntimeTimeout bounds the total read of the whole AI stream — every
	// serial model call plus the complete answer — not a single packet, so a
	// slow provider handling multi-turn work can legitimately exceed the
	// former fixed 120s.
	RuntimeTimeout time.Duration
}

// Upload holds the endpoints the upload worker calls for derived extraction
// and posture/posture-document analysis.
type Upload struct {
	AIServiceURL             string
	HealthDocumentServiceURL string
}

// Consultation holds the durable execution budget for one consultation run:
// every serial model call (diagnosis, structuring, knowledge tool rounds,
// final composition) plus all persistence.
type Consultation struct {
	ExecutionTimeout time.Duration
}

// Episodic holds the shadow-proposal deadlines used at the prune boundary. A
// proposal is a full model round trip, and one pass bounds the serial
// proposal calls plus ledger writes; both used to be fixed too low for a real
// call.
type Episodic struct {
	ProposalTimeout time.Duration
	PassBudget      time.Duration
}

// Deployment carries the Agent deployment pointers. The values are trimmed but
// otherwise unresolved: service.AgentDeploymentPolicy owns their defaults,
// validation and the fail-closed errors for an inconsistent rollout.
type Deployment struct {
	DiagnosisChampionConfigurationID   string
	DiagnosisChallengerConfigurationID string
	DiagnosisRolloutStage              string
	DiagnosisCanaryBPS                 string
	DiagnosisRolloutSalt               string
	DiagnosisPromotionRecord           string

	TreatmentChampionConfigurationID   string
	TreatmentChallengerConfigurationID string
	TreatmentRolloutStage              string
	TreatmentCanaryBPS                 string
	TreatmentRolloutSalt               string
	TreatmentPromotionRecord           string

	AssessmentChampionConfigurationID   string
	AssessmentChallengerConfigurationID string
	AssessmentRolloutStage              string
	AssessmentCanaryBPS                 string
	AssessmentRolloutSalt               string
	AssessmentPromotionRecord           string

	ConsultationChampionConfigurationID string
	PostureChampionConfigurationID      string
	KnowledgeCuratorConfigurationID     string
	KnowledgeSplitterConfigurationID    string
}

// HealthDocument carries the health-document rollout pointers with the same
// split as Deployment: reading here, stage defaults and validation in
// service.HealthDocumentDeploymentPolicy.
type HealthDocument struct {
	Stage                        string
	ChampionConfigurationID      string
	QualificationConfigurationID string
	RollbackConfigurationID      string
}

// Captcha carries the ALTCHA human-verification secrets: the HMAC key
// that signs and verifies proof-of-work challenges, and the Standard
// Webhooks secret Supabase signs Before User Created hook calls with.
// Both are trimmed but otherwise passed through unresolved —
// internal/captcha owns the fail-closed behaviour while either is
// absent or malformed.
type Captcha struct {
	HMACKey    string
	HookSecret string
}

// Config is the resolved process configuration injected by the composition
// root.
type Config struct {
	AI             AI
	Upload         Upload
	Consultation   Consultation
	Episodic       Episodic
	Deployment     Deployment
	HealthDocument HealthDocument
	Captcha        Captcha
}

// Load reads every configuration value the API composition root injects.
func Load(getenv Getenv) Config {
	// Endpoints are taken verbatim, as the callers always did: only an empty
	// value falls back.
	aiServiceURL := getenv(aiServiceURLEnv)
	if aiServiceURL == "" {
		aiServiceURL = defaultAIServiceURL
	}
	healthDocumentServiceURL := getenv(healthDocumentServiceURLEnv)
	if healthDocumentServiceURL == "" {
		healthDocumentServiceURL = aiServiceURL
	}

	return Config{
		AI: AI{
			ServiceURL:     aiServiceURL,
			RuntimeTimeout: seconds(getenv, aiRuntimeTimeoutEnv, defaultAIRuntimeTimeout),
		},
		Upload: Upload{
			AIServiceURL:             aiServiceURL,
			HealthDocumentServiceURL: healthDocumentServiceURL,
		},
		Consultation: Consultation{
			ExecutionTimeout: seconds(getenv, consultationExecutionTimeoutEnv, DefaultConsultationExecutionTimeout),
		},
		Episodic: Episodic{
			ProposalTimeout: seconds(getenv, episodicProposalTimeoutEnv, defaultEpisodicProposal),
			PassBudget:      seconds(getenv, episodicPassBudgetEnv, defaultEpisodicPass),
		},
		Deployment: Deployment{
			DiagnosisChampionConfigurationID:   value(getenv, diagnosisChampionConfigurationIDEnv),
			DiagnosisChallengerConfigurationID: value(getenv, diagnosisChallengerConfigurationIDEnv),
			DiagnosisRolloutStage:              value(getenv, diagnosisRolloutStageEnv),
			DiagnosisCanaryBPS:                 value(getenv, diagnosisCanaryBPSEnv),
			DiagnosisRolloutSalt:               value(getenv, diagnosisRolloutSaltEnv),
			DiagnosisPromotionRecord:           value(getenv, diagnosisPromotionRecordEnv),

			TreatmentChampionConfigurationID:   value(getenv, treatmentChampionConfigurationIDEnv),
			TreatmentChallengerConfigurationID: value(getenv, treatmentChallengerConfigurationIDEnv),
			TreatmentRolloutStage:              value(getenv, treatmentRolloutStageEnv),
			TreatmentCanaryBPS:                 value(getenv, treatmentCanaryBPSEnv),
			TreatmentRolloutSalt:               value(getenv, treatmentRolloutSaltEnv),
			TreatmentPromotionRecord:           value(getenv, treatmentPromotionRecordEnv),

			AssessmentChampionConfigurationID:   value(getenv, assessmentChampionConfigurationIDEnv),
			AssessmentChallengerConfigurationID: value(getenv, assessmentChallengerConfigurationIDEnv),
			AssessmentRolloutStage:              value(getenv, assessmentRolloutStageEnv),
			AssessmentCanaryBPS:                 value(getenv, assessmentCanaryBPSEnv),
			AssessmentRolloutSalt:               value(getenv, assessmentRolloutSaltEnv),
			AssessmentPromotionRecord:           value(getenv, assessmentPromotionRecordEnv),

			ConsultationChampionConfigurationID: value(getenv, consultationChampionConfigurationIDEnv),
			PostureChampionConfigurationID:      value(getenv, postureChampionConfigurationIDEnv),
			KnowledgeCuratorConfigurationID:     value(getenv, knowledgeCuratorConfigurationIDEnv),
			KnowledgeSplitterConfigurationID:    value(getenv, knowledgeSplitterConfigurationIDEnv),
		},
		HealthDocument: HealthDocument{
			Stage:                        value(getenv, healthDocumentStageEnv),
			ChampionConfigurationID:      value(getenv, healthDocumentChampionConfigurationIDEnv),
			QualificationConfigurationID: value(getenv, healthDocumentQualificationConfigurationIDEnv),
			RollbackConfigurationID:      value(getenv, healthDocumentRollbackConfigurationIDEnv),
		},
		Captcha: Captcha{
			HMACKey:    value(getenv, altchaHMACKeyEnv),
			HookSecret: value(getenv, beforeUserCreatedHookSecretEnv),
		},
	}
}

func value(getenv Getenv, name string) string {
	return strings.TrimSpace(getenv(name))
}

// seconds resolves a positive whole-second duration. Unset, non-numeric, zero
// and negative values keep the caller's safe default.
func seconds(getenv Getenv, name string, fallback time.Duration) time.Duration {
	raw := value(getenv, name)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return time.Duration(parsed) * time.Second
}
