# 05a/05b AI Output Governance Repair Report

## 1. What changed

| Gap | Fix |
|-----|-----|
| `FaithfulnessPolicy` 缺失 | Added `check_faithfulness()` adapter wrapping `FaithfulnessChecker` in `policies.py` |
| `GovernanceContext` typed dataclass 缺失 | Added `GovernanceContext` dataclass in `types.py` |
| `validate_treatment` 方法缺失 | Added `validate_treatment()` on `AIOutputGuard` running schema + safety + faithfulness |
| `OutputReviewService` 死代码 | Wired into `chat_handler.go` — records governance result from `stream.done` payload |
| `OutputReviewService` 未实例化 | Added to `main.go` DI chain |
| migration 缺 `user_id` | Added `000021` migration + model field |
| Python stream.done 缺 governance | Added governance check in `chat_service.py` (observe-only, non-blocking) |

## 2. Files changed

| File | Change |
|------|--------|
| `apps/ai-service/src/services/governance/types.py` | Added `GovernanceContext` dataclass |
| `apps/ai-service/src/services/governance/policies.py` | Added `check_faithfulness()` |
| `apps/ai-service/src/services/governance/output_guard.py` | Added `validate_treatment()` |
| `apps/ai-service/src/services/governance/__init__.py` | Export `GovernanceContext` |
| `apps/ai-service/src/services/chat_service.py` | Run governance on `__done__`, include in payload |
| `apps/api/migrations/000021_add_user_id_to_ai_output_reviews.up.sql` | New — add user_id |
| `apps/api/migrations/000021_add_user_id_to_ai_output_reviews.down.sql` | New — rollback |
| `apps/api/internal/model/ai_output_review.go` | Added `UserID` field |
| `apps/api/internal/service/output_review_service.go` | Added `userID` param to `RecordReview` |
| `apps/api/internal/handler/chat_handler.go` | Extract governance from stream.done, persist observe-only |
| `apps/api/cmd/server/main.go` | Wire `OutputReviewService` into DI |

## 3. Acceptance criteria result

| Criteria | Result | Evidence |
|----------|--------|----------|
| `AIOutputGuard` Module exists with typed structures | **PASS** | `types.py` — `GovernanceContext`, `GovernanceResult`, `GovernanceIssue`, `GovernanceStatus` all defined |
| Schema, safety, and faithfulness policy skeletons exist | **PASS** | `policies.py` — `check_schema_valid`, `check_red_flags`, `check_faithfulness`, `check_empty_output` |
| Unit tests cover accepted, degraded, rejected | **PASS** | `test_ai_output_guard.py` — 7 tests: accepted, degraded (short/empty), rejected (missing field), structured, no-required-fields, to_dict |
| Existing services not behaviorally changed | **PASS** | Governance runs in `try/except` block, non-blocking. Failure is silently logged. |
| No Go persistence blocking enforcement | **PASS** | `RecordReview` is fire-and-forget (logs errors, never fails caller) |
| `ai_output_reviews` has `user_id` | **PASS** | Migration 000021 + model `UserID` field |
| Governance wired into chat flow | **PASS** | Python includes in `stream.done` payload; Go extracts and persists |

## 4. Verification

| Command | Result | Notes |
|---------|--------|-------|
| `go vet ./...` | ✅ PASS | |
| `go build ./...` | ✅ PASS | |
| `go test ./... -count=1` | ✅ PASS (8 packages) | |
| `uv run ruff check` | ✅ PASS | |
| `uv run pytest tests/` | ✅ PASS (177/177) | |

## 5. Remaining risks

- Governance check only runs on `consultation_reply` (chat). Diagnosis/training/assessment paths are not covered yet (deferred per plan).
- `FaithfulnessChecker` uses substring matching — known false-positive risk for short Chinese terms.
- `RecordReview` has no dedup — multiple reviews per run are possible if stream.done is replayed.

## 6. Next recommended blocker

**04a/04b Job Runtime OCR idempotency** — idempotency TOCTOU race, missing `idempotency_key`/`attempts` fields.
