# ADR 0010: Promote Qualified Agent Baselines Before External Users

- Status: Accepted
- Date: 2026-09-01
- Owners: BodySense
- Related: ADR 0005, ADR 0009, Diagnosis/Treatment promotion governance

## Context

Diagnosis v3 (`diag-config-5a4a13627e14b4cf`) and Treatment v2
(`treat-config-f68eec9846664596`) have already passed their repository-versioned
qualification, non-inferiority, replay and deterministic policy suites. The
original rollout policies were intentionally conservative for a product with
meaningful external-user traffic: shadow and each canary step require 20 live
observations before progression.

BodySense is still a single-owner/pre-user product. Repeating one developer's
traffic until a counter reaches 20 does not provide the independent live-traffic
signal that the sample gate was designed to collect. Keeping the older v1
configurations as the repository defaults would also leave the codebase serving a
known older behavior purely because the production rollout ladder has no useful
population yet.

## Decision

Use a one-time **owner-approved pre-user baseline promotion** for the already
qualified pairs:

```text
Diagnosis v1 -> Diagnosis v3
Treatment v1 -> Treatment v2
```

This waiver does not claim that the live shadow/canary progression was completed.
The immutable `diagnosis_promotion_v1` and `treatment_promotion_v1` artifacts stay
unchanged as historical qualification evidence for those transitions.

After the migration:

```text
Diagnosis v3 = repository Champion/default serving baseline
Treatment v2 = repository Champion/default serving baseline

Diagnosis v1 = explicit rollback + historical replay target
Treatment v1 = explicit rollback + historical replay target

active Challenger = none
```

A future Diagnosis v4 or Treatment v3 must introduce a new immutable Challenger
and a new matching promotion record before `shadow`, `canary` or `promoted` may be
used. The ordinary multi-user rollout ladder remains the default governance once
BodySense has meaningful external traffic.

## Runtime contract

Champion, Challenger and rollback target are separate concepts:

- `*_CHAMPION_CONFIGURATION_ID` identifies the current stable serving baseline;
- `*_CHALLENGER_CONFIGURATION_ID` is optional and empty when there is no active
  rollout candidate;
- `*_ROLLBACK_CONFIGURATION_ID` identifies the previous stable version;
- `champion` serves Champion directly;
- `rollback` serves the explicit rollback target;
- `shadow/canary/promoted` require a distinct Challenger and an approved promotion
  record for that exact Champion -> Challenger pair.

The deprecated `TREATMENT_AGENT_CONFIGURATION_ID` alias is retired as serving
authority so stale environment state cannot silently downgrade the new baseline.

## Consequences

### Positive

- clean Dev/Staging/Production environments serve the newest qualified behavior;
- repository vocabulary matches reality: latest is Champion, not a permanently
  "promoted Challenger";
- rollback remains one configuration change and does not require schema rollback;
- historical qualification/replay evidence remains immutable and interpretable;
- future rollouts cannot accidentally reuse the old v1 -> latest promotion record.

### Trade-off

The promotion does not add live multi-user evidence. That is explicitly accepted
because no such population exists yet. Before BodySense gains external users, the
standard shadow/canary policy remains available for future Challengers; once
external traffic exists, this pre-user waiver must not be reused as a generic
promotion shortcut.

## Validation

The migration is complete only when:

1. clean deployment policy serves Diagnosis v3 and Treatment v2 in `champion`;
2. no active Challenger is synthesized by default;
3. explicit `rollback` serves Diagnosis v1 / Treatment v1;
4. historical v1 -> latest promotion-policy tests remain green;
5. future non-Champion rollout without a distinct approved pair fails closed;
6. longitudinal E2E persists latest Agent configuration IDs;
7. repository release verification passes;
8. Dev, Staging and Production are validated on the same merged SHA.
