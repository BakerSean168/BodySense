package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"gorm.io/gorm"
)

const diagnosisReplayAuditRevision = "diagnosis-historical-replay-audit-v1"

type diagnosisReplayConfigurationAudit struct {
	Version         string   `json:"version"`
	ConfigurationID string   `json:"configuration_id"`
	DatabaseSamples int      `json:"database_samples"`
	Passed          int      `json:"passed"`
	Failed          int      `json:"failed"`
	Evidence        string   `json:"evidence"`
	Failures        []string `json:"failures"`
}

type diagnosisReplayAuditReport struct {
	Revision       string                              `json:"revision"`
	GeneratedAt    time.Time                           `json:"generated_at"`
	Accepted       bool                                `json:"accepted"`
	TotalSamples   int                                 `json:"total_samples"`
	Passed         int                                 `json:"passed"`
	Failed         int                                 `json:"failed"`
	Configurations []diagnosisReplayConfigurationAudit `json:"configurations"`
}

func runDiagnosisReplayAudit(ctx context.Context, db *gorm.DB, limit int) {
	if limit <= 0 {
		log.Fatal("replay-limit-per-configuration must be positive")
	}
	repo := repository.NewDiagnosisAnalysisRepository(db)
	replay := service.NewDiagnosisReplayService(service.NewDiagnosisAnalysisService(repo), nil)
	report := diagnosisReplayAuditReport{
		Revision:    diagnosisReplayAuditRevision,
		GeneratedAt: time.Now().UTC(),
		Accepted:    true,
	}
	for _, historical := range service.DiagnosisHistoricalReplayConfigurations() {
		var rows []model.DiagnosisAnalysisRecord
		err := db.WithContext(ctx).
			Select("id", "user_id", "agent_configuration_id").
			Where("agent_configuration_id = ? AND replay_input IS NOT NULL AND replay_input <> '{}'::jsonb", historical.ConfigurationID).
			Order("created_at ASC").
			Limit(limit).
			Find(&rows).Error
		if err != nil {
			log.Fatalf("query replayable %s analyses: %v", historical.Version, err)
		}
		item := diagnosisReplayConfigurationAudit{
			Version:         historical.Version,
			ConfigurationID: historical.ConfigurationID,
			DatabaseSamples: len(rows),
			Evidence:        "database_replay",
		}
		if len(rows) == 0 {
			item.Evidence = "identity_and_synthetic_regression_only"
		}
		for _, row := range rows {
			replayed, err := replay.HistoricalReplay(ctx, row.UserID, row.ID)
			if err != nil {
				item.Failed++
				item.Failures = append(item.Failures, fmt.Sprintf("%s: %v", row.ID, err))
				continue
			}
			if replayed.SourceConfigurationID != historical.ConfigurationID ||
				replayed.TargetConfigurationID != historical.ConfigurationID ||
				!replayed.ArtifactIntegrity.Match ||
				!replayed.Comparison.Hard.Match ||
				!replayed.Comparison.Semantic.Match ||
				!replayed.Comparison.Presentation.Match ||
				replayed.InputFingerprint == "" {
				item.Failed++
				item.Failures = append(item.Failures, fmt.Sprintf("%s: replay invariant mismatch", row.ID))
				continue
			}
			item.Passed++
		}
		report.TotalSamples += item.DatabaseSamples
		report.Passed += item.Passed
		report.Failed += item.Failed
		if item.Failed > 0 {
			report.Accepted = false
		}
		report.Configurations = append(report.Configurations, item)
	}
	writeOperatorJSON(report)
	if !report.Accepted {
		log.Fatal("DIAGNOSIS_HISTORICAL_REPLAY_AUDIT=FAIL")
	}
	fmt.Fprintln(os.Stderr, "DIAGNOSIS_HISTORICAL_REPLAY_AUDIT=PASS")
}

func runDiagnosisHistorySnapshot(ctx context.Context, db *gorm.DB) {
	snapshot, err := diagnosisHistorySnapshotFromDB(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	writeOperatorJSON(snapshot)
	fmt.Fprintln(os.Stderr, "DIAGNOSIS_HISTORY_SNAPSHOT=PASS")
}

func runDiagnosisHistoryVerify(ctx context.Context, db *gorm.DB, snapshotFile string) {
	if snapshotFile == "" {
		log.Fatal("-snapshot-file is required for diagnosis-history-verify")
	}
	var reader io.Reader
	if snapshotFile == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(snapshotFile)
		if err != nil {
			log.Fatalf("open Diagnosis history snapshot: %v", err)
		}
		defer file.Close()
		reader = file
	}
	var before service.DiagnosisHistorySnapshot
	if err := json.NewDecoder(reader).Decode(&before); err != nil {
		log.Fatalf("decode Diagnosis history snapshot: %v", err)
	}
	if before.Revision != service.DiagnosisHistorySnapshotRevision {
		log.Fatalf("unsupported Diagnosis history snapshot revision %q", before.Revision)
	}
	current, err := diagnosisHistorySnapshotFromDB(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	verification := service.VerifyDiagnosisHistorySnapshot(before, current)
	writeOperatorJSON(verification)
	if !verification.Unchanged {
		log.Fatal("DIAGNOSIS_HISTORY_IMMUTABILITY=FAIL")
	}
	fmt.Fprintln(os.Stderr, "DIAGNOSIS_HISTORY_IMMUTABILITY=PASS")
}

func diagnosisHistorySnapshotFromDB(ctx context.Context, db *gorm.DB) (service.DiagnosisHistorySnapshot, error) {
	var analyses []model.DiagnosisAnalysisRecord
	if err := db.WithContext(ctx).Order("id ASC").Find(&analyses).Error; err != nil {
		return service.DiagnosisHistorySnapshot{}, fmt.Errorf("load Diagnosis analyses for history audit: %w", err)
	}
	var candidates []model.DiagnosisCandidateRecord
	if err := db.WithContext(ctx).Order("id ASC").Find(&candidates).Error; err != nil {
		return service.DiagnosisHistorySnapshot{}, fmt.Errorf("load Diagnosis candidates for history audit: %w", err)
	}
	return service.BuildDiagnosisHistorySnapshot(analyses, candidates, time.Now().UTC())
}

func writeOperatorJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		log.Fatalf("encode operator report: %v", err)
	}
}
