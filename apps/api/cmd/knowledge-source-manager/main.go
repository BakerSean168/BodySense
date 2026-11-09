package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: knowledge-source-manager <register-thought-forest|register-source> [flags]")
	}
	db, err := database.Connect(database.ConfigFromEnv())
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	// One-shot operator tooling keeps stdout machine-readable JSON. Database
	// connection/errors still go to stderr, while routine SQL tracing is silent.
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	ctx := context.Background()
	switch os.Args[1] {
	case "register-thought-forest":
		registerThoughtForest(ctx, db, os.Args[2:])
	case "register-source":
		registerSource(ctx, db, os.Args[2:])
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func registerSource(ctx context.Context, db *gorm.DB, args []string) {
	flags := flag.NewFlagSet("register-source", flag.ExitOnError)
	snapshotPath := flags.String("snapshot", "", "Exact bodysense.source-snapshot.* JSON artifact")
	operator := flags.String("operator-id", "", "Durable Knowledge operator user UUID")
	_ = flags.Parse(args)
	if *snapshotPath == "" {
		log.Fatal("snapshot is required")
	}
	payload, err := os.ReadFile(*snapshotPath)
	if err != nil {
		log.Fatalf("read source snapshot: %v", err)
	}
	var artifact sourceSnapshotArtifact
	if err := json.Unmarshal(payload, &artifact); err != nil {
		log.Fatalf("decode source snapshot: %v", err)
	}
	if _, ok := supportedSourceSnapshotSchemas[artifact.SchemaVersion]; !ok {
		log.Fatalf("unsupported source snapshot schema %q", artifact.SchemaVersion)
	}
	authority := service.NewKnowledgeOperatorAuthority(repository.NewUserRepository(db))
	operatorID, err := authority.Require(ctx, *operator)
	if err != nil {
		log.Fatalf("authorize Knowledge operator: %v", err)
	}

	var source *model.KnowledgeSource
	var created bool
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		registry := service.NewKnowledgeSourceRegistry(repository.NewKnowledgeSourceRepository(tx))
		var registerErr error
		source, created, registerErr = registry.RegisterOrValidate(
			ctx,
			operatorID,
			service.RegisterKnowledgeSourceInput{
				SourceKey:          artifact.Source.SourceKey,
				SourceType:         artifact.Source.SourceType,
				Title:              artifact.Source.Title,
				Author:             artifact.Source.Author,
				ProblemSlug:        artifact.Source.ProblemSlug,
				ProblemDisplayName: artifact.Source.ProblemDisplayName,
				OriginalFilePath:   artifact.Source.OriginalFilePath,
				Language:           artifact.Source.Language,
				LicenseStatus:      artifact.Source.LicenseStatus,
				ContentHash:        artifact.Source.ContentHash,
				CanonicalURL:       artifact.Source.CanonicalURL,
				SourceVersion:      artifact.Source.SourceVersion,
				Provenance:         artifact.Source.Provenance,
				Metadata: map[string]any{
					"snapshot_id":         artifact.SnapshotID,
					"snapshot_schema":     artifact.SchemaVersion,
					"snapshot_repository": "bodysense-knowledge-snapshots",
				},
			},
		)
		return registerErr
	}); err != nil {
		log.Fatalf("register knowledge source: %v", err)
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"source_key": source.SourceKey,
		"source_id":  source.ID,
		"created":    created,
	}, "", "  ")
	if err != nil {
		log.Fatalf("encode registration result: %v", err)
	}
	fmt.Println(string(encoded))
}

type sourceSnapshotArtifact struct {
	SchemaVersion string `json:"schema_version"`
	SnapshotID    string `json:"snapshot_id"`
	Source        struct {
		SourceKey          string         `json:"source_key"`
		SourceType         string         `json:"source_type"`
		Title              string         `json:"title"`
		Author             string         `json:"author"`
		ProblemSlug        string         `json:"problem_slug"`
		ProblemDisplayName string         `json:"problem_display_name"`
		OriginalFilePath   string         `json:"original_file_path"`
		Language           string         `json:"language"`
		CanonicalURL       string         `json:"canonical_url"`
		SourceVersion      string         `json:"source_version"`
		ContentHash        string         `json:"content_hash"`
		LicenseStatus      string         `json:"license_status"`
		Provenance         map[string]any `json:"provenance"`
	} `json:"source"`
}

var supportedSourceSnapshotSchemas = map[string]struct{}{
	"bodysense.source-snapshot.medlineplus.v1": {},
}

func registerThoughtForest(ctx context.Context, db *gorm.DB, args []string) {
	flags := flag.NewFlagSet("register-thought-forest", flag.ExitOnError)
	snapshotPath := flags.String("snapshot", "", "Exact bodysense.health.snapshot.v3 JSON artifact")
	operator := flags.String("operator-id", "", "Durable Knowledge operator user UUID")
	_ = flags.Parse(args)
	if *snapshotPath == "" {
		log.Fatal("snapshot is required")
	}
	payload, err := os.ReadFile(*snapshotPath)
	if err != nil {
		log.Fatalf("read Thought Forest snapshot: %v", err)
	}
	authority := service.NewKnowledgeOperatorAuthority(repository.NewUserRepository(db))
	operatorID, err := authority.Require(ctx, *operator)
	if err != nil {
		log.Fatalf("authorize Knowledge operator: %v", err)
	}

	var report *service.ThoughtForestRegistrationReport
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		registry := service.NewKnowledgeSourceRegistry(repository.NewKnowledgeSourceRepository(tx))
		var registerErr error
		report, registerErr = service.RegisterThoughtForestSnapshot(ctx, registry, operatorID, payload)
		return registerErr
	}); err != nil {
		log.Fatalf("register Thought Forest snapshot: %v", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("encode registration report: %v", err)
	}
	fmt.Println(string(encoded))
}
