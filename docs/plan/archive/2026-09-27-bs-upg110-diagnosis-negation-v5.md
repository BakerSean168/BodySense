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
