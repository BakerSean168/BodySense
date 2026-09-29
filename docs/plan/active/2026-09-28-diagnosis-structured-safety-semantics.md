# Diagnosis structured safety semantics

Status: Active. Batch A foundation and Batch B DGS-SAFE-060/070 successor evidence are implemented. DGS-SAFE-080 has begun in controlled staging: v8 exposed a provider-capacity failure, v9 removed the duplicate context and executed successfully, and the first v9 shadow comparison exposed an outcome-only rollout-comparator mismatch that is now being repaired with a versioned structured-authority policy. DGS-SAFE-080 is not yet accepted and DGS-SAFE-090 remains outstanding. Decision: [ADR 0017](../../adr/0017-adopt-structured-safety-semantics.md).

## Batch B checkpoint — DGS-SAFE-060/070

The immutable successor is `diag-config-62d312942b76a154` (`diagnosis-v8-structured-safety.yaml`). Its prompt, output, governance, and Go decision revisions are `diagnosis-prompt-v5-structured-safety`, `diagnosis-output-v3-structured-safety`, `diagnosis-governance-v8-structured-safety`, and `diagnosis-decision-policy-v2-structured-safety`. The default Champion remains v3, and `diagnosis_promotion_v6` only registers v3→v8 as a known route. No rollout stage or serving pointer changed.

`SafetyEnvelopeV2` now includes `body-state-safety-coverage-v1`, pinned to `body-state-safety-capture-v1`. Coverage is complete only when every eligible active discomfort fact has all five strict boolean fields and a matching marker, with at least one such fact. The Go projector sorts covered/incomplete source refs; Python validates the typed envelope. Go and Python v8 bypass the model for active blockers or incomplete coverage. Go v2 remains final authority and uses the same frozen envelope sent to Python. Valid model findings escalate for review without mutating durable `SafetyState`. v8 post-agent governance validates source refs and does not rescan prose. Historical v3–v7 routes retain their detector and decision policy v1.

The ten-case [structured-safety dataset](../../../apps/ai-service/data/evals/diagnosis_structured_safety_qualification.yaml) has fingerprint `7ff22d4eaa9b1f6e8402f7df5647da9d77315b18da6a8a7809afb44d4e4b3876`. Its versioned applicability declarations yield a five-case shared subset covering development, holdout, regression, and challenge, plus five v8-only cases for contracts absent from the historical runtime. The v3 Champion passes 5/5 applicable cases; v8 passes 10/10, including 6/6 critical safety cases. The paired shared comparison is non-inferior over 5/5 cases with zero critical regressions. The five candidate-only cases are reported separately with their applicability reasons; they are v8 contract evidence, not claims of v3 improvement. The dedicated policy report passes 21/21 checks, including explicit producer capture, Go preflight consistency, Go final authority, and shared fixtures. The old dataset and historical reports retain their fingerprints.

The v6 [promotion readiness report](../../../apps/ai-service/data/evals/reports/diagnosis_promotion_readiness_v6.json) is **ready for shadow** because both configurations qualify on their applicable cases, use the same full dataset fingerprint, the shared paired comparison is non-inferior, and the policy report is 100%. This authorizes entry to a shadow experiment only. `interaction_experiment.required` remains true because projection, Python preflight, model output, Python governance, and Go final authority cross service boundaries. No staging, shadow, canary, promotion, or rollback exercise has occurred. The default Champion remains v3.

The Consultation symptom form now asks a mandatory, first, exhaustive safety checklist within its three-field limit. The web form submits selected options as a list. Python attaches `body-state-safety-capture-v1` and five strict booleans only for a valid select-all answer; Go independently validates the bound checklist and persists those values on the same confirmed capture. AI extraction remains unmarked and excluded from reasoning. Invalid checklist answers create no complete capture, so v8 abstains on incomplete coverage. Go v8 preflight uses its own payload: incomplete coverage is an accepted abstain with a capture-specific summary, while an active structured blocker is rejected with its explicit reason. The v1 historical preflight and v2 deny-overrides decision rule are unchanged.

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

## v9 immutable context repair (2026-09-29)

DGS-SAFE-080 remains incomplete. The retained first v8 staging shadow observation failed
before comparison: Groq qwen/qwen3.8-27b returned provider capacity/request-size 429
(7000 ITPM limit, approximately 7434 requested input tokens); OpenRouter fallback
was unavailable due to insufficient credits. v8 never executed. Champion v3 executed
(approximately 6428 input tokens), then historical prose governance falsely blocked
repeated structured concepts (放射痛/头晕/外伤), despite coverage_complete=true and
zero envelope blockers. This is legacy prose-governance evidence, not a v8 unsafe
relaxation. The original observation must remain retained without reset or deletion.

