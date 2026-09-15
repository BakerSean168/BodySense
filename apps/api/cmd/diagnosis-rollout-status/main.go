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
	deployment, err := service.NewAgentDeploymentPolicy()
	if err != nil {
		log.Fatalf("load Diagnosis deployment policy: %v", err)
	}
	champion := flag.String("champion", deployment.DiagnosisConfigurationID(), "Champion configuration ID")
	challenger := flag.String("challenger", "", "Challenger configuration ID (required)")
	stage := flag.String("stage", service.DiagnosisRolloutShadow, "Rollout stage to summarize")
	canaryBPS := flag.Int("canary-bps", 1000, "Canary basis-point step to summarize")
	limit := flag.Int("limit", 1000, "Maximum recent observations")
	flag.Parse()
	if *challenger == "" {
		log.Fatal("-challenger is required; retired configurations are not runtime rollout targets")
	}

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
