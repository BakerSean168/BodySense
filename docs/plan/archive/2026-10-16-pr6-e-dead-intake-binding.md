# PR6 E1 · Delete dead Ask User V1 binding

Status: COMPLETE / MERGED as [PR #293](https://github.com/BakerSean168/BodySense/pull/293), merge `dd740fb650581563e4f0036891484d9ff707d9bc` (2026-10-16). The local/CI-pending statements below are chronological checkpoints, not current pending work. PR6-C subsequently removed the legacy tables/API; final E acceptance is tracked in [PR6 E closeout](./2026-10-16-pr6-e-closeout.md).

Following B4a PR #292, remove the seed-based symptom-intake-binding-v1
normalizer and its obsolete unit test from the AI Ask User tool. The current
symptom-intake-binding-v2 references the exact committed record_body_state fact
and retains its runtime and deterministic E2E coverage. No Go persistence or
medical decision changes.

Local validation passed: Ruff check/format (309 files), 722 Python unit tests,
full Go tests against freshly migrated PG18, Go build/vet/staticcheck SA/S1/QF,
migration/domain validators and 107 plan checks. CI was pending at that local checkpoint.

E1 closed only dead binding removal. Its post-C architecture cleanup and full native browser acceptance are recorded in the separate E closeout linked above.
