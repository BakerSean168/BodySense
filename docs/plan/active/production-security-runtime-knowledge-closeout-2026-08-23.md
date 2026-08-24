# Production Security, Runtime Resilience & Knowledge Closeout Plan — 2026-08-23

> Status: **ACTIVE**
>
> Program class: **post-v0.5.2 production closeout**
>
> Repository baseline: `main` / `v0.5.2` / `2e64e67bc43202fb4d3dca91905112af1930be95`
>
> Production baseline: Alibaba Cloud ECS `body.bakersean.top`, schema `49:false`, coherent `v0.5.2` Web/API/AI/runtime revision, deploy watcher enabled and healthy.
>
> Environment ownership: **GCP-dev = development + production operations control plane; Alibaba Cloud = sole production runtime; Oracle2 = detached from BodySense.**
>
> Scope: security/privacy boundaries, Consultation process-failure liveness, disaster recovery, Knowledge Lifecycle productionization, production hardening, and final governance/documentation convergence.
>
> Non-goal: this plan does **not** redesign the successful Longitudinal BodyState, Diagnosis/Treatment ownership model, Training projection, LiteLLM provider boundary, StreamEvent contract, or completed Agent-platform migration.

---

## 0. Executive decision summary

The 2026-08 Agent-platform and Consultation/RAG hardening programs established the correct large-scale architecture:

```text
React
= projection consumer + user interaction

Go
= authentication / authorization
= durable business and health truth
= Run/event ledger
= deterministic business and clinical authority
= immutable Agent configuration selection

Python AI Service
= typed semantic Agent execution
= LangGraph/PydanticAI runtime
= tool orchestration and RAG computation

LiteLLM
= the only physical LLM provider/routing/fallback boundary

PostgreSQL
= durable domain + runtime ledger
```

The `v0.5.2` production audit did **not** find a P0 outage or evidence that the North-Star domain model should be replaced. It did find a new, coherent class of production-readiness gaps that should be repaired as a dedicated program rather than reopening completed migration plans.

### Audit findings ledger

| ID | Severity | Observed behavior | Desired behavior / invariant |
| --- | --- | --- | --- |
| PROD-SEC-01 | **P1** | `GET /consultations/:id/interaction-metrics` validates authentication but not conversation ownership | Every user-scoped object lookup must prove owner identity at the service/repository boundary |
| PROD-PRIV-01 | **P1** | deleting a shared conversation only soft-deletes `conversations`; the public `conversation_shares` snapshot remains readable by token | deleting conversation history must atomically revoke all public shares for that conversation |
| PROD-AUTH-01 | **P1** | logout deletes refresh/session cache, but a still-valid access JWT can repopulate the cache; production access TTL is 168h | logout/session revocation must invalidate the authenticated session family; access credentials should be short-lived |
| PROD-RUN-01 | **P1** | a running Consultation execution can die with the API/AI process while DB state remains `running` / `active_run_id`; no startup stale-run reconciliation exists | process death/deploy cannot permanently lock a conversation; lost execution ownership must become recoverable/terminal |
| PROD-DR-01 | **P1** | PostgreSQL deployment backups, runtime backups, and uploaded files remain in the same ECS failure domain | durable user data must have an independently recoverable off-host copy; restore must be exercised |
| PROD-KNOW-01 | **P1** | production has `knowledge_sources=0`, `knowledge_units=0`, `knowledge_publications=0`; online RAG therefore has no published corpus | production Knowledge must enter through governed register → ingest → review → qualify → publish lifecycle |
| PROD-KNOW-02 | **P2** | authenticated users can reach global Knowledge ingestion/list/stats routes; there is no operator/admin authorization model | mutation and operational knowledge surfaces must be restricted to an explicit operator authority |
| PROD-AUTH-02 | **P2** | login/register/refresh have no rate limiting; browser persists access + refresh tokens in localStorage; no CSP | credential abuse and browser credential exposure need production-safe boundaries |
| PROD-OPS-01 | **P2** | Gin starts in debug mode and trusts all proxies; host has ~1.6 GiB RAM, no swap and low free headroom; Compose has no memory limits | production mode, trusted proxy chain, bounded resources, and capacity alerts are explicit |
| PROD-SUPPLY-01 | **P2** | `pnpm audit --prod` reports 26 advisories (14 high), mixed between tooling and runtime paths | runtime-exploitable advisories are removed or explicitly justified; dependency checks become reviewable evidence |
| PROD-GOV-01 | **P2** | Diagnosis production Champion intentionally remains on `diagnosis-authority-pre-envelope-v0`; newer deterministic decision authority remains Challenger | promotion occurs only after explicit production-shaped evidence and a recorded decision |
| PROD-DOC-01 | **P2** | deployment docs still name Oracle2 as the development host and several historical architecture docs expose stale implementation percentages | current-system docs must describe only current truth; historical plans must not masquerade as current status |
| PROD-PRIV-02 | **P2** | product distinguishes neither “delete chat history” from “erase durable health data” nor exposes a complete privacy erasure path | retention/deletion semantics must be explicit and enforceable across derived health data and uploads |

### Recommended dependency order

```text
security / privacy containment
        ↓
session authority + abuse protection
        ↓
run liveness + deploy-safe execution
        ↓
off-host durability + restore evidence
        ↓
Knowledge operator boundary
        ↓
Knowledge lifecycle + production corpus
        ↓
Diagnosis promotion / supply-chain / ops hardening
        ↓
architecture-doc convergence + production release gate
```

The order is intentional: **prevent unauthorized exposure and unrecoverable state before increasing the amount of production data or promoting more Agent behavior.**

---

# 1. Current-system map and evidence

## 1.1 Deployment topology

Canonical topology for this plan:

```text
Developer / ChatGPT
        │
        ▼
GCP-dev (gcp-dev)
  - primary BodySense development host
  - local Docker / release validation
  - production operations control point
        │
        ▼
GitHub main
  - protected PR flow
  - CI / release-please
        │
        ▼
Alibaba Cloud ACR
  - immutable application/runtime images
  - coherent prod-latest pointers
        │
        ▼
Alibaba Cloud ECS
  body.bakersean.top
  - sole production runtime
  - systemd deployment watcher
  - PostgreSQL 16 / pgvector
  - Redis
  - LiteLLM
  - AI Service
  - Go API
  - Web
  - Caddy
```

Oracle2 and DigitalOcean are **not** BodySense runtime/development dependencies and must not re-enter the topology through this plan.

## 1.2 Protected product path

The current longitudinal health loop is retained:

```text
Conversation interaction
        ↓
Longitudinal BodyState
        ↓
immutable DiagnosisAnalysis @ BodyState revision
        ↓
user candidate assessment / governance
        ↓
TreatmentRevision proposal
        ↓
explicit user acceptance
        ↓
TrainingPlan execution projection
        ↓
Training feedback / Outcome
        ↓
BodyState + Treatment review
```

Important current invariant:

> `TreatmentRevision` is the treatment source of truth. `TrainingPlan` is an executable projection of an accepted Treatment revision, not a second treatment authority.

This plan must not recreate legacy Journey/MedicalRecord truth or a second training-plan generator.

## 1.3 Production evidence captured during audit

Production was healthy on `v0.5.2` at audit time:

```text
release:      v0.5.2
revision:     2e64e67bc43202fb4d3dca91905112af1930be95
schema:       49:false
watcher:      enabled + active
blocked rev:  none
```

Application services were healthy and no recent OOM/restart evidence was observed.

Production data was still small:

```text
users=2
conversations=0
shares=0
body_states=0
diagnosis_analyses=0
treatments=0
user_uploads=0
```

Production Knowledge was empty:

```text
knowledge_sources=0
knowledge_units=0
knowledge_publications=0
published knowledge=0
```

This low-data state is an advantage: privacy/session/durability corrections can be completed before real longitudinal health data accumulates.

---

# 2. Target outcome

This program is complete when BodySense can make the following production claims with test and deployment evidence:

1. **Authorization** — a logged-in user cannot read, mutate, replay, cancel, inspect metrics for, or share/revoke another user's durable object by guessing its UUID.
2. **Deletion/privacy** — deleting conversation history revokes any public share immediately; account/health-data erasure has an explicit, auditable policy distinct from chat deletion.
3. **Session security** — logout/revocation actually invalidates the session family; refresh rotation is atomic; long-lived bearer credentials are not persisted in browser localStorage.
4. **Run liveness** — API/AI restart or production deploy cannot leave a user permanently blocked by an orphaned `running` Run.
5. **Durability** — database backups and user uploads survive loss of the production ECS; restore is proven by an automated or scripted restore drill.
6. **Knowledge governance** — only explicitly authorized operators can mutate the global Knowledge lifecycle; only reviewed/qualified/published units can enter production retrieval.
7. **RAG readiness** — production has a versioned, auditable, non-empty published corpus and retrieval/evidence behavior is qualified before it is relied on by Diagnosis/Treatment.
8. **Operational baseline** — production runs in release mode with explicit proxy/resource settings and a bounded dependency-vulnerability policy.
9. **Governance** — Diagnosis Challenger promotion remains evidence-driven; no code implementation silently promotes it.
10. **Documentation** — authoritative docs match the GCP-dev → GitHub/ACR → Alibaba production topology and current implemented boundaries.

