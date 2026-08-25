package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: knowledge-publication-manager <publish-reviewed|rollback|observe-eval|status> [flags]")
	}

	db, err := database.Connect(database.ConfigFromEnv())
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	publicationRepo := repository.NewKnowledgePublicationRepository(db)
	operatorAuthority := service.NewKnowledgeOperatorAuthority(repository.NewUserRepository(db))
	publicationService := service.NewKnowledgePublicationService(
		publicationRepo,
		database.NewTransactionManager(db),
	)
	observationService := service.NewKnowledgePublicationObservationService(
		publicationRepo,
		repository.NewKnowledgePublicationObservationRepository(db),
	)

	ctx := context.Background()
	switch os.Args[1] {
	case "publish-reviewed":
		publishReviewed(ctx, publicationService, operatorAuthority, os.Args[2:])
	case "rollback":
		rollback(ctx, publicationService, operatorAuthority, os.Args[2:])
	case "observe-eval":
		observeEval(ctx, observationService, os.Args[2:])
	case "status":
		publicationStatus(ctx, observationService, os.Args[2:])
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func publishReviewed(
	ctx context.Context,
	publicationService *service.KnowledgePublicationService,
	operatorAuthority *service.KnowledgeOperatorAuthority,
	args []string,
) {
	flags := flag.NewFlagSet("publish-reviewed", flag.ExitOnError)
	publicationKey := flags.String("publication-key", "", "Immutable publication key")
	batchKey := flags.String("batch-key", "", "Publication batch family key")
	reviewedSnapshotPath := flags.String("reviewed-snapshot", "", "reviewed-knowledge-snapshot.v1 JSON artifact")
	publishedBy := flags.String("published-by", "", "Durable operator user UUID")
	summary := flags.String("summary", "", "Publication summary")
	_ = flags.Parse(args)
	if *reviewedSnapshotPath == "" {
		log.Fatal("reviewed-snapshot is required")
	}
	operatorID, err := operatorAuthority.Require(ctx, *publishedBy)
	if err != nil {
		log.Fatalf("authorize knowledge publication operator: %v", err)
	}
	payload, err := os.ReadFile(*reviewedSnapshotPath)
	if err != nil {
		log.Fatalf("read reviewed snapshot: %v", err)
	}
	artifact, err := service.ParseReviewedKnowledgeSnapshot(payload)
	if err != nil {
		log.Fatalf("validate reviewed snapshot: %v", err)
	}
	publication, err := publicationService.PublishReviewedBatch(ctx, service.PublishReviewedKnowledgeBatchInput{
		PublicationKey:   *publicationKey,
		BatchKey:         *batchKey,
		PublishedBy:      operatorID.String(),
		Summary:          *summary,
		ReviewedSnapshot: *artifact,
	})
	if err != nil {
		log.Fatalf("publish reviewed knowledge batch: %v", err)
	}
	printJSON(publication)
}

func rollback(
	ctx context.Context,
	publicationService *service.KnowledgePublicationService,
	operatorAuthority *service.KnowledgeOperatorAuthority,
	args []string,
) {
	flags := flag.NewFlagSet("rollback", flag.ExitOnError)
	publicationKey := flags.String("publication-key", "", "Published batch key to rollback")
	rollbackKey := flags.String("rollback-key", "", "Immutable rollback publication key")
	rolledBackBy := flags.String("rolled-back-by", "", "Durable operator user UUID")
	reason := flags.String("reason", "", "Rollback reason")
	_ = flags.Parse(args)

	operatorID, err := operatorAuthority.Require(ctx, *rolledBackBy)
	if err != nil {
		log.Fatalf("authorize knowledge rollback operator: %v", err)
	}
	publication, err := publicationService.RollbackBatch(ctx, service.RollbackKnowledgeBatchInput{
		PublicationKey:         *publicationKey,
		RollbackPublicationKey: *rollbackKey,
		RolledBackBy:           operatorID.String(),
		Reason:                 *reason,
	})
	if err != nil {
		log.Fatalf("rollback knowledge batch: %v", err)
	}
	printJSON(publication)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func printJSON(value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Fatalf("encode result: %v", err)
	}
	fmt.Println(string(encoded))
}

type publishedEvalReport struct {
	SchemaVersion     string `json:"schema_version"`
	EvaluatorRevision string `json:"evaluator_revision"`
	Publication       struct {
		PublicationID    string `json:"publication_id"`
		PublicationKey   string `json:"publication_key"`
		PublishedVersion int    `json:"published_version"`
	} `json:"publication"`
	Observations []struct {
		CaseID           string   `json:"case_id"`
		Query            string   `json:"query"`
		RetrievalStatus  string   `json:"retrieval_status"`
		CitationStatus   string   `json:"citation_status"`
		GroundingStatus  string   `json:"grounding_status"`
		IdentityStatus   string   `json:"identity_status"`
		ProvenanceStatus string   `json:"provenance_status"`
		TopSimilarity    *float64 `json:"top_similarity"`
		ReturnedUnitKey  string   `json:"returned_unit_key"`
		Reasons          []string `json:"reasons"`
	} `json:"observations"`
}

func observeEval(
	ctx context.Context,
	observationService *service.KnowledgePublicationObservationService,
	args []string,
) {
	flags := flag.NewFlagSet("observe-eval", flag.ExitOnError)
	publicationKey := flags.String("publication-key", "", "Exact published batch key")
	reportPath := flags.String("report", "", "published-knowledge-eval.v1 JSON report")
	runKey := flags.String("run-key", "", "Immutable observation run key")
	kind := flags.String("kind", "predeploy_eval", "Observation kind")
	_ = flags.Parse(args)
	if *publicationKey == "" || *reportPath == "" || *runKey == "" {
		log.Fatal("publication-key, report and run-key are required")
	}
	payload, err := os.ReadFile(*reportPath)
	if err != nil {
		log.Fatalf("read eval report: %v", err)
	}
	var report publishedEvalReport
	if err := json.Unmarshal(payload, &report); err != nil {
		log.Fatalf("decode eval report: %v", err)
	}
	if report.SchemaVersion != "bodysense.published-knowledge-eval.v1" {
		log.Fatalf("unsupported eval report schema %q", report.SchemaVersion)
	}
	if report.Publication.PublicationKey != *publicationKey {
		log.Fatalf("eval report publication key mismatch")
	}
	publicationID, err := uuid.Parse(report.Publication.PublicationID)
	if err != nil {
		log.Fatalf("invalid eval report publication_id: %v", err)
	}
	for _, observation := range report.Observations {
		metadata, err := json.Marshal(map[string]any{
			"query":             observation.Query,
			"top_similarity":    observation.TopSimilarity,
			"returned_unit_key": observation.ReturnedUnitKey,
			"reasons":           observation.Reasons,
		})
		if err != nil {
			log.Fatalf("encode observation metadata: %v", err)
		}
		err = observationService.Record(ctx, service.RecordKnowledgePublicationObservationInput{
			PublicationKey:           *publicationKey,
			ExpectedPublicationID:    publicationID,
			ExpectedPublishedVersion: report.Publication.PublishedVersion,
			ObservationKey:           *runKey + ":" + observation.CaseID,
			ObservationKind:          *kind,
			EvaluatorRevision:        report.EvaluatorRevision,
			CaseID:                   observation.CaseID,
			RetrievalStatus:          observation.RetrievalStatus,
			CitationStatus:           observation.CitationStatus,
			GroundingStatus:          observation.GroundingStatus,
			IdentityStatus:           observation.IdentityStatus,
			ProvenanceStatus:         observation.ProvenanceStatus,
			Metadata:                 datatypes.JSON(metadata),
		})
		if err != nil {
			log.Fatalf("record eval observation %s: %v", observation.CaseID, err)
		}
	}
	fmt.Printf("RECORDED_OBSERVATIONS=%d\n", len(report.Observations))
}

func publicationStatus(
	ctx context.Context,
	observationService *service.KnowledgePublicationObservationService,
	args []string,
) {
	flags := flag.NewFlagSet("status", flag.ExitOnError)
	publicationKey := flags.String("publication-key", "", "Exact published batch key")
	kind := flags.String("kind", "predeploy_eval", "Observation kind")
	_ = flags.Parse(args)
	if *publicationKey == "" {
		log.Fatal("publication-key is required")
	}
	summary, err := observationService.Summary(ctx, *publicationKey, *kind)
	if err != nil {
		log.Fatalf("summarize publication observations: %v", err)
	}
	gate := service.EvaluateKnowledgePublicationGate(summary)
	printJSON(map[string]any{"summary": summary, "gate": gate})
	if gate.Action != "continue" {
		os.Exit(2)
	}
}
