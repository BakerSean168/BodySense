# BS-UPG-110 Diagnosis information-gap claim-surface v7

Status: Complete. Diagnosis v7 is a qualified Challenger only; no deployment,
canary, or promotion occurred.

## Scope

Add a Diagnosis v7 successor from the exact v6 behavior. The only runtime
behavior change is post-agent claim projection: top-level `information_gaps` is
treated as unresolved evidence-gap metadata and excluded from current-user
red-flag scanning. The negation detector and all v3-v6 behavior remain frozen.

## Guardrails

- v3-v6 manifests, IDs, projections, detector mappings, and governance reports
  remain unchanged.
- v7 reuses v6 model, prompt, schema, tool, evidence, decision, and generation
  settings; only governance revision changes.
- Current-claim fields and unknown fields remain fail-closed scan-visible.
- Go registers v7 and `diagnosis_promotion_v5` without changing the v3
  Champion or compose defaults.

## Evidence

- Dedicated information-gap boundary qualification: 9/9, identity-bound to
  `diag-config-0206f70742d8a7a1`.
- General Diagnosis qualification: 7/7, critical safety 4/4, non-inferior to
  v6 with zero critical regressions.
- Promotion readiness: ready for shadow with no reasons; rollout gates remain
  20 shadow samples, 500/2500/5000 canary bps, and 10000 promotion bps.
