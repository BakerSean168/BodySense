# Diagnosis Decision Authority and SafetyEnvelope

> Status: Historical Phase-6 implementation checkpoint; DecisionAuthority and rollout machinery are implemented. Diagnosis v4 is now the repository Champion; v3 remains historical/replay evidence with its original governance semantics.

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

The current live path uses the cumulative immutable configuration:

```text
diag-config-4a517fea19cb6c49
prompt:   diagnosis-prompt-v4-evidence-gap
tools:    diagnosis-evidence-acquisition-tools-v2
evidence: diagnosis-evidence-gap-v2
governance: diagnosis-governance-v4-claim-surface
decision: diagnosis-decision-policy-v1
```

The Go control plane accepts only repository-known immutable configuration IDs and
binds each ID to its expected decision-policy revision. The v3 manifest remains in
the offline eval archive with `diagnosis-governance-v3` so historical qualification
and replay retain their original semantics; it is not a live serving target.

### Diagnosis v4 claim surface

The v4 post-agent governance projection scans fields that assert facts about the
current user. Candidate `name`, `typical_symptoms`, and `differential` are generic
education about a possible candidate, so red-flag terms in those fields do not by
themselves assert that the user has the red flag. Candidate `basis`, `impact`, and
`reasoning_summary`, the overall summary, and unknown fields remain current-claim
surface and are scanned fail-closed. The deterministic pre-agent BodyState gate
and the high-recall `RedFlagDetector` are unchanged. The v3 broad projection is
retained exactly for historical qualification and replay.

## Qualification evidence

The v4 Agent configuration passes 7/7 on the same Diagnosis qualification dataset
and is paired non-inferior to v3 with pass-rate delta `+0.000` and zero critical
regressions. The v3 qualification evidence remains immutable.

The Go policy has a versioned fixture suite covering normal, degraded, insufficient-information, critical-gap, new-red-flag, active-safety, Python-rejection, unknown-governance, and malformed-safety states. The fixture explicitly proves that high candidate confidence cannot override hard blockers.
