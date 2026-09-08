# Exercise-ready study tracks

> Generated from the machine-readable ledgers. This is a curated learner-facing lens over the ready prerequisite graph, not a second source of truth.
> Tracks overlap intentionally. Complete a prerequisite once and reuse the same evidence across every track that depends on it.

Current executable curriculum: **113 ready/verified nodes**.

## 1. Web/browser request foundation

Build the browser -> React -> HTTP -> API mental model used by every later frontend/full-stack exercise.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-FSO-0.1](../../exercises/bs-fso-0-1.md) | L4 |
| 2 | [BS-FSO-0.3](../../exercises/bs-fso-0-3.md) | L4 |
| 3 | [BS-FSO-0.4](../../exercises/bs-fso-0-4.md) | L4 |
| 4 | [BS-FSO-0.5](../../exercises/bs-fso-0-5.md) | L4 |
| 5 | [BS-FSO-0.6](../../exercises/bs-fso-0-6.md) | L4 |
| 6 | [BS-FSO-2.11](../../exercises/bs-fso-2-11.md) | L4 |
| 7 | [BS-FSO-2.17](../../exercises/bs-fso-2-17.md) | L4 |
| 8 | [BS-FSO-3.1](../../exercises/bs-fso-3-1.md) | L4 |

Prerequisite closure outside this track: none.

## 2. React component model, async effects and hooks

Build the React render/state/event mental model first, then connect browser async work, effects, stable identity and reusable hooks without cargo-cult memoization.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-P1-CONCEPT-COMPONENT](../../exercises/bs-p1-concept-component.md) | L4 |
| 2 | [BS-P1-CONCEPT-JSX](../../exercises/bs-p1-concept-jsx.md) | L4 |
| 3 | [BS-P1-CONCEPT-PROPS](../../exercises/bs-p1-concept-props.md) | L4 |
| 4 | [BS-P1-CONCEPT-RENDER-CYCLE](../../exercises/bs-p1-concept-render-cycle.md) | L4 |
| 5 | [BS-P1-CONCEPT-USESTATE](../../exercises/bs-p1-concept-usestate.md) | L4 |
| 6 | [BS-P1-CONCEPT-EVENT-HANDLING](../../exercises/bs-p1-concept-event-handling.md) | L4 |
| 7 | [BS-P1-CONCEPT-STATE-PROP-OWNERSHIP](../../exercises/bs-p1-concept-state-prop-ownership.md) | L4 |
| 8 | [BS-P1-CONCEPT-IMMUTABLE-ARRAY-STATE](../../exercises/bs-p1-concept-immutable-array-state.md) | L4 |
| 9 | [BS-P1-CONCEPT-ASYNC-STATE-UPDATES](../../exercises/bs-p1-concept-async-state-updates.md) | L4 |
| 10 | [BS-P1-CONCEPT-HOOK-RULES](../../exercises/bs-p1-concept-hook-rules.md) | L4 |
| 11 | [BS-P2-CONCEPT-ASYNC-RUNTIME](../../exercises/bs-p2-concept-async-runtime.md) | L4 |
| 12 | [BS-P2-CONCEPT-PROMISES](../../exercises/bs-p2-concept-promises.md) | L4 |
| 13 | [BS-P2-CONCEPT-EFFECTS](../../exercises/bs-p2-concept-effects.md) | L4 |
| 14 | [BS-P2-CONCEPT-REACT-KEYS](../../exercises/bs-p2-concept-react-keys.md) | L4 |
| 15 | [BS-P2-CONCEPT-CONTROLLED-COMPONENT](../../exercises/bs-p2-concept-controlled-component.md) | L4 |
| 16 | [BS-P7-CONCEPT-HOOKS-MENTAL-MODEL](../../exercises/bs-p7-concept-hooks-mental-model.md) | L4 |
| 17 | [BS-P7-CONCEPT-CUSTOM-HOOKS](../../exercises/bs-p7-concept-custom-hooks.md) | L4 |
| 18 | [BS-P7-CONCEPT-USEMEMO](../../exercises/bs-p7-concept-usememo.md) | L4 |
| 19 | [BS-P7-CONCEPT-USECALLBACK](../../exercises/bs-p7-concept-usecallback.md) | L4 |
| 20 | [BS-P7-CONCEPT-REACT-MEMO](../../exercises/bs-p7-concept-react-memo.md) | L4 |

