# PR6 B4a · Remove legacy candidate persistence

First independent B4 slice, following B3 PR #291 (`b36364ec6`).

- Apply BS-D9: Assessment's generated source selections stay in its immutable
  report/replay. Do not insert unadmitted candidates into either BodyState ledger.
  Pin the report to the input BodyState revision without creating a revision.
- Remove report fields that falsely imply a durable BodyObservation or clinical
  review identity. Preserve source references and Go-rendered report content.
- Delete lifestyle candidate projection, review commands, routes and React UI.
  User-authored lifestyle editing continues through the native V2 record path.
- Regenerate Go/TypeScript/Zod/Postman contracts. No compatibility response.

The next B4 slice replaces legacy Safety JSON with explicit records and validated
resolution. C remains responsible for legacy tables/primitives and other routes.

Local validation passed: report-only persistence/revision behavior, lifestyle
user edits, OpenAPI transport and browser fixture updates; full Go/PG18 suite,
build/vet/gofmt/staticcheck (SA/S1/QF), migration/domain validators, generated
contracts/Postman checks, Web typecheck/eslint and 485 Vitest tests, plan
formatting and 107 plan checks. Full CI, including browser E2E, remains pending.

No deployment or staging/production database operations.
