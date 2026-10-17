# DX-RETIRE-001 · Delete legacy candidate assessment / diagnosis_analyses / diagnosis_candidates paths

Status: QUEUED. Phase 3. Lane A (Diagnosis), after DX-003 and UI-DX-001. SF cross-reference: SF-40.
**Owner role:** Diagnosis Retirement Owner. **Dependencies:** `DX-003`, `UI-DX-001`.
**Exclusive resources:** `diagnosis-legacy`, plus the shared hotspots `database-migrations` and `public-openapi` (rebase rule).

## Why this ticket exists · 2026-10-17

**GitHub:** [#341](https://github.com/BakerSean168/BodySense/issues/341) (epic [#301](https://github.com/BakerSean168/BodySense/issues/301)) · Wave 3 · size L · blocked by: DX-003 #310, UI-DX-001 #312 · blocks: REL-001 #317.

DX-002 (#305) delivered the native Diagnosis aggregate, decision/settlement transactions and native HTTP (#322 #332 #334 #336 #337 #338). Its last item was deleting the legacy per-run candidate/assessment stack, which also made DX-003 (#310) wait on #305. The legacy stack still has live consumers: Agent output, the Diagnosis window and Treatment qualification. So the deletion moves here, after the Agent writes native proposals (#310) and the window reads the native API (#312). #305 is closed and the `#310 blocked by #305` dependency is removed.

## Scope (code-verified on main `1abcc9f16`)

`candidate_assessment`, `diagnosis_analyses`, `diagnosis_candidates` and `diagnosis-analyses` still appear in roughly 28–37 non-test files, counting generated code:

- **Baseline:** legacy `diagnosis_analyses`, `diagnosis_candidates`, `diagnosis_candidate_assessments`, `diagnosis_analysis_freshness` and `diagnosis_rollout_observations` in `000001_vnext_baseline.{up,down}.sql`. Remove them, then regenerate checksums.
- **Go:** the legacy model, repository, service, HTTP and validator paths (`diagnosis_analysis*`, `diagnosis_application_service`, `diagnosis_replay_service`, legacy freshness, `diagnosis_*_audit`, `health_workspace_service`, `consultation/runtime.go`, `cmd/domain-validator/diagnosis_audit.go`, privacy-erasure entries).
- **Contracts:** `/api/v1/diagnosis-analyses/**` and `/consultations/{id}/diagnosis`. After removing them, regenerate the Go, TS and Zod artifacts and Postman.
- **Web:** the legacy consultation Diagnosis panel, history panel, hooks and services; the workspace types; `confirmedConditions.ts`.
- **AI paths:** the legacy candidate structure in `ai-service` diagnosis models and service, and the treatment-side readers of candidate assessments. Coordinate with TP-002 slice 3 (Treatment deletion): whoever merges second rebases.

## Acceptance

- `git grep -i "candidate_assessment\|diagnosis_analyses\|diagnosis_candidates\|diagnosis-analyses"` returns nothing outside tests and historical docs.
- No compatibility layer or flag. The native Run/Proposal path is the only Diagnosis path.
- These checks pass: the full PG18 suite on a fresh `pgvector/pgvector:pg18`, the migration and domain validators, `contracts:check-generated`, `postman:verify`, web typecheck and vitest, and ai-service ruff and pytest.

**Protected invariants:** owner isolation; AI never becomes a fact; a Diagnosis is adopted only through a user decision with a Go receipt; Safety fails closed; replay input stays frozen. No invented Diagnosis Q-17 TTL or clinical threshold.
