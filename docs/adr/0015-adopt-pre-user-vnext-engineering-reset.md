# ADR 0015: Adopt a Pre-User vNext Engineering Reset

- Status: Accepted
- Date: 2026-09-13
- Decider: BodySense owner
- Master plan: `docs/plan/active/2026-09-13-bodysense-vnext-engineering-reset.md`

## Context

BodySense has accumulated multiple implementation generations while its architecture matured rapidly. The project already has strong accepted ownership decisions, but runtime code still carries migration-era aliases, compatibility projections, historical serving branches, duplicated cross-language contract definitions and a long schema migration lineage.

The project currently has no external user population whose client compatibility must be preserved and no business data that requires a schema/data migration bridge. The owner has also frozen concurrent product modification for the duration of the refactor.

Under those constraints, an ordinary expand/dual-read/dual-write/contract migration would preserve complexity that has no user or data value.

## Decision

BodySense will perform a coordinated vNext engineering reset on the integration branch `refactor/bodysense-vnext`.

The refactor may make breaking internal/public development-contract changes before final release, provided final product behavior and accepted domain/safety invariants are preserved or intentionally re-decided by ADR.

### Compatibility policy

The vNext runtime will not retain code solely for pre-vNext compatibility.

Delete rather than bridge:

- old request/response aliases;
- old state spellings and status aliases;
- old auth-token fallback semantics;
- old DB columns/tables used only by prior models;
- legacy serving Agent configuration aliases/branches;
- deprecated environment variables and compatibility adapters;
- dual read/write paths;
- compatibility projections that exist only for old clients/data.

Historical artifacts may remain as offline documentation/eval fixtures when they provide useful evidence, but they cannot remain selectable production paths merely to preserve development history.

### Database policy

Because business data does not require preservation, the final vNext release will establish a clean PostgreSQL 18 schema baseline rather than carrying the old migration chain forward as a production requirement.

The cutover is an explicit destructive environment reset, not a fake migration that invents values or compatibility semantics for nonexistent important data.

### Contract policy

ADR 0014's measured boundary-specific strategy is adopted as the production direction:

- public REST: OpenAPI 3.1 spec-first;
- public StreamEvent: JSON Schema 2020-12 first;
- internal Go/Python Agent protocol: Proto + Buf + Protovalidate;
- transport decisions independent; HTTP/NDJSON retained initially;
- generated models remain transport-boundary types, not domain models.

### Branch policy

The master refactor uses one integration branch and multiple bounded implementation branches. `main` receives no partial refactor. Each child branch must be independently reviewed and validated before merging into the integration branch.

## Protected invariants

This reset is not permission to weaken:

- authentication/authorization;
- BodyState durable ownership;
- Diagnosis/Treatment provenance and Go decision authority;
- safety gates;
- explicit cancellation and HITL semantics;
- data privacy/security;
- backup/release/deployment fail-closed behavior;
- evidence/mechanism qualification requirements.

Operational rollback and immutable release history remain safety mechanisms, not compatibility debt.

## Consequences

### Positive

- final code can represent one current architecture instead of every prior migration step;
- schema and runtime state vocabularies can be canonical immediately;
- contract codegen can replace duplicated handwritten surfaces directly;
- tests can target the final trust boundaries rather than old/new parity bridges;
- the active migration chain becomes small and understandable;
- the refactor can delete more code than it adds.

### Negative

- pre-vNext local/dev data will not be preserved automatically;
- old development clients/bundles are not supported after cutover;
- the refactor must be completed and validated as one coordinated release rather than merged piecemeal to `main`;
- destructive schema reset requires explicit deployment handling.

## Rejected alternative

### Incremental compatibility migration

Rejected for this reset because there is no external-user or business-data requirement that justifies dual paths. Keeping compatibility layers would directly conflict with the goal of simplifying the repository before the product has a compatibility obligation.
