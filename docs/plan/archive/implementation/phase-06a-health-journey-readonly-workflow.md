# Phase 06a: HealthJourney Readonly Workflow

## Goal

Add a read-only Go `HealthJourneyWorkflow` Module that derives a user's current health journey stage and available actions from existing tables.

## Why

The architecture plan calls for a formal health journey state machine. The first implementation should not create a new truth source immediately; it should derive state from existing profile, uploads, assessment, consultation, and training data, making frontend and Agent behavior less scattered.

## Current State

- Existing tables include `user_profiles`, `user_uploads`, `assessment_reports`, `consultation_sessions`, `training_plans`, and `training_logs`.
- There is no `health_journeys` table.
- Frontend pages infer next actions locally.
- Go services operate by feature area rather than a unified journey workflow.
- ContextBuilder from Phase 01a can later inject derived journey state.

## Scope

### Allowed

- Add a Go `HealthJourneyWorkflow` Module that computes journey state from existing repositories.
- Add a read endpoint for current journey state and available actions.
- Add DTOs for stage, stage reasons, and available actions.
- Add tests with fake repositories or integration-style service tests.
- Optionally inject read-only journey state into ContextBuilder if Phase 01a is complete and the change is small.

### Not Allowed

- Do not add `health_journeys` or `health_journey_events` tables.
- Do not mutate business state from this workflow.
- Do not redesign Dashboard or onboarding UI.
- Do not migrate training or consultation logic.
- Do not introduce AI-generated journey state.

## Target Files

- `apps/api/internal/workflow/health_journey.go` (new, likely)
- `apps/api/internal/workflow/health_journey_test.go` (new, likely)
- `apps/api/internal/handler/health_journey_handler.go` (new, likely)
- `apps/api/internal/dto/health_journey.go` (new, likely)
- `apps/api/internal/repository/*` (likely, only if missing list helpers are needed)
- `apps/api/cmd/server/main.go` or route wiring file (likely)
- `apps/api/internal/context/context_builder.go` (likely, optional)

## Design Notes

Suggested stages:

```txt
profile_incomplete
profile_ready
assets_uploaded
assessment_ready
consulting
diagnosis_ready
plan_ready
training_active
reassessment_due
completed
```

Suggested interface:

```go
type HealthJourneyWorkflow interface {
    GetState(ctx context.Context, userID uuid.UUID) (*JourneyState, error)
}
```

`JourneyState` should include:

- `stage`
- `available_actions`
- `missing_requirements`
- `active_consultation_id`
- `latest_assessment_id`
- `active_training_plan_id`
- `derived_from` metadata for debugging.

Keep stage derivation deterministic and documented.

## Implementation Steps

1. Create `internal/workflow/` if it does not exist.
2. Define stage and action constants.
3. Implement repository reads needed for profile existence/completeness, uploads, latest assessment, active consultation, and active training plan.
4. Implement deterministic stage derivation in priority order.
5. Add a read-only handler endpoint, likely `GET /api/v1/health-journey`.
6. Add tests for at least:
   - no profile -> `profile_incomplete`
   - profile only -> `profile_ready`
   - completed assessment -> `assessment_ready`
   - active training plan -> `training_active`
7. If ContextBuilder injection is included, add `journey_state` into the Python request metadata/context without changing Python behavior.

## Invariants

- No existing table is modified.
- Existing feature pages and routes continue to work.
- Derived journey state is read-only and can be recomputed.
- Agent context injection, if added, is additive and optional.

## Verification Commands

```bash
pnpm nx run api:lint
pnpm nx run api:test
```

Fallback:

```bash
cd apps/api
go vet ./...
go test ./...
```

## Acceptance Criteria

- [ ] `HealthJourneyWorkflow` returns deterministic read-only state.
- [ ] A read endpoint exposes state and available actions.
- [ ] Tests cover core stage derivation.
- [ ] No new health journey persistence tables are added.
- [ ] Existing profile/upload/assessment/consultation/training behavior is unchanged.

## Regression Risks

- Stage priority can be wrong if multiple artifacts exist; document precedence.
- Repository methods may need indexes later for efficient latest-record lookup.
- Frontend may start depending on derived action names; keep names stable once exposed.
- ContextBuilder injection could increase prompt size if not kept compact.

## Out of Scope Follow-ups

- Persistent `health_journeys`.
- Journey events.
- Dashboard redesign.
- AI-generated journey summaries.

## Final Response Format for Coding Agent

```md
Changed files:
- ...

Behavior changes:
- ...

Tests run:
- ...

Tests passed / failed:
- ...

Known risks:
- ...

Follow-up tasks:
- ...
```

