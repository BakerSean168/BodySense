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

## Consequences

- A later immutable Diagnosis successor can consume typed safety evidence without changing the meaning of historical configurations.
- Old analyses retain their historical replay inputs; new analyses freeze the envelope alongside the pinned BodyState.
- A projection failure prevents an ordinary Diagnosis call and must be surfaced as a fail-closed result.
- Structured detail coverage is deliberately narrow until producers emit a reviewed, versioned safety assertion contract.
