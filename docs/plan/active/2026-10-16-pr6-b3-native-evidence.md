# PR6 B3 · Native evidence audit

Replace Diagnosis citation writes to `body_state_evidence` and Treatment's
global evidence lookup with the selected analysis's immutable evidence audit.

- Build native fact/observation and statement-evidence references from the
  owner-scoped frozen Diagnosis input, never from AI-supplied source claims.
- Keep external citations and AI hypotheses in the analysis audit only.
  These references carry no BodyState authority.
- Treatment consumes only its selected analysis's evidence audit and freezes
  the exact input in replay. No global pool or legacy evidence writes.
  Preserve external string evidence IDs instead of discarding non-UUID IDs.
  External citations remain explicitly classified as audit with no authority.
- Preserve native source identities in Assessment's selectable evidence values.
- B4 owns Safety resolution and Assessment candidate admission; C owns deletion
  of the legacy tables, routes and remaining repository/service primitives.

Validation: behavior tests for persisted native references, owner/revision
rejection, selected-analysis Treatment evidence and replay equality; full Go
build/vet/staticcheck and fresh PG18 suite; migration/domain validators;
contract generation/checks; affected Python unit/lint/format and plan checks.

No deployment or staging/production database operations.
