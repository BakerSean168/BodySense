# Diagnosis v4 claim-surface governance repair

## Objective

Introduce a new immutable Diagnosis governance/configuration revision that keeps
current-user claim scanning strict while treating candidate education fields as
non-user-claim text. Preserve the exact Diagnosis v3 behavior for historical
qualification and replay.

## Guardrails

- System of record: Python owns immutable Agent manifests and governance
  projection; Go owns the mutable serving pointer, DecisionAuthority v1, and
  evidence-availability admission.
- Trust boundaries: model output remains untrusted until Pydantic validation,
  the Python governance gate, and the Go deterministic authority policy.
- Provenance: configuration IDs are content fingerprints; historical reports
  and v3 manifests are not rewritten. New promotion evidence is generated only
  by the existing qualification/readiness scripts.
- Safety: the BodyState pre-agent red-flag gate and high-recall
  `RedFlagDetector` remain unchanged; only the v4 post-agent Diagnosis claim
  projection changes.
- Verification: regression tests cover generic education acceptance and
  user-specific claim rejection, v3/v4 policy behavior, immutable resolution,
  qualification, evidence contracts, Go authority registration, and replay.

## Vertical slices

1. Add failing Python governance tests for v4 generic-vs-current claim fields,
   including the explicit `differential` classification.
2. Implement the v4 claim-surface projection while retaining the v3 serializer
   unchanged.
3. Add the v4 immutable manifest, resolver/default tests, and calculate its
   content-derived configuration ID.
4. Add the v4 successor qualification/promotion evidence through the existing
   scripts; do not hand-edit generated reports.
5. Register the new authority ID in Go, update Diagnosis evidence contracts,
   and preserve v3 replay/config compatibility.
6. Run focused Python/Go checks, repository-required affected checks, then
   commit, push, and open a non-merged PR if all gates pass.

## Closure evidence

Status: complete. The v4 successor is `diag-config-4a517fea19cb6c49` with
`diagnosis-governance-v4-claim-surface`; v3 remains
`diag-config-5a4a13627e14b4cf` with `diagnosis-governance-v3` in the offline
archive and replay path.

- Canonical Diagnosis qualification: v4 7/7, non-inferior to v3, zero critical
  regressions.
- Canonical promotion readiness: ready for shadow; required EvidenceGap policy
  suite 5/5.
- Python focused suite: 49 passed; full Python suite with OCR extra: 517 passed.
- Ruff, focused Go service tests, `go vet ./...`, and `go test ./...`: passed.
- Focused web contract test: 25 passed; development, staging, and production
  Compose configuration checks passed with disposable placeholders.
- Commit: `edafe0fa703ceb91ae48db7fea4f972a98bd1ac8`.
- Pull request: https://github.com/BakerSean168/BodySense/pull/202 (open,
  targeting `main`, not merged).
- No staging or production deployment was performed.
