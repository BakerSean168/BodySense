# ADR 0007: Separate stable Profile from longitudinal Lifestyle and Body Metrics

- Status: Accepted
- Date: 2026-08-27
- Supersedes: the temporary profile health-context fields introduced during rapid UI iteration
- Related: ADR 0004 (Longitudinal BodyState)

## Context

BodySense originally accumulated identity, body measurements, lifestyle, injury history, and current health context in `user_profiles`. That is convenient for a form, but it creates the wrong ownership boundary:

- sleep, activity, exercise, weight, alcohol/caffeine use, and recovery patterns change over time;
- overwriting them destroys clinically useful temporal context;
- onboarding, manual profile editing, and later Consultation can otherwise create competing truths;
- Assessment can accidentally treat the static profile as a second health-state aggregate;
- adding one column per new lifestyle concept turns `user_profiles` into an unbounded health-record table.

The existing BodyState aggregate already owns durable user health state, provenance, revisions, validity intervals, observations, and supersession. A second Lifestyle aggregate would duplicate that ownership.

## Decision

### 1. `UserProfile` owns stable identity only

The persisted Profile contract is intentionally narrow:

```text
user_profiles
├─ id / user_id
├─ gender
├─ birth_date
└─ timestamps
```

`age_years` is derived at read time from `birth_date` and is never persisted.

Height, weight, lifestyle, exercise, sleep, injury history, occupation, fixed sleep clocks, free-form self description, and similar health context are not Profile fields.

### 2. Body measurements are BodyState Observations

Current height and weight use:

```text
anthropometry.height
anthropometry.weight
```

They are user-reported observations with `observed_at`. Replacing the current value closes the old current observation and creates a new observation linked by `supersedes_observation_id`. BMI is a derived projection of the current height and weight and is not persisted as identity state.

### 3. Lifestyle is a BodyState taxonomy plus a current projection

The canonical current lifestyle sections are:

```text
lifestyle.activity
lifestyle.sleep
lifestyle.exercise
lifestyle.nutrition
lifestyle.substances
lifestyle.recovery
```

These are ordinary BodyState Facts and therefore inherit origin, review state, validity, lifecycle, provenance, revision identity, and temporal history.

`GET /api/v1/lifestyle` returns a `LifestyleSnapshot` projection of active facts. It is not backed by a `lifestyles` table and is not a second aggregate.

### 4. Injury summary is BodyState health history

The onboarding/editor summary uses `history.injury_summary`. It is a current temporal summary, not a Profile column. More granular historical injury facts can later use their own health-history taxonomy without changing Profile ownership.

### 5. Real change and correction are different commands

When an old statement was wrong, callers use `CorrectFact`.

When an old statement used to be true and later changed, callers use a temporal transition:

```text
old fact
  lifecycle_state = inactive
  valid_until = T

new fact
  valid_from = T
  supersedes_fact_id = old.id
  lifecycle_state = active
```

BodyState Observations use the analogous `supersedes_observation_id` chain for current singleton measurements.

### 6. All capture channels converge before persistence

```text
Onboarding context --+
Lifestyle editor ----+----> BodyState application boundary ----> BodyState truth/history
Consultation tool ---+
Body metrics editor -+
Health-history UI ---+
```

The onboarding command coordinates stable Profile and the complete initial BodyState context through the existing database `TransactionManager`, so a failed BodyState write also rolls back the Profile write. All mutable onboarding health fields are committed as one `BodyStateCurrentContextPatch`, producing at most one semantic revision.

The Consultation Agent may emit `record_lifestyle_context` only for lifestyle information the user explicitly stated. It must not infer a lifestyle fact from symptoms, a job title, or external knowledge. Python never owns BodyState persistence.

Because the normalized summary is still model-mediated extraction, the Go runtime persists it as a durable **candidate**, not as current truth:

```text
origin = ai_extracted
review_state = unverified
excluded_from_reasoning = true
```

`LifestyleSnapshot.pending_updates` exposes those candidates to the user. Accepting a candidate is an explicit user-review command: the previous confirmed current fact is closed, the candidate is promoted to `confirmed + active + included`, and the supersession is committed in one BodyState revision. Rejecting a candidate keeps the durable audit record but marks it `rejected + excluded_from_reasoning`. Direct onboarding/editor input remains immediately confirmed because the user is editing the canonical structure directly.

### 7. Assessment receives stable Profile and BodyState separately

Assessment input is intentionally split:

```text
profile            = stable identity context
body_state         = current health truth
report_indicators  = current external-report input
posture_analysis   = current posture-analysis input
```

Mutable health information is never embedded into `profile` to make the prompt convenient.

### 8. Destructive development migration is intentional

BodySense is in rapid development and the temporary profile fields do not hold important production data that warrants a compatibility bridge. Migration 58 therefore drops the obsolete health columns instead of backfilling or dual-writing them.

No future code should reintroduce those columns merely for backward compatibility without a new explicit ADR.

## Consequences

### Positive

- one health source of truth: BodyState;
- onboarding and later conversation naturally converge;
- lifestyle changes can be correlated with symptoms without claiming causation;
- conversational memory is durable without allowing model extraction to silently rewrite long-term health truth;
- stable Profile stays small and durable;
- UI can still present a friendly persistent “生活方式” page without creating another persistence model;
- Assessment/Diagnosis/Treatment share the same current health state boundary.

### Costs

- onboarding has a dedicated coordinated application command spanning Profile and BodyState;
- optimistic BodyState revision conflicts must be handled explicitly;
- current-projection APIs need taxonomy discipline as new lifestyle concepts are added;
- historical UI is derived from BodyState revisions/supersession rather than a simple profile edit log.

## Non-goals

This ADR does not turn BodySense into a calorie tracker, habit-completion tracker, or wearable time-series platform. Nutrition and substance sections capture health-relevant context only. Fine-grained event streams can be introduced later as separate Observations/records when a real product use case requires them.
