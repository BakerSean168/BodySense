# TECH SCHOOL Backend Master Class -> BodySense 1:1 Lecture Parity

> Source baseline: `techschool/simplebank` public README checked 2026-09-07.
> Coverage: backend lectures #0 through #77. No lecture is silently omitted.
> Translation rule: preserve the engineering problem, use BodySense's current production choice unless the exercise is explicitly a comparison.

Legend:

- `DIRECT` — exercise directly against BodySense.
- `COMPARE` — compare the source technology with the current BodySense choice; do not force migration.

## Lecture #0 — Environment

| Source | Mode | BodySense rep |
|---|---|---|
| #0 | DIRECT | Reconstruct the BodySense local toolchain from repository manifests: WSL/Linux shell, Go, Node/pnpm/Nx, Python/uv, Docker and editor integration. Start only the minimum services needed for one traced request and explain why each process exists. |

## Section 1 — PostgreSQL and transactions

| Source | Mode | BodySense rep |
|---|---|---|
| #1 | DIRECT | Reverse-engineer a BodySense domain slice into an ER diagram; verify primary keys, foreign keys, ownership and cardinality against migrations/models. |
| #2 | DIRECT | Start PostgreSQL through BodySense dev infrastructure, inspect the real schema and connect with `psql` or an equivalent client. |
| #3 | DIRECT | Trace the migration chain, run safe migration validation in a disposable database, and explain up/down/baseline semantics. |
| #4 | COMPARE | Compare `database/sql`, sqlx/sqlc-style generated SQL and BodySense GORM/repository usage. Identify where ORM abstraction helps and where explicit SQL would be clearer. |
| #5 | DIRECT | Add or strengthen a repository/DB test using deterministic fixtures and explain isolation/cleanup. |
| #6 | DIRECT | Trace `TransactionManager` and one multi-repository mutation. Write the atomicity invariant before reading the implementation. |
| #7 | DIRECT | Identify the rows/resources that can be locked in one BodySense transaction. Construct a concurrent timeline that could block or deadlock. |
| #8 | DIRECT | Prove or refute a lock-ordering rule for the selected BodySense flow with a concurrency test or SQL-level experiment. |
| #9 | DIRECT | Run isolation/read-phenomena experiments on PostgreSQL and map dirty/non-repeatable/phantom/write-skew concepts to an actual BodySense mutation. |
| #10 | DIRECT | Read BodySense CI database lanes and explain how PostgreSQL-backed tests are selected, executed and made release-relevant. |

## Section 2 — REST API, validation, testing and authentication

| Source | Mode | BodySense rep |
|---|---|---|
| #11 | DIRECT | Trace one Gin REST endpoint from route registration through handler/service/repository and verify status/body semantics. |
| #12 | DIRECT | Trace configuration from environment/repository defaults into Go runtime. Identify startup-time validation and secret boundaries. |
| #13 | DIRECT | Test one handler/service with a substituted dependency. Explain mock vs fake vs real DB trade-offs rather than chasing an arbitrary coverage percentage. |
| #14 | DIRECT | Choose a BodySense mutation with meaningful invariants; write explicit request validation and test invalid boundary values. |
| #15 | DIRECT | Audit one table relationship for unique/FK/check constraints and prove the database rejects an impossible state. |
| #16 | DIRECT | Follow a database error from driver/GORM to repository/service/handler and verify the public error does not leak internal details. |
| #17 | DIRECT | Read the bcrypt password path and explain salt/cost/hash verification; add a focused regression test if a useful gap exists. |
| #18 | DIRECT | Build a stronger matcher/assertion for a request or repository call where naive equality would hide a bug. |
| #19 | COMPARE | Compare PASETO's design goals with BodySense JWT access tokens + opaque refresh credentials + Redis session authority. Decide what threats each design addresses. |
| #20 | DIRECT | Trace JWT creation and validation, claims, algorithm checks, TTL and session binding in BodySense. |
| #21 | DIRECT | Trace login end to end: credential check -> token issuance -> refresh credential persistence -> response -> frontend auth state. |
| #22 | DIRECT | Trace authentication middleware and resource authorization separately. Prove one request is rejected for identity failure and another for ownership/permission failure. |

