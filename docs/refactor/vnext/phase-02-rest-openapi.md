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

## Checkpoint 6 — Privacy erasure boundary

Status: COMPLETE ON PHASE BRANCH

The destructive privacy workflow now uses the generated OpenAPI boundary:

```text
GET  /api/v1/privacy/erasure-plan
POST /api/v1/privacy/erasure
```

### Safety semantics preserved and made explicit

The old Gin handler performed two important transport side effects after a durable erasure request was accepted: `Cache-Control: no-store` and immediate clearing of the refresh cookie. The strict-server migration does not drop those behaviors. The 200 plan response and 202 acceptance response now model no-store headers, and the 202 contract also declares `Set-Cookie` for refresh credential removal.

Refresh cookie construction/clearing moved into `internal/auth/refresh_cookie.go`, shared by the existing authentication handler and the OpenAPI privacy adapter. Cookie name, `/api/v1/auth` path, HttpOnly, Secure and SameSite=Strict therefore no longer have two handwritten implementations.

### Confirmation is a boundary invariant

The destructive phrase is modeled as the exact literal `DELETE ALL BODY DATA` in both plan and request schemas. Wrong phrases and unknown fields are rejected by OpenAPI validation before the erasure application service. The service retains its own confirmation guard as defense in depth for non-HTTP callers.

The browser privacy service now uses generated Orval/Zod functions through the existing authenticated fetch seam; malformed plan/acceptance responses fail closed.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         26
missing                       69
coverage                   27.37%
```

### Verification

```text
pnpm contracts:verify                  PASS
pnpm lint                              PASS
pnpm typecheck                         PASS
pnpm test                              PASS
  contracts                            12/12
  Web                                  230/230
  Python                               475/475
  Go                                   go test ./... PASS
pnpm build                             PASS
git diff --check                       PASS
legacy PrivacyHandler                  DELETED
handwritten privacy route regs         NONE
feature-source hardcoded privacy URLs  NONE
```

### Security-domain blocker discovered

`BS-VNEXT-REST-013` records a Phase 02 architectural prerequisite: the generated server is currently mounted as one protected Gin group. The remaining login/register/refresh/logout routes, public share route and operator-only Knowledge routes require different middleware domains. They must not be migrated by registering the full generated server into multiple groups or by weakening existing guards.

## Checkpoint 7 — Client diagnostics telemetry

Status: COMPLETE ON PHASE BRANCH

```text
POST /api/v1/client-diagnostics
```

is now owned by the generated protected OpenAPI router. The legacy Gin `ClientDiagnosticHandler` and handwritten route were removed.

### Privacy-safe request boundary

The public request schema now makes the browser telemetry envelope explicit:

- `schemaVersion` is exactly `1`;
- category is one of `chat.transport`, `body3d.viewer`, `app.runtime`;
- severity is one of `info`, `warn`, `error`;
- event/message/id/resource fields retain bounded lengths;
- `elapsedMs >= 0`;
- at most 24 attributes are accepted;
- attribute keys are bounded and values are scalar string/number/boolean/null only;
- nested objects and arrays fail before logging.

The existing Go sanitizer remains as defense in depth and continues stripping origin/query information from diagnostic resource URLs before logging. The strict adapter also preserves Gin request-id and User-Agent context by reading the concrete `*gin.Context` supplied by the generated Gin wrapper.

### Numeric parity

A first generation pass revealed that an unqualified OpenAPI `number` became Go `float32`. The schema now marks elapsed time and numeric scalar attributes as `format: double`, preserving the prior `float64` telemetry semantics.

### Browser behavior preserved

`reportClientDiagnostic` now calls the generated operation through `openApiAuthFetch`, but remains deliberately fire-and-forget. Failed telemetry responses are swallowed and cannot interfere with the user-facing path being observed. Browser-side finite-number filtering and 0.1 ms rounding remain intact.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         27
missing                       68
coverage                   28.42%
```

