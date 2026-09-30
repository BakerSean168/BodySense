# ADR 0017: Adopt Structured Safety Semantics

## Status

Accepted for Batch A foundation; runtime authority switch deferred.

## Date

2026-09-28

## Context

BodyState already owns durable safety state and Go owns Diagnosis DecisionAuthority. The current Diagnosis path also flattens facts and model output into prose for `RedFlagDetector`. That loses source, polarity, review, and time boundaries. A negative phrase in one source can affect interpretation of a positive phrase in another. This ADR defines the replacement contract without changing the current v3–v7 execution path.

## Decision

Go owns durable `SafetyState` and the canonical, revision-pinned `SafetyEnvelopeV2` projection. `SafetyAssertionV1` represents evidence semantics: bounded concept, polarity, temporality, review state, source kind/reference, optional observation time and bounded provenance. `SafetyState` represents business authority and explicit review status. Python validates and may reason over the envelope, but a future successor must not reconstruct safety authority from prose.

The envelope has schema revision `body-state-safety-envelope-v2`, policy revision `body-state-safety-policy-v1`, exact BodyState revision, assertions, typed `SafetyBlockerV1` objects, `requires_review`, and `legacy_state_present`. A blocker identifies concept, nonempty source reference, `SafetySourceKindV1`, and `SafetyBlockerReasonV1`; its reason is `structured_current_signal` for a BodyState fact or `legacy_active_state` for an active legacy state. `requires_review` is exactly whether blockers exist. It carries no user identity. Optional assertion provenance is a JSON-safe object; free-form text within it is never authority. Batch A projects no durable fact provenance by default, avoiding accidental transfer of sensitive data. The projector reads only supported structured boolean fact details, one fact at a time. It ignores non-object details and free-form or non-boolean known fields. It never interprets `Fact.Value` or free-form detail strings and never mutates BodyState.

Current, confirmed, reasoning-eligible `present` or `uncertain` assertions block. Current, unverified positive or uncertain assertions require review and block fail-closed if they are included in the eligible input; excluded facts produce no assertion. Absent assertions never clear a positive durable state. Historical and resolved assertions do not block. Legacy `requires_review` and `active` states block; `monitoring` follows existing Diagnosis policy and does not block. `resolved` and `cleared_by_review` do not block. Malformed, contradictory, or unknown legacy state fails closed as a projection error, so no normal AI call is authorized by that projection.

Legacy `{}` remains readable; existing `has_red_flags`/`status`/`flags` JSON remains readable without a destructive migration. Recognizable legacy flag categories map to bounded concepts; every other positive flag maps to `unknown_red_flag`. Legacy `monitoring` with `has_red_flags=true` retains its mapped present assertions without blockers, matching existing Diagnosis policy. Active legacy states emit one blocker per mapped assertion and remain blocked even when a current absent assertion exists. Only explicit review can clear durable positive safety. Python validates blocker/assertion identity, current positive or uncertain polarity, and source/reason compatibility at the transport boundary; it does not recompute Go policy.

`RedFlagDetector` remains available for historical replay, consultation/free-text intake fallback, posture compatibility, and shadow telemetry. It is not the north-star Diagnosis authority. Existing v3–v7 manifests, policies, defaults, and promotion records remain immutable. Batch A sends the envelope over the internal Go→Python request and freezes it in replay input; the Python service still executes existing v3–v7 behavior.

## Batch B refinement: capture coverage and successor authority

The branch-local `SafetyEnvelopeV2` now carries `SafetyCoverageV1` (`body-state-safety-coverage-v1`). Coverage requires at least one current active, reasoning-eligible `discomfort` fact and, for every such fact, a `body-state-safety-capture-v1` marker plus strict booleans for trauma, radiating pain, numbness, weakness, and dizziness in that same source. Missing marker, key, or object details make that source incomplete without creating a projection error. Other fact kinds do not affect coverage. Existing strict boolean assertions remain projected without the marker for historical transport compatibility. Coverage does not clear an active blocker, and absence in one source cannot clear another source's positive assertion.

The immutable v8 successor uses Go's frozen envelope for preflight and final authority. Incomplete coverage abstains before model execution; active blockers block before model execution. Its output schema adds current, present-or-uncertain `safety_findings` with exact pinned BodyState fact or observation references. Python governance validates those references and does not scan Diagnosis prose for red flags under v8. A valid finding is escalation evidence for Go, not a durable BodyState mutation. Go decision policy v2 applies deny-overrides and strips ordinary candidates on block, abstain, or escalation. v3–v7 retain decision policy v1 and their historical detector paths.

## Consequences

- A later immutable Diagnosis successor can consume typed safety evidence without changing the meaning of historical configurations.
- Old analyses retain their historical replay inputs; new analyses freeze the envelope alongside the pinned BodyState.
- A projection failure prevents an ordinary Diagnosis call and must be surfaced as a fail-closed result.
- Structured detail coverage is deliberately narrow until producers emit a reviewed, versioned safety assertion contract.

