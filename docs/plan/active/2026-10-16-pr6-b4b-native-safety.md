# PR6 B4b · Native Safety restrictions and resolutions

Baseline: E1 PR #293, main dd740fb650581563e4f0036891484d9ff707d9bc.

Product owner confirmed on 2026-10-16: any positive result of the Go five-concept
screen is restricted + all_training + professional_clearance. Continue Diagnosis
collection; only reviewed Go urgent rules may end formal Diagnosis. This is a
product execution mapping, not a validated clinical severity classification.

Replace body_states.safety_state JSON and global clear timestamps with separate
owner-bound restriction/resolution records. Lift only the explicitly named
restriction after exact revision, source and lift-basis validation. Preserve all
other active restrictions, source levels, replay inputs and fail-closed reads.
Go remains the only medical/Safety decision authority.

No new rule may map AI text directly to urgent or permit unsourced clearance.
No automatic plan rewrite; preserve existing review/pause behavior until the
Training/Safety vertical slice implements native Trainer gates.

Validation will cover owner isolation, overlapping restrictions, source rejection,
read failure, stale revision, concurrent add/lift and idempotency on PG18; full
local checks, generated contracts and browser CI before merge.

## B4b1: atomic restriction recording

First independently mergeable slice: persist one immutable restriction per
positive concept of a complete Go-validated screen, using the same transaction,
owner and revision as that screen. Negative screens create no restrictions.
Keep source/revision lineage owner-bound; source removal cannot silently delete
restrictions. Account erasure still removes them and reports their counts.

This slice changes only recording. It does not claim the B4b cutover is complete.
The next slice adds exact restriction resolution, switches native reads and
Diagnosis/Treatment policy, and deletes the old JSON/global-clear/AI-write paths.
No compatibility projection or deployment is introduced.

### B4b1 local evidence

- Red → green: real interaction-resume PG18 tests first failed for missing
  restrictions, then passed after atomic recording was implemented.
- Full PG18 Go suite, build, vet and SA*/S1*/QF* staticcheck passed.
- Fresh baseline apply, migration history/checksums and domain validators passed;
  no down/replay validation requested.
- 107 plan validation tests passed. Public contracts, Web and Python were not
  changed, so their generators and language checks are outside this slice.

The focused tests cover all five positive concepts, negative screens, exact retry,
wrong owner/revision, rejection of urgent writes, rollback on restriction storage
failure, preservation through source withdrawal/deletion attempts, privacy counts
and whole-account erasure. These checks do not prove the remaining B4b native
read/clearance/Diagnosis/Treatment cutover, which is explicitly still pending.

## B4b2a: restriction-specific professional clearance

Product owner confirmed the first clearance scope on 2026-10-16: an authenticated
user explicitly submits professional clearance for the exact restriction and all
training, bound to their original user message and verbatim statement. Keep the
source as user-reported professional clearance, never a clinical document. No
clinical-document path opens before document admission exists.

Use an owner-serialized, expected-revision checked transaction and an idempotent
resolution receipt. Persist the minimum original source in statement_evidence;
revision/outbox contain identifiers only. Reject foreign/deleted/assistant sources,
ambiguous or ordinary positive text, stale revision and changed-key retries.
Read resolutions with restrictions in the pinned BodyState snapshot. Invalidated
clearance evidence restores the conservative gate; it never silently permits
training. Resolving one restriction must preserve all overlapping restrictions.

Switch Diagnosis to distinguish restricted from urgent, Treatment to native
restriction and coverage reads, and remove JSON/global-clear/AI-write authority.
Freeze the native decisions in replay. Full Trainer gates remain a later vertical.

This independently mergeable slice exposes the native restriction read and
exact-target resolution commands. Diagnosis/Treatment and the old JSON/global
clear route are unchanged until B4b2b switches their consumers and removes them.
No readiness gate is raised. Native resolution receipts are not evidence that
legacy consumers have been switched.

The first command deliberately accepts only the complete confirmation message
`专业人员已针对这条限制及全部训练放行` (optional final `。`). The authenticated user
must submit the exact restriction ID; quoting, negating or embedding this phrase
in ordinary text is rejected. No model interprets free text as clearance. More
natural phrasing requires a separately source-bound confirmation interaction.

Confirmation messages must postdate the target restriction and may bind only one
restriction; old clearance cannot be reused for a new risk. StatementEvidence
retains the minimum original statement after ordinary Chat pruning. Blocked or
erased evidence makes that resolution ineffective in the native projection.

### B4b2a local evidence

- Initial red test failed for missing exact-target command/repository boundary;
  native PG18 restriction/clearance behavior and HTTP mapping now pass.
- Full PG18 `go test ./...`, build, vet and SA*/S1*/QF* staticcheck passed;
  focused resolution race tests passed.
- Tests reject foreign/assistant/deleted/missing sources, ordinary/quoted/negated
  prose, clearance predating a restriction, reused messages, stale revisions,
  changed idempotency bindings and duplicate lifts. They cover independent
  overlapping restrictions, concurrent retries/distinct lifts, outbox rollback,
  source invalidation, owner-bound SQL lineage and account erasure.
- Fresh baseline apply, migration validator without replay, domain validator,
  checksums/history and 107 plan checks passed.
- Public Go/TS generation, contract lint/generated checks and Postman verification
  (113 operations), Web typecheck/lint and 17 relevant Vitest tests passed.
- Fixed the shared integration fixture's unclosed database pools after the full
  suite exhausted PostgreSQL connections; the rerun passed without raising limits.
- Exact-head CI remains required. No deployment or domain readiness claim.
