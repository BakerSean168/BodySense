# AI Service Agent Platform Closeout — 2026-08-21

> Status: Completed in `refactor/agent-platform-closeout`
> Scope: finish the remaining cross-role convergence after Diagnosis, Treatment, Assessment and Consultation.

## Closeout findings repaired

The closeout audit found several cases where the repository looked North-Star-compliant but runtime
behavior was weaker than the documentation implied:

1. `AIService.generate()` / `generate_stream()` preserved an explicit manifest logical model in the
   request object but still sent the route-registry model to LiteLLM. Manifest model pinning was
   therefore silently ignored.
2. `title-v1` and `posture-v1` incorrectly declared `bodysense-structured`; their real logical groups
   are `bodysense-text` and `bodysense-posture`.
3. Posture jobs selected and persisted a Go configuration id but did not send that id to Python, so the
   provenance could describe a configuration that was never actually executed.
4. The Posture FastAPI response model could discard Agent provenance and Go persisted the HTTP
   `{status,result}` envelope where Web/Assessment expect the direct analysis payload.
5. Title had a Python manifest but no Go-owned config pointer, identity verification or durable title
   Agent provenance.
6. Knowledge Curator/Splitter manifests existed, but Go did not own their selection and LLM execution
   lineage was not persisted with the knowledge source.
7. Go and Python disagreed on the Knowledge ingestion path contract: Go required an absolute path while
   Python only accepted a data-root-relative path.
8. Assessment/Consultation/Title/Posture internal HTTP configuration ids were optional on Python
   boundaries, leaving a silent second deployment authority.

## Implemented closeout

### Shared model boundary

- `AIService` now honors manifest-pinned `logical_model` for normal and streaming calls.
- Regression tests prove explicit manifest models cannot be replaced by route defaults.
- Title uses `bodysense-text`; Posture uses `bodysense-posture`; immutable ids were recomputed and the
  Go registrations updated to the exact fingerprints.

### Posture

- Go sends the selected immutable config id to Python.
- Python preserves `agent_configuration` and `execution_provenance` in the typed response.
- Go validates id/role/decision-policy/logical-model and execution lineage before persistence.
- Go adds a deterministic generation decision trace.
- The direct posture result is persisted, fixing the previous envelope/data-shape mismatch.

### Title

- Go owns the Title Champion pointer and validates repository-known ids at startup.
- Title's Python boundary requires the Go-selected id and returns exact config/execution provenance.
- Conversations now persist title config id/config JSON/execution provenance/Go decision trace.
- Manual title rename clears stale AI provenance.
- Migration `000049_add_title_agent_provenance` makes the audit fields durable.

### Knowledge Curator / Splitter

- Go owns the repository-known Splitter/Curator configuration ids.
- Callers can enable LLM splitting/refinement but cannot supply the serving immutable id; Go overwrites
  any caller value with the deployment selection.
- Python requires the exact ids whenever those LLM capabilities are enabled.
- Splitter/Curator record provider, physical model, call count and fallback/degraded status.
- Pipeline carries the records into `knowledge_sources.metadata.agent_execution`.
- Go validates returned Agent identities before returning ingestion success.
- Ingestion path contract is consistently data-root-relative at the HTTP boundary.

### Program boundary

- Assessment and Consultation plans moved from active to completed archives.
- `agent-platform-role-governance.md` defines role-appropriate governance and explicitly classifies
  OCR/ASR/Embedding/pose estimation as mechanisms, not fake Agents.
- The Nx AI-service test/coverage targets now request the repository's `dev` and `ocr` extras explicitly,
  so a clean `uv` environment can load the complete FastAPI application instead of depending on a
  previously populated virtualenv. Local `serve` likewise requests the OCR runtime extra.

## Final release-gate evidence — 2026-08-22

- Focused Python Agent boundary suite: 24 passed.
- Focused Go deployment/knowledge/title/upload packages: green.
- Repository release gate: `REPO_QUALITY=PASS`.
  - Python unit suite: 281 passed.
  - Web unit suite: 140 passed.
  - `go test ./...`: green.
  - Diagnosis qualification: 7/7; evidence policy: 5/5; promotion ready for shadow.
  - Treatment qualification: 4/4; EvidenceGap policy: 5/5; promotion ready for shadow.
  - LiteLLM gateway, PydanticAI adapter and AI-service gateway smoke tests: green.
  - Production Web/API builds and diff checks: green.
- Hermetic local deployment validator rebuilt API/AI/Web images from this worktree and reported
  `API_HEALTH=PASS`, `AI_HEALTH=PASS`, and `WEB_HEALTH=PASS`.
- Migration replay: version 49 full-up, latest-down to 48, then replay-up to 49 all passed.
- Domain validator: BodyState semantics, Treatment activation atomicity, outcome feedback atomicity and
  overall domain semantics all passed.
- Browser E2E: 3/3 Playwright longitudinal tests passed.
- Diagnosis shadow: 3 observations, 0 blockers.
- Treatment shadow: 3 observations, 0 blockers; 3 Champion revisions served, 0 Challenger revisions
  persisted; decision-trace and replay-input checks passed.
- Final result: `LOCAL_DEPLOY_VALIDATION=PASS`.

The implementation is release-gate-ready. PR/CI checks are the remaining repository integration gate.
