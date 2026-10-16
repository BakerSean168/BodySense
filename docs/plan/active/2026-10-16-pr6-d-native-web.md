# PR6 D · Native V2 Web reads

Status: local validation passed; PR/CI pending.

## Scope and sequence

The product owner approved D before B2b so each PR compiles independently without a
legacy compatibility projection. Merge order: D → B2b → B3 → B4 → C; E may advance.
This slice reads the existing V2 records, observations, hypotheses and Episode APIs.
It does not change public contracts or persistence.

## Decisions

- Generated OpenAPI clients and Zod output schemas remain the transport authority.
  Browser aggregate types derive their rows from those clients.
- The aggregate reads all paginated sources with a bounded cursor walk, validates
  page and row ownership, and requires the same BodyState revision across sources.
  Revision drift or incomplete reads fail and can be retried; they are not empty data.
- Query keys include the signed-in owner and remain under workspace invalidation.
- Facts display the user's statement and canonical region associations. Assertion
  status is not a symptom lifecycle. No trend or clinical confirmation is inferred.
- Episode identities remain distinct from statement counts. Observations currently
  have no region links, and hypotheses do not become facts or map markers.
- Go workspace capabilities own Safety prompts. Web no longer interprets the old
  safety JSON. Legacy Workbench commands and browser projection types are deleted.
- Person reuses existing V2 fact amendments and Episode association controls, with
  native raw observation display. Hypothesis/ConcernGroup interaction is a later slice.

## Acceptance and validation

- Web typecheck, ESLint, build and affected Vitest, including owner mismatch,
  revision drift, pagination failures, multiregion statements and failed-read UI.
- Full Go suite against an isolated migrated PostgreSQL 18 database; Go build,
  vet and staticcheck; migration and domain validators.
- Prettier for this plan and `pnpm plan:check`.
- CI green before rebase and exact-head merge. No readiness gates raised, no deploy.

Local evidence: Web typecheck, ESLint and build passed; full Vitest 81 files /
484 tests passed, followed by 31 affected tests including the added retry case.
Full PG18 Go suite, build, vet, staticcheck, migration replay/checksum and domain
validators passed. Plan validation passed. Public contracts and baseline unchanged.