## Section 3 — Containers and production delivery

| Source | Mode | BodySense rep |
|---|---|---|
| #23 | DIRECT | Read a BodySense Dockerfile and explain multi-stage build, build context, runtime image and artifact minimization. |
| #24 | DIRECT | Explain how BodySense containers/services reach each other and why host/bridge/container DNS addresses differ. |
| #25 | DIRECT | Read the dev/prod Compose topology and explain service ordering, health checks, persistence and restart behavior. |
| #26 | COMPARE | Map the source AWS account/bootstrap concerns to BodySense's actual cloud accounts. Focus on identity, quotas, billing boundaries and least privilege. |
| #27 | DIRECT | Trace artifact build/publish from GitHub Actions to BodySense's registry; identify immutable artifact identity and failure gates. |
| #28 | COMPARE | Compare managed production PostgreSQL/RDS concerns with BodySense's current production database: backups, networking, credentials, upgrades and recovery. |
| #29 | DIRECT | Audit how BodySense production secrets are injected. Verify no secret belongs in repository history or image layers. |
| #30 | COMPARE | Learn Kubernetes control-plane/workload concepts and map them to the simpler current BodySense deployment. State what scale/operability threshold would justify Kubernetes. |
| #31 | COMPARE | Learn operational inspection concepts represented by kubectl/k9s, then perform equivalent process/container/service inspection in the current BodySense environment. |
| #32 | COMPARE | Model how the current BodySense services would be represented as Kubernetes workloads/services without actually migrating production. |
| #33 | DIRECT | Trace BodySense DNS from hostname to ingress/proxy/service and identify which layer owns each record. |
| #34 | DIRECT | Trace current reverse-proxy routing and compare it with Kubernetes Ingress concepts. |
| #35 | DIRECT | Trace TLS issuance/termination/renewal in the current deployment and explain certificate failure modes. |
| #36 | DIRECT | Trace automatic deployment from merged/released code to running BodySense artifact, including rollback/reconciliation semantics. |

## Section 4 — Sessions, RPC, API contracts and logging

| Source | Mode | BodySense rep |
|---|---|---|
| #37 | DIRECT | Deep-read refresh token rotation, replay detection, session family revocation and Redis authority in `AuthService`; construct a two-request race. |
| #38 | DIRECT | Generate or reconstruct database documentation from current migrations/schema and verify it against executable state. |
| #39 | COMPARE | Learn RPC/gRPC fundamentals and compare them with BodySense Go <-> Python HTTP/stream boundaries. |
| #40 | COMPARE | Design a small protobuf contract for one internal call as a paper/spike exercise; compare generated IDL guarantees with current Pydantic/JSON contracts. |
| #41 | COMPARE | Run an isolated gRPC hello/health spike only if useful; focus on connection/channel/deadline/error semantics, not production adoption. |
| #42 | COMPARE | Model how an existing BodySense internal API would look as RPC and identify auth/ownership changes. |
| #43 | COMPARE | Compare gRPC-Gateway's dual-protocol idea with BodySense's explicit REST/public and internal service boundaries. |
| #44 | COMPARE | Map gRPC metadata to BodySense HTTP headers/context propagation: auth, request IDs, tracing and deadlines. |
| #45 | DIRECT | Audit current API contract documentation/route authority. If machine-readable OpenAPI is incomplete, design the smallest useful improvement rather than copying Swagger setup blindly. |
| #46 | COMPARE | Compare embedding frontend assets into a Go binary with BodySense's current web artifact/CDN/proxy strategy. |
| #47 | DIRECT | Validate one request at the transport boundary and ensure machine-readable error code + human-readable message remain stable. |
| #48 | COMPARE | Compare application-start migrations with BodySense's explicit migration/release validation strategy and explain which failure model is safer here. |
| #49 | DIRECT | Trace one PATCH/partial update path. Prove omitted, explicit-null and zero values cannot be confused where semantics differ. |
| #50 | COMPARE | Express the same partial-update contract in protobuf optional/presence terms and compare with JSON/Go pointer/nullability semantics. |
| #51 | DIRECT | Add/verify authorization around one sensitive API and prove identity alone is insufficient. |
| #52 | COMPARE | Translate structured gRPC logging concepts to BodySense internal service logging/tracing fields. |
| #53 | DIRECT | Audit HTTP logging middleware: method/path/status/duration/request identity, secret/PHI redaction and error correlation. |

