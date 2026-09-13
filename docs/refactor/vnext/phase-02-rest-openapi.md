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

## Checkpoint 3 — Complete BodyState public route family

Status: COMPLETE ON PHASE BRANCH

The remaining BodyState route family has moved behind the generated OpenAPI boundary. Together with Checkpoint 1, all eleven `/api/v1/body-state...` operations now have one transport authority:

```text
GET    /api/v1/body-state
POST   /api/v1/body-state/facts
POST   /api/v1/body-state/facts/{id}/correct
PATCH  /api/v1/body-state/facts/{id}/temporal
PATCH  /api/v1/body-state/facts/{id}/review
POST   /api/v1/body-state/observations
PATCH  /api/v1/body-state/observations/{id}/review
POST   /api/v1/body-state/hypotheses
PATCH  /api/v1/body-state/hypotheses/{id}/lifecycle
GET    /api/v1/body-state/evidence
POST   /api/v1/body-state/safety/resolve
```

### Runtime/ownership cleanup

- all handwritten BodyState route registrations were removed from `cmd/server`;
- `internal/handler/body_state_handler.go` and its handler tests were deleted;
- the now-unused handwritten `internal/dto/body_state.go` request surface was deleted;
- a hidden shared mutation-error helper that other non-OpenAPI handlers relied on was moved to `handler/utils.go` rather than keeping a dead BodyState handler as a utility container;
- generated OpenAPI types remain absent from model/service/repository packages.

### Concurrency contract repaired

The legacy public safety endpoint accepted `expected_revision` in JSON but discarded it before persistence. vNext now threads the value through `ResolveSafetyState` to `SetSafetyState` and the repository revision lock. Internal detector-originated safety projection writes remain explicitly unconditional by passing `nil`; the two semantics are no longer accidentally conflated.

### Idempotent mutation semantics

BodyState repositories intentionally return `entity + nil revision` when a retry/no-op finds that the durable state is already current. The public mutation contract therefore models `revision` as **nullable**. A null revision means “successful command, no new durable revision created”; optimistic-lock conflict remains an explicit 409. Both Go transport and generated Web Zod tests cover this case.

### Web command migration

All existing workspace BodyState network commands now use generated Orval functions through `openApiAuthFetch` / `withOpenApiError`:

- add/review/correct/temporal fact;
- review observation;
- hypothesis lifecycle update;
- safety resolution.

The feature boundary remains handwritten and narrower than transport: mutation methods return `{ fact }` or `void` where that is all the feature consumes. Review/lifecycle/safety command strings are finite TypeScript unions matching the OpenAPI enums rather than generic `string`.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         12
missing                       83
coverage                   12.63%
```

### Verification

```text
pnpm contracts:verify              PASS
pnpm lint                          PASS
pnpm typecheck                     PASS
pnpm test                          PASS
  contracts                        12/12
  Web                              219/219
  Python                           475/475
  Go                               go test ./... PASS
pnpm build                         PASS
git diff --check                   PASS
old BodyState handler              DELETED
old BodyState request DTO          DELETED
handwritten BodyState route regs   NONE
Go generated imports in domain     NONE
```

The production build also shows the validated OpenAPI/Zod path adding measurable bytes to the ConsultationPage chunk. `BS-VNEXT-REST-007` tracks this as a Phase 02 bundle review item; runtime validation will not be weakened merely to improve bundle size.

## Checkpoint 4 — Lifestyle, Body Metrics, Injury History and Onboarding Context

Status: COMPLETE ON PHASE BRANCH

Nine adjacent health-context routes now share the same OpenAPI-first boundary:

```text
GET   /api/v1/lifestyle
PUT   /api/v1/lifestyle
POST  /api/v1/lifestyle/candidates/{id}/accept
POST  /api/v1/lifestyle/candidates/{id}/reject
GET   /api/v1/body-metrics
PUT   /api/v1/body-metrics
GET   /api/v1/health-history/injury
PUT   /api/v1/health-history/injury
PUT   /api/v1/onboarding/context
```

### Public concurrency reset

Pre-vNext request DTOs made revision guards optional, and onboarding omitted the BodyState revision entirely. The vNext browser contract is intentionally stricter:

- lifestyle/body-metrics/injury-history mutations require `expected_revision`;
- candidate accept/reject also require `expected_revision`;
- onboarding requires `expected_body_state_revision`; the current first-use UI sends `0`;
- range/schema failures (for example height above 250 cm) are rejected by OpenAPI middleware before the service;
- there is no compatibility alias that silently turns a missing revision into an unconditional write.

### Handler retirement

The four legacy Gin handler files for Lifestyle, Body Metrics, Health History and Onboarding Context were deleted, and their route registrations were removed from `cmd/server`. Generated strict server methods now own all nine paths. The temporary `bodyStateHandleMutationError` utility also became unreferenced and was deleted from handler utilities.

### Web runtime trust

Profile services and the workspace lifestyle editor now call generated Orval clients through the shared auth/error adapter. Handwritten profile models remain presentation/application projections and are populated explicitly after generated Zod response validation.

The new tests prove:

- malformed Body Metrics response data fails closed;
- lifestyle update sends the current revision;
- onboarding sends explicit revision 0;
- missing revision and invalid metric ranges never reach the Go application adapter.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         21
missing                       74
coverage                   22.11%
```

