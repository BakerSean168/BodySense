# PR6 B2b · Native V2 reads

Status: COMPLETE / MERGED as [PR #290](https://github.com/BakerSean168/BodySense/pull/290), merge `d93a7d9a27ca207927058e802ed98c4b968a77b3` (2026-10-16). The local/CI-pending statements below are chronological checkpoints, not current pending work. PR6-C subsequently removed the legacy tables/API; final E acceptance is tracked in [PR6 E closeout](./2026-10-16-pr6-e-closeout.md).

Scope: migrate Assessment, Diagnosis, workspace and GET `/body-state` to native
V2 records; delete `GetSnapshot` and `projectBodyContextIntoSnapshot`.

- Preserve V2 statement, source, assertion and time semantics. Never manufacture
  old discomfort/lifestyle/anthropometry rows.
- Pin the native aggregate and its history in one read snapshot. Reject owner
  mismatches, revision mismatches and incomplete bounded reads.
- Go projects Safety from asserted positive symptom facts and safety screens.
  Unscreened symptoms remain incomplete; unavailable Safety fails closed.
- Freeze the exact Agent input in existing replay envelopes.
- D merged first as PR #288 (`b93ce7c0a`) so native responses compile independently.
  B3 owns legacy evidence persistence removal. B4 owns Safety resolution JSON
  replacement and Assessment candidate writes;
  C removes the transitional public endpoints and workspace body_state field.

Validation: full Go build/vet/staticcheck and PG18 suite after applying baseline
migrations; domain/migration validators; generated REST/Postman checks; affected
Python checks and unit tests; plan formatting and `pnpm plan:check`.

CI follow-up: migrate browser assertions from legacy lifestyle values and pending
workspace candidates to native statements and persisted Assessment reports. The
longitudinal fixtures now capture and screen the same symptom; an unrelated
screen cannot authorize an earlier symptom. The Assessment E2E model selects real
activity evidence through the existing catalog, with a nonempty report assertion
so source rendering and persistence cannot pass vacuously.

No deployment or staging/production database operations.