### Verification

```text
pnpm contracts:verify                       PASS
pnpm lint                                   PASS
pnpm typecheck                              PASS
pnpm test                                   PASS
  contracts                                 12/12
  Web                                       232/232
  Python                                    475/475
  Go                                        go test ./... PASS
pnpm build                                  PASS
git diff --check                            PASS
legacy ClientDiagnosticHandler              DELETED
handwritten client-diagnostics route        NONE
feature hardcoded client-diagnostics URL    NONE
```

## Checkpoint 8 — Assessment REST boundary

Status: COMPLETE ON PHASE BRANCH

The full Assessment REST family now uses the generated protected OpenAPI router:

```text
POST /api/v1/assessment/generate
GET  /api/v1/assessment
GET  /api/v1/assessment/{id}
POST /api/v1/assessment/{id}/replay
GET  /api/v1/assessment/{id}/regression-export
```

The legacy `AssessmentHandler` was deleted. Assessment application and replay services retain domain/runtime ownership; the new transport adapter only validates, maps and classifies HTTP behavior.

### Versioned report contract

Current generation is explicit: `POST /assessment/generate` can return only `assessment-output-v2`. Historical reads remain a discriminated public union:

```text
AssessmentReport = AssessmentReportV2 | AssessmentReportV1
                  discriminated by contract_revision
```

V2 now exposes its real evidence semantics rather than generic objects:

- the six canonical coverage domains are required;
- available sources use the fixed `body_state | report | posture_analysis` vocabulary;
- evidence gaps have typed domains/sources and `required=false`;
- evidence-grounded observations use the fixed observation-kind vocabulary and exactly one evidence reference;
- new reports do not expose pseudo health grades or dimension scores.

Historical v1 keeps the immutable A-D grade and five numeric dimension scores, while its non-reconstructable v2 coverage fields remain explicit empty compatibility structures instead of being invented during reads.

### Replay and regression export

Historical/counterfactual replay and regression export now have structured response schemas. Frozen regression `inputs` remain an intentional `JsonObject`: they are historical cross-version snapshots and must not be falsely normalized to the current input model.

Counterfactual replay now distinguishes an invalid configuration selector from an internal failure. Unknown configuration ids are typed as `ErrAssessmentReplayConfiguration` and map to HTTP 400 `INVALID_CONFIGURATION`; unavailable frozen replay artifacts remain 409 conflicts.

### Server-side response semantic validation

While adding Assessment characterization tests, a missing-domain fixture proved that decoding into generated Go structs was not sufficient runtime validation: missing required nested fields were silently represented as Go zero values and could be returned as HTTP 200.

`strictOpenAPIConvert` now validates application read models against the actual generated kin-openapi component schema before decoding into generated Go types. This enforces required fields, enums, bounds and collection constraints. The focused repair was applied to every complex response projection already migrated in Phase 02 (Assessment, HealthWorkspace, BodyState reads, stable Profile, Privacy and health-context projections), while request-to-application mapping remains behind the request validator.

### Browser boundary

The Web Assessment service now uses generated Fetch + Zod for generate/get/list and then projects into the existing feature model. The UI still receives its v1/v2 domain-facing union, not transport generator types. Tests prove a v2 response missing one canonical evidence domain fails closed, while list responses accept both immutable v1 history and current v2 reports.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         32
missing                       63
coverage                   33.68%
```

### Verification

```text
pnpm contracts:verify                 PASS
pnpm lint                             PASS
pnpm typecheck                        PASS
pnpm test                             PASS
  contracts                           12/12
  Web                                 236/236
  Python                              475/475
  Go                                  go test ./... PASS
