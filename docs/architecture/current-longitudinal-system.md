# BodySense Current Longitudinal System

> Status: authoritative current implementation
> Updated: 2026-08-20
> Supersedes: linear Health Journey, session `health_features`, session diagnosis/treatment JSON, and direct Training plan mutation.

## 1. Product loop

```text
Consultation / Posture / Assessment
                |
                v
        durable BodyState
     Fact / Observation / Hypothesis / Evidence
                |
                v
 DiagnosisAnalysis pinned to one BodyState revision
                |
     independent candidate assessments
                |
                v
 TreatmentRevision proposal
                |
      explicit final acceptance gate
                |
                v
 Intervention + Training execution projection
                |
                v
             Outcome
                |
                +----------------------> BodyState revision
```

There is no user-level terminal `completed` state. `HealthWorkspace` derives current capabilities and next actions from durable objects.

## 2. State ownership

| State                                                       | Owner                | Rules                                                                                                                                |
| ----------------------------------------------------------- | -------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| Conversation messages, run envelopes, public runtime events | Go                   | Durable public ledger, request idempotency, one active run per user conversation                                                     |
| LangGraph checkpoints, tool loop, interrupt/resume          | Python               | Agent runtime only; not business truth                                                                                               |
| BodyState current projection and revisions                  | Go                   | One per user, optimistic concurrency, semantic revisions                                                                             |
| DiagnosisAnalysis and candidate assessments                 | Go                   | Analysis immutable; user assessment separate and independently editable                                                              |
| Treatment and TreatmentRevision                             | Go                   | AI may propose through an exact immutable Agent configuration; Go verifies/persists provenance and alone accepts; accepted revisions are immutable |
| Intervention, TrainingPlan, TrainingLog, Outcome            | Go                   | Training is an execution projection of an accepted revision                                                                          |
| Capability/action projection                                | Go `HealthWorkspace` | Pure read; no hidden mutation from GET                                                                                               |
| React query cache and workbench preferences                 | Web                  | Server projection cache only; URL owns active conversation/workspace mode; Zustand owns presentation preferences, never health truth |

## 3. Mandatory mutation invariants

### Diagnosis

- Input is an exact durable BodyState revision.
- Ordinary analysis is blocked by active durable safety state.
- Python returns typed candidate-oriented output.
- Go persists immutable analysis identity and evidence references.
- Analysis freshness is evaluated independently from historical immutability.

### Treatment generation and acceptance

Generation and final acceptance both require:

- eligible DiagnosisAnalysis;
- fresh DiagnosisAnalysis;
- at least one candidate assessed as `confirmed` or `unsure`;
- no active safety review state.

Final acceptance additionally re-reads current BodyState and rejects a proposal when a material related change occurred after its source revision. UI capabilities are advisory; the mutation boundary enforces the invariant again.

Every AI-generated proposal also records the exact immutable Treatment Agent configuration and PydanticAI/LiteLLM execution provenance. Go selects the configuration, verifies configuration ID/role/decision-policy/runtime/logical-model identity before persistence, and never treats Agent provenance as acceptance authority.

### Outcome feedback

`Outcome(user_id, source_type, source_key)` is idempotent. If Outcome persistence succeeds but BodyState projection fails, retrying the same source identity repairs the missing BodyState link rather than returning early forever.

### Training execution

- Only an accepted TreatmentRevision can produce a TrainingPlan.
- Projection creation is idempotent and recoverable through `POST /api/v1/training/current/ensure`.
- `HealthWorkspace.active_training_plan` makes the plan discoverable after reload, login, or device change.
- Training feedback creates Outcomes; it never mutates an accepted plan in place.

### Assessment

Assessment is a derived report, not a second health truth or Treatment system.

- It consumes Profile, Posture analysis and current BodyState.
- It emits traceable Observation candidates and information gaps.
- Observations are projected into BodyState with content-addressed source keys before the report is stored.
- It does not emit executable exercise, nutrition, or treatment prescriptions.

### Safety and consultation input

