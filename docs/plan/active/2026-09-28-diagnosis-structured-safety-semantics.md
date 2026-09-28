# Diagnosis structured safety semantics

Status: Active. Batch A foundation is implemented on this branch; Batch B requires a separately qualified successor. Decision: [ADR 0017](../../adr/0017-adopt-structured-safety-semantics.md).

## Current state and failure history

BodyState persists facts, review status, exclusions, and a separate `SafetyState`. Go pins a BodyState revision, calls Python Diagnosis, evaluates its own `diagnosis-decision-policy-v1`, and stores a frozen replay input. The Python pre-agent gate and post-agent governance still construct text for `RedFlagDetector`. The default Champion is v3 (`diag-config-5a4a13627e14b4cf`); v4 through v7 are immutable, known alternatives, not a new default. Exact identities are guarded by existing configuration tests.

The successive negation experiments explain the need for a source-aware contract. v5 introduced explicit negation awareness, yet its post-agent rule required an absence cue adjacent to a red-flag term; a valid intervening phrase remained a blocker. v6 introduced a finite bridge allowlist to cover that concrete case. v7 tested list-style negation. These revisions refine text matching but cannot make a sentence represent source identity, fact review state, time, and durable review authority. A v8 design must therefore be a new structured successor, rather than another implicit change to v3–v7. The observed v5/v6 history is recorded in the archived [v5](../archive/2026-09-27-bs-upg110-diagnosis-negation-v5.md) and [v6](../archive/2026-09-27-bs-upg110-diagnosis-negation-v6.md) plans; this plan does not depend on unmerged v8 code.

Concrete boundary failures to prevent: `dizziness=false` in source A cannot cancel `dizziness=true` in source B; current absence cannot clear an active legacy review state; a monitoring legacy state must retain evidence without gaining an active blocker; prose in `Fact.Value` or unrelated `Details` cannot create safety authority. Unknown or malformed durable `SafetyState` must stop normal Diagnosis delivery.

## Authority and data flow

`SafetyState` is durable business review authority. `SafetyAssertionV1` records a bounded concept, polarity, temporality, review state, source identity, optional observation time, and optional JSON-safe provenance. An assertion is evidence, including negative or nonblocking evidence. `SafetyEnvelopeV2` is Go's deterministic, revision-pinned projection of that evidence plus typed active blockers. `requires_review` is derived only from blockers. A blocker names the same concept/source as a current present or uncertain assertion and states the bounded reason. Optional provenance and free-form notes never authorize or clear a blocker.

```text
BodyState revision + durable SafetyState + eligible structured fact details
      -> Go pure projector -> SafetyEnvelopeV2
      -> internal Diagnosis request -> Python strict transport validation
      -> frozen private replay input
      -> [Batch B only] successor pre-agent and post-agent decisions
      -> Go versioned DecisionAuthority -> persisted governed result
```

Go owns projection policy and final delivery authority. Python validates the wire object and, in Batch B, may consume it under an immutable configuration. The transport validator checks identity and consistency, not policy reconstruction. `Fact.Value`, free-form detail text, and provenance remain outside the authority inputs. Unknown JSON in legacy `SafetyState` fails closed; non-object or non-boolean non-authority fact details are ignored. Snapshot and envelope must refer to the same exact revision. A projection failure returns `INVALID_SAFETY_STATE` before the ordinary AI call. No transaction, new persistence table, or mutable configuration pointer is introduced by Batch A.

## Work items and acceptance

