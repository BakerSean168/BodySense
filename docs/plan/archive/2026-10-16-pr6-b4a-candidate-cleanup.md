# PR6 B4a · Remove legacy candidate persistence

Status: COMPLETE / MERGED as [PR #292](https://github.com/BakerSean168/BodySense/pull/292), merge `b8ba34fa192dc9a1dd2fa46b93505713b95881d8` (2026-10-16). The local/CI-pending statements below are chronological checkpoints, not current pending work. PR6-C subsequently removed the legacy tables/API; final E acceptance is tracked in [PR6 E closeout](./2026-10-16-pr6-e-closeout.md).

First independent B4 slice, following B3 PR #291 (`b36364ec6`).

- Apply BS-D9: Assessment's generated source selections stay in its immutable
  report/replay. Do not insert unadmitted candidates into either BodyState ledger.
  Pin the report to the input BodyState revision (including zero for an empty
  ledger) without creating a revision.
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
formatting and 107 plan checks. Full CI, including browser E2E, was pending at that local checkpoint.

Revision-zero regression: the HTTP boundary test reproduced a 500 caused by the
old minimum-one contract; the native input revision now accepts zero and is
preserved in the response. The empty-input browser case asserts this value.
Both Assessment browser E2E scenarios passed locally after the correction.

No deployment or staging/production database operations.