---

# 3. Protected contracts and non-goals

## 3.1 Domain and authority contracts

The following are protected:

1. One user owns one longitudinal `BodyState` truth.
2. Conversation is interaction history, not health truth.
3. `DiagnosisAnalysis` remains immutable and pinned to an exact BodyState revision/configuration/provenance.
4. Treatment remains revisioned and requires explicit acceptance before becoming current.
5. Training remains a projection of accepted Treatment, not an independent AI-generated treatment source.
6. Outcome observations do not automatically prove causality.
7. Go owns final durable business/clinical state transitions.
8. Python owns semantic Agent execution, not user authorization or durable health truth.

## 3.2 Agent/runtime contracts

1. LiteLLM remains the only physical LLM provider/fallback boundary.
2. Go selects repository-known immutable Agent configurations.
3. Python must execute the exact selected identity and return verifiable identity/provenance.
4. `StreamEvent v1` and durable `(run_id, seq)` semantics remain compatible unless an explicit versioned migration is introduced.
5. HTTP/SSE disconnect does not mean user cancellation.
6. Explicit cancellation remains an authorized durable command.
7. LangGraph checkpoint remains runtime state, not BodyState truth.

## 3.3 Release/deployment contracts

1. Alibaba Cloud remains the only production runtime.
2. GCP-dev remains development/ops control; Oracle2 must not be reintroduced.
3. `main` → successful CI → release-please → immutable ACR images → coherent prod pointers → Alibaba watcher remains the release chain.
4. Published migrations remain immutable; schema changes require new migrations and both PG16 production-baseline and PG18 replay validation.
5. Production provider secrets stay outside images/repository.

## 3.4 Explicit non-goals

This plan will not:

- migrate BodySense production to Oracle2, GCP, Kubernetes, Temporal, or a new cloud platform;
- replace Go/Python/React service ownership;
- reintroduce direct provider routing outside LiteLLM;
- make LangGraph checkpoint the business source of truth;
- auto-promote Diagnosis/Treatment Challenger configs simply because implementation is complete;
- make generated/unreviewed Knowledge searchable to production;
- build a large enterprise IAM/RBAC product when a bounded operator authority is sufficient;
- introduce a second mutable health summary/MedicalRecord aggregate.

---

# 4. North-Star closeout architecture

## 4.1 Security boundary

```text
Browser
  │
  ├─ short-lived access credential (memory)
  │
  └─ secure refresh/session cookie
          │
          ▼
Go Auth Boundary
  - atomic refresh rotation
  - session-family revocation
  - abuse/rate policy
          │
          ▼
Authorization Boundary
  user identity + object identity
          │
          ├─ Conversation/Run/Interaction
          ├─ Diagnosis/Treatment/Training
          ├─ Upload
          └─ Knowledge operator capability
```

A repository/service method that can read a user-owned object without a user identity should be treated as suspicious unless it is explicitly an internal/public-share path.

## 4.2 Durable Consultation execution ownership

```text
Run row
  status
  active execution identity
  lease owner
  lease expiry / heartbeat
        │
        ▼
Go process owns lease while model execution is active
        │
        ├─ normal completion -> terminal event + clear active_run_id
        ├─ explicit cancel -> terminal event + clear active_run_id
        └─ process loss -> lease expires
                         ↓
                  startup/periodic reconciliation
                         ↓
              safe failed/recoverable terminal state
                         ↓
                  conversation unlocked
```

The lease is not a distributed workflow engine. It is a minimal proof of live process ownership so stale database state cannot permanently block the user.

## 4.3 Governed Knowledge path

```text
Authorized Knowledge Operator
        ↓
Register Source
  metadata + license/provenance
        ↓
Knowledge Ingestion Job
  ASR / split / curate
        ↓
Generated/Curated Units
        ↓
Review + deterministic quality gates
        ↓
Embedding
        ↓
Publication batch
        ↓
Published Knowledge
        ↓
production search_knowledge
        ↓
Evidence provenance + citation
```

Invariant:

> **No startup bootstrap, migration, script, or Agent may bypass review/publication by inserting data directly into the online-visible state.**

## 4.4 Disaster-recovery boundary

```text
Alibaba ECS local state
  ├─ PostgreSQL
  └─ temporary/runtime files
        │
        ├─ validated local pre-deploy backup (rollback purpose)
        │
        └─ independent off-host backup
               └─ Alibaba OSS / S3-compatible object storage

User uploads
        ↓
Object storage as durable source
        ↓
API metadata + owned object key
```

Local pre-deploy backups and off-host disaster recovery have different purposes and both remain useful.

---

# 5. Phase roadmap

## Phase 0 — Security & privacy containment

**Why first:** these are authorization/credential/privacy invariants. They must be repaired before growing production data or exposing new Knowledge administration.

Deliverables:

- object-ownership characterization suite and repair;
- conversation-delete/share revocation transaction;
- explicit public-share DTO and lifecycle policy;
- session-family model with real revocation and atomic refresh rotation;
- browser credential storage migration;
- auth rate limiting, release proxy/mode configuration, CSP;
- explicit chat deletion vs privacy erasure contract.

Exit gate:

```text
cross-user authorization tests PASS
share-after-delete test returns 404
logout invalidates session family
refresh replay/reuse tests PASS
browser no longer persists refresh bearer token in localStorage
auth abuse tests return deterministic 429
```

## Phase 1 — Run liveness & disaster resilience

**Why second:** once security boundaries are correct, production must survive process loss and host loss without trapping users or losing their durable health data.

Deliverables:

- execution lease/heartbeat on active Consultation Runs;
- startup/periodic stale-run reconciliation;
- user-visible cancel/recover path;
- deploy watcher active-run drain/defer behavior;
- off-host PostgreSQL backup + integrity/restore validation;
- object storage abstraction/migration for user uploads;
- capacity/swap/resource hardening and operator runbook.

Exit gate:

```text
kill/restart simulation never leaves permanent RUN_IN_PROGRESS
stale active_run_id is reconciled deterministically
production deploy defers active running inference or closes it safely
off-host backup exists and checksum validates
fresh database restore passes domain validator
upload survives API/ECS filesystem loss by object-store retrieval
```

## Phase 2 — Knowledge Lifecycle productionization

**Why third:** production RAG should only be populated after operator authorization and disaster/security boundaries are trustworthy.

Deliverables:

- bounded operator authorization for global Knowledge administration;
- KnowledgeSourceRegistry;
- durable ingestion-job path;
- review/quality/publication/rollback workflow;
- retrieval qualification suite;
- governed publication of initial curated corpus;
- Diagnosis/Treatment evidence-gap behavior validated against empty/unavailable/partial Knowledge states.

Exit gate:

```text
unauthorized member cannot mutate Knowledge lifecycle
source → ingest → review → publish works end-to-end
unreviewed/generated units are never returned by production search
publication rollback removes a batch from online visibility without deleting provenance
production published knowledge > 0
retrieved citations resolve to source + unit + publication identity
retrieval qualification meets predeclared thresholds
```

## Phase 3 — Governance, supply chain & source-of-truth closeout

**Why last:** promotion and documentation should describe a production system whose security, liveness, durability, and Knowledge evidence plane are already real.

Deliverables:

- Diagnosis Challenger shadow/canary evidence and explicit promotion/hold decision;
- dependency advisory triage + upgrades + bounded CI policy;
- production mode/proxy/resource docs synchronized with actual deployment;
- authoritative architecture docs cleaned of stale topology/status claims;
- full local deploy + release + Alibaba production validation;
- plan closure evidence and archive.

Exit gate:

```text
full repository release gate PASS
local production-shaped deploy PASS
production smoke / health / ownership / RAG checks PASS
no unresolved P0/P1 findings
P2 deferrals have explicit owner/reason
current architecture docs match deployed topology
```

---

# 6. Execution-ready tickets

## BS-PROD-000 — Freeze v0.5.2 production characterization baseline

**Goal:** Turn the audit findings into reproducible tests/fixtures before behavior changes.