| Ticket | Phase and dependency | Scope and acceptance |
| --- | --- | --- |
| DGS-SAFE-000 | A, first | Record ADR, protected contracts, authority owner, replay and rollout boundaries. Acceptance: ADR and plan agree on source, blocker, and failure semantics. |
| DGS-SAFE-010 | A, after 000 | Define Go closed concepts, polarity, temporality, review, source, blocker reason, assertion, blocker, envelope; implement pure sorted projector. Acceptance: boolean true yields current present assertion and typed blocker; false yields current absent assertion only; independent sources remain independent; excluded or non-authority details do not affect projection. |
| DGS-SAFE-020 | A, after 010 | Adapt existing legacy `SafetyState` JSON strictly. Acceptance: active/requires_review maps every positive flag to assertion and blocker; monitoring keeps assertions without blockers; resolved/cleared false does not block; unknown positive flags become `unknown_red_flag`; malformed, contradictory, or unknown state fails closed. |
| DGS-SAFE-030 | A, after 010–020 | Carry envelope over internal Go→Python request and freeze it in replay input using an explicit nullable parameter. Acceptance: revision/policy match is enforced, old replay JSON without the optional field still decodes, and no public API field changes. |
| DGS-SAFE-040 | A, after 030 | Mirror bounded Go types in strict Python models. Acceptance: schema/policy, revision, source/reason, blocker/assertion match, current positive polarity, and `requires_review` are validated; v3–v7 runtime behavior remains unchanged. |
| DGS-SAFE-050 | A, after 010–040 | Shared Go/Python fixture and targeted regressions. Acceptance: all source, legacy, prose, excluded, unverified, malformed, typed mismatch, and optional provenance cases pass; default and historical configuration identity tests pass. |
| DGS-SAFE-060 | B, after A | Add a new immutable Diagnosis manifest and new policy revision consuming the envelope before the agent and when authorizing output. Acceptance: no normal candidates when typed blockers require review; absence in another source cannot clear them; all outcomes have pinned configuration, policy, envelope, and decision trace. |
| DGS-SAFE-070 | B, after 060 | Add source-boundary replay cases, eval slices, and shadow comparison against Champion. Acceptance: zero critical unsafe relaxations, complete artifact identity, and predeclared non-inferiority/promotion evidence on the same dataset fingerprint. |
| DGS-SAFE-080 | B, after 070 | Exercise existing Champion→shadow→canary 5%→25%→50%→promoted state machine with explicit human promotion. Acceptance: each gate meets the existing minimum clean observation count and pause/rollback thresholds; no automatic Champion swap. |
| DGS-SAFE-090 | B, after 080 | Audit historical replay and rehearse rollback. Acceptance: old replay inputs and v3–v7 identities remain readable; reverting the serving pointer restores prior Champion without rewriting persisted analyses. |

## Batch A exact boundary

Implemented here: the versioned Go projector, strict legacy adapter, internal request field, optional frozen replay field, Python transport model, shared contract fixture, and regression tests. The projector reads only active, reasoning-eligible, confirmed or unverified facts with supported boolean detail keys. It does not inspect fact prose or copy arbitrary provenance. Provenance is optional on the contract and round-tripped using a synthetic non-identifying fixture. No Batch A consumer makes serving decisions from this envelope; the v3–v7 Python text detector, post-agent governance, Go DecisionAuthority v1, and default Champion continue their existing live behavior, except malformed or unknown durable `SafetyState` fails closed before the call.

## Batch B successor and capture migration

Create a new immutable configuration (candidate v8) with a new governance/decision-policy identity. Route only that configuration through structured pre-agent blocking and structured output checks. Keep the old detector for historical v3–v7 replay, consultation/free-text intake fallback, posture compatibility, and shadow diagnostics. Go's new decision revision must make a typed blocker deny normal delivery independent of model confidence or prose; Python must not be able to clear it. Post-agent safety findings need a separate typed, reviewable path rather than silently mutating BodyState.

Capture migration is additive. Existing `SafetyState` JSON remains readable; current fact producers may continue emitting their existing details. Inventory each producer and add versioned, bounded safety fields only after its capture/review semantics are reviewed. Do not infer `false` from missing keys. Preserve per-source references and review state. Any later provenance projection needs an explicit allowlist of small, non-sensitive keys and matching privacy tests. No backfill should invent historical certainty or rewrite old analyses. If a producer cannot emit a trusted structured field, its free text stays on the legacy intake path until it can.

## Replay, evaluation, rollout, and rollback