Implementation plan: derive pure compact BodyState/history prompt views; filter
successor-only safety detail metadata for v3–v7 compatibility; preserve v8 prompt
construction and all historical identities/evidence; add v9 with full separate envelope,
DecisionPolicyV2 and promotion v7 lineage; qualify on the unchanged dataset; run full
Python/Go checks and docs links before one commit and branch push.

Frozen replay, governance BodyState/envelope, source IDs and safety decisions remain
complete and unchanged. v9 removes transport duplication and isolates new rollout
evidence. No provider spending, deployment, Champion swap or PR is part of this repair.
After merge, staging must verify actual provider usage, new shadow samples, reviewed
canary gates and explicit promotion; DGS-SAFE-090 replay/rollback remains outstanding.

### Independent review repair

The first v9 implementation incorrectly applied compact/deduplicated prompt views to
v3–v7 while retaining their old configuration IDs. Independent review rejected that
as an immutable-replay violation. The repaired implementation now dispatches three
explicit context modes: byte-stable legacy v3–v7 plus fact-only forward-compatibility
sanitation, fully frozen v8, and deduplicated/compact v9. Tests pin legacy and v8
reference strings byte-for-byte and verify that safety-key filtering never crosses from
fact payloads into observation/hypothesis/current-context details. The composite
`current_context.updated` revision is covered explicitly: only `changes.facts[]` payloads
are sanitized while `changes.observations[]` remains byte/value-identical.

### v9 deterministic qualification outcome

- Configuration: `diag-config-ba10b8e6820c3691`; prompt:
  `diagnosis-prompt-v6-structured-safety-context`; promotion: `diagnosis_promotion_v7`.
- Unchanged dataset fingerprint:
  `7ff22d4eaa9b1f6e8402f7df5647da9d77315b18da6a8a7809afb44d4e4b3876`.
  v9 inherits v8's explicit applicability in the evaluator without editing the corpus.
- Qualification 10/10; policy 37/37; v3 shared cases 5/5 non-inferior,
  five candidate-only cases explicit, zero critical regressions. Readiness is true
  for shadow only; interaction_experiment.required remains true.
- Representative dynamic JSON (including full envelope): 10,219 → 5,325 characters
  (47.89% reduction). This is not a provider token estimate.
- Python: prompt context 19 passed; diagnosis/config/governance/eval selection
  remains covered by the full suite; full suite 655 passed. Ruff passes; Pyright reports
  zero errors/warnings. Fresh environment requires both dev and ocr extras for the
  existing OCR imports; no dependency or lockfile change.
- Go: config/promotion selection 52 passed; full suite 703 passed and 15 skipped
  test/subtest events across 16 passing packages (11 packages have no tests);
  `go vet ./...` passes. Historical configuration/dataset/eval files (68 tracked
  files) byte-compare unchanged against the base. Five local links in changed
  documents resolve; `git diff --check` passes.

Reproduce v9 artifacts from `apps/ai-service` (all deterministic):

```bash
uv run --extra dev --extra ocr python scripts/run_diagnosis_eval.py \
  --dataset "$PWD/data/evals/diagnosis_structured_safety_qualification.yaml" \
  --configuration-id diag-config-ba10b8e6820c3691 \
  --compare-to data/evals/reports/diagnosis_structured_safety_v8.json \
  --json-output data/evals/reports/diagnosis_structured_safety_v9.json
uv run --extra dev --extra ocr python scripts/run_diagnosis_structured_safety_policy_eval.py --context-successor
uv run --extra dev --extra ocr python scripts/run_diagnosis_promotion_eval.py \
  --policy data/evals/diagnosis_promotion_policy_v7.json \
  --json-output data/evals/reports/diagnosis_promotion_readiness_v7.json
```

Historical v3–v7 serialization remains byte-stable for frozen inputs that predate the
structured-safety detail fields; current inputs receive fact-only compatibility sanitation
without compaction or history deduplication. Historical v8 serialization remains fully
unchanged. Only v9 uses compact JSON and deduplicates `recent_revisions`; history
`source` remains semantic provenance while v9-only row IDs/user IDs are removed.
Observation/hypothesis/non-fact `details` are explicitly preserved. No staging evidence
was changed by this repair.

## v9 staging shadow and rollout-authority comparator v2 (2026-09-29)

