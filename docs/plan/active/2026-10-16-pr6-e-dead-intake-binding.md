# PR6 E1 · Delete dead Ask User V1 binding

Following B4a PR #292, remove the seed-based symptom-intake-binding-v1
normalizer and its obsolete unit test from the AI Ask User tool. The current
symptom-intake-binding-v2 references the exact committed record_body_state fact
and retains its runtime and deterministic E2E coverage. No Go persistence or
medical decision changes.

Local validation passed: Ruff check/format (309 files), 722 Python unit tests,
full Go tests against freshly migrated PG18, Go build/vet/staticcheck SA/S1/QF,
migration/domain validators and 107 plan checks. CI remains pending.

Remaining E work: clean legacy API/table architecture references after C and run
final full browser E2E on the pure V2 stack. This slice does not claim that gate.
