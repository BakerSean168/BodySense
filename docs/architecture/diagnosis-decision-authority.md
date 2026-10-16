# Diagnosis Decision Authority and SafetyEnvelope

> **Implementation boundary (2026-10-16, PR6 through #297):** BodyState persistence, evidence reads and Safety use native V2 records. DiagnosisAnalysis candidates, Treatment and legacy Training remain transitional implementations; this document does not claim the complete V2 product contract. Product rules remain in [docs/product](../product/README.md).

> Status: Current authority boundary with historical Phase-6 lineage retained. Diagnosis v10 (`diag-config-3f64de162dc937ee`) is the serving Champion; v3 is the retained historical rollback/replay target.

## Authority boundary

Python produces typed reasoning, candidate content, evidence-acquisition traces, and runtime governance evidence. Go is the only component allowed to decide whether that output may become an ordinary durable Diagnosis result.

The current serving control path is:

```text
One owner/revision-pinned native BodyState read
  -> Go projects native restrictions, resolutions and symptom-screen coverage
  -> frozen SafetyEnvelopeV2 + exact Agent input/replay
  -> Python structured reasoning and runtime governance evidence
  -> Go EvaluateDiagnosisDecisionV2 / ApplyDiagnosisDecision
  -> immutable DiagnosisAnalysis + decision_authority / evidence audit
```

Candidate confidence, model prose, legacy `red_flags`, and Judge scores cannot override a deny. Python findings are typed evidence for deterministic Go validation, never authority to write, downgrade or clear durable Safety restrictions.

## Current deny-overrides order

`diagnosis-decision-policy-v2-structured-safety` is a pure Go decision function:

1. unsupported/missing envelope or policy, owner-read failure, mismatched revision, unknown concepts or inconsistent coverage/review flags fail closed;
2. an active native `urgent` restriction blocks ordinary analysis; `restricted`/`advisory` do not by themselves terminate Diagnosis;
3. incomplete exact-symptom capture coverage abstains;
4. malformed/rejected governance or invalid typed output blocks;
5. a validated structured current runtime safety finding escalates; malformed findings block;
6. unresolved critical EvidenceGap or `insufficient_information` abstains;
7. partial status or degraded governance allows degraded delivery;
8. completed + accepted + candidates allows normal delivery; all other combinations block.

`block`, `escalate`, and `abstain` remove ordinary candidates before persistence. Block/escalate persist a `safety_blocked` analysis; abstention persists `insufficient_information`. They do not create an independent BodyState fact or mutate Safety restrictions from AI prose.

## Native SafetyEnvelope inputs

- asserted positive native symptom Facts and exact Go-bound `safety_screens`;
- immutable `safety_restrictions` and exact-target `safety_resolutions`, including current source availability;
- stable owner/revision identity, structured source references and explicit coverage;
- typed Diagnosis output, governance and evidence-acquisition outcomes.

There is no durable Safety JSON, global-clear timestamp/endpoint, global evidence pool or AI-write authority. The complete five-concept positive screen maps to restricted/all-training/professional-clearance; that mapping is not a clinically validated urgent classifier. A clearance is bound to one restriction and the original user message. Missing/invalidated evidence cannot silently lift a restriction, and resolving one leaves overlapping restrictions effective. Full Trainer execution gates and clinical-document clearance admission remain separate unfinished verticals.

Historical Phase-6 v1 used broad safety/red-flag inputs. Its repository-known immutable configurations and persisted replay envelopes remain historical audit evidence; they do not describe current serving authorization or restore removed persistence/API paths.

## Configuration boundary

The current live path uses the v10 immutable Champion configuration:

```text
diag-config-3f64de162dc937ee
prompt:   diagnosis-prompt-v6-structured-safety-context
tools:    diagnosis-evidence-acquisition-tools-v2
evidence: diagnosis-evidence-gap-v2
governance: diagnosis-governance-v8-structured-safety
decision: diagnosis-decision-policy-v2-structured-safety
max_tokens: 960
```

Historical v3 (`diag-config-5a4a13627e14b4cf`) remains repository-known with decision policy v1 and is retained as the rollback/replay target. Its manifest and persisted artifacts are not rewritten by the v10 promotion.

The Go control plane accepts only repository-known immutable configuration IDs and
binds each ID to its expected decision-policy revision. The v4 manifest is also
repository-known and binds to the same `diagnosis-decision-policy-v1` revision:

```text
diag-config-4a517fea19cb6c49
governance: diagnosis-governance-v4-claim-surface
```

It remains immutable historical qualification evidence under `diagnosis_promotion_v2`; it no longer defines the live default. ADR 0018 records the later v10 production promotion.

### Diagnosis v4 and v5 claim surface

The v4 post-agent governance projection scans fields that assert facts about the
current user. Candidate `name`, `typical_symptoms`, and `differential` are generic
education about a possible candidate, so red-flag terms in those fields do not by
themselves assert that the user has the red flag. Candidate `basis`, `impact`, and
`reasoning_summary`, the overall summary, and unknown fields remain current-claim
surface and are scanned fail-closed. The deterministic pre-agent BodyState gate
and the high-recall literal `RedFlagDetector` revision v1 are unchanged for v3
and v4. The v3 broad projection is retained exactly for historical qualification, replay, and emergency rollback.

Diagnosis v5 keeps this exact v4 projection and changes only the explicit,
versioned red-flag interpretation: detector revision v2 suppresses a keyword
only when a narrow local clause explicitly asserts absence (`没有`, `无`, `未见`,
`否认`, `不伴`, and equivalent supported cues). Mixed clauses and current
positive assertions remain red flags; phrases such as `无法缓解` remain
positive. The legacy literal revision remains the default for every existing
caller and for v3/v4, so v4 was not mutated.

The v5 immutable manifest is `diag-config-375187050b203078` with governance
revision `diagnosis-governance-v5-negation-aware-claims`. It remains a known
qualified historical successor only; v3 and v4 remain resolvable historical evidence while v10 is the current default.

## Qualification evidence

The v4 Agent configuration passes 7/7 on the same Diagnosis qualification dataset
and is paired non-inferior to v3 with pass-rate delta `+0.000` and zero critical
regressions. It is qualified and ready for shadow under `diagnosis_promotion_v2`,
and remains distinct from the current v10 Champion.

The Go policy has a versioned fixture suite covering normal, degraded, insufficient-information, critical-gap, new-red-flag, active-safety, Python-rejection, unknown-governance, and malformed-safety states. The fixture explicitly proves that high candidate confidence cannot override hard blockers.
