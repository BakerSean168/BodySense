export const tracks = [
  {
    id: 'web-browser-foundation',
    name: 'Web/browser request foundation',
    goal: 'Build the browser -> React -> HTTP -> API mental model used by every later frontend/full-stack exercise.',
    ids: ['FSO-0.1','FSO-0.3','FSO-0.4','FSO-0.5','FSO-0.6','FSO-2.11','FSO-2.17','FSO-3.1'],
  },
  {
    id: 'react-component-hooks',
    name: 'React component model, async effects and hooks',
    goal: 'Build the React render/state/event mental model first, then connect browser async work, effects, stable identity and reusable hooks without cargo-cult memoization.',
    ids: [
      'FSO-P1-CONCEPT-COMPONENT','FSO-P1-CONCEPT-JSX','FSO-P1-CONCEPT-PROPS','FSO-P1-CONCEPT-RENDER-CYCLE',
      'FSO-P1-CONCEPT-USESTATE','FSO-P1-CONCEPT-EVENT-HANDLING','FSO-P1-CONCEPT-STATE-PROP-OWNERSHIP',
      'FSO-P1-CONCEPT-IMMUTABLE-ARRAY-STATE','FSO-P1-CONCEPT-ASYNC-STATE-UPDATES','FSO-P1-CONCEPT-HOOK-RULES',
      'FSO-P2-CONCEPT-ASYNC-RUNTIME','FSO-P2-CONCEPT-PROMISES','FSO-P2-CONCEPT-EFFECTS','FSO-P2-CONCEPT-REACT-KEYS','FSO-P2-CONCEPT-CONTROLLED-COMPONENT',
      'FSO-P7-CONCEPT-HOOKS-MENTAL-MODEL','FSO-P7-CONCEPT-CUSTOM-HOOKS','FSO-P7-CONCEPT-USEMEMO','FSO-P7-CONCEPT-USECALLBACK','FSO-P7-CONCEPT-REACT-MEMO',
    ],
  },
  {
    id: 'react-testing-failure-isolation',
    name: 'React component testing and failure isolation',
    goal: 'Test React through user-observable behavior, realistic interaction semantics and explicit render-failure boundaries rather than private implementation details.',
    ids: [
      'FSO-P5-CONCEPT-COMPONENT-TEST-RENDER','FSO-P5-CONCEPT-TESTING-LIBRARY-QUERIES',
      'FSO-P5-CONCEPT-USER-EVENT-TESTING','FSO-P5-CONCEPT-STATEFUL-COMPONENT-TESTS','FSO-P5-CONCEPT-TEST-QUERY-VARIANTS',
      'FSO-P5-CONCEPT-FRONTEND-INTEGRATION-BOUNDARY','FSO-P5-CONCEPT-E2E-BLACK-BOX','FSO-P5-CONCEPT-NEGATIVE-E2E','FSO-P5-CONCEPT-TEST-COVERAGE','FSO-P7-CONCEPT-ERROR-BOUNDARY',
    ],
  },
  {
    id: 'frontend-routing-build-architecture',
    name: 'Frontend routing, build and application architecture',
    goal: 'Make URL state, route-owned data, accessibility semantics, build transforms and repository/feature boundaries explicit so SPA behavior survives deep links and production builds.',
    ids: [
      'FSO-P5-CONCEPT-CLIENT-ROUTING','FSO-P5-CONCEPT-ROUTE-PARAMS','FSO-P5-CONCEPT-IMPERATIVE-NAVIGATION','FSO-P5-CONCEPT-ROUTE-DATA-OWNERSHIP','FSO-P5-CONCEPT-FORM-LABELS',
      'FSO-P7-CONCEPT-TRANSPILATION','FSO-P7-CONCEPT-BUNDLING','FSO-P7-CONCEPT-VITE-DEV-PROD','FSO-P7-CONCEPT-VITE-CONFIG',
      'FSO-P7-CONCEPT-FEATURE-ORGANIZATION','FSO-P7-CONCEPT-MONOREPO-TOPOLOGY',
    ],
  },
  {
    id: 'typescript-runtime-trust',
    name: 'TypeScript contracts and runtime trust',
    goal: 'Separate structural static typing from runtime validation, then encode variant/state contracts safely.',
    ids: [
      'FSO-P9-CONCEPT-STRUCTURAL-TYPING','FSO-P9-CONCEPT-TYPE-ERASURE','FSO-P9-CONCEPT-UNKNOWN-NARROWING',
      'FSO-P9-CONCEPT-DISCRIMINATED-UNIONS','FSO-P9-CONCEPT-EXHAUSTIVE-NARROWING',
      'FSO-P9-CONCEPT-TYPED-SERVER-DATA','FSO-P9-CONCEPT-SCHEMA-VALIDATION',
    ],
  },
  {
    id: 'http-auth-web-security',
    name: 'HTTP, authentication and web security',
    goal: 'Understand request semantics and middleware, then prove authentication, revocation, browser token storage and server-side authorization boundaries.',
    ids: [
      'FSO-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY','FSO-P3-CONCEPT-MIDDLEWARE-CHAIN','FSO-P3-CONCEPT-SAME-ORIGIN-CORS','FSO-P3-CONCEPT-HTTP-ERROR-TAXONOMY',
      'TECH-20','TECH-21','TECH-22','TECH-37',
      'FSO-P4-CONCEPT-BEARER-AUTHORIZATION','FSO-P4-CONCEPT-TOKEN-REVOCATION','FSO-P5-CONCEPT-BROWSER-TOKEN-PERSISTENCE',
      'FSO-P7-CONCEPT-BROKEN-AUTHZ','FSO-P7-CONCEPT-XSS','FSO-P7-CONCEPT-SECURITY-HEADERS','FSO-P7-CONCEPT-DEPENDENCY-SECURITY',
    ],
  },
  {
    id: 'frontend-state-realtime',
    name: 'Frontend state, server cache and realtime recovery',
    goal: 'Choose the correct state owner, synchronize server mutations, and recover push streams without treating transport state as durable truth.',
    ids: [
      'FSO-P6-CONCEPT-STATE-OWNERSHIP-CHOICE','FSO-P6-CONCEPT-TANSTACK-QUERY','FSO-P6-CONCEPT-QUERY-MUTATION-INVALIDATION','FSO-P7-CONCEPT-SERVER-PUSH-SYNC','BS-A7',
    ],
  },
  {
    id: 'fso-part-8-graphql',
    name: 'Full Stack Open Part 8 — GraphQL',
    goal: 'Follow the real Part 8 topic: schema/query contracts, resolver boundaries, mutations/errors, Apollo client/cache, auth context, subscriptions and N+1, using an isolated GraphQL lab plus BodySense architecture comparisons rather than silently replacing GraphQL with SSE.',
    ids: [
      'FSO-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY',
      'FSO-P8-CONCEPT-APOLLO-SERVER',
      'FSO-P8-CONCEPT-RESOLVER-ARGS-CONTEXT',
      'FSO-P8-CONCEPT-GRAPHQL-MUTATION',
      'FSO-P8-CONCEPT-GRAPHQL-ERRORS',
      'FSO-P8-CONCEPT-APOLLO-CLIENT',
      'FSO-P8-CONCEPT-GRAPHQL-VARIABLES',
      'FSO-P8-CONCEPT-NORMALIZED-CACHE',
      'FSO-P8-CONCEPT-APOLLO-CACHE-UPDATE',
      'FSO-P8-CONCEPT-GRAPHQL-AUTH-CONTEXT',
      'FSO-P8-CONCEPT-GRAPHQL-SUBSCRIPTIONS',
      'FSO-P8-CONCEPT-GRAPHQL-NPLUS1',
    ],
  },
  {
    id: 'go-backend-reliability',
    name: 'Go backend, database and concurrency reliability',
    goal: 'Move from schema/migrations and repository boundaries through joins/query projections, transaction locks/isolation, authentication and durable jobs.',
    ids: [
      'TECH-01','TECH-03','TECH-05','TECH-06','TECH-07','TECH-09','TECH-11','TECH-15','TECH-16',
      'FSO-P13-CONCEPT-DATABASE-LAYER-STRUCTURE','FSO-P13-CONCEPT-FOREIGN-KEY-JOIN','FSO-P13-CONCEPT-RELATIONAL-QUERYING',
      'FSO-P13-CONCEPT-RELATIONAL-PROJECTIONS','FSO-P13-CONCEPT-MIGRATION-MODEL-SEPARATION',
      'FSO-P13-CONCEPT-TRANSACTIONAL-OWNED-INSERT','FSO-P13-CONCEPT-DIRECT-DATABASE-INSPECTION','FSO-P13-CONCEPT-EAGER-VS-LAZY-LOAD',
      'TECH-20','TECH-21','TECH-22','TECH-37','TECH-54',
    ],
  },
  {
    id: 'containers-production-delivery',
    name: 'Containers and production delivery',
    goal: 'Understand image/runtime/network persistence first, then make CI/deploy reproducible, gated, recoverable and revision-identifiable.',
    ids: [
      'FSO-P12-CONCEPT-IMAGE-VS-CONTAINER','FSO-P12-CONCEPT-DOCKERFILE','FSO-P12-CONCEPT-DOCKER-COMPOSE','FSO-P12-CONCEPT-DOCKER-NETWORK-DNS','FSO-P12-CONCEPT-DOCKER-VOLUMES',
      'TECH-10','TECH-25',
      'FSO-P11-CONCEPT-REPRODUCIBLE-PIPELINE','FSO-P11-CONCEPT-CI-QUALITY-GATES','FSO-P11-CONCEPT-BRANCH-PROTECTION','FSO-P11-CONCEPT-SAFE-DEPLOYMENT-SYSTEM','FSO-P11-CONCEPT-DEPLOYED-REVISION-PROVENANCE',
    ],
  },
  {
    id: 'production-agent-engineering',
    name: 'Production Agent engineering',
    goal: 'Learn typed execution, durable ownership, evidence/admissibility, deterministic authority, eval/rollout, replay, HITL/recovery and failure attribution as one production system.',
    ids: ['BS-A1','BS-A2','BS-A3','BS-A4','BS-A5','BS-A6','BS-A7','BS-A8'],
  },
];

export const trackById = new Map(tracks.map((track) => [track.id, track]));
