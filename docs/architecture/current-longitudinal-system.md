# BodySense Current Longitudinal System

> Status: authoritative current implementation
> Updated: 2026-08-23
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

| State                                                       | Owner                | Rules                                                                                                                                              |
| ----------------------------------------------------------- | -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| Conversation messages, run envelopes, public runtime events | Go                   | Durable public ledger, request idempotency, one active run per user conversation                                                                   |
| LangGraph checkpoints, tool loop, interrupt/resume          | Python               | Agent runtime only; not business truth                                                                                                             |
| BodyState current projection and revisions                  | Go                   | One per user, optimistic concurrency, semantic revisions                                                                                           |
| DiagnosisAnalysis and candidate assessments                 | Go                   | Analysis immutable; user assessment separate and independently editable                                                                            |
| Treatment and TreatmentRevision                             | Go                   | AI may propose through an exact immutable Agent configuration; Go verifies/persists provenance and alone accepts; accepted revisions are immutable |
| Intervention, TrainingPlan, TrainingLog, Outcome            | Go                   | Training is an execution projection of an accepted revision                                                                                        |
| Capability/action projection                                | Go `HealthWorkspace` | Pure read; no hidden mutation from GET                                                                                                             |
| React query cache and workbench preferences                 | Web                  | Server projection cache only; URL owns active conversation/workspace mode; Zustand owns presentation preferences, never health truth               |

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
Treatment v2 additionally records a bounded `EvidenceGap` acquisition trace on the immutable TreatmentRevision. `user_fact` gaps cannot invoke external RAG, external knowledge searches have a finite per-run budget, and every stop reason is auditable. Under ADR 0010, v2 is the repository Champion/default serving baseline; v1 remains the explicit rollback and historical replay target. A future Challenger requires its own immutable promotion record.
Successful Treatment proposals also persist the Go `generation_decision_trace`; successful acceptance writes `acceptance_decision_trace` atomically with the acceptance/current-pointer transaction. These traces record the deterministic authority facts and exact checked BodyState revision. Malformed or unknown durable safety state now fails closed instead of being interpreted as safe.
Treatment proposals also freeze their exact Agent input in a private replay envelope. Historical replay recomputes Go generation authority without calling a model; counterfactual replay can run another immutable Treatment configuration against the same frozen input and reports hard/semantic/presentation drift without persistence side effects. Revisions predating frozen input are explicitly not replayable.
Treatment rollout selection is Go-owned and stable per user. Shadow/canary runs use the frozen replay envelope after the served proposal is durable; anonymous `treatment_rollout_observations` capture comparison/blocker signals while the served revision stores `rollout_provenance`. The hermetic validator proves v1 serving + v2 shadow with zero v2 TreatmentRevision persistence.

### Outcome feedback

`Outcome(user_id, source_type, source_key)` is idempotent. If Outcome persistence succeeds but BodyState projection fails, retrying the same source identity repairs the missing BodyState link rather than returning early forever.

### Training execution

- Only an accepted TreatmentRevision can produce a TrainingPlan.
- Projection creation is idempotent and recoverable through `POST /api/v1/training/current/ensure`.
- `HealthWorkspace.active_training_plan` makes the plan discoverable after reload, login, or device change.
- Training feedback creates Outcomes; it never mutates an accepted plan in place.

### Assessment

Assessment is a derived report, not a second health truth or Treatment system.

- The application receives stable Profile context plus current BodyState, report indicators and **completed governed Posture analysis**; Profile is not health-observation evidence, and the v3 Agent itself only receives the canonical evidence catalog. Assessment never directly interprets raw images.
- It emits traceable Observation candidates with exact evidence refs; evidence coverage/gaps are derived deterministically by the application.
- Observations are projected into BodyState inside the same transaction as the report, using report-scoped source keys (`assessment:<report_id>:observation:<index>`). Cross-report evidence/content-addressed idempotency is an explicit `DECISION-GAP`: report-history semantics and evidence-candidate deduplication are both valid models, so no stronger key should be introduced until the longitudinal meaning is chosen.
- It does not emit executable exercise, nutrition, or treatment prescriptions.

### Safety and consultation input