The first controlled v9 shadow smoke ran on staging revision
`7c7b6a7d063532f9d3724172489c75fea5c7d583` with v3 served and v9 replayed. The v9
provider call executed successfully, so the context repair closed the prior v8 7000-ITPM
request-size failure. The retained source analysis
`434143fa-ffcb-4b21-8c00-ce3e821bfbfa` showed a different governance boundary:
Champion v3 executed, then its historical prose `red_flag_safety` detector rejected the
model output because the output mentioned `放射痛` and `外伤` while explaining that those
signals were absent. The same frozen replay input had complete SafetyEnvelopeV2 coverage,
zero active blockers, and `requires_review=false`; v9 completed normally with no
`safety_findings`, configuration mismatch, forbidden side effect, or shadow error.

`diagnosis_promotion_v7` used the historical outcome-only
`diagnosis-rollout-policy-v1`, so it correctly recorded the raw `block -> allow-normal`
change as `unsafe_relaxation=true`. That observation is retained as immutable staging
evidence and is not deleted, reset, or reinterpreted. DGS-SAFE-080 therefore did not
advance under v7.

The repair introduces a new immutable promotion record, `diagnosis_promotion_v8`, for
the same v3 Champion and v9 Challenger. It explicitly binds
`diagnosis-rollout-policy-v2-structured-authority`. The v2 comparator does not hide the
raw hard or semantic mismatch. Instead it adds a separate authority classification and
marks a difference gate-equivalent only when all of the following are proven on the
exact frozen replay: artifact identity matches; SafetyEnvelopeV2 is present and complete;
there are zero active blockers and no review requirement; the Champion is decision-policy
v1 and was blocked only by legacy `red_flag_safety` post-agent governance; every legacy
red-flag category must exactly match a concept that is `current + confirmed + absent` on
every covered SafetyEnvelope source (unmapped categories such as `infection`, `systemic`,
or broad `neurological` remain unsafe); the Challenger
is decision-policy v2, governance accepted, has zero safety findings, and has no forbidden
side effect. Any missing condition remains `unsafe_authority_relaxation=true` and triggers
the existing rollback gate. Abstain-to-allow and non-prose governance blocks are never
authorized by this migration rule.

Rollout evidence is also cohort-bound by both rollout-policy revision and promotion
record. Historical v7 observations without v2 authority metadata remain in the v1 cohort;
new v8/v2 shadow errors and missing reports remain in the v8 cohort and pause progression
instead of disappearing through filtering. Canary comparisons normalize served/shadow
direction before classifying Champion versus Challenger. Approved authority migrations
remain visible in raw comparison JSON, increment `authority_migrations`, and are excluded
only from hard/semantic mismatch rate gates.

Deterministic evidence for the comparator is
`data/evals/reports/diagnosis_rollout_authority_policy_v1.json`: 15/15 checks pass,
including fail-closed negative cases, v1 preservation, v7/v8 cohort isolation, shadow
error retention, missing-report retention, promotion-cohort separation, and canary
direction normalization. `diagnosis_promotion_readiness_v8.json` requires both the
structured-safety policy (37/37) and rollout-authority policy (17/17), uses the unchanged
qualification dataset fingerprint
`7ff22d4eaa9b1f6e8402f7df5647da9d77315b18da6a8a7809afb44d4e4b3876`, and is ready
for shadow only; `interaction_experiment.required` remains true.

DGS-SAFE-080 acceptance still requires a new `diagnosis_promotion_v8` shadow cohort of
at least 20 clean observations, followed by the predeclared 5%/25%/50% canary gates. No
production promotion is authorized by this comparator repair.

### Rollout comparator v2 validation

Before delivery, the comparator repair passed the complete local acceptance set: the new
rollout-authority policy report is deterministic at 15/15, promotion v8 readiness is
`ready_for_shadow=true` with both required policy reports at 100%, and a second
regeneration produced identical policy/readiness hashes. Regenerating historical
`diagnosis_promotion_v7` readiness is byte-for-byte identical to its committed artifact.
The AI service passes Ruff, Pyright (0 errors/warnings) and 656/656 pytest cases. The Go
API passes `go test ./...` and `go vet ./...`, including the rollout status command build.
Six protected v7/v9 qualification/configuration artifacts byte-compare unchanged against
`origin/main`; five local links in the changed ADR/plan resolve and `git diff --check`
passes. These results permit a new v8 shadow experiment only; they do not close
DGS-SAFE-080 or authorize canary/promotion by themselves.