## Model-facing context views (v9 clarification)

Frozen replay and governance retain the full BodyState, relevant history and
SafetyEnvelopeV2. Prompt serialization is explicitly versioned rather than treated as
an implementation detail:

- v3–v7 keep their historical JSON serialization, including BodyState
  `recent_revisions`, the separately supplied history, row/user identifiers, and normal
  JSON spacing. Compatibility sanitation removes only structured-safety authority keys
  that were introduced later from `fact.details`. For an old frozen input that never
  contained those successor fields, the complete generated instruction string remains
  byte-for-byte identical to the pre-v9 runtime.
- v8 is fully frozen for replay: no sanitation, deduplication, compaction, or field
  removal is applied, and the original SafetyEnvelopeV2 suffix remains exact.
- v9 owns the new model-facing projection: BodyState drops duplicate
  `recent_revisions` and top-level `user_id`, history drops transport row/user IDs, and
  dynamic JSON is compact. Only fact payload `details` lose the six structured-safety
  authority keys; observation, hypothesis, current-context, and arbitrary non-fact
  `details` remain untouched. SafetyEnvelopeV2 is still supplied separately as the sole
  structured safety authority.

The projection is never written back to replay storage and is never substituted for
the full BodyState/envelope used by post-agent governance. This preserves immutable
historical identities while giving v9 a separately versioned context contract.

## Rollout authority comparison (v2 clarification)

A structured-safety successor intentionally changes the source of safety authority. An
outcome-only rollout comparator therefore cannot distinguish a real unsafe relaxation
from removal of a historical prose-detector false positive. Historical
`diagnosis-rollout-policy-v1` remains immutable and continues to classify any restrictive
Champion outcome followed by a Challenger allow outcome as unsafe.

New structured-authority experiments may opt into
`diagnosis-rollout-policy-v2-structured-authority` through a new immutable promotion
record. v2 preserves the raw hard, semantic, and presentation comparisons and adds a
separate, auditable authority classification. A Champion block may be gate-equivalent to
a Challenger allow only when the frozen SafetyEnvelopeV2 is complete, has no active
blocker or review requirement, the Champion block is exclusively legacy
`red_flag_safety` post-agent governance under decision policy v1, every red-flag
category exactly matches a concept proven `current + confirmed + absent` across every
covered SafetyEnvelope source, and the structured Challenger under decision policy v2 is
accepted with no safety findings or forbidden side effects. Broad or unmapped legacy
categories are never inferred from prose and remain unsafe. Artifact identity must also
match. Every omitted or contradictory premise fails closed as an unsafe authority
relaxation.

Rollout observations are summarized by rollout-policy revision and, for controlled
experiments, promotion record. This preserves failed/retired experiment evidence rather
than deleting rows or retroactively changing their meaning. Runtime errors and missing
comparison reports carry the same cohort identity so filtering cannot make failures
disappear. Authorized migrations remain countable as `authority_migrations`; their raw
mismatches stay durable even though they do not consume hard/semantic mismatch-rate
budget. Stable assignment, minimum sample counts, forbidden-side-effect gates, identity
gates, provider-error pause rules, canary steps, and explicit human promotion remain
unchanged.

## Rollout authority comparison (v3 pre-agent source typing)

Staging provider qualification later exposed a second historical prose-detector source:
the Python pre-agent safety gate can block before the model runs and stores its red-flag
categories in `safety_summary.red_flags`, while the durable Go decision still records the
v1 block reason `agent_output_failed_safety_governance`. That source was deliberately not
authorized by v2, whose contract was post-agent `red_flag_safety` governance only.
Changing v2 in place would make one rollout-policy revision mean different things across
repository commits, so v2 remains immutable.

`diagnosis-rollout-policy-v3-structured-authority` extends the evidence model with an
explicit legacy prose source identity. It preserves the complete v2 path and additionally
recognizes a pre-agent legacy source only when all of the following are exact: baseline
status is `safety_blocked`; decision policy is v1; the durable decision outcome is
`block`; the sole decision reason is `agent_output_failed_safety_governance`; execution
provenance is `status=bypassed` and `reason=python_pre_agent_safety_gate`; governance is
otherwise accepted with no issues; `safety_summary.red_flags.has_red_flags=true`; and
every emitted flag has a non-empty category. The existing structured proof is still
required afterward: complete frozen SafetyEnvelope coverage, no blockers or review
requirement, and every legacy category proven `current + confirmed + absent` across every
covered source. The structured Challenger must still be decision-policy v2, governance
accepted, free of safety findings, free of forbidden side effects, and artifact-identical.
Any missing source identity, mixed governance signal, unmapped category, incomplete
coverage, or contradictory evidence fails closed.

Policy revision and promotion record jointly define the rollout cohort. A failed or paused
v2 cohort is therefore retained unchanged and cannot be reinterpreted by v3. v3 uses a new
promotion record even when Champion and Challenger Agent configuration IDs are unchanged.
All progression thresholds and stop rules remain identical to v2.