pnpm build                            PASS
git diff --check                      PASS
legacy AssessmentHandler              DELETED
handwritten Assessment route regs     NONE
feature-source Assessment URLs        NONE
complex migrated response projections semantic-schema validated
```

Known baseline test stderr noise and the existing BodyExplorer3D bundle warning remain unchanged. `BS-VNEXT-REST-007` continues to track generated-client bundle review for the end of Phase 02.

## Checkpoint 9 — Conversation / share / durable-event REST boundary

Status: COMPLETE ON PHASE BRANCH

The conversation browser surface is now owned by the generated OpenAPI router:

```text
GET    /api/v1/conversations
GET    /api/v1/conversations/{id}
PATCH  /api/v1/conversations/{id}
DELETE /api/v1/conversations/{id}
PATCH  /api/v1/conversations/{id}/pin
PUT    /api/v1/conversations/{id}/title
POST   /api/v1/conversations/{id}/title
POST   /api/v1/conversations/{id}/share
DELETE /api/v1/conversations/{id}/share
GET    /api/v1/conversations/share/{token}
GET    /api/v1/conversations/{id}/runs
GET    /api/v1/conversations/{id}/runs/{runId}/events
```

The legacy ConversationHandler and RuntimeEventHandler are deleted. The public share read is registered in the unauthenticated capability-URL partition; all user-owned conversation, run and event operations stay in the authenticated partition.

### Public projection boundary

The first handoff draft mirrored persistence models directly into OpenAPI. That would have frozen internal fields such as `user_id`, provider conversation identifiers, active execution pointers and frozen agent configuration/provenance into the browser contract. The final adapter instead projects explicit public transport types before strict schema validation.

Characterization tests prove the conversation list omits persistence-only identity/provider/agent fields while preserving the browser fields required by the consultation UI. Shared snapshots are validated as public `ConversationMessage` objects rather than arbitrary JSON objects.

### Browser boundary

The consultation feature no longer constructs handwritten URLs for migrated conversation operations. It calls the generated Orval Fetch + Zod operations through `openApiAuthFetch` or `openApiPublicFetch`, then maps transport output into the existing feature-domain model. Pagination naming is deliberately normalized from transport `hasMore/nextCursor` to feature `has_more/next_cursor`; generated transport types do not leak into React state.

Durable run-event reads now validate the OpenAPI response first and then pass each event through the canonical `parseStreamEvent` runtime contract before it reaches recovery logic.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         48
missing                       47
coverage                   50.53%
```

### Verification

```text
pnpm contracts:lint                         PASS (2 known route-ambiguity warnings for static share vs {id})
pnpm contracts:check-generated              PASS
Go httpapi + cmd/server tests                PASS
Web consultation service                    20/20 PASS
Web typecheck                               PASS
public share bypasses bearer auth           PASS
conversation public-projection leak test    PASS
feature handwritten /api/v1/conversations  NONE
git diff --check                            PASS
```

The OpenAPI linter's two `no-ambiguous-paths` warnings describe the long-standing public share shape `/conversations/share/{token}` overlapping the `{id}` namespace in abstract OpenAPI routing. Gin's static-segment precedence is covered by the security-domain characterization test. The path remains unchanged in Phase 02 so the canonical 95-route baseline is not rewritten during migration.

## Checkpoint 10 — Consultation runtime / SSE / thread REST boundary

Status: COMPLETE ON PHASE BRANCH

Eight durable consultation routes are now owned by the generated OpenAPI boundary:

```text
POST /api/v1/consultation-runs
POST /api/v1/consultation-runs/{id}/cancel
POST /api/v1/consultation-runs/{id}/replay
POST /api/v1/consultation-runs/{id}/replay/counterfactual
GET  /api/v1/consultations/{id}
GET  /api/v1/consultations/{id}/thread
POST /api/v1/consultations/{id}/interrupts/{interactionId}/answers
GET  /api/v1/consultations/{id}/interaction-metrics
```

The old `ConsultationHandler`, `ThreadProjectionHandler`, and consultation HTTP DTO package are removed. Runtime commands now live under `internal/consultation`, so neither generated OpenAPI types nor legacy HTTP DTOs leak into the durable Agent runtime.

