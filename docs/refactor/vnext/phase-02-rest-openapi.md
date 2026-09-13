# vNext Phase 02 — Public REST / OpenAPI-first

- Status: IN PROGRESS
- Branch: `refactor/vnext-02-rest-openapi`
- Parent: `refactor/bodysense-vnext`
- Canonical authority: `packages/contracts/openapi/bodysense.v1.openapi.yaml`

## Goal

Move browser-facing REST endpoints from duplicated handwritten Go/TypeScript transport definitions to one OpenAPI 3.1 authority while preserving handwritten application/domain ownership.

The migration unit is a **complete route vertical slice**:

```text
OpenAPI 3.1
  -> generated Go strict transport
  -> OpenAPI request validator
  -> handwritten generated-to-domain adapter
  -> existing application service/domain
  -> generated typed response
  -> generated Orval Fetch client + Zod runtime validation
  -> handwritten auth/error/feature adapter
  -> existing feature/UI contract
```

Generated types may not be imported by Go model/service/repository packages or exposed as feature/domain contracts in Web.

## Checkpoint 1 — POST /api/v1/body-state/facts

Status: COMPLETE ON PHASE BRANCH

### Contract corrections made before promotion

The architecture spike was evidence, not production authority. Its BodyState mutation response represented `revision` as a scalar integer. Current production behavior returns the full immutable `BodyStateRevision` object, so the vNext OpenAPI schema models that real shape.

The public mutation contract is intentionally stricter than the pre-vNext handwritten Gin binding:

- `expected_revision` is required and `>= 0`;
- `fact` is required;
- `fact.kind` and `fact.value` are required;
- unknown request fields are rejected;
- response shape is runtime validated in the browser;
- all documented errors use `{ "error": { "code", "message" } }`.

This is a deliberate pre-user breaking reset, not a compatibility migration.

### Go boundary

- oapi-codegen `v2.8.0` generates Gin + strict-server + models + embedded spec.
- the existing BodySense `AuthMiddleware` remains the authentication/session authority;
- `gin-middleware` validates the OpenAPI request after authentication and before the application adapter;
- the handwritten `httpapi.PublicServer` maps generated transport models into `model.BodyStateFact`, calls `BodyStateService`, then maps the durable result back into generated response models;
- the old handwritten `protected.POST("/body-state/facts", bodyStateHandler.UpsertFact)` registration is removed.

Characterization proves malformed requests do not reach the service:

- missing `expected_revision` -> 400 before service call;
- unexpected nested request field -> 400 before service call.

### Web boundary

Orval generates a Fetch client with:

- Zod Mini runtime response validation;
- `forceSuccessResponse: true`;
- `useRuntimeFetcher: true`.

`openApiAuthFetch` injects the existing `authFetch`, so API-origin resolution, bearer token handling and refresh behavior remain one handwritten authority. Generated error aliases are not treated as business contracts; `withOpenApiError` normalizes the generated non-2xx error into the existing `ApiRequestError`.

`workspaceApi.addFact` deliberately continues to expose only `{ fact }` to feature code even though the transport response also contains `revision`. This prevents generated transport shape from leaking upward merely because the wire became more precise.

### Shared error envelope

Phase 02 also closed a pre-existing public inconsistency: authentication/authorization middleware used `{error:string,message:string}` while application handlers used `{error:{code,message}}`. Public middleware and handlers now share `dto.NewErrorResponse(code,message)`, matching the OpenAPI `ErrorEnvelope`.

Legacy session-less token acceptance is not removed here; it remains a Phase 07 compatibility-retirement item.

### Verification

```text
pnpm contracts:verify                         PASS
pnpm lint                                     PASS
pnpm typecheck                                PASS
pnpm test                                     PASS
  contracts                                   12/12
  Web                                         215/215
  Python                                      475/475
  Go                                          go test ./... PASS
pnpm build                                    PASS
git diff --check                              PASS
Go generated imports in model/service/repo    NONE
```

Known baseline test stderr noise and the existing BodyExplorer3D bundle warning are unchanged and tracked separately in the vNext finding ledger.

## Checkpoint 2 — GET /api/v1/health-workspace

