package service

import (
	"fmt"
	"os"
	"strings"
)

const (
	healthDocumentCandidateConfigurationID = "hdex-config-f2495c95b6ed9de2"
	legacyTesseractConfigurationID         = "hdex-config-14af808ef184bf8b"
	defaultHealthDocumentChampionID        = legacyTesseractConfigurationID

	HealthDocumentStageChampion      = "champion"
	HealthDocumentStageQualification = "qualification"
	HealthDocumentStageRollback      = "rollback"
)

type healthDocumentModelArtifact struct {
	Role     string
	Filename string
	SHA256   string
}

type healthDocumentVerifierArtifact struct {
	Language string
	SHA256   string
}

type healthDocumentConfigurationRegistration struct {
	MechanismRevision               string
	OutputSchemaRevision            string
	ExecutionTopologyRevision       string
	PDFStrategyRevision             string
	NativeTextEngine                string
	NativeTextEngineVersion         string
	NativeTextQualityPolicyRevision string
	NativeTextQualityPolicySHA256   string
	OCREngine                       string
	OCREngineVersion                string
	RuntimeEngine                   string
	RuntimeVersion                  string
	ModelFamily                     string
	ModelType                       string
	ModelArtifacts                  []healthDocumentModelArtifact
	PDFRasterDPI                    int
	DetectorLimitType               string
	DetectorLimitSideLen            int
	GlobalMaxSideLen                int
	RecBatchNum                     int
	ClsBatchNum                     int
	ORTIntraOpNumThreads            int
	ORTInterOpNumThreads            int
	IndicatorParserRevision         string
	IndicatorParserSHA256           string
	AdmissibilityPolicyRevision     string
	AdmissibilityPolicySHA256       string
	EngineAdapterSHA256             string
	WorkerSHA256                    string
	VerificationRevision            string
	VerifierEngine                  string
	VerifierEngineVersion           string
	VerifierLanguages               []string
	VerifierLanguageArtifacts       []healthDocumentVerifierArtifact
	VerifierPSM                     int
	VerifierStrategyRevision        string
	VerifierAdapterSHA256           string
	VerifierWorkerSHA256            string
	VerificationPolicySHA256        string
	OrchestratorSHA256              string
	ServingAllowed                  bool
	ChampionEligible                bool
	QualificationEligible           bool
	RollbackEligible                bool
	Legacy                          bool
	Wrapper                         string
	WrapperVersion                  string
	Languages                       []string
	OCRServiceSHA256                string
}

var knownHealthDocumentConfigurations = map[string]healthDocumentConfigurationRegistration{
	healthDocumentCandidateConfigurationID: {
		MechanismRevision:               "health-document-extraction-v20",
		OutputSchemaRevision:            "health-document-output-v2",
		ExecutionTopologyRevision:       "primary-then-verifier-subprocess-v1",
		PDFStrategyRevision:             "native-text-first-v1",
		NativeTextEngine:                "pymupdf",
		NativeTextEngineVersion:         "1.28.0",
		NativeTextQualityPolicyRevision: "health-document-native-text-quality-v1",
		NativeTextQualityPolicySHA256:   "c594a92d70679ef0da41a21c1fdf520a2feaec6e081adc6d67509be1db9fd09d",
		OCREngine:                       "rapidocr",
		OCREngineVersion:                "3.9.2",
		RuntimeEngine:                   "onnxruntime",
		RuntimeVersion:                  "1.29.0",
		ModelFamily:                     "PP-OCRv6",
		ModelType:                       "small",
		ModelArtifacts: []healthDocumentModelArtifact{
			{Role: "det", Filename: "PP-OCRv6_det_small.onnx", SHA256: "090f04abcd9d9a7498bc4ebf677e4cb9bdce1fe4197ddb7e529f1ef44e1ff94f"},
			{Role: "rec", Filename: "PP-OCRv6_rec_small.onnx", SHA256: "6f327246b50388f3c176ae304bd95767ea6dc0c9ae92153ef8cbe210b3c14884"},
			{Role: "cls", Filename: "ch_ppocr_mobile_v2.0_cls_mobile.onnx", SHA256: "e47acedf663230f8863ff1ab0e64dd2d82b838fceb5957146dab185a89d6215c"},
		},
		PDFRasterDPI:                150,
		DetectorLimitType:           "max",
		DetectorLimitSideLen:        608,
		GlobalMaxSideLen:            960,
		RecBatchNum:                 1,
		ClsBatchNum:                 1,
		ORTIntraOpNumThreads:        1,
		ORTInterOpNumThreads:        1,
		IndicatorParserRevision:     "health-indicator-parser-v3-table-rows",
		IndicatorParserSHA256:       "eec0ec8da623f6b05855a3e27bcb19868936b05a99c0520450882f27572b34f9",
		AdmissibilityPolicyRevision: "ocr-indicator-admissibility-v2",
		AdmissibilityPolicySHA256:   "5d67fa3d0dfa96a915c8392db6e0004be2b87b508df106a2eb17a84fd4eda79b",
		EngineAdapterSHA256:         "89918e6983eb8b82c5b6d8ebe1edbfdf6f95e8d61526c4bdf45f983500984db6",
		WorkerSHA256:                "19b60effa34f5dc9daa0ece053d9a13957340c424aee5fbc3376fa4b9a4a6e14",
		VerificationRevision:        "health-document-row-verification-v7-percent-unit-normalization",
		VerifierEngine:              "tesseract",
		VerifierEngineVersion:       "5.5.0",
		VerifierLanguages:           []string{"chi_sim", "eng"},
		VerifierLanguageArtifacts: []healthDocumentVerifierArtifact{
			{Language: "chi_sim", SHA256: "a5fcb6f0db1e1d6d8522f39db4e848f05984669172e584e8d76b6b3141e1f730"},
			{Language: "eng", SHA256: "7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2"},
		},
		VerifierPSM:              6,
		VerifierStrategyRevision: "full-ocr-page-tsv-geometry-v6-percent-unit-normalization",
		VerifierAdapterSHA256:    "b212c823b6cc7bf76bd0666f656f969208566c55d9355514bf02bd2c29800507",
		VerifierWorkerSHA256:     "a88514650cf6e9e294c657059d37890da71bbc751c35fcb984accd533c33ba03",
		VerificationPolicySHA256: "e62b2aed48cd0350519da5bad1858b10e2c50f4d3f5e9c662061285424d5e88d",
		OrchestratorSHA256:       "579a07181c3e003d59ab8ac01120bd84080d408495989857947c8f373015c72c",
		ServingAllowed:           true,
		QualificationEligible:    true,
	},
	legacyTesseractConfigurationID: {
		MechanismRevision:           "health-document-tesseract-baseline-v1",
		ExecutionTopologyRevision:   "per-document-subprocess-v1",
		PDFStrategyRevision:         "raster-all-pages-300dpi-v1",
		OCREngine:                   "tesseract",
		OCREngineVersion:            "5.5.0",
		Wrapper:                     "pytesseract",
		WrapperVersion:              "0.3.13",
		Languages:                   []string{"chi_sim", "eng"},
		PDFRasterDPI:                300,
		IndicatorParserRevision:     "legacy-regex-v1",
		IndicatorParserSHA256:       "22d6e1c8689557c8b5a878004a647b428950b47c44602c958ce1cb54e383eec7",
		AdmissibilityPolicyRevision: "ocr-indicator-admissibility-v1",
		OCRServiceSHA256:            "cd31066a2be9f80e38a00c636f10d31bdf83c5b21165c8084acb9cb7cde60220",
		ServingAllowed:              true,
		ChampionEligible:            true,
		RollbackEligible:            true,
		Legacy:                      true,
	},
}