### SSE boundary

`startConsultationRun` and `resumeConsultationInteraction` remain true streaming operations. The strict OpenAPI adapter passes Gin's real `ResponseWriter` to the runtime, which writes and flushes SSE directly; the returned response object is a no-op visitor so the strict handler cannot buffer or write the stream a second time.

A characterization test proves an emitted SSE frame is written exactly once. Contract-invalid image upload IDs are rejected by the OpenAPI request validator before the consultation runtime is invoked.

On the Web, streaming calls use generated URL builders plus generated Zod request schemas while retaining raw `authFetch` responses for incremental SSE consumption. Non-streaming consultation reads/cancel/metrics use the generated Orval Fetch + Zod clients and map transport projections back to feature-domain types.

### Public projection boundary

The thread contract exposes the browser workbench projection, not the persistence model. Active turn events are projected as public RuntimeEvent v1 envelopes; tool calls explicitly normalize absent result/error to JSON null; body-state load failure remains non-fatal and is represented as `body_state: null`, preserving legacy semantics.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         56
missing                       39
coverage                   58.95%
```

### Verification

```text
pnpm contracts:lint                         PASS (same 2 known share-path ambiguity warnings)
pnpm contracts:check-generated              PASS
Go consultation/httpapi/cmd-server tests    PASS
Web consultation service                    21/21 PASS
Web typecheck                               PASS
SSE single-write characterization           PASS
invalid image UUID rejected pre-runtime     PASS
legacy Consultation/Thread handlers         REMOVED
legacy consultation HTTP DTO                REMOVED
git diff --check                            PASS
```

## Checkpoint 11 — Diagnosis application / analysis / replay REST boundary

Status: COMPLETE ON PHASE BRANCH

The complete Diagnosis REST family is now OpenAPI-authoritative:

```text
POST /api/v1/consultations/{id}/diagnosis
GET  /api/v1/diagnosis-analyses
GET  /api/v1/diagnosis-analyses/{analysisId}
PUT  /api/v1/diagnosis-analyses/{analysisId}/assessment
POST /api/v1/diagnosis-analyses/{analysisId}/replay
GET  /api/v1/diagnosis-analyses/{analysisId}/regression-export
```

### Application ownership

The former `DiagnosisHandler` mixed HTTP concerns with BodyState readiness, safety gating, Agent configuration selection, immutable replay input, rollout observation, Evidence persistence, hypothesis projection, governance review and consultation phase transitions. That orchestration now lives in `service.DiagnosisApplicationService`, which exposes a transport-neutral `Analyze` use case and stable application error codes. The OpenAPI adapter owns only authentication, status-code mapping and public projection.

The old `DiagnosisHandler` is deleted. Its three characterization tests were preserved at the application layer: selected Agent configuration identity, missing consultation session, and unavailable BodyState diagnosis domain.

### Public projection hardening

The previous read path embedded persistence models for freshness and candidate assessments, leaking `user_id` into browser JSON. The vNext public schema and adapter now remove that persistence identity. Health Workspace reuses the same diagnosis sanitizer so its strict generated response cannot regress to the old leak.

`candidate_assessments` and `freshness` are optional augmentations of the base immutable analysis projection. This matches actual service semantics: Analyze does not inherently create candidate assessments, and freshness evaluation is best-effort on the direct analysis route. Health Workspace retains its stronger product invariant and fails closed if an existing diagnosis projection lacks review state.

### Replay and compatibility boundary

Historical/counterfactual replay has a structured generated report. Regression export remains an explicitly opaque developer-dataset envelope. Analyze temporarily returns a generated `JsonObject` because the characterized legacy pre-envelope governance-rejected branch returns a transient non-durable object without analysis identity; this looseness is confined to one compatibility mapper and is not used for durable history.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         62
missing                       33
coverage                   65.26%
```

