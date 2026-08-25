package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: knowledge-source-manager register-thought-forest [flags]")
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
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
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