Status: COMPLETE ON PHASE BRANCH

### Contract audit before migration

The generic pre-vNext Web request hid a real wire mismatch: `HealthWorkspace.body_state` was declared as the full `BodyStateSnapshot`, which requires `user_id`, but `dto.HealthWorkspaceBodyState` never sends `user_id`. The vNext contract does **not** add a fake field to preserve that TypeScript assumption.

Instead:

- `BodyStateProjection` now describes the minimum projection consumed by Body Explorer, Workbench and Diagnosis actions;
- the full thread/body-state `BodyStateSnapshot extends BodyStateProjection` and still requires `user_id`;
- `WorkspaceBodyState` models the actual health-workspace projection;
- `WorkspaceDiagnosis` is a workspace-specific application read model instead of treating the composite endpoint as a generic `DiagnosisAnalysis`;
- generated Zod validation rejects the old imaginary `body_state.user_id` field.

### OpenAPI response authority

The production schema now models the composite read surface rather than using generic `object` placeholders for known business structure. It includes BodyState facts/observations/hypotheses/revisions, Diagnosis candidates/freshness/assessments, Treatment/current revision/interventions, TrainingPlan, Outcome, trends, capabilities and actions. Metadata that is intentionally opaque remains `JsonObject`.

Go pointer fields with `omitempty` are represented as optional response properties instead of falsely requiring JSON `null`. Diagnosis freshness reasons are explicitly modeled, including revision/change-type provenance.

### Go boundary

- the generated strict router owns `GET /api/v1/health-workspace`;
- the old handwritten route registration and `HealthWorkspaceHandler` were deleted;
- `httpapi.GetHealthWorkspace` calls the existing application service and presents the result as the generated `HealthWorkspace`;
- a strict transitional presenter uses `json.Decoder.DisallowUnknownFields()` while the application read model still lives in `internal/dto`; this remaining package-ownership debt is tracked as `BS-VNEXT-REST-004` and must close before Phase 02 completes;
- response tests run the actual JSON through kin-openapi `ValidateResponse`, not only Go compile-time typing.

### Web boundary

`workspaceApi.get` now calls the generated `getHealthWorkspace` with `openApiAuthFetch`. The generated Zod schema validates the network payload first; handwritten projection functions then expose only the application fields needed by workspace features. Generated transport types do not become feature-domain types.

Citation metadata remains wire-opaque for now; the workspace adapter exposes a renderable `Citation` only when a string `title` exists and validates each optional known string field instead of asserting the object.

### Verification

```text
pnpm contracts:verify                         PASS
pnpm lint                                     PASS
pnpm typecheck                                PASS
pnpm test                                     PASS
  contracts                                   12/12
  Web                                         217/217
  Python                                      475/475
  Go                                          go test ./... PASS
pnpm build                                    PASS
git diff --check                              PASS
OpenAPI response validation                   PASS
old HealthWorkspaceHandler                    DELETED
Go generated imports in model/service/repo    NONE
```

Known baseline test stderr noise and the existing BodyExplorer3D bundle warning remain unchanged.

## Public-route coverage gate

Phase 02 now has a deterministic migration ledger generated from the immutable Phase 00 route inventory plus the current OpenAPI authority:

```text
baseline Go routes          96
operational exclusions       1  GET /api/health
browser-facing eligible     95
OpenAPI-authoritative        2
missing                     93
coverage                  2.11%
```

`contracts/public-route-policy.json` is the only place where a baseline route may be explicitly classified as operational, retired, or an approved new vNext route. The generated report refuses an OpenAPI operation that is absent from the Phase 00 baseline unless policy explicitly approves it.

`pnpm contracts:verify` checks that the committed coverage JSON/Markdown are current. Final Phase 02 acceptance will run the same tool with `--require-complete`; during migration, missing routes are visible debt rather than an artificial CI failure that would force fake schemas.

## Next batch

Migrate the remaining BodyState route family as one coherent contract batch. This reuses the BodyState schemas already proven by Checkpoint 1 and allows the legacy `BodyStateHandler` to be retired instead of leaving mixed ownership for the same aggregate.