Prerequisite closure outside this track: `BS-FSO-0.1`, `BS-FSO-0.5`, `BS-FSO-0.4`, `BS-FSO-0.3`.

## 3. React component testing and failure isolation

Test React through user-observable behavior, realistic interaction semantics and explicit render-failure boundaries rather than private implementation details.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-P5-CONCEPT-COMPONENT-TEST-RENDER](../../exercises/bs-p5-concept-component-test-render.md) | L4 |
| 2 | [BS-P5-CONCEPT-TESTING-LIBRARY-QUERIES](../../exercises/bs-p5-concept-testing-library-queries.md) | L4 |
| 3 | [BS-P5-CONCEPT-USER-EVENT-TESTING](../../exercises/bs-p5-concept-user-event-testing.md) | L4 |
| 4 | [BS-P5-CONCEPT-STATEFUL-COMPONENT-TESTS](../../exercises/bs-p5-concept-stateful-component-tests.md) | L4 |
| 5 | [BS-P5-CONCEPT-TEST-QUERY-VARIANTS](../../exercises/bs-p5-concept-test-query-variants.md) | L4 |
| 6 | [BS-P5-CONCEPT-FRONTEND-INTEGRATION-BOUNDARY](../../exercises/bs-p5-concept-frontend-integration-boundary.md) | L4 |
| 7 | [BS-P5-CONCEPT-E2E-BLACK-BOX](../../exercises/bs-p5-concept-e2e-black-box.md) | L4 |
| 8 | [BS-P5-CONCEPT-NEGATIVE-E2E](../../exercises/bs-p5-concept-negative-e2e.md) | L4 |
| 9 | [BS-P5-CONCEPT-TEST-COVERAGE](../../exercises/bs-p5-concept-test-coverage.md) | L4 |
| 10 | [BS-P7-CONCEPT-ERROR-BOUNDARY](../../exercises/bs-p7-concept-error-boundary.md) | L4 |

Prerequisite closure outside this track: `BS-P1-CONCEPT-COMPONENT`, `BS-FSO-0.1`, `BS-P1-CONCEPT-EVENT-HANDLING`, `BS-P1-CONCEPT-USESTATE`, `BS-P1-CONCEPT-RENDER-CYCLE`, `BS-P2-CONCEPT-PROMISES`, `BS-P2-CONCEPT-ASYNC-RUNTIME`, `BS-FSO-0.5`, `BS-FSO-0.4`, `BS-FSO-0.3`, `BS-FSO-3.1`, `BS-P4-CONCEPT-BEARER-AUTHORIZATION`, `BS-TECH-22`, `BS-TECH-21`, `BS-TECH-20`, `BS-TECH-15`, `BS-TECH-11`, `BS-TECH-01`, `BS-P3-CONCEPT-MIDDLEWARE-CHAIN`.

## 4. Frontend routing, build and application architecture

Make URL state, route-owned data, accessibility semantics, build transforms and repository/feature boundaries explicit so SPA behavior survives deep links and production builds.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-P5-CONCEPT-CLIENT-ROUTING](../../exercises/bs-p5-concept-client-routing.md) | L4 |
| 2 | [BS-P5-CONCEPT-ROUTE-PARAMS](../../exercises/bs-p5-concept-route-params.md) | L4 |
| 3 | [BS-P5-CONCEPT-IMPERATIVE-NAVIGATION](../../exercises/bs-p5-concept-imperative-navigation.md) | L4 |
| 4 | [BS-P5-CONCEPT-ROUTE-DATA-OWNERSHIP](../../exercises/bs-p5-concept-route-data-ownership.md) | L4 |
| 5 | [BS-P5-CONCEPT-FORM-LABELS](../../exercises/bs-p5-concept-form-labels.md) | L4 |
| 6 | [BS-P7-CONCEPT-TRANSPILATION](../../exercises/bs-p7-concept-transpilation.md) | L4 |
| 7 | [BS-P7-CONCEPT-BUNDLING](../../exercises/bs-p7-concept-bundling.md) | L4 |
| 8 | [BS-P7-CONCEPT-VITE-DEV-PROD](../../exercises/bs-p7-concept-vite-dev-prod.md) | L4 |
| 9 | [BS-P7-CONCEPT-VITE-CONFIG](../../exercises/bs-p7-concept-vite-config.md) | L4 |
| 10 | [BS-P7-CONCEPT-FEATURE-ORGANIZATION](../../exercises/bs-p7-concept-feature-organization.md) | L4 |
| 11 | [BS-P7-CONCEPT-MONOREPO-TOPOLOGY](../../exercises/bs-p7-concept-monorepo-topology.md) | L4 |