type HealthDocumentDeploymentPolicy struct {
	stage                        string
	championConfigurationID      string
	qualificationConfigurationID string
	rollbackConfigurationID      string
}

func NewHealthDocumentDeploymentPolicy() (*HealthDocumentDeploymentPolicy, error) {
	stage := strings.TrimSpace(os.Getenv("HEALTH_DOCUMENT_STAGE"))
	if stage == "" {
		stage = HealthDocumentStageChampion
	}
	if stage != HealthDocumentStageChampion &&
		stage != HealthDocumentStageQualification &&
		stage != HealthDocumentStageRollback {
		return nil, fmt.Errorf("invalid HEALTH_DOCUMENT_STAGE %q", stage)
	}

	champion := strings.TrimSpace(os.Getenv("HEALTH_DOCUMENT_CHAMPION_CONFIGURATION_ID"))
	if champion == "" {
		champion = defaultHealthDocumentChampionID
	}
	registration, ok := knownHealthDocumentConfigurations[champion]
	if !ok || !registration.ServingAllowed || !registration.ChampionEligible {
		return nil, fmt.Errorf("invalid health-document champion configuration id %q", champion)
	}

	qualification := strings.TrimSpace(os.Getenv("HEALTH_DOCUMENT_QUALIFICATION_CONFIGURATION_ID"))
	if qualification == "" {
		qualification = healthDocumentCandidateConfigurationID
	}
	registration, ok = knownHealthDocumentConfigurations[qualification]
	if !ok || !registration.ServingAllowed || !registration.QualificationEligible {
		return nil, fmt.Errorf("invalid health-document qualification configuration id %q", qualification)
	}

	rollback := strings.TrimSpace(os.Getenv("HEALTH_DOCUMENT_ROLLBACK_CONFIGURATION_ID"))
	if rollback == "" {
		rollback = legacyTesseractConfigurationID
	}
	registration, ok = knownHealthDocumentConfigurations[rollback]
	if !ok || !registration.ServingAllowed || !registration.RollbackEligible {
		return nil, fmt.Errorf("invalid health-document rollback configuration id %q", rollback)
	}

	return &HealthDocumentDeploymentPolicy{
		stage:                        stage,
		championConfigurationID:      champion,
		qualificationConfigurationID: qualification,
		rollbackConfigurationID:      rollback,
	}, nil
}

func (p *HealthDocumentDeploymentPolicy) ConfigurationID() string {
	if p == nil {
		return defaultHealthDocumentChampionID
	}
	switch p.stage {
	case HealthDocumentStageQualification:
		return p.qualificationConfigurationID
	case HealthDocumentStageRollback:
		return p.rollbackConfigurationID
	default:
		return p.championConfigurationID
	}
}

func (p *HealthDocumentDeploymentPolicy) Stage() string {
	if p == nil {
		return HealthDocumentStageChampion
	}
	return p.stage
}
