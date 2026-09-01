# ADR 0011: Adopt explicit OCR report-indicator evidence admissibility

- Status: Accepted
- Date: 2026-09-01
- Scope: OCR mechanism output, report-indicator evidence governance, Assessment configuration/replay
- Related: ADR 0009, Agent Platform Role Governance, Assessment Feature Line 1

## Context

ADR 0009 made Assessment evidence-grounded, but one upstream evidence class still had a weaker admission rule than BodyState.

BodyState already excludes an item from reasoning when it is unverified, inactive, rejected, or `excluded_from_reasoning`. Report indicators followed a different path:

```text
OCR job status = completed
  -> OCRResult.indicators[]
  -> Assessment report_indicators
  -> report evidence catalog
```

`completed` only means the OCR job finished. It does not prove that every regex match is accurate enough to support a durable health observation.

The pre-existing OCR confidence was also not a safe evidence contract:

- indicator confidence was a heuristic derived from unit/reference-range context;
- OCR engine confidence and parser confidence were separate signals;
- `HealthIndicator.confidence` defaulted to `high`, so missing confidence could fail open;
- no versioned policy converted confidence into evidence admissibility;
- Python and Go Assessment catalogs accepted every completed report indicator.

This created an inconsistent trust boundary: an unverified BodyState candidate could not bootstrap another health observation, while a low-confidence OCR match could.

## Decision

### 1. OCR completion and evidence admissibility are separate states

The OCR mechanism continues to persist the extraction result even when an indicator is not safe for automatic health reasoning.

```text
extraction lifecycle
pending -> processing -> completed | failed

indicator evidence lifecycle
extracted -> admissible | needs_review | rejected
```

`ocr_status=completed` must never be interpreted as `evidence_admissible=true`.

### 2. Missing confidence fails closed

`HealthIndicator.confidence` and `OCRResult.confidence` use the explicit vocabulary:

```text
high | medium | low | unknown
```

The default is `unknown`, not `high`.

An absent confidence value therefore cannot silently gain stronger evidence authority than an explicitly measured value.

### 3. A versioned deterministic admissibility policy owns auto-admission

The first policy revision is:

```text
ocr-indicator-admissibility-v1
```

Its current automatic rule is deliberately conservative:

```text
OCR confidence == high
AND indicator confidence == high
AND indicator has non-empty name/value
  -> admissible

medium / low / unknown on either confidence signal
  -> needs_review

malformed indicator
  -> rejected
```

This is an **evidence admission rule**, not a clinical interpretation. `admissible` means the extracted value may enter the Assessment evidence catalog; it does not mean the value is medically normal, abnormal, diagnosed, or user-confirmed.

A future product may add explicit user/reviewer confirmation or promote reviewed report values into a normalized durable report-fact model. That would be a new authority layer, not a reinterpretation of this policy.

### 4. The evidence decision is persisted with the indicator

Each current OCR indicator carries:

```json
{
  "confidence": "high",
  "evidence_admissibility": {
    "status": "admissible",
    "policy_revision": "ocr-indicator-admissibility-v1",
    "reason_codes": ["high_confidence_ocr_and_indicator"]
  }
}
```

Review-required indicators remain available in `user_uploads.ocr_result` for UI/operator review and future reprocessing. They are not deleted merely because Assessment cannot use them.

### 5. Assessment evidence policy advances to v3

Changing which upstream evidence may enter the catalog is behavior-significant. The existing Assessment v3 identity therefore remains immutable.

The new serving configuration is:

```text
assessment-v4
configuration id: assess-config-e579030c2b8b540c
output schema: assessment-output-v2
evidence policy: assessment-evidence-contract-v3
```

`assessment-evidence-contract-v3` requires report evidence to contain the exact supported admissibility provenance:

```text
evidence_admissibility.status == admissible
AND
evidence_admissibility.policy_revision == ocr-indicator-admissibility-v1
```