Prerequisite closure outside this track: `BS-P1-CONCEPT-COMPONENT`, `BS-FSO-0.1`, `BS-FSO-0.5`, `BS-FSO-0.4`, `BS-FSO-0.3`, `BS-P1-CONCEPT-EVENT-HANDLING`, `BS-P1-CONCEPT-USESTATE`, `BS-P1-CONCEPT-RENDER-CYCLE`, `BS-P6-CONCEPT-TANSTACK-QUERY`, `BS-P6-CONCEPT-STATE-OWNERSHIP-CHOICE`, `BS-FSO-0.6`, `BS-FSO-2.11`, `BS-P2-CONCEPT-CONTROLLED-COMPONENT`, `BS-P1-CONCEPT-JSX`, `BS-P9-CONCEPT-STRUCTURAL-TYPING`, `BS-FSO-3.1`.

## 5. TypeScript contracts and runtime trust

Separate structural static typing from runtime validation, then encode variant/state contracts safely.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-P9-CONCEPT-STRUCTURAL-TYPING](../../exercises/bs-p9-concept-structural-typing.md) | L4 |
| 2 | [BS-P9-CONCEPT-TYPE-ERASURE](../../exercises/bs-p9-concept-type-erasure.md) | L4 |
| 3 | [BS-P9-CONCEPT-UNKNOWN-NARROWING](../../exercises/bs-p9-concept-unknown-narrowing.md) | L4 |
| 4 | [BS-P9-CONCEPT-DISCRIMINATED-UNIONS](../../exercises/bs-p9-concept-discriminated-unions.md) | L4 |
| 5 | [BS-P9-CONCEPT-EXHAUSTIVE-NARROWING](../../exercises/bs-p9-concept-exhaustive-narrowing.md) | L4 |
| 6 | [BS-P9-CONCEPT-TYPED-SERVER-DATA](../../exercises/bs-p9-concept-typed-server-data.md) | L4 |
| 7 | [BS-P9-CONCEPT-SCHEMA-VALIDATION](../../exercises/bs-p9-concept-schema-validation.md) | L4 |

Prerequisite closure outside this track: `BS-FSO-2.11`, `BS-FSO-0.5`, `BS-FSO-0.4`, `BS-FSO-0.1`, `BS-FSO-0.3`.

## 6. HTTP, authentication and web security

Understand request semantics and middleware, then prove authentication, revocation, browser token storage and server-side authorization boundaries.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY](../../exercises/bs-p3-concept-http-safety-idempotency.md) | L4 |
| 2 | [BS-P3-CONCEPT-MIDDLEWARE-CHAIN](../../exercises/bs-p3-concept-middleware-chain.md) | L4 |
| 3 | [BS-P3-CONCEPT-SAME-ORIGIN-CORS](../../exercises/bs-p3-concept-same-origin-cors.md) | L4 |
| 4 | [BS-P3-CONCEPT-HTTP-ERROR-TAXONOMY](../../exercises/bs-p3-concept-http-error-taxonomy.md) | L4 |
| 5 | [BS-TECH-20](../../exercises/bs-tech-20.md) | L4 |
| 6 | [BS-TECH-21](../../exercises/bs-tech-21.md) | L4 |
| 7 | [BS-TECH-22](../../exercises/bs-tech-22.md) | L4 |
| 8 | [BS-TECH-37](../../exercises/bs-tech-37.md) | L4 |
| 9 | [BS-P4-CONCEPT-BEARER-AUTHORIZATION](../../exercises/bs-p4-concept-bearer-authorization.md) | L4 |
| 10 | [BS-P4-CONCEPT-TOKEN-REVOCATION](../../exercises/bs-p4-concept-token-revocation.md) | L4 |
| 11 | [BS-P5-CONCEPT-BROWSER-TOKEN-PERSISTENCE](../../exercises/bs-p5-concept-browser-token-persistence.md) | L4 |
| 12 | [BS-P7-CONCEPT-BROKEN-AUTHZ](../../exercises/bs-p7-concept-broken-authz.md) | L4 |
| 13 | [BS-P7-CONCEPT-XSS](../../exercises/bs-p7-concept-xss.md) | L4 |
| 14 | [BS-P7-CONCEPT-SECURITY-HEADERS](../../exercises/bs-p7-concept-security-headers.md) | L4 |
| 15 | [BS-P7-CONCEPT-DEPENDENCY-SECURITY](../../exercises/bs-p7-concept-dependency-security.md) | L4 |