### Verification

```text
pnpm contracts:lint                         PASS (same 2 known share-path ambiguity warnings)
pnpm contracts:check-generated              PASS
Go test ./...                               PASS
Web consultation service                    23/23 PASS
Web typecheck                               PASS
handwritten diagnosis feature URLs          NONE
nested freshness/assessment user_id leak    BLOCKED BY PROJECTION TEST
Diagnosis application characterizations     PASS
git diff --check                            PASS
```

## Checkpoint 12 — Treatment / TrainingPlan acceptance / Outcome REST boundary

Status: COMPLETE ON PHASE BRANCH

The complete revisioned Treatment + Outcome REST family is now OpenAPI-authoritative:

```text
POST /api/v1/treatments/proposals
GET  /api/v1/treatments/current
POST /api/v1/treatments/current/review
GET  /api/v1/treatments/revisions
GET  /api/v1/treatments/revisions/{revisionId}
POST /api/v1/treatments/revisions/{revisionId}/replay
GET  /api/v1/treatments/revisions/{revisionId}/regression-export
POST /api/v1/treatments/revisions/{revisionId}/accept
POST /api/v1/treatments/revisions/{revisionId}/reject
POST /api/v1/outcomes
GET  /api/v1/outcomes
```

The legacy `TreatmentHandler` is deleted. Read-only current-treatment preview remains separate from the mutating review command. Acceptance remains atomic: the generated adapter delegates exclusively to `TrainingService.AcceptTreatmentAndEnsurePlan`, preserving the treatment-acceptance + TrainingPlan projection transaction boundary.

### Public projection hardening

The old persistence-backed JSON exposed `user_id` on Treatment, Intervention, TrainingPlan and Outcome. Those fields have been removed from the public OpenAPI components. One transport presenter sanitizes those exact persistence identities and is reused by the standalone endpoints and Health Workspace, including nested revision interventions.

The Web workspace feature no longer models these persistence identities and no longer contains handwritten Treatment/Outcome URLs. Proposal, accept/reject, current review and Outcome recording use generated Orval Fetch + Zod clients behind `openApiAuthFetch`.

### Replay / feedback semantics

Treatment replay has a structured generated report and a regression test proving the service report shape satisfies the public schema. Regression export remains an intentionally opaque developer dataset envelope. Outcome idempotency is preserved: an existing result returns 200, a new Outcome returns 201.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         73
missing                       22
coverage                   76.84%
```

### Verification

```text
pnpm contracts:lint                         PASS (same 2 known share-path ambiguity warnings)
pnpm contracts:check-generated              PASS
Go test ./...                               PASS
Web workspace OpenAPI tests                 10/10 PASS
Web typecheck                               PASS
handwritten treatment/outcome URLs          NONE
Treatment/Intervention/Plan/Outcome user_id BLOCKED BY PROJECTION TEST
atomic acceptance boundary                  PASS
Treatment replay strict-schema regression   PASS
git diff --check                            PASS
```

## Checkpoint 13 — Training execution / feedback / reassessment REST boundary

Status: COMPLETE ON PHASE BRANCH

The complete Training execution REST family is now OpenAPI-authoritative:

```text
GET  /api/v1/training
GET  /api/v1/training/{id}
GET  /api/v1/training/{id}/today
POST /api/v1/training/{id}/checkin
PUT  /api/v1/training/{id}/log
GET  /api/v1/training/{id}/progress
POST /api/v1/training/{id}/reassess
```

The legacy `TrainingHandler` and `ReassessmentHandler` are deleted. The generated adapter maps validated transport inputs into `TrainingFeedbackInput` and preserves the existing service ownership for daily task materialization, check-in Outcome persistence, structured feedback, deterministic progress, and reassessment-driven Treatment proposal generation.

### Public projection hardening

TrainingPlan continues to use the public projection introduced with Treatment migration. TrainingLog now has its own explicit browser projection and does not expose persistence `user_id`. Feedback responses reuse the Treatment/Outcome public presenters, so a nested Outcome or Treatment proposal cannot reintroduce persistence identity through the Training API.

The Web Training feature no longer contains handwritten `/api/v1/training` calls. List/get/today/check-in/log/progress/reassess all use generated Orval Fetch + Zod clients behind `openApiAuthFetch` while preserving the existing feature-level types.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         80
missing                       15
coverage                   84.21%
```

