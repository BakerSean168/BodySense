# Diagnosis Decision Authority and SafetyEnvelope

> Status: Historical Phase-6 implementation checkpoint; DecisionAuthority and rollout machinery are implemented. Diagnosis v3 remains the current repository Champion; v4 is a qualified Challenger, ready for shadow under `diagnosis_promotion_v2` only.

## Authority boundary

Python produces typed reasoning, candidate content, evidence-acquisition traces, and runtime governance evidence. Go is the only component allowed to decide whether that output may become an ordinary durable Diagnosis result.

The Phase 6 control path is:

```text
BodyState safety state
        +
Python Diagnosis payload
(status / governance / red flags / unresolved critical gaps)
        |
        v
Go DiagnosisDecisionPolicy v1
        |
        +--> allow-normal
        +--> allow-degraded
        +--> abstain
        +--> escalate
        +--> block
        |
        v
ApplyDiagnosisDecision
        |
        v
immutable DiagnosisAnalysis
```

Candidate confidence and any future semantic/Judge score are deliberately absent from the authority input. They can describe evidence quality but cannot override a business or safety deny.

## Deny-overrides order

`diagnosis-decision-policy-v1` is a pure deterministic function with this precedence:

1. unsupported policy revision or malformed/unknown policy facts -> `block`;
2. active durable BodyState safety review -> `block`;
3. Python hard governance rejection or `safety_blocked` status -> `block`;
4. a new runtime red flag -> `escalate`;
5. unresolved critical EvidenceGap -> `abstain`;
6. `insufficient_information` -> `abstain`;
7. partial status or degraded runtime governance -> `allow-degraded`;
8. completed + accepted + at least one candidate -> `allow-normal`;
9. every other inconsistent state -> `block`.

`block`, `escalate`, and `abstain` remove ordinary candidate delivery before persistence. `block`/`escalate` become durable `safety_blocked` analyses; `abstain` becomes durable `insufficient_information`. The raw durable analysis records the Go `decision_authority` object.

## SafetyEnvelope facts

The minimal deterministic envelope uses only facts already supported by current business semantics:

- durable `BodyState.safety_state`;
- Python runtime governance verdict as evidence, not authority;
- structured Diagnosis status and candidate cardinality;
- positive runtime `red_flags`;
- `EvidenceAcquisitionTrace.unresolved_critical_gaps`.

Unknown enum values, contradictory safety state, missing required fields, and unknown configuration/policy revisions fail closed.

## Configuration boundary

The current live path uses the v3 immutable Champion configuration:

```text
diag-config-5a4a13627e14b4cf
prompt:   diagnosis-prompt-v4-evidence-gap
tools:    diagnosis-evidence-acquisition-tools-v2
evidence: diagnosis-evidence-gap-v2
governance: diagnosis-governance-v3
decision: diagnosis-decision-policy-v1
```

The Go control plane accepts only repository-known immutable configuration IDs and
binds each ID to its expected decision-policy revision. The v4 manifest is also
repository-known and binds to the same `diagnosis-decision-policy-v1` revision:

```text
diag-config-4a517fea19cb6c49
governance: diagnosis-governance-v4-claim-surface
```

It remains a qualified Challenger, ready for shadow under
`diagnosis_promotion_v2`; qualification does not change the live v3 default.
ADR0010 forbids silently treating qualification as promotion.

### Diagnosis v4 and v5 claim surface

The v4 post-agent governance projection scans fields that assert facts about the
current user. Candidate `name`, `typical_symptoms`, and `differential` are generic
education about a possible candidate, so red-flag terms in those fields do not by
themselves assert that the user has the red flag. Candidate `basis`, `impact`, and
`reasoning_summary`, the overall summary, and unknown fields remain current-claim
surface and are scanned fail-closed. The deterministic pre-agent BodyState gate
and the high-recall literal `RedFlagDetector` revision v1 are unchanged for v3
and v4. The v3 broad projection is retained exactly for the current Champion,
historical qualification, and replay.

Diagnosis v5 keeps this exact v4 projection and changes only the explicit,
versioned red-flag interpretation: detector revision v2 suppresses a keyword
only when a narrow local clause explicitly asserts absence (`没有`, `无`, `未见`,
`否认`, `不伴`, and equivalent supported cues). Mixed clauses and current
positive assertions remain red flags; phrases such as `无法缓解` remain
positive. The legacy literal revision remains the default for every existing
caller and for v3/v4, so v4 was not mutated.

The v5 immutable manifest is `diag-config-375187050b203078` with governance
revision `diagnosis-governance-v5-negation-aware-claims`. It remains a known
qualified successor Challenger only; v3 remains Champion/default and v4 remains
resolvable historical/qualified evidence.

Diagnosis v6 keeps the v5 claim projection and changes only the explicit-negation
bridge detector to `red-flag-detector-negation-bridge-v3`, under governance
revision `diagnosis-governance-v6-negation-bridge-claims`. Diagnosis v7 keeps
that detector and the v4-v6 current-claim projection, but excludes only
top-level `information_gaps` because the field is unresolved evidence-gap
metadata rather than an asserted current-user fact. Candidate `basis`,
`impact`, `reasoning_summary`, the overall summary, unknown fields, and nested
candidate fields remain scan-visible; v3-v6 projections are unchanged.

The v7 immutable manifest is `diag-config-0206f70742d8a7a1` with governance
revision `diagnosis-governance-v7-information-gap-surface`. It is a qualified
successor Challenger only; v3 remains Champion/default and no automatic
promotion is implied.

## Qualification evidence

The v4 Agent configuration passes 7/7 on the same Diagnosis qualification dataset
and is paired non-inferior to v3 with pass-rate delta `+0.000` and zero critical
regressions. It is qualified and ready for shadow under `diagnosis_promotion_v2`,
but remains distinct from the current v3 Champion.

The Go policy has a versioned fixture suite covering normal, degraded, insufficient-information, critical-gap, new-red-flag, active-safety, Python-rejection, unknown-governance, and malformed-safety states. The fixture explicitly proves that high candidate confidence cannot override hard blockers.
