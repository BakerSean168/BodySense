# Phase 05a: AIOutputGuard Skeleton

## Goal

Introduce a Python `AIOutputGuard` Module that centralizes schema, safety, faithfulness, and business-rule validation for AI outputs, without changing Go persistence yet.

## Why

AI health outputs need consistent governance before they become diagnosis, treatment, assessment, training, or knowledge content. Current validation exists but is scattered inside individual services. This ticket creates a deep Module interface for reuse.

## Current State

- `apps/ai-service/src/services/diagnosis_service.py` validates diagnosis and treatment JSON with Pydantic models.
- `apps/ai-service/src/services/red_flag_detector.py` detects red flag symptoms.
- `apps/ai-service/src/services/faithfulness_checker.py` checks treatment exercises against RAG results.
- `apps/ai-service/src/services/assessment_service.py` has service-specific validation.
- No unified `AIOutputGuard` or governance result type exists.
- No `ai_output_reviews` table exists yet.

## Scope

### Allowed

- Add `services/governance/` package with guard types and policy skeletons.
- Define `GovernanceResult` with statuses `accepted`, `repaired`, `degraded`, `rejected`.
- Wrap existing red flag and faithfulness logic as policies where practical.
- Add schema policy hooks for Pydantic model validation.
- Add unit tests for accepted, rejected, degraded paths.
- Optionally integrate one low-risk path in Python only if behavior is exactly preserved; prefer skeleton and tests first.

### Not Allowed

- Do not add Go `ai_output_reviews` persistence.
- Do not change Go assessment/training/diagnosis persistence behavior.
- Do not add retry loops or automatic repair with extra LLM calls.
- Do not change prompts.
- Do not block existing user-visible outputs unless integration is explicitly feature-flagged.

## Target Files

- `apps/ai-service/src/services/governance/__init__.py` (new)
- `apps/ai-service/src/services/governance/output_guard.py` (new)
- `apps/ai-service/src/services/governance/policies.py` (new, likely)
- `apps/ai-service/src/services/governance/types.py` (new, likely)
- `apps/ai-service/tests/unit/test_ai_output_guard.py` (new, likely)
- `apps/ai-service/src/services/diagnosis_service.py` (likely, only for optional adapter usage)

## Design Notes

Suggested interface:

```python
class AIOutputGuard:
    async def validate(
        self,
        output_type: str,
        raw_output: Any,
        context: GovernanceContext,
    ) -> GovernanceResult:
        ...
```

Suggested result:

```python
class GovernanceResult(BaseModel):
    status: Literal["accepted", "repaired", "degraded", "rejected"]
    output_type: str
    validated_output: Any | None = None
    issues: list[GovernanceIssue] = []
    metadata: dict[str, Any] = {}
```

Policy categories:

- `schema`
- `safety`
- `faithfulness`
- `business_rule`

Do not make the guard depend on FastAPI or Go DTOs. It should be pure Python domain logic.

## Implementation Steps

1. Create `services/governance/`.
2. Define `GovernanceContext`, `GovernanceIssue`, and `GovernanceResult`.
3. Implement a basic policy interface.
4. Implement `SchemaPolicy` that can validate via a supplied Pydantic model.
5. Implement `SafetyPolicy` adapter around `RedFlagDetector`.
6. Implement `FaithfulnessPolicy` adapter around `FaithfulnessChecker`.
7. Implement `AIOutputGuard.validate(...)` to run configured policies and choose final status.
8. Add unit tests using small fake policies and existing red flag/faithfulness fixtures.
9. Document how existing services should later call the guard before returning AI output.

## Invariants

- Existing diagnosis/treatment/assessment/training behavior remains unchanged unless a feature-flagged adapter is explicitly tested.
- Existing Pydantic schemas remain the schema truth for their output types.
- Guard result is structured and serializable.
- No database writes occur from Python governance code.

## Verification Commands

```bash
pnpm nx run ai-service:lint
pnpm nx run ai-service:typecheck
pnpm nx run ai-service:test
```

Fallback:

```bash
cd apps/ai-service
uv run ruff check .
uv run pyright src
uv run pytest tests/unit/test_ai_output_guard.py tests/unit/test_diagnosis_service.py tests/unit/test_faithfulness_checker.py tests/unit/test_red_flag_detector.py
```

## Acceptance Criteria

- [ ] `AIOutputGuard` Module exists with typed context/result/issue structures.
- [ ] Schema, safety, and faithfulness policy skeletons exist.
- [ ] Unit tests cover accepted, degraded, and rejected outcomes.
- [ ] Existing services are not behaviorally changed unless explicitly feature-flagged.
- [ ] No Go persistence or migrations are included.

## Regression Risks

- Policy ordering may produce surprising status decisions; keep status precedence documented.
- Pydantic model typing may be too generic for pyright; use explicit protocols.
- Existing detector/checker limitations still apply and should be surfaced as metadata or issues.

## Out of Scope Follow-ups

- `ai_output_reviews` table.
- LLM repair/retry.
- Golden bad-case suite expansion.
- Enforcing governance before Go business persistence.

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

