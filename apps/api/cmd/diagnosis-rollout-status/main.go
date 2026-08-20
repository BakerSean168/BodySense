package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
)

func main() {
	champion := flag.String("champion", "diag-config-f492eb1c0c6676ae", "Champion configuration ID")
	challenger := flag.String("challenger", "diag-config-5a4a13627e14b4cf", "Challenger configuration ID")
	stage := flag.String("stage", service.DiagnosisRolloutShadow, "Rollout stage to summarize")
	canaryBPS := flag.Int("canary-bps", 1000, "Canary basis-point step to summarize")
	limit := flag.Int("limit", 1000, "Maximum recent observations")
	flag.Parse()

	db, err := database.Connect(database.ConfigFromEnv())
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	rollout := service.NewDiagnosisRolloutService(repository.NewDiagnosisRolloutRepository(db))
	summary, err := rollout.Summary(context.Background(), *champion, *challenger, *stage, *canaryBPS, *limit)
	if err != nil {
		log.Fatalf("summarize Diagnosis rollout: %v", err)
	}
	gate := service.EvaluateDiagnosisRolloutGate(summary)
	progression := service.EvaluateDiagnosisRolloutProgression(*stage, *canaryBPS, summary)
	encoded, _ := json.MarshalIndent(map[string]any{"summary": summary, "gate": gate, "progression": progression}, "", "  ")
	fmt.Println(string(encoded))
	if gate.Action != "continue" {
		log.Fatalf("DIAGNOSIS_ROLLOUT_GATE=%s", gate.Action)
	}
	fmt.Println("DIAGNOSIS_ROLLOUT_GATE=continue")
}