### Verification

```text
pnpm contracts:verify                       PASS (same 2 known share-path ambiguity warnings)
Go test ./...                               PASS
Web Training service                        3/3 PASS
Web typecheck                               PASS
handwritten Training production URLs        NONE
TrainingLog user_id leak                    BLOCKED BY PROJECTION TEST
nested feedback Outcome/Proposal projection PASS
git diff --check                            PASS
```

## Checkpoint 14 — Upload / health-document review REST boundary

Status: COMPLETE ON PHASE BRANCH

The complete authenticated Upload and health-document human-review REST family is now OpenAPI-authoritative:

```text
POST   /api/v1/uploads
GET    /api/v1/uploads
GET    /api/v1/uploads/posture-analysis
GET    /api/v1/uploads/{id}
DELETE /api/v1/uploads/{id}
GET    /api/v1/uploads/{id}/health-document-review
GET    /api/v1/uploads/{id}/extractions/{runId}/reviews
POST   /api/v1/uploads/{id}/extractions/{runId}/reviews
GET    /api/v1/uploads/{id}/extractions/{runId}/source
```

The legacy `UploadHandler` and `HealthDocumentReviewHandler` are deleted. Multipart upload remains owned by `UploadService`: the generated strict boundary parses the OpenAPI multipart stream into a standard `multipart.FileHeader` and delegates validation, private-object storage, durable manifest creation and derived OCR/posture jobs to the existing application service. The private source endpoint remains streaming and preserves the stored PDF/JPEG/PNG/WebP media type with `X-Content-Type-Options: nosniff`.

### Public projection hardening

Pre-user vNext removes three compatibility/persistence identities that the browser never needed:

- `UserUpload.user_id`
- deprecated `file_path` (which was only a projection of private `storage_key`)
- `DocumentIndicatorReviewRecord.reviewer_user_id`

`storage_backend`, `storage_key` and `agent_configuration_id` remain server-private as well. Generated Zod schemas are strict: a nominal 200 response that reintroduces `user_id` or `file_path` fails closed in the Web boundary.

`GET /uploads` now returns the explicit `{ uploads: [...] }` response component rather than a bare array so Orval applies generated Zod runtime validation to each upload manifest. This is an intentional pre-user contract reset, not a compatibility alias.

### Web ownership

`uploadStore` now uses generated list/create/delete clients through the central `openApiAuthFetch` authority. Multipart request input is parsed with the generated `CreateUploadRequest` Zod schema before transmission. Health-document review context/action/source operations also use generated clients; the append-review request is validated by the generated request schema, 404 context remains mapped to `null`, and Blob source loading keeps the existing UI behavior. Feature-level upload/review types remain handwritten application models and generated transport types do not leak into components.

### Coverage after this batch

```text
Phase 00 routes               96
operational exclusions         1
browser-facing eligible       95
OpenAPI-authoritative         89
missing                        6
coverage                   93.68%
```

### Verification

```text
pnpm contracts:verify                       PASS (same 2 known share-path ambiguity warnings)
Go test ./...                               PASS
Web UploadStore + Review UI                 9/9 PASS
Web typecheck                               PASS
handwritten Upload production URLs          NONE
legacy Upload/Review handlers               REMOVED
multipart -> FileHeader characterization    PASS
private source MIME + nosniff                PASS
upload/review persistence identity leaks     BLOCKED BY PROJECTION/ZOD TESTS
git diff --check                            PASS
```
