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