Prerequisite closure outside this track: `BS-FSO-3.1`, `BS-FSO-0.4`, `BS-FSO-0.1`, `BS-FSO-0.3`, `BS-FSO-0.5`, `BS-TECH-15`, `BS-TECH-11`, `BS-TECH-01`, `BS-P11-CONCEPT-REPRODUCIBLE-PIPELINE`, `BS-TECH-10`, `BS-TECH-05`, `BS-TECH-03`.

## 7. Frontend state, server cache and realtime recovery

Choose the correct state owner, synchronize server mutations, and recover push streams without treating transport state as durable truth.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-P6-CONCEPT-STATE-OWNERSHIP-CHOICE](../../exercises/bs-p6-concept-state-ownership-choice.md) | L4 |
| 2 | [BS-P6-CONCEPT-TANSTACK-QUERY](../../exercises/bs-p6-concept-tanstack-query.md) | L4 |
| 3 | [BS-P6-CONCEPT-QUERY-MUTATION-INVALIDATION](../../exercises/bs-p6-concept-query-mutation-invalidation.md) | L4 |
| 4 | [BS-P7-CONCEPT-SERVER-PUSH-SYNC](../../exercises/bs-p7-concept-server-push-sync.md) | L4 |
| 5 | [BS-A7](../../exercises/bs-a7.md) | L4 |

Prerequisite closure outside this track: `BS-FSO-0.5`, `BS-FSO-0.4`, `BS-FSO-0.1`, `BS-FSO-0.3`, `BS-FSO-0.6`, `BS-FSO-2.11`, `BS-FSO-2.17`, `BS-A1`, `BS-A2`.

## 8. Go backend, database and concurrency reliability

Move from schema/migrations and repository boundaries through joins/query projections, transaction locks/isolation, authentication and durable jobs.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-TECH-01](../../exercises/bs-tech-01.md) | L4 |
| 2 | [BS-TECH-03](../../exercises/bs-tech-03.md) | L4 |
| 3 | [BS-TECH-05](../../exercises/bs-tech-05.md) | L4 |
| 4 | [BS-TECH-06](../../exercises/bs-tech-06.md) | L4 |
| 5 | [BS-TECH-07](../../exercises/bs-tech-07.md) | L4 |
| 6 | [BS-TECH-09](../../exercises/bs-tech-09.md) | L4 |
| 7 | [BS-TECH-11](../../exercises/bs-tech-11.md) | L4 |
| 8 | [BS-TECH-15](../../exercises/bs-tech-15.md) | L4 |
| 9 | [BS-TECH-16](../../exercises/bs-tech-16.md) | L4 |
| 10 | [BS-P13-CONCEPT-DATABASE-LAYER-STRUCTURE](../../exercises/bs-p13-concept-database-layer-structure.md) | L4 |
| 11 | [BS-P13-CONCEPT-FOREIGN-KEY-JOIN](../../exercises/bs-p13-concept-foreign-key-join.md) | L4 |
| 12 | [BS-P13-CONCEPT-RELATIONAL-QUERYING](../../exercises/bs-p13-concept-relational-querying.md) | L4 |
| 13 | [BS-P13-CONCEPT-RELATIONAL-PROJECTIONS](../../exercises/bs-p13-concept-relational-projections.md) | L4 |
| 14 | [BS-P13-CONCEPT-MIGRATION-MODEL-SEPARATION](../../exercises/bs-p13-concept-migration-model-separation.md) | L4 |
| 15 | [BS-P13-CONCEPT-TRANSACTIONAL-OWNED-INSERT](../../exercises/bs-p13-concept-transactional-owned-insert.md) | L4 |
| 16 | [BS-P13-CONCEPT-DIRECT-DATABASE-INSPECTION](../../exercises/bs-p13-concept-direct-database-inspection.md) | L4 |
| 17 | [BS-P13-CONCEPT-EAGER-VS-LAZY-LOAD](../../exercises/bs-p13-concept-eager-vs-lazy-load.md) | L4 |
| 18 | [BS-TECH-20](../../exercises/bs-tech-20.md) | L4 |
| 19 | [BS-TECH-21](../../exercises/bs-tech-21.md) | L4 |
| 20 | [BS-TECH-22](../../exercises/bs-tech-22.md) | L4 |
| 21 | [BS-TECH-37](../../exercises/bs-tech-37.md) | L4 |
| 22 | [BS-TECH-54](../../exercises/bs-tech-54.md) | L4 |