Safety events are fail-closed. Interaction answers and extracted health inputs must be durably written to BodyState before the Agent proceeds or the event is exposed as successfully committed.

## 4. Runtime contract

The current runtime deliberately uses **disconnect means cancel** semantics:

1. the browser opens one SSE request;
2. Go owns the run envelope and public event log;
3. Python streams typed runtime events;
4. Go persists and forwards them;
5. HTTP disconnect cancels the producer, marks the run failed/assistant message aborted, and clears `active_run_id`;
6. the user may submit a new request with a new request ID.

The event log supports history, page hydration and replay of already-produced events. It is not a promise that model generation continues in the background after the request disconnects.

Run creation locks the user and target conversation transactionally. A second request with another request ID receives `409 RUN_IN_PROGRESS` while an active run exists.

## 5. Public domain API

```text
POST /api/v1/consultation-runs
GET  /api/v1/consultations/:id/thread
POST /api/v1/consultations/:id/diagnosis

GET  /api/v1/body-state
POST /api/v1/body-state/facts
POST /api/v1/body-state/observations
POST /api/v1/body-state/hypotheses
POST /api/v1/body-state/safety/resolve

GET  /api/v1/diagnosis-analyses
PUT  /api/v1/diagnosis-analyses/:analysisId/assessment

POST /api/v1/treatments/proposals
POST /api/v1/treatments/revisions/:revisionId/accept
POST /api/v1/treatments/revisions/:revisionId/reject
POST /api/v1/treatments/current/review

POST /api/v1/training/current/ensure
GET  /api/v1/training/:id
PUT  /api/v1/training/:id/log
POST /api/v1/training/:id/checkin

POST /api/v1/outcomes
GET  /api/v1/health-workspace
```

## 6. React presentation architecture

The web application is a projection consumer, not another domain owner.

```text
React route
├── URL identity
│   ├── /consultation/:conversationId
│   └── ?view=state|diagnosis|treatment|progress
├── TanStack Query server state
│   ├── canonical query key/options factories
│   ├── conversation/thread/diagnosis projections
│   └── HealthWorkspace projection
├── feature-owned mutation hooks
│   ├── BodyState commands
│   ├── Diagnosis assessment commands
│   ├── Treatment/Outcome commands
│   └── centralized invalidation/error mapping
├── assistant-ui runtime
│   └── active streaming turn remains mounted across panel collapse
└── Zustand presentation preferences
    ├── chat expanded/collapsed
    ├── last desktop chat width
    └── mobile chat/workspace surface
```

The desktop consultation route is an immersive workbench:

- chat is a resizable companion and can collapse completely;
- State, Diagnosis, Treatment and Progress are explicit route-addressable modes;
- conversation history is an accessible drawer rather than a permanent third column;
- the body map is an information organizer, not a diagnostic visualization;
- component rendering does not directly edit TanStack caches;
- route-level error boundaries preserve a clear recovery path without implying that durable server data was lost;
- semantic status tokens distinguish success, warning and safety states without relying on color alone;
- reduced-motion preferences are respected globally.

`ConsultationPage` composes feature hooks. Query options, cache reconciliation, mutation invalidation and API error normalization live outside presentation components. Backend mutation boundaries remain authoritative when UI capability projections are stale.

## 7. Verification contract

The release gate is shared by local validation and CI:

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm validate:local-deploy
```

`validate:local-deploy` additionally verifies Docker builds, API/AI/Web health, empty PostgreSQL migration chain, latest migration down/up replay, and Playwright against real services. The E2E-only deterministic AI stub is enabled solely by `BODYSENSE_E2E_STUB_AI=1` in development/test/e2e environments.

The longitudinal browser suite covers:

- registration and profile;
- durable BodyState revision and reload;
- Diagnosis and independent candidate assessment;
- Treatment readiness gate;
- safety change between proposal and acceptance;
- fresh re-analysis and explicit acceptance;
- TrainingPlan discovery after page reload;
- training feedback → Outcome → BodyState revision;
- Treatment review recommendation.
