# BodySense Master Course · Coverage Status

> Generated from machine-readable ledgers. Do not hand-edit counts in this file.
> Baseline date: 2026-09-08

## What the numbers mean

Lifecycle is monotonic:

`SOURCE_INDEXED -> MAPPED -> EXERCISE_READY -> LEARNER_VERIFIED`

- **SOURCE_INDEXED**: source item has a stable identifier/location, but no coverage claim beyond source inventory.
- **MAPPED**: source objective has a BodySense DIRECT / COMPARE / OPTIONAL mapping.
- **EXERCISE_READY**: the exercise has concrete targets, prerequisites, prediction, task, failure case, verification evidence and an L4 gate.
- **LEARNER_VERIFIED**: the learner has actually met the required mastery level with recorded evidence.

These states intentionally separate source integrity, curriculum mapping, executable practice and actual learning.

## Full Stack Open

Active current-source exercise inventory: **356 records across Parts 0-14**.

| Part | Current source exercises | Mapped | Exercise ready | Learner verified | Source authority |
|---:|---:|---:|---:|---:|---|
| 0 | 6 | 6 | 5 | 1 | pinned course-repository snapshot |
| 1 | 14 | 14 | 0 | 0 | pinned course-repository snapshot |
| 2 | 20 | 20 | 2 | 0 | pinned course-repository snapshot |
| 3 | 22 | 22 | 1 | 0 | pinned course-repository snapshot |
| 4 | 23 | 23 | 0 | 0 | pinned course-repository snapshot |
| 5 | 31 | 31 | 0 | 0 | pinned course-repository snapshot |
| 6 | 22 | 22 | 0 | 0 | pinned course-repository snapshot |
| 7 | 20 | 20 | 0 | 0 | pinned course-repository snapshot |
| 8 | 30 | 30 | 0 | 0 | current courses.mooc.fi API snapshot |
| 9 | 35 | 35 | 0 | 0 | current courses.mooc.fi API snapshot |
| 10 | 30 | 30 | 0 | 0 | current courses.mooc.fi API snapshot |
| 11 | 24 | 24 | 0 | 0 | current courses.mooc.fi API snapshot |
| 12 | 25 | 25 | 0 | 0 | current courses.mooc.fi API snapshot |
| 13 | 28 | 28 | 0 | 0 | current courses.mooc.fi API snapshot |
| 14 | 26 | 26 | 0 | 0 | current courses.mooc.fi API snapshot |

Core Parts 0-7: **158/158 numbered source exercises semantically mapped** against pinned repository snapshot `0711aef8a451c4458263e5587ccda85f08fd7a96`. This is mapping coverage, not completed learning.

Advanced Parts 8-14: **198/198 current exercise records semantically mapped** after indexing from the public courses.mooc.fi Course Material API. Snapshot retrieval: `2026-09-07T15:30:35.638Z`; source-state SHA-256: `4a8093083af985c19e04b03bf876fd148676bd6c2b65327c7628d8b20122689d`. Mapping is complete at exercise-objective level; current section-heading audit is also complete, while paragraph/example-level prose parity and exercise readiness remain separate.

The previous repository snapshot's Parts 8-11 are retained only as **104 historical exercise records** for comparison; they are not counted as current-course parity.

Current source-section inventory: **279 core h3 teaching headings (Parts 0-7)** + **385 current MOOC headings (Parts 8-14)**. **469 explicit section/subheading/prose-derived concepts** have already been semantically decomposed and mapped.

Primary section semantic-audit disposition: **664/664** reviewed; **461** concept-mapped, **106** explicitly redundant, **97** non-engineering/logistics, **0** pending.

Pinned-core nested-heading audit: **196/196 h4-h6 units dispositioned**; **118** covered by current exercises, **4** current alternative exercise variants retained, **19** explicitly removed-track headings retained only historically, **0** pending. This closes the known h3-only core indexing gap but still does not equal exhaustive paragraph/example-level prose parity.

Targeted pinned-core prose-risk audit: **10/10 selected high-risk h3 fragments reviewed**, **2 newly exposed paragraph-level concepts**, **0 pending**. This layer is deliberately risk-selected and fingerprinted; it improves omission detection without claiming exhaustive prose parity.

The MOOC metadata snapshot intentionally stores only identifiers, page/chapter metadata, headings and short exercise titles; it does not copy exercise assignments, answers or course prose.

