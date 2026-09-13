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

## Next slice

`GET /api/v1/health-workspace`.

Unlike the architecture spike, the production schema must model the actual composite workspace projection closely enough that runtime response validation is meaningful; generic `object` placeholders will not be promoted merely to claim route coverage.