**Why now:** The current system is healthy. Repairs should prove they close a specific gap without accidentally redesigning working contracts.

**Scope:**

- authorization characterization for user-owned endpoints;
- share deletion/public token behavior;
- logout/access-token behavior;
- stale Run behavior under simulated process loss;
- empty Knowledge production/readiness behavior;
- production topology assertions.

**Out of scope:** implementing fixes.

**Protected contracts:** all contracts in §3.

**Implementation:**

1. Add focused Go HTTP/service tests for cross-user access to Consultation interaction metrics and adjacent run/interaction endpoints.
2. Add a test proving the current share snapshot remains independent from later message changes, then a failing characterization for delete-with-active-share.
3. Add auth tests that distinguish “user exists” cache from “session revoked” semantics.
4. Add a Runtime test that creates an active run without a live execution owner and proves a second request is blocked under current behavior.
5. Add Knowledge readiness fixtures for empty, generated-only, and published corpus states.
6. Add a topology validation assertion that production docs/config name GCP-dev as dev/ops, Alibaba as sole production, and never Oracle2 as active BodySense infrastructure.

**Tests / verification:**

```bash
go test ./apps/api/internal/handler ./apps/api/internal/service ./apps/api/internal/consultation -count=1
pnpm --filter web test
pnpm --filter ai-service test
```

**Acceptance:** every P1 behavior in the finding ledger has a deterministic failing/characterization test or a documented production check before its repair ticket changes behavior.

**Dependencies:** none.

**Risks:** test setup may accidentally depend on production-only state; keep fixtures synthetic and user-isolated.

---

## BS-PROD-001 — Close the user-object authorization boundary

**Goal:** No authenticated user can access another user's interaction metrics or any adjacent user-scoped runtime object by UUID.

**Why now:** `interaction-metrics` is a confirmed IDOR and indicates a service signature that makes ownership omission possible.

**Scope:**

- `apps/api/internal/handler/consultation_handler.go`;
- Interaction service/repository methods used by metrics;
- systematic user-owned endpoint sweep for the same signature pattern;
- cross-user tests.

**Out of scope:** global Knowledge authorization; handled in BS-PROD-020.

**Protected contracts:** public URL shapes remain compatible; same-owner responses preserve payload shape.

**Implementation:**

1. Change metrics service/repository entry points to require `userID + conversationID` rather than accepting only conversation ID.
2. Resolve ownership in the same durable boundary used by other Consultation paths.
3. Prefer “not found for this owner” semantics where the existing API uses that anti-enumeration convention.
4. Search handler/service/repository call graphs for user-owned UUID lookups that drop `uid` before persistence access.
5. Add negative tests with two users for every repaired path.
6. Add a focused structural test/lint assertion only if it can reliably detect recurrence without false positives; do not add brittle grep gates.

**Tests:**

```bash
go test ./apps/api/internal/handler ./apps/api/internal/service ./apps/api/internal/repository -count=1
```

**Acceptance:** User A receives no metrics/data for User B's object; owner behavior remains unchanged; adjacent ownership sweep has no unresolved P1 bypass.

**Dependencies:** BS-PROD-000.

**Risks:** returning different 403/404 semantics can create object-existence leakage; preserve existing not-found convention.

---

## BS-PROD-002 — Make conversation deletion revoke public exposure atomically

**Goal:** Once the user deletes conversation history, any public share token for that conversation is unusable immediately.

**Why now:** current soft deletion does not revoke the separately persisted public snapshot.

**Scope:**

- ConversationService deletion transaction;
- ConversationShare repository/service;
- public share DTO;
- optional expiry field/lifecycle;
- Web deletion/share language.

**Out of scope:** deleting longitudinal BodyState/Diagnosis/Treatment; handled by BS-PROD-005.

**Protected contracts:** snapshot sharing remains intentionally read-only and excludes health-domain projections; existing share URLs continue until revoked/expired.

**Implementation:**

1. Introduce a unit-of-work transaction for `revoke shares + soft-delete conversation`.
2. Make delete-all reuse the same server-side invariant rather than relying on client sequencing for privacy correctness.
3. Define a minimal `PublicSharedMessageDTO`; do not serialize the persistence `Message` model directly.
4. Exclude provider/model/token/error/internal metadata unless explicitly required for the public product.
5. Add optional explicit share expiry/revocation metadata and enforce it in lookup.
6. Update UI copy to distinguish “delete chat history” from “erase health data”.
7. Add regression tests: share → delete → public GET = 404; revoked/expired share = 404; unrelated BodyState remains intact.

**Tests:**

```bash
go test ./apps/api/internal/handler ./apps/api/internal/service ./apps/api/internal/repository -count=1
pnpm --filter web test
```

**Acceptance:** no public snapshot remains readable after conversation deletion; API returns only the minimal public schema.

**Dependencies:** BS-PROD-000.

**Risks:** historical shares may require migration/backfill for expiry metadata; preserve explicit revocation compatibility.

---

## BS-PROD-003 — Replace token-cache semantics with real session-family authority

**Implementation status (2026-08-23): VALIDATED.** Atomic single-winner refresh rotation, replay-family revocation, logout-family revocation, short-lived access TTL, session-authority fail-closed behavior, legacy-token DB fail-closed behavior, and concurrent/race tests are implemented. Focused `-race` and Go full-suite gates pass.

**Goal:** Logout/revocation invalidates the active session; refresh rotation is atomic and replay-safe; access credentials are short-lived.

**Why now:** current `UserSessionCache` verifies user existence rather than session validity, so deleting it on logout does not revoke a valid 7-day access JWT.

**Scope:**

- `apps/api/internal/auth/`;
- `apps/api/internal/service/auth_service.go`;
- `apps/api/internal/middleware/auth.go`;
- Redis session/refresh representation;
- auth DTO/API compatibility migration.

**Out of scope:** SSO/OAuth/social login.

**Protected contracts:** user identity remains UUID-based; password hashing remains bcrypt cost >=12; deleted users remain rejected.

**Decision:** introduce an explicit **session family** identity. Access tokens carry the session ID; Redis/DB owns revocation/refresh-family state. User-existence caching must be separated from session authorization.

**Implementation:**

1. Separate `user exists` cache from authenticated-session state.
2. Add session/family ID to access claims and refresh state.
3. Make refresh token consumption/rotation atomic using Redis transaction/Lua or an equivalently race-safe primitive.
4. Detect refresh reuse/replay and revoke the affected family.
5. Make logout revoke the whole active session family.
6. Reduce production access TTL from 168h to a short-lived value; choose the exact TTL in one configuration constant and document it.
7. Support a bounded compatibility window for any pre-migration token only if required; fail closed rather than silently accepting unowned session state.
8. Test concurrent refresh requests, logout + old access token, deleted user, Redis failure policy, and token-family reuse.

**Tests:**

```bash
go test ./apps/api/internal/auth ./apps/api/internal/middleware ./apps/api/internal/service -count=1
go test -race ./apps/api/internal/service ./apps/api/internal/middleware -count=1
```

**Acceptance:** after logout the old access credential is rejected; only one concurrent refresh rotation succeeds; reuse revokes/blocks the family deterministically.

**Dependencies:** BS-PROD-000.

**Risks:** authentication migration can lock out users; release must include explicit compatibility/rollback steps and synthetic login/refresh smoke tests.

---

## BS-PROD-004 — Move browser auth to secure cookie + in-memory access and add abuse/browser hardening

**Implementation status (2026-08-23): IMPLEMENTED / LOCAL GATES GREEN.** Refresh is `Secure`/`HttpOnly`/`SameSite=Strict` in production, access credentials are memory-only, auth responses are `no-store`, Origin checks and Redis abuse limits are active, trusted proxies/Gin release mode/CSP are configured, and Web unit/type/lint/build plus Go gates pass. Production-shaped browser E2E remains part of BS-PROD-033/034 before release.

**Goal:** long-lived bearer credentials are not persisted in JavaScript-readable localStorage, and public auth endpoints have bounded abuse controls.

**Why now:** this completes the session model from BS-PROD-003 at the browser/edge boundary.

**Scope:**

- `apps/web/src/stores/authStore.ts` and auth service/hooks;
- Go refresh/logout cookie handling;
- same-origin Caddy/Web headers;
- auth rate limiter;
- CSP/security headers;
- Gin release/proxy configuration.

**Out of scope:** third-party identity provider integration.

**Protected contracts:** same-origin SPA flow; authenticated API semantics; logout UX.

**Implementation:**