## TECH SCHOOL Backend Master Class

Public README baseline: `97f000fe58ad01a0774179ffa8884ac7784cf263`.

- Public lecture IDs/titles indexed: **78/78**.
- Title-level BodySense mappings: **78/78**.
- Exercise-ready lecture reps: **16/78**.
- Learner-verified lecture reps: **4/78**.

This does **not** claim that paid/video-internal teaching semantics were audited. The source authority is the public README title/index only.

## BodySense Agent extension

- Project-defined modules mapped: **8/8**.
- Exercise-ready: **8/8**.
- Learner-verified: **1/8**.

## Exercise-ready spine

There are currently **125** executable cards and **20** learner-verified cards.

- `BS-FSO-0.1` -> [card](../../exercises/bs-fso-0-1.md)
- `BS-FSO-0.3` -> [card](../../exercises/bs-fso-0-3.md)
- `BS-FSO-0.4` -> [card](../../exercises/bs-fso-0-4.md)
- `BS-FSO-0.5` -> [card](../../exercises/bs-fso-0-5.md)
- `BS-FSO-0.6` -> [card](../../exercises/bs-fso-0-6.md)
- `BS-FSO-2.11` -> [card](../../exercises/bs-fso-2-11.md)
- `BS-FSO-2.17` -> [card](../../exercises/bs-fso-2-17.md)
- `BS-FSO-3.1` -> [card](../../exercises/bs-fso-3-1.md)
- `BS-P2-CONCEPT-ASYNC-RUNTIME` -> [card](../../exercises/bs-p2-concept-async-runtime.md)
- `BS-P2-CONCEPT-PROMISES` -> [card](../../exercises/bs-p2-concept-promises.md)
- `BS-P2-CONCEPT-EFFECTS` -> [card](../../exercises/bs-p2-concept-effects.md)
- `BS-P7-CONCEPT-USEMEMO` -> [card](../../exercises/bs-p7-concept-usememo.md)
- `BS-P7-CONCEPT-REACT-MEMO` -> [card](../../exercises/bs-p7-concept-react-memo.md)
- `BS-P7-CONCEPT-USECALLBACK` -> [card](../../exercises/bs-p7-concept-usecallback.md)
- `BS-P7-CONCEPT-XSS` -> [card](../../exercises/bs-p7-concept-xss.md)
- `BS-P7-CONCEPT-DEPENDENCY-SECURITY` -> [card](../../exercises/bs-p7-concept-dependency-security.md)
- `BS-P7-CONCEPT-BROKEN-AUTHZ` -> [card](../../exercises/bs-p7-concept-broken-authz.md)
- `BS-P1-CONCEPT-COMPONENT` -> [card](../../exercises/bs-p1-concept-component.md)
- `BS-P1-CONCEPT-JSX` -> [card](../../exercises/bs-p1-concept-jsx.md)
- `BS-P1-CONCEPT-PROPS` -> [card](../../exercises/bs-p1-concept-props.md)
- `BS-P1-CONCEPT-RENDER-CYCLE` -> [card](../../exercises/bs-p1-concept-render-cycle.md)
- `BS-P1-CONCEPT-USESTATE` -> [card](../../exercises/bs-p1-concept-usestate.md)
- `BS-P1-CONCEPT-EVENT-HANDLING` -> [card](../../exercises/bs-p1-concept-event-handling.md)
- `BS-P1-CONCEPT-STATE-PROP-OWNERSHIP` -> [card](../../exercises/bs-p1-concept-state-prop-ownership.md)
- `BS-P1-CONCEPT-IMMUTABLE-ARRAY-STATE` -> [card](../../exercises/bs-p1-concept-immutable-array-state.md)
- `BS-P1-CONCEPT-ASYNC-STATE-UPDATES` -> [card](../../exercises/bs-p1-concept-async-state-updates.md)
- `BS-P1-CONCEPT-HOOK-RULES` -> [card](../../exercises/bs-p1-concept-hook-rules.md)
- `BS-P2-CONCEPT-REACT-KEYS` -> [card](../../exercises/bs-p2-concept-react-keys.md)
- `BS-P2-CONCEPT-CONTROLLED-COMPONENT` -> [card](../../exercises/bs-p2-concept-controlled-component.md)
- `BS-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY` -> [card](../../exercises/bs-p3-concept-http-safety-idempotency.md)
- `BS-P3-CONCEPT-MIDDLEWARE-CHAIN` -> [card](../../exercises/bs-p3-concept-middleware-chain.md)
- `BS-P3-CONCEPT-SAME-ORIGIN-CORS` -> [card](../../exercises/bs-p3-concept-same-origin-cors.md)
- `BS-P3-CONCEPT-HTTP-ERROR-TAXONOMY` -> [card](../../exercises/bs-p3-concept-http-error-taxonomy.md)
- `BS-P4-CONCEPT-BEARER-AUTHORIZATION` -> [card](../../exercises/bs-p4-concept-bearer-authorization.md)
- `BS-P4-CONCEPT-TOKEN-REVOCATION` -> [card](../../exercises/bs-p4-concept-token-revocation.md)
- `BS-P5-CONCEPT-FORM-LABELS` -> [card](../../exercises/bs-p5-concept-form-labels.md)
- `BS-P5-CONCEPT-BROWSER-TOKEN-PERSISTENCE` -> [card](../../exercises/bs-p5-concept-browser-token-persistence.md)
- `BS-P5-CONCEPT-COMPONENT-TEST-RENDER` -> [card](../../exercises/bs-p5-concept-component-test-render.md)
- `BS-P5-CONCEPT-TESTING-LIBRARY-QUERIES` -> [card](../../exercises/bs-p5-concept-testing-library-queries.md)
- `BS-P5-CONCEPT-USER-EVENT-TESTING` -> [card](../../exercises/bs-p5-concept-user-event-testing.md)
- `BS-P5-CONCEPT-STATEFUL-COMPONENT-TESTS` -> [card](../../exercises/bs-p5-concept-stateful-component-tests.md)
- `BS-P5-CONCEPT-TEST-COVERAGE` -> [card](../../exercises/bs-p5-concept-test-coverage.md)
- `BS-P5-CONCEPT-FRONTEND-INTEGRATION-BOUNDARY` -> [card](../../exercises/bs-p5-concept-frontend-integration-boundary.md)
- `BS-P5-CONCEPT-E2E-BLACK-BOX` -> [card](../../exercises/bs-p5-concept-e2e-black-box.md)
- `BS-P5-CONCEPT-NEGATIVE-E2E` -> [card](../../exercises/bs-p5-concept-negative-e2e.md)
- `BS-P5-CONCEPT-CLIENT-ROUTING` -> [card](../../exercises/bs-p5-concept-client-routing.md)
- `BS-P5-CONCEPT-ROUTE-PARAMS` -> [card](../../exercises/bs-p5-concept-route-params.md)
- `BS-P5-CONCEPT-IMPERATIVE-NAVIGATION` -> [card](../../exercises/bs-p5-concept-imperative-navigation.md)
- `BS-P5-CONCEPT-ROUTE-DATA-OWNERSHIP` -> [card](../../exercises/bs-p5-concept-route-data-ownership.md)
- `BS-P6-CONCEPT-TANSTACK-QUERY` -> [card](../../exercises/bs-p6-concept-tanstack-query.md)
- `BS-P6-CONCEPT-QUERY-MUTATION-INVALIDATION` -> [card](../../exercises/bs-p6-concept-query-mutation-invalidation.md)
- `BS-P6-CONCEPT-STATE-OWNERSHIP-CHOICE` -> [card](../../exercises/bs-p6-concept-state-ownership-choice.md)
- `BS-P7-CONCEPT-HOOKS-MENTAL-MODEL` -> [card](../../exercises/bs-p7-concept-hooks-mental-model.md)
- `BS-P7-CONCEPT-CUSTOM-HOOKS` -> [card](../../exercises/bs-p7-concept-custom-hooks.md)
- `BS-P7-CONCEPT-BUNDLING` -> [card](../../exercises/bs-p7-concept-bundling.md)
- `BS-P7-CONCEPT-VITE-DEV-PROD` -> [card](../../exercises/bs-p7-concept-vite-dev-prod.md)
- `BS-P7-CONCEPT-TRANSPILATION` -> [card](../../exercises/bs-p7-concept-transpilation.md)
- `BS-P7-CONCEPT-VITE-CONFIG` -> [card](../../exercises/bs-p7-concept-vite-config.md)
- `BS-P7-CONCEPT-ERROR-BOUNDARY` -> [card](../../exercises/bs-p7-concept-error-boundary.md)
- `BS-P7-CONCEPT-MONOREPO-TOPOLOGY` -> [card](../../exercises/bs-p7-concept-monorepo-topology.md)
- `BS-P7-CONCEPT-FEATURE-ORGANIZATION` -> [card](../../exercises/bs-p7-concept-feature-organization.md)
- `BS-P7-CONCEPT-SERVER-PUSH-SYNC` -> [card](../../exercises/bs-p7-concept-server-push-sync.md)
- `BS-P9-CONCEPT-STRUCTURAL-TYPING` -> [card](../../exercises/bs-p9-concept-structural-typing.md)
- `BS-P9-CONCEPT-TYPE-ERASURE` -> [card](../../exercises/bs-p9-concept-type-erasure.md)
- `BS-P9-CONCEPT-UNKNOWN-NARROWING` -> [card](../../exercises/bs-p9-concept-unknown-narrowing.md)
- `BS-P9-CONCEPT-DISCRIMINATED-UNIONS` -> [card](../../exercises/bs-p9-concept-discriminated-unions.md)
- `BS-P9-CONCEPT-EXHAUSTIVE-NARROWING` -> [card](../../exercises/bs-p9-concept-exhaustive-narrowing.md)
- `BS-P9-CONCEPT-TYPED-SERVER-DATA` -> [card](../../exercises/bs-p9-concept-typed-server-data.md)
- `BS-P9-CONCEPT-SCHEMA-VALIDATION` -> [card](../../exercises/bs-p9-concept-schema-validation.md)
- `BS-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY` -> [card](../../exercises/bs-p8-concept-graphql-schema-query.md)
- `BS-P8-CONCEPT-APOLLO-SERVER` -> [card](../../exercises/bs-p8-concept-apollo-server.md)
- `BS-P8-CONCEPT-RESOLVER-ARGS-CONTEXT` -> [card](../../exercises/bs-p8-concept-resolver-args-context.md)
- `BS-P8-CONCEPT-GRAPHQL-MUTATION` -> [card](../../exercises/bs-p8-concept-graphql-mutation.md)
- `BS-P8-CONCEPT-GRAPHQL-ERRORS` -> [card](../../exercises/bs-p8-concept-graphql-errors.md)
- `BS-P8-CONCEPT-GRAPHQL-AUTH-CONTEXT` -> [card](../../exercises/bs-p8-concept-graphql-auth-context.md)
- `BS-P8-CONCEPT-APOLLO-CACHE-UPDATE` -> [card](../../exercises/bs-p8-concept-apollo-cache-update.md)
- `BS-P8-CONCEPT-GRAPHQL-SUBSCRIPTIONS` -> [card](../../exercises/bs-p8-concept-graphql-subscriptions.md)
- `BS-P8-CONCEPT-GRAPHQL-NPLUS1` -> [card](../../exercises/bs-p8-concept-graphql-nplus1.md)
- `BS-P8-CONCEPT-APOLLO-CLIENT` -> [card](../../exercises/bs-p8-concept-apollo-client.md)
- `BS-P8-CONCEPT-GRAPHQL-VARIABLES` -> [card](../../exercises/bs-p8-concept-graphql-variables.md)
- `BS-P8-CONCEPT-NORMALIZED-CACHE` -> [card](../../exercises/bs-p8-concept-normalized-cache.md)
- `BS-P11-CONCEPT-REPRODUCIBLE-PIPELINE` -> [card](../../exercises/bs-p11-concept-reproducible-pipeline.md)
- `BS-P11-CONCEPT-DEPLOYED-REVISION-PROVENANCE` -> [card](../../exercises/bs-p11-concept-deployed-revision-provenance.md)
- `BS-P11-CONCEPT-CI-QUALITY-GATES` -> [card](../../exercises/bs-p11-concept-ci-quality-gates.md)
- `BS-P11-CONCEPT-SAFE-DEPLOYMENT-SYSTEM` -> [card](../../exercises/bs-p11-concept-safe-deployment-system.md)
- `BS-P11-CONCEPT-BRANCH-PROTECTION` -> [card](../../exercises/bs-p11-concept-branch-protection.md)
- `BS-P12-CONCEPT-IMAGE-VS-CONTAINER` -> [card](../../exercises/bs-p12-concept-image-vs-container.md)
- `BS-P12-CONCEPT-DOCKERFILE` -> [card](../../exercises/bs-p12-concept-dockerfile.md)
- `BS-P12-CONCEPT-DOCKER-COMPOSE` -> [card](../../exercises/bs-p12-concept-docker-compose.md)
- `BS-P12-CONCEPT-DOCKER-VOLUMES` -> [card](../../exercises/bs-p12-concept-docker-volumes.md)
- `BS-P12-CONCEPT-DOCKER-NETWORK-DNS` -> [card](../../exercises/bs-p12-concept-docker-network-dns.md)
- `BS-P13-CONCEPT-EAGER-VS-LAZY-LOAD` -> [card](../../exercises/bs-p13-concept-eager-vs-lazy-load.md)
- `BS-P13-CONCEPT-MIGRATION-MODEL-SEPARATION` -> [card](../../exercises/bs-p13-concept-migration-model-separation.md)
- `BS-P13-CONCEPT-DIRECT-DATABASE-INSPECTION` -> [card](../../exercises/bs-p13-concept-direct-database-inspection.md)
- `BS-P13-CONCEPT-DATABASE-LAYER-STRUCTURE` -> [card](../../exercises/bs-p13-concept-database-layer-structure.md)
- `BS-P13-CONCEPT-FOREIGN-KEY-JOIN` -> [card](../../exercises/bs-p13-concept-foreign-key-join.md)
- `BS-P13-CONCEPT-TRANSACTIONAL-OWNED-INSERT` -> [card](../../exercises/bs-p13-concept-transactional-owned-insert.md)
- `BS-P13-CONCEPT-RELATIONAL-PROJECTIONS` -> [card](../../exercises/bs-p13-concept-relational-projections.md)
- `BS-P13-CONCEPT-RELATIONAL-QUERYING` -> [card](../../exercises/bs-p13-concept-relational-querying.md)
- `BS-P5-CONCEPT-TEST-QUERY-VARIANTS` -> [card](../../exercises/bs-p5-concept-test-query-variants.md)
- `BS-P7-CONCEPT-SECURITY-HEADERS` -> [card](../../exercises/bs-p7-concept-security-headers.md)
- `BS-TECH-01` -> [card](../../exercises/bs-tech-01.md)
- `BS-TECH-03` -> [card](../../exercises/bs-tech-03.md)
- `BS-TECH-05` -> [card](../../exercises/bs-tech-05.md)
- `BS-TECH-06` -> [card](../../exercises/bs-tech-06.md)
- `BS-TECH-07` -> [card](../../exercises/bs-tech-07.md)
- `BS-TECH-09` -> [card](../../exercises/bs-tech-09.md)
- `BS-TECH-10` -> [card](../../exercises/bs-tech-10.md)
- `BS-TECH-11` -> [card](../../exercises/bs-tech-11.md)
- `BS-TECH-15` -> [card](../../exercises/bs-tech-15.md)
- `BS-TECH-16` -> [card](../../exercises/bs-tech-16.md)
- `BS-TECH-20` -> [card](../../exercises/bs-tech-20.md)
- `BS-TECH-21` -> [card](../../exercises/bs-tech-21.md)
- `BS-TECH-22` -> [card](../../exercises/bs-tech-22.md)
- `BS-TECH-25` -> [card](../../exercises/bs-tech-25.md)
- `BS-TECH-37` -> [card](../../exercises/bs-tech-37.md)
- `BS-TECH-54` -> [card](../../exercises/bs-tech-54.md)
- `BS-A1` -> [card](../../exercises/bs-a1.md)
- `BS-A2` -> [card](../../exercises/bs-a2.md)
- `BS-A3` -> [card](../../exercises/bs-a3.md)
- `BS-A4` -> [card](../../exercises/bs-a4.md)
- `BS-A5` -> [card](../../exercises/bs-a5.md)
- `BS-A6` -> [card](../../exercises/bs-a6.md)
- `BS-A7` -> [card](../../exercises/bs-a7.md)
- `BS-A8` -> [card](../../exercises/bs-a8.md)

The next curriculum milestone is to expand the fingerprinted high-risk prose selection and continue promoting high-value mapped nodes to `EXERCISE_READY` while preserving prerequisite closure. Placement assessment starts only on ready prerequisite slices.