### Verification

```text
pnpm contracts:verify                  PASS
pnpm lint                              PASS
pnpm typecheck                         PASS
Web typecheck --skip-nx-cache          PASS
pnpm test                              PASS
  contracts                            12/12
  Web                                  223/223
  Python                               475/475
  Go                                   go test ./... PASS
pnpm build                             PASS
git diff --check                       PASS
legacy health-context handlers         DELETED
handwritten route registrations        NONE
```

Bundle observation remains open under `BS-VNEXT-REST-007`: at this point ConsultationPage is ~355.71 kB / 105.27 kB gzip and Vite has extracted a generated/shared module of ~45.11 kB / 11.59 kB gzip. Runtime validation remains mandatory; feature/tag splitting is evaluated before Phase 02 closes.

## Checkpoint 5 — Current identity and stable profile

Status: COMPLETE ON PHASE BRANCH

Three identity/profile routes now use the generated OpenAPI boundary:

```text
GET  /api/v1/me
GET  /api/v1/profile
PUT  /api/v1/profile
```

### Stable-profile contract reset

`UserProfile` is already reduced at the persistence/domain level to stable identity context: gender, birth date, derived age and persistence metadata. Migration 58 previously removed mutable health fields such as height, weight, occupation, sleep and injury history from `user_profiles`, so this checkpoint does not preserve those historical profile-as-health-record fields.

The legacy `PUT /profile` handler decoded the request directly into `model.UserProfile`. vNext replaces that with an explicit generated command containing only:

```text
gender
birth_date
```

Client-controlled `id`, `user_id`, `age_years`, `created_at` and `updated_at` are not accepted by the public schema. Gender is also one finite vocabulary (`male | female`) across profile editing and onboarding.

### Explicit profile absence

The original `GET /profile` returned a root JSON `null` for a new user. Although legal JSON/OpenAPI, the selected Go and Web generators represented root-level nullability differently. vNext uses an explicit read envelope instead:

```json
{ "profile": null }
```

or

```json
{ "profile": { "...": "validated profile" } }
```

`PUT /profile` similarly returns `{ "profile": ... }`. This removes generator-specific ambiguity and makes first-use absence an explicit read-model state.

### Runtime trust and handler retirement

- `/me` and `/profile` route ownership moved to the generated strict server;
- legacy `ProfileHandler`, `AuthHandler.Me` and `dto.UserResponse` were removed from the production path;
- Web profile state uses generated Zod validation before projecting into store state;
- session verification/fetch-user validates `/me` through the generated response schema rather than `safeJson<User>` assertions;
- malformed `/me` identity data fails closed and clears the session during verification;
- profile editing exposes a narrow `UserProfileUpdate` instead of `Partial<UserProfile>`.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         24
missing                       71
coverage                   25.26%
```

### Verification

```text
pnpm lint                              PASS
pnpm typecheck                         PASS
pnpm test                              PASS
  contracts                            12/12
  Web                                  227/227
  Python                               475/475
  Go                                   go test ./... PASS
pnpm build                             PASS
pnpm contracts:verify                  PASS
git diff --check                       PASS
legacy ProfileHandler                  DELETED
handwritten /me + /profile routes      NONE
```

The generated-client bundle observation remains tracked by `BS-VNEXT-REST-007`; runtime response validation remains mandatory.