1. Store refresh/session credential in `Secure; HttpOnly; SameSite` cookie.
2. Keep short-lived access token in memory; on reload, restore the session through the refresh cookie rather than localStorage.
3. Remove refresh/access token persistence from Zustand localStorage state.
4. Add CSRF reasoning/tests for cookie-bearing state-changing auth routes; use SameSite plus explicit anti-CSRF token/origin validation where required.
5. Add Redis-backed login/register/refresh rate limits keyed by normalized account/IP dimensions.
6. Configure explicit trusted reverse proxy addresses/ranges before relying on client IP.
7. Set `GIN_MODE=release` in production.
8. Add a CSP compatible with the built React application; tighten iteratively from report-only only if required to avoid breaking production.
9. Add browser tests for reload, expiration, logout, refresh failure and multiple-tab behavior.

**Tests:**

```bash
pnpm --filter web test
go test ./apps/api/internal/handler ./apps/api/internal/middleware ./apps/api/internal/service -count=1
pnpm e2e
```

**Acceptance:** no refresh bearer token is readable from localStorage; logout/reload behavior remains correct; abusive auth attempts receive deterministic `429`; production starts Gin in release mode with explicit proxy trust.

**Dependencies:** BS-PROD-003.

**Risks:** cookie/CSP changes can create subtle browser regressions; require real browser E2E before merge.

---

## BS-PROD-005 — Define privacy erasure and retention semantics

**Implementation status (2026-08-23): VALIDATED FOR CURRENT LOCAL OBJECT BACKEND.** A dry-run + explicit-confirmation durable erasure workflow, authentication tombstone, retry leases, transaction/cascade boundary, retention matrix, UI distinction, and synthetic PostgreSQL erasure test are implemented. `scripts/validate-privacy-erasure.sh` replays migrations including `000054 down/up` and proves synthetic user/session/share/BodyState/Diagnosis/Treatment/Outcome/upload erasure. BS-PROD-013 will replace the local object cleaner behind the same deletion port.

**Goal:** Product, API and persistence distinguish chat-history deletion from longitudinal health-data erasure.

**Why now:** Conversation is intentionally not the health truth, so “delete chat” cannot honestly imply “all health data removed”.

**Scope:**

- privacy/domain documentation;
- account/data deletion service boundary;
- BodyState/Diagnosis/Treatment/Outcome/Upload/share retention map;
- user-facing copy and confirmation;
- audit/redaction strategy for immutable provenance where deletion law/policy requires it.

**Out of scope:** jurisdiction-specific legal advice; the plan defines engineering semantics and configurable retention policy.

**Protected contracts:** immutable analysis/provenance remains immutable during normal operation; privacy erasure is a separate privileged lifecycle action.

**Implementation:**

1. Inventory every table/object containing user-derived health or identity data.
2. Classify each as delete, anonymize/redact, or retain-with-justification under privacy erasure.
3. Define one service-level erasure operation rather than scattered repository deletes.
4. Ensure shares and object-store uploads are included.
5. Ensure auth/session credentials are revoked before/with erasure.
6. Add idempotency and partial-failure recovery/audit semantics.
7. Update UI wording so chat deletion and full data/account deletion are distinct.
8. Add integration tests against a synthetic user with BodyState, analysis, treatment, outcomes, uploads and shares.

**Acceptance:** an engineering retention matrix exists and the synthetic erasure test leaves no user-accessible or publicly shared health data outside explicitly documented retained/redacted records.

**Dependencies:** BS-PROD-002, BS-PROD-003; object-store deletion is finalized with BS-PROD-013.

**Risks:** destructive operations have high blast radius; implement dry-run/reporting and explicit confirmation before exposing UI action.

---

## BS-PROD-010 — Add durable Consultation execution leases and stale-run reconciliation

**Implementation status (2026-08-24): VALIDATED INCLUDING PROCESS-RESTART PROJECTION RECOVERY.** Migrations `000052/000053`, owner-bound lease heartbeat, startup/periodic stale-run reconciliation, `execution_lost` durable terminal events, active-run clearing, and `waiting_user` separation are implemented. Reconciliation now also terminalizes the matching persisted assistant message (`status=failed`, `error.code=execution_lost`) before clearing `active_run_id`, then appends both `run.failed` and `message.failed`. `scripts/validate-run-leases.sh` proves double-reconciler single ownership, completion-vs-reconciler single terminal winner, and no reclamation of `waiting_user`; the production-shaped API-container restart E2E additionally proves the terminal projection survives thread refetch and browser reconstruction.

**Goal:** process death can never leave a conversation permanently blocked by a Run that no process owns.

**Why now:** detached HTTP context protects against transport loss, not API/AI process loss.

**Scope:**

- Run persistence model/migration;
- Consultation Runtime ownership;
- startup/periodic reconciliation;
- terminal event/projection consistency.

**Out of scope:** Temporal/Celery/distributed workflow engine or transparent continuation of an interrupted model token stream.

**Protected contracts:** transport disconnect is not cancellation; explicit cancel remains distinct; durable event ordering remains monotonic.

**Decision:** implement a small **execution lease**, not a new Job runtime for chat inference.

**Implementation:**

1. Add lease owner/expiry/heartbeat fields (or an equivalent lease table) for actively executing runs via a new migration.
2. Acquire the lease atomically when a Run transitions into active model execution.
3. Heartbeat only while the owning process is alive.
4. On normal completion/cancel/failure, atomically terminate lease and clear conversation active-run state.
5. On startup and periodically, find `running` runs with expired/missing ownership.
6. Transition them to an explicit terminal/recoverable failure reason such as `execution_lost`; append a durable public-safe event; clear `active_run_id`.
7. Preserve `waiting_user` semantics separately: a persisted HITL wait is not a lost execution merely because no process owns a live HTTP stream.
8. Test completion-vs-reconciler, cancellation-vs-reconciler and double-reconciler races.

**Tests:**

```bash
go test ./apps/api/internal/consultation ./apps/api/internal/service ./apps/api/internal/repository -count=1
go test -race ./apps/api/internal/consultation ./apps/api/internal/service ./apps/api/internal/repository -count=1
```

**Acceptance:** simulated process death followed by startup reconciliation produces a deterministic terminal/recoverable Run and allows the next user Run; no terminal race creates duplicate/conflicting final events.

**Dependencies:** BS-PROD-000.

**Risks:** overly aggressive leases can kill legitimate long inference; expiry must exceed heartbeat jitter and be covered by slow-call tests.

---

## BS-PROD-011 — Add user recovery controls and deploy-aware Run drain/defer

**Implementation status (2026-08-24): VALIDATED / LOCAL PRODUCTION-SHAPED GATES GREEN.** Web exposes explicit cancel and durable recovery, stale/null thread seeds cannot erase a newer local terminal event, and a provisional `new` assistant runtime promotes to the server-authoritative conversation runtime only after the refetched thread proves the latest assistant message is terminal. A real Docker API-process restart now yields durable `run.failed` + `message.failed`, `message.status=failed`, `error.code=execution_lost`, a visible “本次执行已安全停止 / 系统已安全回收” state, and an unlocked composer. `scripts/local-deploy-validate.sh` passes all 5 browser flows and reports `LOCAL_DEPLOY_VALIDATION=PASS`. `scripts/validate-deploy-run-preflight.sh` proves valid live leases defer deploy, `waiting_user` and expired leases do not block, and an unverifiable database fails closed to `DEFER`.

**Goal:** users and the production deployer interact cleanly with active Runs instead of relying on hidden five-minute polling timeouts.

**Why now:** the backend has explicit cancel semantics, but the Web does not expose them and the deploy watcher currently recreates services without considering active inference.

**Scope:**

- React active-turn actions;
- Consultation API cancel client;
- durable recovery UX;
- deploy watcher active-run preflight.

**Out of scope:** guaranteeing uninterrupted model continuation across process restart.

**Protected contracts:** cancellation is explicit; transport loss remains recoverable; waiting-user HITL state survives deploy.

**Implementation:**

1. Add typed `cancelRun(runId)` client method and user-visible cancel action while a cancellable Run is active.
2. When durable recovery observes `execution_lost`, offer a clear retry/restart action rather than silent polling exhaustion.
3. Make recovery timeout/error state explicit and preserve replayed text/events already persisted.
4. Add a production watcher preflight against **actively executing** leases/runs.
5. If a valid running execution exists, defer the automated deploy and let the watcher retry on its next schedule; do not kill it silently.
6. Do not block deployment merely because a durable `waiting_user` HITL interaction exists.
7. Add browser E2E for cancel and recover-after-restart.

**Tests:**

```bash
pnpm --filter web test
pnpm e2e
bash scripts/production-deploy-watch.sh --check-only
```

