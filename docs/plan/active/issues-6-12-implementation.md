# Issues 6-12 Implementation Plan

## Status: ✅ COMPLETE

All issues (6-12) have been implemented and merged to dev.

## Overview

Complete implementation of issues #6 through #12 for the BodySense project. Each issue follows the feature branch workflow: `feature/{issue-number}-{short-name}` → merge to `dev`.

## Dependency Chain

```
Issue 6 (Consultation Chat) ──► Issue 8 (Info Panel + Body Viz)
       │                               │
       │                               ▼
       └──────────────────────► Issue 9 (Session Save + History)
                                       │
                                       ▼
Issue 7 (Assessment Report) ◄── Issue 10 (Diagnosis + Treatment)
                                       │
                                       ▼
                              Issue 11 (Training Plan + Checkin)
                                       │
                                       ▼
                              Issue 12 (Progress Tracking + Reassess)
```

## Implementation Order

### Phase 1: Issue 6 — Consultation Chat (LLM Streaming + Symptom Extraction) ✅
- **Branch**: `feature/6-consultation-chat` → PR #18
- **DB**: `consultation_sessions` migration
- **Python**: LLMProvider protocol, ChatService, consultation prompts, SSE endpoint
- **Go**: Consultation model/repo/service/handler, SSE proxy
- **React**: Chat page with SSE streaming, message list, input box
- **Libraries**: OpenAI Python SDK (already available via reranker)

### Phase 2: Issue 7 — Health Assessment Report ✅
- **Branch**: `feature/7-assessment-report` → PR #19
- **DB**: `assessment_reports` migration
- **Python**: Assessment service with LLM + RAG context
- **Go**: Assessment model/repo/service/handler
- **React**: Assessment list + detail pages

### Phase 3: Issue 8 — Info Panel + Body Visualization ✅
- **Branch**: `feature/8-info-panel-body-viz` → PR #20
- **React**: Info panel component, SVG body visualization, SSE event listener
- **No backend changes** (uses existing SSE stream from Issue 6)

### Phase 4: Issue 9 — Session Save + History ✅
- **Branch**: `feature/9-session-history` → PR #21
- **Go**: Session persistence logic, history list API with pagination
- **React**: History page, session detail view

### Phase 5: Issue 10 — Diagnosis + Treatment Plan ✅
- **Branch**: `feature/10-diagnosis-treatment` → PR #22
- **Python**: Diagnosis prompt, treatment prompt, conversation phase management
- **Go**: Confirm diagnosis API, trigger treatment generation
- **React**: Diagnosis cards, confirm UI, treatment plan display

### Phase 6: Issue 11 — Training Plan + Daily Check-in ✅
- **Branch**: `feature/11-training-plan` → PR #23
- **DB**: `training_plans` + `training_logs` migrations
- **Python**: Training plan generation service
- **Go**: Training CRUD APIs, check-in API
- **React**: Training plan page, daily task list, check-in button

### Phase 7: Issue 12 — Progress Tracking + Reassessment ✅
- **Branch**: `feature/12-progress-tracking` → PR #24
- **Python**: Reassessment service
- **Go**: Progress stats API, reassessment API
- **React**: Progress dashboard with charts, photo comparison, reassessment UI

## Libraries to Consider

- **assistant-ui** (@assistant-ui/react) — AI chat UI components for React
- **@tanstack/react-query** — Server state management (mentioned in tech doc)
- **recharts** — Charts for progress tracking (Issue 12)
- **lucide-react** — Icons (mentioned in tech doc)
- **zustand** — Already in use for state management

## Validation Per Issue

Each issue must pass:
1. `pnpm nx run web:lint && pnpm nx run web:typecheck` (frontend)
2. `cd apps/api && go vet ./... && go test ./...` (backend)
3. `cd apps/ai-service && uv run ruff check . && uv run pytest` (AI service)
4. Manual E2E verification if possible