Missing metadata, `needs_review`, `rejected`, unknown status, or a forged/unknown policy revision all fail closed and the indicator is absent from the current catalog.

Assessment v3 (`assess-config-c6cfff22aa362fff`) remains repository-known with `assessment-evidence-contract-v2` for historical replay/counterfactual comparison only. It cannot serve new durable reports.

### 6. Python and Go independently enforce the same admission rule

Python builds the generation-time catalog using the selected immutable Assessment configuration's evidence-policy revision.

Go reconstructs the catalog from the frozen request before persistence and independently checks the same `status + policy_revision` pair. It does not trust Python merely because Python returned a structurally valid observation.

The durable invariant becomes:

```text
completed OCR
  != admissible report evidence

report_indicator Assessment observation
  => exact report ref exists
  => exact OCR admissibility policy is known
  => status == admissible
  => Python gate passes
  => Go gate passes
```

### 7. Replay preserves historical semantics

Replay selects evidence behavior from the target Assessment configuration registration:

```text
v1/v2 -> legacy output-v1 behavior
v3    -> assessment-evidence-contract-v2
v4    -> assessment-evidence-contract-v3
```

Therefore a historical v3 replay can still reconstruct a report whose old OCR payload lacked admissibility metadata. A v4 counterfactual replay against the same frozen input excludes that indicator, making the policy change visible rather than silently rewriting history.

### 8. No relational migration is required

`user_uploads.ocr_result` is JSONB and the new indicator fields are additive. No schema migration or backfill invents confidence/admissibility for old OCR rows.

Existing completed OCR rows that lack `evidence_admissibility` are intentionally unavailable to Assessment v4 until they are reprocessed by the current OCR mechanism or later pass an explicit review workflow. This is a fail-closed compatibility choice.

## Qualification

The current deterministic Assessment evidence qualification is `assessment-evidence-contract-v3` and includes nine cases. It adds an explicit `review-required-report-is-unavailable` case to the prior source/kind/ref/coverage matrix.

Additional regression coverage proves:

- missing indicator confidence defaults to `unknown`;
- only high/high extraction is auto-admissible;
- medium OCR or medium indicator confidence produces `needs_review`;
- the real `/api/ocr/extract` response attaches admissibility metadata;
- legacy Assessment evidence-v2 keeps old report indicators replayable;
- current evidence-v3 excludes missing/review-required/forged admissibility metadata;
- admissible report indicators are selectable in both Python and Go;
- Go preserves the nested metadata through `UserUpload.OCRResult -> assessmentInputsFromUploads -> durable evidence catalog`.

## Consequences

### Positive

- an OCR false positive no longer gains health-evidence authority merely because the job completed;
- missing confidence can no longer fail open as `high`;
- review-required data remains visible without influencing durable health reasoning;
- Python, Go and replay all use an explicit versioned evidence-admission boundary;
- the immediate previous Assessment contract remains historically reproducible.

### Trade-offs

- some legitimate medium-confidence report values will not participate in Assessment until reprocessed or reviewed;
- existing legacy OCR rows without the new metadata stop contributing to new v4 reports;
- the high/high heuristic is still not a substitute for full OCR mechanism provenance or human confirmation.

The remaining OCR mechanism-provenance work (engine/parser/rendering/extractor revisions) is tracked separately in the active documentation/code alignment audit and is not claimed solved by this ADR.

## Rejected alternatives

### Treat every completed OCR result as evidence

Rejected because job completion is an execution state, not a correctness/admissibility state.

### Filter only `confidence == low`

Rejected because missing/unknown values would still fail open, and OCR confidence plus indicator confidence represent different uncertainty sources.

### Change Assessment v3 in place

Rejected because evidence catalog membership is behavior-significant and historical replay must retain the old v3 semantics.

### Drop review-required indicators from storage

Rejected because extraction and downstream evidence authority are separate concerns. Keeping the candidate enables UI review, debugging and future reprocessing without letting it influence health truth.