Safety events are fail-closed. Interaction answers and extracted health inputs must be durably written to BodyState before the Agent proceeds or the event is exposed as successfully committed.

## 4. Runtime contract

The Consultation runtime uses **transport disconnect != business cancellation** semantics:

1. the browser opens an SSE request;
2. Go creates the durable Run, selects the exact immutable Consultation configuration and owns the public event ledger;
3. Python resolves that exact manifest and emits `runtime.agent_configuration` as the **first internal control-plane event**;
4. before accepting ordinary semantic/tool/state output, Go verifies configuration ID, role, decision-policy revision and logical model, then persists the run-local execution identity immediately;
5. Python streams the remaining typed internal events; Go validates their supported type/channel/payload, maps them to public events, assigns/persists the public `(run_id, seq)` ordering and forwards SSE;
6. an HTTP/SSE disconnect does **not** cancel durable inference. The run-scoped execution context survives transport cancellation up to its execution deadline;
7. explicit `POST /api/v1/consultation-runs/:id/cancel` is the authorized cancellation command. It atomically moves an active/waiting Run to `cancelled`, cancels any pending HITL interaction, cancels the registered producer when one is live, and produces terminal `run.cancelled`;
8. completion/cancellation/failure are conditional terminal transitions, so a late competitor cannot overwrite the winner.

HITL resume is continuation of one logical LangGraph thread. Go reloads the interrupted source Run and sends its durable `agent_configuration_id`; Python reconciles that ID against the checkpointed Consultation manifest **before** executing `Command(resume=...)`. A Champion change while the user is waiting never silently switches the resumed thread.

The public event ledger has one sequence domain per Run: `seq` is monotonic and unique for a given `run_id`. Live events use the Go `StreamWriter`; out-of-band events for waiting/inactive runs allocate `MAX(seq)+1` transactionally while locking the owning Run row. React replay therefore deduplicates by `(run_id, seq)`, not by event type. Server timestamps (for example interaction `created_at`) are part of durable projection input; reducers do not invent time/random identity during replay.

The shared `@bodysense/contracts` runtime parser is the browser trust boundary for both live SSE and durable recovery rows. Unknown versions/types/channels or malformed event-specific payloads become explicit protocol failures; raw `JSON.parse(...) as StreamEvent` is not an accepted network boundary. Go applies the corresponding fail-closed rule to Python NDJSON rather than skipping malformed lines.

Run creation locks the user and target conversation transactionally. A second request with another request ID receives `409 RUN_IN_PROGRESS` while an active run exists. Process-crash-resumable model execution remains out of scope: transport detachment is not a durable worker/job runtime.

## 5. Public domain API

```text
POST /api/v1/consultation-runs
POST /api/v1/consultation-runs/:id/cancel
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

## 5.1 Online RAG resource boundary

`KnowledgeLibrary` is lifecycle-owned by the FastAPI application. Startup creates one bounded `psycopg_pool.AsyncConnectionPool`, registers pgvector on async connections and fails within a bounded connect timeout when PostgreSQL is unavailable. Search/list/stats and full source→segment→unit→clip ingestion use async connections/cursors; ingestion remains one database transaction. Shutdown closes only the pool owned by that library instance. Hidden per-request connection creation is not allowed.

Remote embedding remains native async I/O. Local `SentenceTransformer` model initialization and `encode()` execute through `asyncio.to_thread` behind a bounded semaphore, so CPU-heavy inference cannot block the event loop or grow unbounded worker concurrency. Hashing fallback remains synchronous because its cost is small and deterministic.

Treatment Grounding Eval v2 is currently an **eval-only diagnostic**. It performs deterministic cited-evidence/admissibility checks first, structured claim support second and allows an optional semantic Judge only for uncertain cases. The production Treatment faithfulness policy remains v1 until v2 disagreement data is reviewed and a separate qualification/promotion decision is made.

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

## Agent platform governance reference

LLM-backed runtime ownership, immutable configuration requirements, and the role-specific distinction
between clinical decision Agents, Posture perception, Title utility, offline Knowledge Agents, and
non-LLM mechanisms are defined in
[`agent-platform-role-governance.md`](./agent-platform-role-governance.md). Model/provider routing is
defined in [`model-gateway-routing.md`](./model-gateway-routing.md).