## Section 5 — Async work, Redis and background processing

| Source | Mode | BodySense rep |
|---|---|---|
| #54 | DIRECT | Trace BodySense durable `JobRuntime`: creation, claim, state transitions, retry budget and event log. Compare DB-backed lifecycle with Redis queue libraries. |
| #55 | DIRECT | Trace how a background job is integrated with a request/service boundary and which state is durable before the worker starts. |
| #56 | DIRECT | Study the transaction/outbox problem: design a failure timeline where DB commit and task enqueue diverge, then inspect BodySense's actual strategy for the chosen job path. |
| #57 | DIRECT | Force a safe worker/job failure and verify error persistence, retry decision and logs are correlated. |
| #58 | DIRECT | Reason about delayed/backoff work and classify fixed delay, exponential backoff, scheduled retry and user-visible waiting states. |
| #59 | COMPARE | Treat email as an external side effect. Design how BodySense would send one asynchronously with idempotency, retries and secret handling; implementation only if a real feature needs it. |
| #60 | DIRECT | Classify BodySense tests into unit/integration/e2e/slow/external and make skipping/selection explicit rather than hiding flaky dependencies. |
| #61 | COMPARE | Translate email-verification domain design into a BodySense-style durable verification workflow; focus on token identity, expiry and idempotency. |
| #62 | COMPARE | Specify the verification endpoint's state machine and replay behavior without adding an unused production feature. |
| #63 | DIRECT | Write a test where a service depends on multiple boundaries (DB/Redis or repository/clock/etc.) and keep the test behavior-focused. |
| #64 | DIRECT | Test an authenticated endpoint end to end through middleware/context rather than bypassing identity setup. |

## Section 6 — Stability and security hardening

| Source | Mode | BodySense rep |
|---|---|---|
| #65 | COMPARE | Review current ORM/query tooling versions and generated-code alternatives. No migration merely to mirror sqlc. |
| #66 | COMPARE | Compare PostgreSQL driver capabilities and connection behavior with BodySense's current GORM/driver stack; identify whether pgx-specific features would materially help. |
| #67 | DIRECT | Audit PostgreSQL error classification in current code and make constraint/transient/internal failures distinguishable where needed. |
| #68 | DIRECT | Explain every relevant Compose port and volume mapping and demonstrate persistence across a controlled restart. |
| #69 | DIRECT | Audit how Go/Python/Node helper binaries are installed/pinned in dev and CI; eliminate unpinned executable drift if found. |
| #70 | DIRECT | Trace BodySense capability/role authorization and distinguish RBAC from resource ownership and policy-based decisions. |
| #71 | COMPARE | Map cloud security-group/network ACL concepts to the actual BodySense production network allowlists/firewall/private connectivity. |
| #72 | COMPARE | Model dual HTTP/RPC deployment only as an architecture exercise unless BodySense actually adopts gRPC. |
| #73 | DIRECT | Review cost-sensitive production resources and identify one measurable cost guardrail without degrading required reliability. |
| #74 | DIRECT | Verify graceful shutdown semantics for Go HTTP, Python runtime/background work and container termination: stop accepting, drain/cancel safely, bound shutdown time. |
| #75 | DIRECT | Explain Go loop-variable capture semantics in the repository's current Go version and find/create a tiny test demonstrating the modern behavior. |
| #76 | DIRECT | Trace CORS from browser origin through current Go/proxy configuration and prove it is not an authentication mechanism. |
| #77 | DIRECT | Audit the current `golang-jwt/jwt/v5` usage, algorithm validation, claims and error handling against the lecture's package-upgrade lesson. |

## Backend lane completion test

This lane is complete when the learner can independently handle a BodySense change that includes:

1. a schema or persistence concern;
2. a transaction/concurrency decision;
3. a Go handler/service/repository change;
4. input validation and public error mapping;
5. authentication/authorization implications;
6. unit/integration tests;
7. async/idempotency implications if side effects exist;
8. container/CI/release verification;
9. an architecture trade-off explanation for at least one source technology BodySense intentionally does not use.