**Acceptance:** user can explicitly cancel; stale execution becomes recoverable; watcher demonstrates defer behavior for active inference and normal deploy behavior when no live lease exists.

**Dependencies:** BS-PROD-010.

**Risks:** calling production watcher in tests must remain check-only/local; never mutate production from CI.

---

## BS-PROD-012 — Establish off-host PostgreSQL disaster recovery and restore drill

**Implementation status (2026-08-24): IMPLEMENTED AND REVIEW-HARDENED (3rd hardening pass — review fixes folded in, hermetic gates green (46/46); the docker-backed integration test was updated for drill-network creation, run-unique container naming and the validator reachability alias, but NOT re-run here because docker is unavailable in this sandbox); committed on the off-host DR feature branch, NOT YET MERGED TO `main`; production execution is operator-only and not yet performed.** `scripts/offhost-s3.py` (stdlib-only SigV4 client) is verified against the AWS-documented known-answer signature and a frozen-clock botocore cross-check, with 17 stdlib tests including a signature-verified fake S3 server and a guard that refuses command-line credentials (env-only). `scripts/production-offhost-backup.sh` (daily custom-format dump + SHA-256 + metadata upload, end-to-end round-trip verification, prefix-scoped retention that keeps the newest day directory, strict last-success freshness state with hourly alerting, secrets only in `.env.production.local` and passed to the client via environment, never argv) and `scripts/restore-production-backup.sh` (mandatory `--restore-pg container:<id|name>` whose isolation is proven fail-closed via `docker inspect` before anything else — refuses container-ID equality with the production postgres container, membership in the production Compose project, host/`none` networking and any Docker network shared with the production postgres container (network enumeration is fail-closed: an inspection/parsing failure is a refusal, never an empty 'isolated' result), a target attached to any network beyond its declared `bodysense.restore-network` (the dedicated drill network must be the container's **sole** network), any published host port (`HostConfig.PortBindings`, making the target host-reachable), a target that is not attached to its declared `bodysense.restore-network=<dedicated non-host drill network>`, a non-running target, and any target that does not declare `bodysense.restore-project=<--target-project>` + `bodysense.disposable-restore=yes`; strict SHA-256 sidecar verification: format, attested filename and digest must match the metadata `checksum_sha256`, and the archive must match both; `pg_restore --list` validation; disposable-database restore on the disposable server; fail-closed schema-revision gates on both sides — the backup only records a success with a revision verified from `schema_migrations` (no table/unreadable/empty ⇒ abort), restore metadata declaring `unknown`/`uninitialized`/empty is refused up front, and post-restore schema-revision equality is always enforced; `migration-validator`/`domain-validator` execution with the database password supplied only via `PGPASSWORD` in the process environment — `--env-file` on the `docker exec` path or inherited for golang — never in `-database-url` or any argv) are written. The restore path resolves the validator api container from the running Compose project (production names it `<project>-api-1`, e.g. `docker-api-1`, not a literal `api`), with an `OFFHOST_API_CONTAINER` override for operators/tests. `scripts/validate-offhost-dr-unit.sh` proves backup/retention/freshness/restore-isolation (ID, running, compose-project, only-declared-drill-network, no-published-host-ports, shared-network-exclusivity and disposable-label refusals)/sidecar-verification/DB-password-argv-leak/solved-validator-container/timer-timezone behavior (46/46) against stubbed PostgreSQL and fake docker; `scripts/validate-offhost-dr.sh` (real PostgreSQL 18 + real pg_dump/restore + real validators on a container **not** named `api`, creates its dedicated drill network, restores into a second disposable `restore-pg` container on that drill network — names are run-unique (`$$`) so cleanup never touches unrelated containers, and the production postgres container carries a `postgres` network alias so the validator container can reach it by DNS — with the disposable labels, including a data round-trip probe) is written for docker-capable CI. systemd units, runtime-bundle wiring, env contract, and the operator runbook are in place. **Not yet performed: integrated delivery (a reviewed PR merged into `main`; the exact merged revision is not `main` today and post-merge checks at that revision are not evidenced), real deployment to the production host, a real OSS object-store run, and the acceptance drill against a live backup** (deliberately operator-only/external; DoD/acceptance below stay UNCHECKED). Review findings on the runtime policy contract were addressed and are covered by hermetic tests: retention is **apply-or-fail** (a failed off-host object listing aborts the backup before `last-success.json` is recorded), freshness is enforced in **whole seconds** (no whole-hour truncation; a future-dated last-success is rejected as `future-dated-last-success`, never treated as fresh), the restore isolation proof and the validator credential flow are hardened as described above, and both systemd timers embed the timezone in their **`OnCalendar=`** expressions (`*-*-* 02:10:00 Asia/Shanghai` / `*-*-* *:00:00 Asia/Shanghai`) so the documented 02:10 schedule is independent of host TZ without the non-standard `[Timer] Timezone=` directive. The integrated revision (including this hardening) is committed on the off-host DR feature branch but has **not** been merged into this repository's `main`: `main`/`origin/main` remain at the BS-PROD-011 runtime-closeout merge, and the sandbox that produced this revision has no reachable origin, so neither the reviewed-PR delivery nor the exact-merge-revision post-merge check can be performed from here — both stay outstanding.

**Goal:** production database can be restored after complete loss of the Alibaba ECS disk.

**Why now:** current pre-deploy `pg_dump` protects releases but shares the same failure domain as PostgreSQL.

**Scope:**

- Alibaba OSS or compatible object-storage backup target;
- encrypted/credential-scoped backup upload;
- retention;
- checksum and archive validation;
- restore script/runbook;
- restore rehearsal on an isolated database.

**Out of scope:** full multi-region HA or zero-RPO synchronous replication.

**Decision:** keep the existing validated local pre-deploy dump for rollback, and add a separate scheduled off-host disaster-recovery backup. Do not conflate them.

**Implementation:**

1. Add an operator-owned backup script that produces custom-format `pg_dump`, SHA-256, metadata and schema revision.
2. Upload to a private OSS/S3-compatible bucket using minimum-scope credentials stored only on the production host/secret manager.
3. Add retention/lifecycle policy independent from the local 14-day deployment backup retention.
4. Add `restore-production-backup.sh` (or equivalent) that verifies checksum and `pg_restore --list` before restore.
5. Restore the newest backup into a disposable database/container and run schema/domain validators.
6. Schedule recurring backups separately from release deployment so “no deploys” does not mean “no backups”.
7. Record last-success/age metrics and alert when backup freshness exceeds policy.

**Tests / verification:**

```text
backup object exists off-host
checksum PASS
pg_restore --list PASS
restore into fresh PostgreSQL PASS
schema_migrations matches expected clean version
domain validator PASS
```

**Acceptance:** documented restore drill succeeds without reading the source ECS database/volume after backup object retrieval.

**Dependencies:** Phase 0 security baseline.

**Risks:** backups contain sensitive health data; bucket must be private, encrypted and access-scoped; never upload secrets/runtime env files with DB dump artifacts.

---

## BS-PROD-013 — Move user uploads behind a durable object-storage abstraction

**Goal:** user reports/posture images survive API container/ECS filesystem loss and retain owner-scoped access/deletion semantics.

**Why now:** `api-uploads` is a Docker named volume on the production host and is not an independent durable store.

**Scope:**

- UploadStorage interface/adapter;
- OSS/S3-compatible production adapter;
- local filesystem dev/test adapter;
- metadata/object-key migration;
- OCR/Posture worker reads;
- privacy deletion.

**Out of scope:** public CDN delivery of private health uploads.

**Protected contracts:** upload ownership, MIME/content validation, 10 MiB limit, OCR/Posture jobs, private access only.

**Implementation:**

1. Define a narrow `UploadStorage` port: put/read/delete/stat by opaque owner-scoped object key.
2. Keep local filesystem adapter for development/tests.
3. Add Alibaba OSS/S3-compatible production adapter with private objects and no public bucket ACL.
4. Store object key rather than host-specific absolute path for new uploads.
5. Update OCR/Posture jobs to read through the storage port; do not leak bucket credentials into Python if Go can stream the file safely.
6. Provide a one-shot migration for existing production upload rows/volume objects if any exist at cutover.
7. Make delete/erasure remove the object idempotently.
8. Add integration tests for upload → job read → delete using an S3-compatible test fixture or isolated object-store emulator.

**Acceptance:** production API can be rebuilt on an empty host filesystem and still retrieve/process uploaded objects from object storage; owner isolation tests remain green.

**Dependencies:** BS-PROD-005 semantics should be agreed before final deletion behavior; can develop in parallel with BS-PROD-012.

**Risks:** object-store network failures become a new dependency; classify retriable vs terminal errors and do not mark DB metadata complete before durable object write succeeds.

---

## BS-PROD-014 — Harden production resource and observability baseline

**Goal:** the small Alibaba host fails predictably instead of relying on unbounded container memory and zero swap.

**Why now:** audit observed ~1.6 GiB RAM, no swap and low free headroom, with LiteLLM the largest resident service.

**Scope:**

- host swap policy;
- Compose resource limits/reservations where supported;
- health/log/backup/run-liveness alert signals;
- disk/image/volume retention.

**Out of scope:** migrating production to a new host class unless measurement proves required.

**Implementation:**

1. Capture 24h/representative memory baseline if existing host telemetry is available; otherwise establish conservative limits from current observed usage plus headroom.
2. Configure a bounded swap file (target 2–4 GiB unless host policy dictates otherwise) as an OOM shock absorber, not normal working memory.
3. Add reasonable service memory limits/reservations; avoid limits below observed peaks.
4. Add alerts/checks for memory pressure, swap churn, disk pressure, container restarts/OOM, stale deploy watcher, stale backups and stale Run reconciliation.
5. Add bounded Docker/image/runtime-backup cleanup that cannot remove active images or required rollback artifacts.
6. Document upgrade trigger thresholds for RAM rather than guessing from one sample.

**Acceptance:** resource policy is versioned/documented; simulated memory/disk warning checks are observable; no service is accidentally constrained below validated production-shaped load.

**Dependencies:** BS-PROD-010/012 signals may feed observability.

**Risks:** Compose resource limits behave differently across runtimes; validate on the actual production Docker Compose implementation before release.

---

## BS-PROD-020 — Add explicit Knowledge operator authorization and source registry

**Goal:** global Knowledge administration is no longer available to any ordinary authenticated member, and every source has durable provenance/license identity before ingestion.

**Why now:** a real production corpus must not be built on an unowned global mutation API.

**Scope:**

- bounded user/operator role or capability;
- middleware/policy for Knowledge admin routes;
- source registration/list/status operations;
- source uniqueness/provenance/license metadata.

**Out of scope:** a generalized enterprise permission platform.

**Decision:** implement the smallest durable role/capability model that makes Knowledge administration explicit and testable. Do not rely on “hidden route” security.

**Implementation:**

1. Add a new migration/model field or capability table for an explicit `member` vs `operator/admin` authority.
2. Add reusable Go authorization middleware/policy.
3. Restrict Knowledge ingestion/mutation/stats as appropriate; keep online search callable only through intended Agent/internal paths.
4. Implement `KnowledgeSourceRegistry` around existing normalized source schema.
5. Enforce stable source identity/content hash and license/provenance status before ingestion.
6. Add two-user tests proving ordinary users cannot mutate global Knowledge.
7. Record operator identity in lifecycle/audit actions.

**Acceptance:** unauthenticated and ordinary member principals cannot register/ingest/publish/rollback global Knowledge; operator actions retain actor provenance.

**Dependencies:** Phase 0 auth/session work.

**Risks:** avoid mixing authorization role with clinical authority; Knowledge operator rights permit content lifecycle operations, not user health-data access by default.

---

## BS-PROD-021 — Move Knowledge ingestion onto the durable JobRuntime boundary

**Goal:** source ingestion is recoverable/auditable and not a long synchronous request hidden behind an admin endpoint.

**Why now:** source registry establishes identity; ingestion can now run against a registered source with a durable job lifecycle.

**Scope:**

- `knowledge.ingest_*` job type;
- Go JobRuntime orchestration;
- Python ingestion worker/API contract;
- generated/curated artifacts and Agent configuration provenance.

**Out of scope:** publication; handled in BS-PROD-022.

**Protected contracts:** Python performs ASR/split/curate/embedding computation; Go owns job/user/operator audit and publication authority.

**Implementation:**

1. Define immutable ingestion input containing `source_id`, source revision/content hash, requested pipeline/config IDs and operator identity.
2. Reuse JobRuntime idempotency/recovery patterns already used by OCR/Posture rather than creating a second job engine.
3. Split durable job states into observable stages without persisting huge intermediate blobs in public events.
4. Validate Python Knowledge Splitter/Curator returned Agent identities against Go-selected repository-known configuration IDs.
5. Make retry finite and stage-aware; failed jobs must not promote generated content into published visibility.
6. Persist artifact/source lineage required for later review.
7. Add restart/retry/idempotency tests.

**Acceptance:** ingestion can be interrupted/retried without duplicate source/unit corruption and produces only non-published lifecycle artifacts.

**Dependencies:** BS-PROD-020.

**Risks:** media jobs can be resource-heavy on the small production host; allow ingestion execution to remain an operator-controlled/offline workload rather than competing with user inference.

---

## BS-PROD-022 — Implement review, quality gate, publication and rollback

**Goal:** only explicitly reviewed/qualified Knowledge can become visible to production RAG, and every publication can be rolled back without destroying provenance.

**Why now:** ingestion alone creates data but not trust.

**Scope:**

- review status transitions;
- deterministic quality checks;
- embedding/version consistency;
- `knowledge_publications` workflow;
- publish/rollback audit;
- operator CLI or minimal internal UI.

**Out of scope:** public end-user editing of Knowledge.

**Protected contracts:** generated content never becomes trusted merely because the LLM curator produced it; publication is Go/operator authority.

**Implementation:**

1. Encode valid lifecycle state transitions and reject illegal skips such as generated → published.
2. Add deterministic checks for source/license/provenance/evidence excerpt/content hash/embedding revision completeness.
3. Require explicit review decision for units entering the publishable set.
4. Create a publication batch identity containing exact unit revisions and embedding/config fingerprints.
5. Publish atomically so online search never sees a partially switched batch.
6. Implement rollback/deprecation as visibility/state transition, not destructive deletion.
7. Record operator, timestamp, reason, source/config identities.
8. Add tests proving unreviewed, rejected, deprecated and rolled-back units are excluded from default search.

**Acceptance:** one controlled source can move from registered to published and back to rolled-back/deprecated with complete audit/provenance and no visibility leakage.

**Dependencies:** BS-PROD-020, BS-PROD-021.

**Risks:** over-automated quality score thresholds can create false trust; deterministic minimum gates do not replace human review for the initial production corpus.

---

## BS-PROD-023 — Qualify retrieval and publish the initial production Knowledge corpus

**Goal:** production RAG becomes non-empty only after measurable retrieval quality and citation provenance are demonstrated.

**Why now:** the repository already contains curated source artifacts, but copying them directly into online tables would bypass the lifecycle just implemented.

**Scope:**

- retrieval eval dataset;
- recall/precision/ranking/citation checks appropriate to current corpus;
- initial governed source registrations;
- review/publication of selected curated units;
- production readiness check.

**Out of scope:** expanding to a large web-scale corpus.

**Implementation:**

1. Inventory the six existing curated source JSON artifacts and verify provenance/license metadata; do not assume all are publishable.
2. Create a small reviewed retrieval qualification set containing expected relevant/irrelevant queries and safety-sensitive evidence cases.
3. Predeclare acceptance thresholds before evaluating the candidate corpus.
4. Register sources through the new SourceRegistry.
5. Ingest/import through the governed lifecycle; review units explicitly.
6. Run retrieval qualification against the exact publication candidate.
7. Publish only passing units/batch.
8. Verify production `search_knowledge` returns published results with source/unit/publication identity and stable citation excerpts.
9. Record publication ID and qualification report as deployment evidence.

**Acceptance:** production has `published knowledge > 0`; qualification report passes predeclared thresholds; all online results resolve to a published batch and source provenance.

**Dependencies:** BS-PROD-022.

**Risks:** low-quality or unclear-license artifacts must be rejected rather than published merely to satisfy the non-empty metric.

---

## BS-PROD-024 — Make empty/unavailable/partial Knowledge behavior explicit in Diagnosis and Treatment governance

**Goal:** evidence-dependent Agent outputs do not silently look equally authoritative when production Knowledge is empty, unavailable, or insufficient.

**Why now:** production currently works with an empty RAG corpus, proving that “service healthy” and “evidence available” are different states.

**Scope:**

- Diagnosis/Treatment evidence-gap semantics;
- retrieval provenance completeness;
- grounding/faithfulness policy behavior;
- UI status where evidence availability materially changes delivery.

**Out of scope:** requiring external Knowledge for claims that are purely a restatement of user facts or deterministic safety rules.

**Protected contracts:** deterministic safety gates remain independent of RAG availability; lack of external evidence must not suppress emergency/safety escalation.

**Implementation:**

1. Classify output claims/policies into user-fact supported, deterministic-policy, and external-knowledge-dependent categories.
2. For external-knowledge-dependent reasoning, preserve explicit `EvidenceGap` when retrieval is unavailable/empty/insufficient.
3. Ensure the Go delivery authority can distinguish “no evidence needed” from “evidence expected but absent”.
4. Extend grounding/eval fixtures for empty corpus, database unavailable, irrelevant results, contradictory result, and sufficiently supported result.
5. Ensure UI does not present evidence-backed language/citations when no qualifying evidence exists.
6. Keep thresholds/promotion policy versioned; do not silently switch Treatment Grounding v2 into production without qualification.

**Acceptance:** the same case produces auditable different evidence status when Knowledge is available vs unavailable, without changing deterministic safety behavior; no fabricated citation/evidence provenance is possible.

**Dependencies:** BS-PROD-023; some eval work can begin earlier.

**Risks:** over-gating can make the product unusable; policy should gate only claims that actually require external support.

---

## BS-PROD-030 — Re-evaluate and decide Diagnosis Challenger promotion

**Goal:** make an evidence-based decision on the `diagnosis-decision-policy-v1` Challenger after the production evidence plane is real.

**Why now:** the current v1 Champion remains deliberately pre-envelope for compatibility; promotion should happen through governance, not architecture enthusiasm.

**Scope:**

- current Champion vs Challenger replay/shadow comparison;
- risk-stratified cases;
- hard guardrails and non-inferiority criteria;
- explicit promote/hold/reject record.

**Out of scope:** creating a new model/prompt/config unless evaluation reveals a separate defect requiring its own Challenger.

**Implementation:**

1. Freeze the exact Champion and Challenger identities and qualification dataset fingerprint.
2. Add newly reviewed regression cases from production-shaped behavior only after privacy sanitization/review.
3. Run historical/counterfactual replay and shadow validation.
4. Compare hard outcomes first: safety block/escalation, forbidden side effects, configuration identity, evidence policy.
5. Compare structured semantic outcomes within predeclared margins.
6. If evidence is sufficient, run bounded canary with sticky assignment and risk strata; otherwise record `HOLD — insufficient production evidence`.
7. Promote only through the existing Go deployment policy pointer and repository-known identity.

**Acceptance:** a dated governance record explains promote/hold/reject using predeclared thresholds; there is no silent config pointer change.

**Dependencies:** Phase 2 production Knowledge/evidence behavior should be stable; may remain HOLD if real production sample remains too small.

**Risks:** sample-size pressure can encourage premature promotion; “not enough evidence” is an acceptable outcome.

---

## BS-PROD-031 — Triage dependency advisories and establish a bounded supply-chain gate

**Goal:** remove runtime-relevant known advisories and make remaining advisory debt explicit/reviewable without treating every build-tool advisory as a production exploit.

**Why now:** `pnpm audit --prod` currently returns 26 advisories / 14 high, while GitHub Actions are already correctly SHA-pinned.

**Scope:**

- JS dependency graph first;
- runtime vs dev/build classification;
- safe upgrades/overrides;
- CI policy with reviewed exceptions.

**Out of scope:** blindly failing every build on every transitive advisory regardless of reachability.

**Implementation:**

1. Capture exact advisory list and dependency paths.
2. Classify each as shipped browser/runtime, build-only, test-only, or unreachable under current usage.
3. Upgrade direct dependencies and lockfile where compatible.
4. Use package overrides only when semver/runtime compatibility is understood and tests cover the affected package.
5. Create a machine-readable allowlist only for time-bounded, justified non-runtime advisories.
6. Add CI audit/report gate that fails newly introduced unreviewed high/critical runtime advisories.
7. Keep GitHub Actions immutable SHA pinning and Dependabot update flow.

**Tests:**

```bash
pnpm audit --prod
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```

**Acceptance:** no unreviewed high/critical runtime-reachable advisory remains; exceptions have reason/expiry; CI detects new regressions.

**Dependencies:** can run in parallel after Phase 0.

**Risks:** aggressive lockfile changes can create wide regression noise; keep upgrades reviewable and isolated from domain changes.

---

## BS-PROD-032 — Converge authoritative architecture and operations documentation

**Goal:** documentation stops causing topology/status mistakes and clearly separates current truth from historical target designs.

**Why now:** current deployment doc still identifies Oracle2 as development even though GCP-dev is canonical; historical partial-status percentages are easy to misread as current system state.

**Scope:**

- `docs/architecture/deployment-architecture.md`;
- `docs/architecture/README.md`;
- `docs/architecture/current-longitudinal-system.md`;
- Knowledge current-status document;
- runbook/active-plan index;
- historical status banners.

**Out of scope:** rewriting every archived plan.

**Implementation:**

1. Correct topology to GCP-dev → GitHub/ACR → Alibaba and explicitly mark Oracle2 detached.
2. Define a small set of `Current source of truth` documents.
3. Move stale implementation percentages into historical notes or update them from real code evidence.
4. Mark superseded target documents clearly at the top if they contain obsolete “current” statements.
5. Update Knowledge Lifecycle status only after Phase 2 evidence is complete.
6. Add links from Active Plan README to this program and from closure docs to the archived plan when complete.
7. Run a repository search for active references to DigitalOcean/Oracle2 BodySense runtime and classify each as archive vs stale.

**Acceptance:** a new engineer can identify current production topology, current domain ownership, current Agent/provider boundary and current unfinished program without consulting chat history.

**Dependencies:** factual corrections can start early; final status update happens after BS-PROD-034.

**Risks:** avoid editing historical records to pretend past decisions never existed; archive preserves history while current docs state present truth.

---

## BS-PROD-033 — Production-shaped closeout review and focused repair passes

**Goal:** review completed batches against contracts, not ticket descriptions, and repair adjacent regressions before release.

**Why now:** this program crosses auth, runtime, storage, Knowledge and deployment boundaries; green unit tests alone are insufficient.

**Scope:** full batch review across five layers: contracts, vertical completeness, behavior, engineering quality, plan integrity.

**Implementation:**

1. Re-run the original finding ledger and mark each `fixed / partially fixed / not reproduced / deferred with reason / new regression`.
2. Cross-user authorization sweep for all protected routes.
3. Browser auth/session/delete/share flows.
4. Run crash/cancel/replay/HITL/restart flows.
5. Backup/restore and upload-object-store flows.
6. Knowledge register/ingest/review/publish/rollback/search flows.
7. Diagnosis/Treatment evidence and promotion governance review.
8. Convert any verified findings into focused repair passes by root cause; do not mix unrelated cleanup.
9. Require narrow tests first, then repository-wide verification.

**Acceptance:** no P0/P1 remains open; all P2 deferrals are explicit and do not violate a protected contract.

**Dependencies:** implementation tickets substantially complete.

**Risks:** avoid scope creep into new product features during review.

---

## BS-PROD-034 — Release, Alibaba production validation and archive

**Goal:** close the plan only after the repaired system is proven through the normal release pipeline and real production smoke checks.

**Why now:** local success is not closure for deployment/session/RAG/DR work.

**Scope:** full release gate, PR/release workflow, production deployment, post-deploy validation, plan archival.

**Implementation:**

1. Run focused checks for all changed packages.
2. Run repository release validation:

```bash
pnpm verify:release
```

3. Run production-shaped local Docker validation:

```bash
pnpm validate:local-deploy
```

4. Review diff hygiene and migration checksums:

```bash
git diff --check
```

5. Merge only through protected PR flow and required checks.
6. Let release-please create the release through the existing successful-main-CI gate.
7. Verify ACR Web/API/AI/runtime images share the release revision.
8. Verify Alibaba watcher deploy state and public health.
9. Execute post-deploy synthetic checks without exposing secrets:
   - login/refresh/logout/revocation;
   - cross-user authorization fixture where safe;
   - share/revoke/delete;
   - Consultation normal/cancel/recovery;
   - Knowledge published retrieval/citation;
   - backup freshness/object-store reachability.
10. Confirm no deploy block, unexpected restart/OOM or stale Run exists.
11. Update current architecture docs with final implemented facts.
12. Mark every ticket closed with evidence, move this file to `docs/plan/archive/`, and restore `docs/plan/active/README.md` to the true remaining-active state.

**Acceptance:** normal CI/release pipeline is green, Alibaba is healthy on the released revision, production smoke/DR/RAG evidence passes, and the plan is archived with no open P0/P1.

**Dependencies:** BS-PROD-033.

**Risks:** production smoke must not create persistent synthetic health data without cleanup; use dedicated test identity or explicitly removable fixtures.

---

# 7. Ticket dependency graph

```text
BS-PROD-000
  ├─ BS-PROD-001 authorization
  ├─ BS-PROD-002 share/delete
  │    └─ BS-PROD-005 privacy erasure
  ├─ BS-PROD-003 session authority
  │    └─ BS-PROD-004 browser/rate/CSP
  │         └─ BS-PROD-020 Knowledge operator auth
  └─ BS-PROD-010 execution lease
       └─ BS-PROD-011 Web recovery + deploy drain

BS-PROD-005 ───────────────┐
BS-PROD-012 DB DR           │
BS-PROD-013 upload storage ─┴─ Phase 1 resilience gate
BS-PROD-014 capacity hardening

BS-PROD-020
  └─ BS-PROD-021 ingestion job
       └─ BS-PROD-022 review/publish/rollback
            └─ BS-PROD-023 retrieval qualification + initial publication
                 └─ BS-PROD-024 evidence availability governance
                      └─ BS-PROD-030 Diagnosis promotion decision

BS-PROD-031 supply chain ───┐
BS-PROD-032 docs ───────────┼─ BS-PROD-033 batch review/repair
all previous tickets ───────┘
                              ↓
                         BS-PROD-034 release/production/archive
```

Parallelism is allowed only when ownership boundaries do not overlap. In particular:

- BS-PROD-001 and BS-PROD-002 can run in parallel after baseline tests.
- BS-PROD-003 should establish the session model before BS-PROD-004 rewrites browser storage/cookies.
- BS-PROD-012 and BS-PROD-013 can run in parallel after privacy/storage semantics are clear.
- BS-PROD-031 can run largely independently, but its lockfile changes should not be mixed into security/runtime behavior commits.

---

# 8. Verification matrix

| Area | Focused evidence | Wider evidence | Production evidence |
| --- | --- | --- | --- |
| Authorization | two-user handler/service tests | Go full suite | synthetic owner/non-owner checks where safe |
| Share/privacy | share/delete DTO tests | Web + Go tests | public token becomes 404 after revoke/delete |
| Sessions | auth/middleware race tests | Web E2E + Go suite | login → refresh → logout → old token rejected |
| Run liveness | lease/reconciler race tests | longitudinal E2E | deploy/restart does not strand `RUN_IN_PROGRESS` |
| DB durability | backup/restore script tests | local PG16 restore/domain validator | off-host backup freshness + isolated restore drill |
| Uploads | storage adapter integration tests | OCR/Posture job tests | object remains available independent of API filesystem |
| Knowledge auth | member/operator tests | Go/API suite | ordinary user cannot mutate global Knowledge |
| Knowledge lifecycle | state-transition/job tests | AI/Go integration | real publication ID + rollback/search checks |
| Retrieval | qualification dataset | repository eval gate | published corpus non-empty and citations traceable |
| Governance | replay/shadow eval | release qualification | dated promote/hold/reject record |
| Supply chain | advisory triage | lint/typecheck/test/build | no unreviewed runtime high/critical advisory |
| Deployment | watcher check-only | `validate:local-deploy` | Alibaba coherent revision + health + no block |

Repository-wide final commands remain:

```bash
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm verify:release
pnpm validate:local-deploy
git diff --check
```

Run focused package/tests before these broad commands on every ticket.

---

# 9. Rollout and rollback policy

## 9.1 Security/session changes

- Do not combine session-model migration with unrelated UI refactors.
- Preserve a rollback path for server/client compatibility during the release window.
- If cookie/session migration fails production smoke, rollback application images only if schema compatibility permits; otherwise use forward repair with the migration contract preserved.

## 9.2 Run lease changes

- Lease fields are additive.
- Reconciler initially logs/metrics candidate stale runs in validation before enabling mutation if needed for safe rollout.
- Do not automatically retry semantic model execution after process loss unless idempotency/side-effect boundaries prove it is safe; first goal is **unlock + explicit recoverability**.

## 9.3 Storage/backup changes

- Local DB backup remains until off-host DR is proven.
- Local upload adapter remains available for development; production cutover uses explicit configuration.
- Do not delete/migrate original production upload volume objects until object-store copy and metadata verification pass.

## 9.4 Knowledge changes

- Initial publication is a new publication batch, never direct table-state editing.
- Rollback changes online visibility while retaining source/unit/publication provenance.
- Empty Knowledge is safer than bypassing review. A failed qualification must leave production corpus empty/previously published rather than force publication.

## 9.5 Diagnosis promotion

- Promotion is independent from code merge.
- A green Challenger implementation may remain unpromoted indefinitely if production evidence is insufficient.

---

# 10. Risk and decision ledger

| Topic | Current decision | Revisit trigger |
| --- | --- | --- |
| Auth storage | secure refresh/session cookie + memory access token | browser/API topology changes away from same-origin |
| Session authority | explicit session family, separate from user-existence cache | introduction of external IdP/SSO |
| Run recovery | lease + fail/unlock, not transparent inference continuation | product requires guaranteed long-running inference across host restart |
| DB DR | periodic encrypted off-host logical backup + restore drill | RPO/RTO requirements demand PITR/replication |
| Upload storage | private OSS/S3-compatible object storage | CDN/public asset requirements emerge |
| Knowledge admin | bounded operator/admin capability | multiple organizations/tenant-specific roles are introduced |
| Knowledge publication | human review + deterministic gates | calibrated automated review evidence justifies partial automation |
| RAG empty state | explicit evidence gap/degrade, never fabricated support | domain policy proves a stronger fail-closed rule is required for a specific output class |
| Diagnosis Challenger | hold until predeclared evidence supports promotion | enough reviewed production-shaped cases accumulate |
| Host sizing | harden current Alibaba host first | sustained memory/swap/latency thresholds exceed policy |

Open questions that **do not block starting Phase 0**:

1. Exact short access-token TTL (the implementation should choose and test one bounded production value rather than preserve 168h).
2. Exact Alibaba OSS bucket/retention configuration; storage interface and restore contract can be implemented independently.
3. Exact retrieval qualification thresholds; they must be predeclared before BS-PROD-023 candidate evaluation.
4. Whether Diagnosis Challenger receives a canary in this program or remains HOLD due to sample size.

---

# 11. Review / repair protocol

Every implementation batch follows:

```text
implement bounded ticket(s)
        ↓
focused tests
        ↓
batch review against protected contracts
        ↓
classify findings P0/P1/P2/P3
        ↓
focused repair pass by root cause
        ↓
wide repository validation
        ↓
PR + required CI
```

Review must verify five layers:

1. **Contract correctness** — auth/session/ownership, schema, routes, StreamEvent identity, lifecycle state machine.
2. **Vertical completeness** — Web/API/runtime/persistence/ops paths connect end-to-end.
3. **Behavioral completeness** — create, delete, revoke, retry, cancel, refresh, restart, empty/error states.
4. **Engineering quality** — single owner, no duplicate authority, race safety, typed validation, observability, clean diffs.
5. **Plan integrity** — acceptance evidence exists and current docs reflect implemented facts.

A green build alone does not close a finding.

---

# 12. Definition of done

This plan may be archived only when all of the following are true:

- [ ] `PROD-SEC-01`, `PROD-PRIV-01`, `PROD-AUTH-01`, `PROD-RUN-01`, `PROD-DR-01`, `PROD-KNOW-01` are closed with evidence.
- [ ] No open P0/P1 remains from the final batch review.
- [ ] ordinary users cannot administer global Knowledge.
- [ ] delete/revoke/session semantics match user-visible language.
- [x] stale Run/process loss cannot permanently lock a conversation.
- [ ] PostgreSQL has an independently restorable off-host backup.
- [ ] user uploads no longer depend on one ECS filesystem for durability.
- [ ] production has a reviewed/qualified/published Knowledge corpus or the plan records a deliberate no-publication decision because qualification failed; it must never bypass governance merely to become non-empty.
- [ ] Diagnosis promotion has an explicit `PROMOTE`, `HOLD`, or `REJECT` governance record.
- [ ] dependency high/critical runtime findings are repaired or time-bounded with documented justification.
- [ ] Gin/proxy/CSP/resource production baseline is explicit and validated.
- [ ] `pnpm verify:release` passes.
- [x] `pnpm validate:local-deploy` passes.
- [ ] required GitHub PR/CI checks pass.
- [ ] Alibaba production deploy is healthy on one coherent immutable revision.
- [ ] authoritative docs name GCP-dev as development/ops, Alibaba as sole production, and Oracle2 as detached.
- [ ] closure evidence is written into this document before it moves to archive.

Until those conditions are met, this file remains the canonical active implementation plan for the post-v0.5.2 production closeout.
