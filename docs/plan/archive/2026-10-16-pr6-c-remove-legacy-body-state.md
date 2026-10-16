# PR6 C · Remove legacy BodyState persistence and API

Base: origin/main `4d3dc793c9f11cf02e9c9179454e4dac3fd9aaf6` (#296).

## Inventory and scope

D, B2b, B3, B4a, E dead binding cleanup and B4b are merged. Native ledger methods still share the aggregate repository; retain aggregate identity, revisions and all V2 methods. Delete legacy fact/observation/evidence entities, mutation methods and public routes. Web no longer consumes workspace body_state; remove its public projection, retaining the internal frozen native snapshot for capabilities and Safety. Migrate domain-validator and privacy erasure fixtures to V2. Directly edit baseline and regenerate checksums and Go/TS/Postman artifacts.

## Guardrails

Native facts/observations/statement evidence own body data; owner checks precede projection. Only user wording becomes facts. Go alone decides Safety using native restrictions/resolutions; unavailable or incomplete coverage remains blocked. Agent reads stay frozen. Keep revision locks, atomic commits, idempotency and immutable source history. No compatibility path, deployment, live database cleanup, or new down migration.

## Verification

Run contract generation/lint/conformance, migration integrity, Go vet/tests including fresh isolated PostgreSQL integration, Web lint/typecheck/tests, Postman generation, architecture and plan checks. Create one PR, validate exact head CI, rebase latest main, force-with-lease, and merge with --match-head-commit without --admin.

## Local acceptance evidence

- Fresh isolated PG18: full `go test -p 2 -race ./... -count=1` passed, including owner isolation, native frozen reads, native Safety overlap/resolution and privacy erasure. Legacy table absence and retired route tests pass.
- Native domain-validator passed assertion replay/correction, canonical laterality, fail-closed unscreened coverage, exact synthetic screen coverage, treatment activation atomicity and outcome feedback atomicity.
- Go vet and configured staticcheck passed. Web lint/typecheck and all 81 files / 485 tests passed.
- Five Playwright tests passed: deterministic assessment evidence, insufficient evidence, full longitudinal loop, durable records after reload and workflow gates after reload.
- Contract lint/conformance and regeneration consistency, 106-route Postman verification, baseline checksums, architecture/mutation checks and 107 plan tests passed.
- Engineering audit: native aggregate identity/revisions and V2 repository/service methods remain because they own active consumers. Old persistence and public contracts have no runtime consumers. Native Safety restrictions/resolutions and frozen Agent inputs remain unchanged; the validator's all-negative screen is disposable fixture data and is installed only after proving missing coverage fails closed.

Implementation and local verification complete. Exact-head CI and merge provenance belong to the slice's GitHub PR. No deployment or business database operation is included.
