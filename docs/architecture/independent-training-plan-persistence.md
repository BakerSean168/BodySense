# Independent TrainingPlan persistence · TP-002 slice 1

Business authority: [TrainingPlan](../product/domain/training.md) and
[Safety](../product/domain/safety.md). Implements the persistence/decision slice
of [#311](https://github.com/BakerSean168/BodySense/issues/311).

`service.NewIndependentTrainingPlanService(db)` is the callable application
boundary. `Propose`, `Accept`, `Reject`, `List`, `Get`, and `Revision` require the
authenticated owner supplied by the trusted caller. `Accept` and `Reject` are
explicit manual user commands; an Agent generation result may call `Propose`
only. The service must never be passed an Agent-invented user decision. There
are no new HTTP routes or generation adapters in this slice; slice 2 binds
those adapters to this service instead of adding a second acceptance path.

A proposal stores a complete candidate snapshot, including stable template and
exercise entry IDs, and the exact base revision for updates. A whole-document
JSON replacement records the accepted change set; before/after snapshots are
recoverable through the parent revision. No patch is silently generated or
applied after authorization. Replacement exercises must use new entry IDs;
entry and template identities cannot move to another plan.

Each write carries an idempotency key. A manual decision additionally carries
its decision ID, presented content fingerprint, target plan, and expected base
revision (both zero for create). A receipt identifies the exact user command,
proposal, plan, and immutable accepted revision. A different payload under the
same key conflicts. An exact retry returns the original receipt/revision and
recomputes Safety from current authoritative state, rather than replaying an
old execution permission.

The application transaction locks `users FOR NO KEY UPDATE`, then the owner's
`body_states FOR UPDATE`, the same head used by native BodyState and Safety
writers, then `diagnoses FOR UPDATE`. It reads the durable proposal, evaluates `EvaluateAcceptance`,
projects TrainerGate from the native ledger, writes a revision and plan CAS,
and commits the receipt and proposal decision together. Database uniqueness
and composite owner/plan FKs enforce idempotency and identity. Any write or
commit failure rolls back the plan, revision, links, and receipt. Rejected
proposals never write a plan. Expired proposals persist only their expiry
reason, with no accepted revision or decision receipt.

Context freshness is deliberately conservative in this first slice: any native
BodyState revision, Safety source availability/validity boundary, or change to
the owner's native `diagnoses.current_revision` invalidates a pending proposal.
The transaction locks the native Diagnosis head after the BodyState head, so
concurrent adoption cannot race its snapshot. A missing aggregate is initialized
at revision zero; it creates no ConditionEntry or DiagnosisRevision and does not
prevent an independent plan with no Diagnosis association. Legacy
`diagnosis_analyses` are never read as an acceptance authority.

Related ConditionEntries must belong to the owner, be adopted, and refer to a
committed revision within the locked head. Their current snapshots and typed
evidence links enter the fingerprint. Current source availability is re-read
from owner-bound statement evidence, formal analysis request evidence, and
admitted document sources; the original assessment-time claim validity is not
an availability authority. Shared source locks keep the checked source stable
through commit. Missing, blocked, unresolvable, or unreadable associated sources
fail closed. These checks preserve existing Diagnosis semantics; they neither
adopt diagnoses nor infer clinical eligibility from source text. Reads are
bounded to 200 related conditions and 1,000 evidence links. This is not a
clinical relevance algorithm. No Diagnosis lifetime or clinical threshold is
introduced. The Training PlanProposal TTL alone is 168 hours.

Training §4.5 permits saving a prescription under an existing restriction.
Consequently the returned TrainerGate is a current **execution projection**,
not a condition that can change plan status or authorize training from a save
receipt. Missing/unscreened/unreadable Safety never projects allow. A stale
proposal context expires before any plan write. A Trainer start/continue must
always perform its own fresh gate check; Trainer sessions/actions are TP-003.

BodyFact, SymptomEpisode, and BodyObservation links validate exact owner and
creation revision and use composite FKs in revision links. Native ConditionEntry
associations use `training_plan_revision_conditions` with owner composite FKs to
both the immutable plan revision and the native condition. Foreign/missing
related IDs produce the same owner-masked not-found result at proposal creation
and acceptance; a known association with unreadable authority is unavailable.

The baseline drops the single-active index. Existing Treatment columns and
consumers remain until slice 3 retires them; no compatibility adapter or flag
is added. Their existing supersede query cannot touch a plan with an accepted
independent revision. Independent plan updates preserve the user's existing
active/paused/completed status and never touch another plan or any logs.

Validation: `scripts/plan/test-tp002-pg18.sh` creates only its own disposable,
network-isolated, memory-bounded PostgreSQL 18 container and Unix socket. It
validates baseline up/down/up and runs the service acceptance tests. Add
`-race` for the serialized race run. The database CI lane runs the same native
service regressions with `-race`. `TP_D1_EXPECTATION=v2
scripts/plan/test-tp-d1-multi-active-compat.sh` proves the removed unique index.
No staging/production access or retained database reset is part of this work.
