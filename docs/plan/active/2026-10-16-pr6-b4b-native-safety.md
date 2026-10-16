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