Batch B qualification starts from frozen historical inputs: old cases without an envelope use their historical configuration; new cases carry the exact pinned envelope. Build deterministic source-boundary, current/absent, legacy monitoring, malformed-state, unknown-concept, and output-governance cases. Record dataset/configuration fingerprints and compare hard safety outcomes first, then semantic candidate/support identities, then presentation. A shadow run may compute a successor result without serving it or writing a new DiagnosisAnalysis; compare against Champion and store only governed observation metadata. Require existing promotion-policy readiness and no critical unsafe relaxation before canary. Canary follows stable subject bucketing and existing observation/rollback gates. Promotion is an explicit state transition after reviewed evidence; no Batch A deployment or promotion is implied.

Rollback selects the prior Champion with the existing rollout mechanism, stops serving the successor, and leaves frozen inputs, immutable manifests, and prior analyses intact. Rehearse the transition in local or controlled staging before promotion. If the envelope or revision is invalid, fail closed rather than falling back to prose. If shadow comparison reveals unsafe relaxation, identity mismatch, or forbidden side effects, trigger the existing rollback outcome and investigate the frozen case before a new attempt.

## Compatibility matrix

| Surface | Batch A | Batch B requirement |
| --- | --- | --- |
| v3–v7 serving/configuration IDs | Unchanged runtime and exact IDs; envelope is transported but not authoritative | Remain immutable and replayable; successor is a new ID |
| Default Champion | v3 unchanged | Change only through existing reviewed promotion state machine |
| Legacy replay JSON without envelope | Decodes because field is optional and omitted when nil | Historical route keeps its original semantics; no invented envelope |
| New replay JSON | Pins envelope to BodyState revision | Successor consumes only exact matching frozen input |
| Public Diagnosis API/OpenAPI | No change | No change unless separately specified and versioned |
| Durable BodyState/SafetyState | Read-only projection; malformed state fails closed | Producer migration additive and explicit |

## Risk register

| Risk | Mitigation and evidence |
| --- | --- |
| Cross-source negation suppresses a positive | Keep independent source references; fixture asserts A absent plus B present yields B blocker. |
| Monitoring evidence disappears or accidentally blocks | Assert mapped current present evidence with zero blockers in shared fixture. |
| Unstructured details change old live behavior | Ignore non-object and non-boolean fact details; keep v3–v7 detector and policy unchanged; regression tests. |
| Unknown durable state silently authorizes candidates | Strict legacy adapter and fail-closed application error; malformed fixture. |
| Transport drift across Go and Python | One shared JSON fixture, strict enum validation, blocker linkage tests, exact revision checks. |
| Provenance leaks identifiers or acquires authority | Project none in Batch A; synthetic round-trip only; later allowlist and privacy review. |
| Historical identity or replay changes | Explicit nil replay parameter, `omitempty`, exact config tests, immutable successor only. |
| Successor improves one negation case but relaxes another | Critical safety evals, paired replay, shadow observations, staged canary, rollback threshold. |

## BS-UPG-110B exit criteria

Batch A: all DGS-SAFE-000–050 acceptance cases green; Go targeted/full tests and vet, Ruff, Pyright, targeted/full Python tests, shared fixture, docs links, and `git diff --check` pass; one amended foundation commit is pushed to this feature branch. Batch B may begin only with the detailed successor design and immutable identity reviewed. BS-UPG-110B may close only when DGS-SAFE-060–090 have qualification artifacts, zero critical unsafe relaxations, shadow and canary gates, explicit promotion decision, rollback exercise, and historical replay compatibility evidence. A documented HOLD is valid when promotion evidence is insufficient; it does not authorize a silent default change.

## Protected contracts and non-goals

Protect exact BodyState revision, durable review authority, Go DecisionAuthority v1 for historical configurations, public OpenAPI, v3–v7 manifests and promotion records, default Champion, and existing RedFlagDetector compatibility paths. Batch A excludes a runtime authority switch, public API change, database migration, new regex, free-text interpretation, deployment, canary, promotion, and PR creation. No staging or production changes belong to this branch repair.
