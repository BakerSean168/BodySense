# Full Longitudinal Domain Refactor Execution Plan

> Status: completed and verified
> Started: 2026-08-16
> Completed: 2026-08-16
> Branch: `full-domain-refactor-20260816`
> Safety snapshot: `/home/ubuntu/projects/bodysense-learning-snapshot-current`
> Pre-refactor tag: `pre-full-domain-refactor-2026-08-16`

## Goal

Complete the migration defined by ADR 0004 and the longitudinal BodyState domain model while preserving the existing consultation runtime contracts.

## Protected contracts

- LangGraph checkpoint, interrupt, and resume ownership remains in Python.
- Go remains the durable business owner and public Runtime Event Log owner.
- Request/run idempotency and StreamEvent v1 remain compatible.
- Conversation remains the long-lived interaction surface, not health truth.
- BodyState, DiagnosisAnalysis, Treatment, Intervention, and Outcome keep user ownership checks.
- Historical DiagnosisAnalysis and accepted TreatmentRevision records are immutable.
- Safety gates remain deterministic and business-governed.

## Implementation sequence

1. Complete durable domain vocabulary: Hypothesis, Evidence, Diagnosis freshness, Treatment revisions, Interventions, Outcomes.
2. Add material-change and safety review policies.
3. Migrate Diagnosis production execution to PydanticAI typed output and targeted evidence retrieval.
4. Add a typed Treatment Agent and typed reassessment/review output.
5. Add long-conversation context retrieval without full transcript replay.
6. Make training an Intervention execution adapter and persist outcomes back into BodyState.
7. Add a single health-workspace projection and replace linear Journey decision logic.
8. Replace the compatibility workbench editor with explicit BodyState mutations and render current Treatment/trends/history.
9. Retain compatibility routes only as adapters; stop treating session JSON fields as business truth.
10. Run focused, package, full-repository, migration, type, build, and contract validation; repair all regressions.

## Completion criteria

- One durable user-scoped BodyState is the health truth.
- Fact, Observation, Hypothesis, and Evidence are distinct.
- Diagnosis pins an exact revision and exposes explicit freshness.
- Treatment generation creates a proposal; user acceptance creates the current accepted revision.
- Material BodyState or safety changes recommend review/pause without rewriting accepted plans.
- Training logs/check-ins create Intervention/Outcome records and accepted outcomes can create BodyState revisions.
- Workspace readiness is capability-based and never terminates at a global `completed` state.
- Legacy consultation fields and training tables are compatibility projections/adapters only.

## Completion evidence

All completion criteria above are implemented on `full-domain-refactor-20260816`.
The final architecture has one durable health loop:

`BodyState revision -> DiagnosisAnalysis -> Treatment proposal -> explicit acceptance -> Intervention/Training -> Outcome -> BodyState revision`

Key closure decisions:

- Diagnosis has one executable path: exact durable BodyState revision -> typed PydanticAI Diagnosis Agent -> immutable DiagnosisAnalysis. The old extracted-info/raw-JSON fallback is removed.
- Treatment generation has one executable AI path: typed Treatment Agent. Compatibility consultation routes may create proposals, but cannot write `consultation_sessions.treatment_plan` or auto-accept a revision.
- The old mutually-exclusive `/consultations/:id/confirm` write is retired; candidate assessments are independent durable user interpretations.
- Training cannot mutate plan phases directly. It executes accepted Treatment interventions and records Outcome/review signals; any new Treatment remains a proposal until explicit acceptance.
- The old Python reassessment endpoint that returned `next_phase_plan` is removed.
- Go-side broad Diagnosis/Treatment RAG is removed. Diagnosis/Treatment Agents may perform targeted evidence retrieval only for explicit evidence gaps.
- Journey emits `longitudinal_monitoring` instead of a global terminal state. `completed` remains only as a client compatibility value.
- Legacy `consultation_sessions.diagnosis` is a compatibility mirror derived from the durable analysis path; no user action can make it an independent diagnosis truth.

Final verification performed on 2026-08-16:

- `cd apps/api && go test ./...` — pass.
- `cd apps/ai-service && .venv/bin/python -m pytest -q` — 215 passed.
- `pnpm nx run @bodysense/web:test` — 137 passed.
- `pnpm lint` — pass, including Go vet, Ruff, and ESLint.
- `pnpm typecheck` — pass; Pyright reports 0 errors and TypeScript checks pass.
- `pnpm build` — pass for Web production build and Go API binary.
- `git diff --check` — pass.
- PostgreSQL empty-database migration chain `000001 -> 000034` — pass.
- Migration `000034` down/up replay — pass.

The pre-refactor snapshot/tag remain available for later learning and code-rewrite exercises.

## Final hardening closure

After the original domain migration, a second review repaired final mutation-boundary invariants: acceptance-time safety/freshness/assessment checks, retryable Outcome projection, recoverable TrainingPlan projection, Assessment-to-BodyState convergence, explicit disconnect cancellation, concurrent-run locking, pure HealthWorkspace reads, and full-loop E2E coverage.

The current architecture is documented in `docs/architecture/current-longitudinal-system.md`.
