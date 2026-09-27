# BS-UPG-110 Diagnosis explicit-negation successor

Status: Complete. Implementation, qualification, and readiness verification passed.

## Objective

Add an immutable Diagnosis v5 successor that preserves v4's current-user claim
surface while opting into an explicit-negation-aware red-flag detector. Keep v3
as Champion/default, preserve v4 and `diagnosis_promotion_v2` unchanged, and
produce real v5 qualification and `diagnosis_promotion_v3` readiness evidence.

## Guardrails and invariants

- Python manifests remain the source of truth for immutable Diagnosis behavior
  and generated configuration IDs.
- Literal detector revision v1 remains the default and is used by v3/v4;
  explicit-negation revision v2 is used only by v5.
- v5 keeps v4's current-user claim projection and generic candidate education
  exclusions; unknown fields remain scan-visible.
- Positive current red flags still bypass the typed agent and fail closed.
- Go keeps v3 Champion/default, v4 known/resolvable, and v5 known as a
  Challenger requiring an exact promotion record and evidence-gap-v2 trace.
- No compose defaults, staging, production, or deployment state changed.

## Independent-review repair

The independent review identified three acceptance blockers, all repaired and
verified in the second repair commit:

- v2 now treats `不否认头晕`, `未否认头晕`, and `不能说没有头晕` as
  conservative positive signals while preserving the exact blocker suppression,
  literal v1 behavior, mixed-clause behavior, historical/current behavior, and
  worsening behavior.
- The dedicated deterministic report
  `data/evals/reports/diagnosis_negation_policy_v2.json` is identity-bound to
  v5, governance revision `diagnosis-governance-v5-negation-aware-claims`, and
  detector revision `red-flag-detector-negation-aware-v2`; it is required at
  100% by `diagnosis_promotion_v3` alongside EvidenceGap 5/5.
- Go rollout admission now requires an exact repository-known promotion
  record/pair for every non-Champion Diagnosis stage. v3 -> v4 accepts only
  `diagnosis_promotion_v2`; v3 -> v5 accepts only `diagnosis_promotion_v3`.

## Final repair evidence

- Negation policy qualification: 10/10 passed, including the independent-source boundary regression.
- v5 general qualification regenerated: 7/7 passed; non-inferior and
  promotion-eligible versus v4.
- Promotion readiness regenerated: `ready_for_shadow=true`, with EvidenceGap
  5/5 and negation policy 10/10 required reports green.
- AI service: 539 tests passed; Ruff passed on touched Python.
- Go: `go vet ./...` and `go test ./...` passed, including exact promotion
  admission tests; `gofmt` and `git diff --check` passed.
- v5 configuration ID remains `diag-config-375187050b203078`; v3 remains the
  default Champion and no deployment occurred.

## Completed verification

- v5 manifest fingerprint: `375187050b20307849aedd3d470c7c09fcf5b5fed9cca80291da88254d9126af`.
- v5 configuration ID: `diag-config-375187050b203078`.
- Deterministic qualification: 7/7, all required splits, critical-safety 4/4.
- v5 vs v4: non-inferior, promotion-eligible, zero critical regressions.
- `diagnosis_promotion_v3`: runtime pair v3 Champion -> v5 Challenger;
  qualification chain v3 -> v4 -> v5; readiness `ready_for_shadow=true`.
- Full AI-service tests: 533 passed; Ruff passed on touched Python.
- Go vet/full tests and focused post-edit Go tests passed.
- `git diff --check` passed.