Prerequisite closure outside this track: `BS-FSO-3.1`, `BS-FSO-0.4`, `BS-FSO-0.1`, `BS-FSO-0.3`, `BS-TECH-10`, `BS-TECH-25`.

## 9. Containers and production delivery

Understand image/runtime/network persistence first, then make CI/deploy reproducible, gated, recoverable and revision-identifiable.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-P12-CONCEPT-IMAGE-VS-CONTAINER](../../exercises/bs-p12-concept-image-vs-container.md) | L4 |
| 2 | [BS-P12-CONCEPT-DOCKERFILE](../../exercises/bs-p12-concept-dockerfile.md) | L4 |
| 3 | [BS-P12-CONCEPT-DOCKER-COMPOSE](../../exercises/bs-p12-concept-docker-compose.md) | L4 |
| 4 | [BS-P12-CONCEPT-DOCKER-NETWORK-DNS](../../exercises/bs-p12-concept-docker-network-dns.md) | L4 |
| 5 | [BS-P12-CONCEPT-DOCKER-VOLUMES](../../exercises/bs-p12-concept-docker-volumes.md) | L4 |
| 6 | [BS-TECH-10](../../exercises/bs-tech-10.md) | L4 |
| 7 | [BS-TECH-25](../../exercises/bs-tech-25.md) | L4 |
| 8 | [BS-P11-CONCEPT-REPRODUCIBLE-PIPELINE](../../exercises/bs-p11-concept-reproducible-pipeline.md) | L4 |
| 9 | [BS-P11-CONCEPT-CI-QUALITY-GATES](../../exercises/bs-p11-concept-ci-quality-gates.md) | L4 |
| 10 | [BS-P11-CONCEPT-BRANCH-PROTECTION](../../exercises/bs-p11-concept-branch-protection.md) | L4 |
| 11 | [BS-P11-CONCEPT-SAFE-DEPLOYMENT-SYSTEM](../../exercises/bs-p11-concept-safe-deployment-system.md) | L4 |
| 12 | [BS-P11-CONCEPT-DEPLOYED-REVISION-PROVENANCE](../../exercises/bs-p11-concept-deployed-revision-provenance.md) | L4 |

Prerequisite closure outside this track: `BS-TECH-05`, `BS-TECH-03`, `BS-TECH-01`.

## 10. Production Agent engineering

Learn typed execution, durable ownership, evidence/admissibility, deterministic authority, eval/rollout, replay, HITL/recovery and failure attribution as one production system.

| # | Exercise | Required level |
|---:|---|---|
| 1 | [BS-A1](../../exercises/bs-a1.md) | L4 |
| 2 | [BS-A2](../../exercises/bs-a2.md) | L4 |
| 3 | [BS-A3](../../exercises/bs-a3.md) | L4 |
| 4 | [BS-A4](../../exercises/bs-a4.md) | L4 |
| 5 | [BS-A5](../../exercises/bs-a5.md) | L4 |
| 6 | [BS-A6](../../exercises/bs-a6.md) | L4 |
| 7 | [BS-A7](../../exercises/bs-a7.md) | L4 |
| 8 | [BS-A8](../../exercises/bs-a8.md) | L4 |

Prerequisite closure outside this track: none.

## How to use the tracks

1. Do **not** read the target code first. Write the card prediction before opening the implementation/tests.
2. Use the smallest verification slice that can distinguish the prediction from the failure case.
3. A green existing test is evidence about the system, not evidence of learner mastery. L4 requires independent explain-back and falsification criteria.
4. When prior production work already proves a card, use placement evidence; do not mechanically rewrite working code.
5. Production changes are optional and should happen only when the card exposes a real, regression-characterized gap.

