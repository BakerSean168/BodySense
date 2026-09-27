# BS-UPG-110 Diagnosis explicit-negation bridge successor

Status: Complete. Diagnosis v6 is a qualified Challenger only; no deployment,
canary, or promotion occurred.

## Outcome

The v5 blocker was post-agent governance using the v2 detector, whose explicit
absence rule required the cue to be immediately adjacent to the red-flag
keyword. v6 adds detector revision
`red-flag-detector-negation-bridge-v3` with a finite bridge allowlist and binds
it to governance revision `diagnosis-governance-v6-negation-bridge-claims`.

The new manifest generated configuration ID
`diag-config-4377355ba2012ce8`. It preserves v5 prompt, schema, tool, evidence,
decision policy, model, and generation settings. v1/v2 detector behavior and
v3/v4/v5 configuration identities remain unchanged and replayable.

## Safety and rollout invariants

- The bridge allowlist is limited to `明显`, `明显的`, `出现`, `存在`, `任何`,
  and `再出现`; arbitrary text is never accepted between cue and keyword.
- Positive, mixed, historical-current-positive, ambiguous, exclusion/double-
  negation, worsening, and cross-source cases remain red flags as required.
- v6 uses the same current-user claim projection as v4/v5, so generic
  candidate education exclusions remain unchanged.
- Go registers v6 and exact promotion record
  `diagnosis_promotion_v4` for Champion v3
  (`diag-config-5a4a13627e14b4cf`) -> Challenger v6
  (`diag-config-4377355ba2012ce8`). Unknown revisions and non-exact pairs fail
  closed; evidence trace validation applies to v6.
- Default Champion remains v3. Compose defaults and deployment configuration
  were not changed, and no rollout was started or manually promoted.
- `unsafe_relaxations=0`; existing canary steps remain `[500, 2500, 5000]`.

## Qualification evidence

- Negation bridge policy: 14/14 passed.
- General Diagnosis qualification: 7/7 passed; critical safety 4/4;
  non-inferior and promotion-eligible versus v5 with zero critical
  regressions.
- `diagnosis_promotion_v4`: qualification chain v3 -> v4 -> v5 -> v6;
  `ready_for_shadow=true`.
- Full AI suite: 557 passed; Ruff and Pyright passed with OCR extras.
- Go full suite and vet passed; `git diff --check` passed.