Replay serialization is part of the rollout evidence contract even when it is not part of the Agent
configuration fingerprint. `ApplyDiagnosisDecision` returns a typed Go `DiagnosisDecision` inside a
`map[string]any`; before replay evidence extraction, both v1 and v2 paths MUST JSON-normalize that
payload so `decision_authority` has the same object shape as persisted/runtime JSON. A replay transport
fix that changes only this serialization behavior does not create a new Agent configuration or rollout
policy revision, but it MUST use a new promotion record after any failed cohort. promotion_v11 remains
immutable evidence of the canary failure that exposed the missing v1 normalization; promotion_v12 is
the fresh cohort for the corrected replay transport under the unchanged rollout policy v3.

## Historical replay and rollback boundary

Historical v3-v7 Diagnosis configurations are release-governance artifacts, not dead code. Their manifests and
configuration IDs remain immutable after v10 becomes Champion because persisted analyses must still resolve the
policy identity that originally produced them. Historical replay may recompute deterministic Go authority from the
frozen input, but it MUST NOT invent a modern SafetyEnvelope for old JSON, call a model merely to prove readability,
or persist a replacement analysis.

Rollback is defined as future-serving pointer movement only. A rollback may select a retained historical Champion for
new requests, but it MUST NOT rewrite existing `diagnosis_analyses`, candidates, replay input, raw output, execution
provenance, or DecisionTrace. DGS-SAFE-090 therefore protects the pre-rollback dataset with opaque row hashes and
requires the same protected root after both rollback and restore. New smoke analyses are allowed and remain immutable
records of the configuration that actually served them.

Production promotion is deliberately separate from staging acceptance. A provider-qualified staging route does not
qualify a different physical production route. If the production `bodysense-diagnosis` model differs from the model
used for final staging acceptance, the production gate remains HOLD until equivalent production-candidate provider
evidence exists. This provider boundary does not change the immutable Agent configuration fingerprint; it is a release
readiness requirement owned by the LiteLLM deployment plane.

## Final v10 acceptance boundary

The legacy v3 Champion is not a semantic ground truth for structured-safety Diagnosis. The v3/v10
rollout comparator remains useful for migration analysis, replay regression, and discovery of taxonomy
coverage gaps, but a random or broader legacy prose-detector block MUST NOT indefinitely veto a v10
configuration that satisfies its own typed safety contract. `diagnosis-rollout-policy-v3-structured-authority`
is therefore frozen as the final legacy-equivalence comparator revision for DGS-SAFE-080. Existing
promotion_v9 through promotion_v12 observations remain immutable historical evidence. No promotion_v13
is created merely to chase another legacy prose category.

Final staging acceptance of immutable Diagnosis v10 (`diag-config-3f64de162dc937ee`) is governed by a
separate `diagnosis-v10-final-acceptance-v1` contract. The hard gate is intentionally independent of a
Champion/Challenger chain and requires all of the following:

- the committed v10 deterministic qualification remains 10/10 and qualified;
- the structured-safety policy report remains 41/41, producing at least 51 deterministic hard checks
  across qualification and policy evidence;
- the staging physical route remains the pinned OpenAI-compatible `gemini-3.7-flash` route through the
  internal LiteLLM boundary; provider credentials remain runtime-only secrets;
- at least 20 paced real-provider executions of v10 pass the production-shaped output contract with no
  transport errors, governance rejections, configuration mismatches, forbidden side effects, or
  candidate/status contract failures;
- real-provider hard-gate samples are scoped to complete, nonblocking, non-legacy structured captures.
  Legacy-state migration cases remain covered by deterministic regression and by the frozen rollout
  evidence, but are not used as a physical-provider reliability oracle.

This split does not relax any safety authority. Incomplete coverage, active blockers, typed safety
findings, final Go decision authority, and fail-closed governance remain deterministic v10 contracts.
It only stops treating historical v3 prose behavior as the definition of correctness for the new
structured architecture. Any future behavior change to the v10 prompt, output schema, governance,
decision policy, generation budget, or typed safety contract still requires a new immutable Agent
configuration rather than mutation of v10.

### Generation budgets are immutable behavior

Diagnosis `generation.max_tokens` participates in the immutable manifest fingerprint and
therefore cannot be changed in place to accommodate a provider limit. A runtime provider
capacity finding must produce a new Agent configuration, while the predecessor remains
replayable with its original generation settings. The structured-safety context successor
(v9) remains fixed at 2048 max tokens. Its staging shadow established that the free-tier
Groq route enforces a 1000 output-tokens-per-minute request ceiling and rejected a request
whose expected output budget was 1196; the fallback route could not fund a 2048-token
request. The budget successor (v10) therefore pins 960 max tokens while retaining the same
prompt, output schema, governance, decision policy, model group, and safety authority.
This is a configuration change, not a gateway-wide override, so historical configurations
and unrelated logical model groups do not drift.
