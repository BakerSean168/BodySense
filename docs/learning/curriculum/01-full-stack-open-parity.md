# Full Stack Open -> BodySense Coverage Parity

> Source baseline: Full Stack Open checked 2026-09-07.
> Rule: preserve the learning objective and incremental difficulty, replace the toy domain with current BodySense.

## Coverage contract

Full Stack Open currently spans Parts 0-14. BodySense uses the source in three lanes:

- **Core parity (`DIRECT`)**: Parts 0-7. Every exercise number has a BodySense rep ID.
- **Transfer parity (`DIRECT` or `COMPARE`)**: Parts 8, 9, 11, 12, 13. Keep every generally useful engineering concept, but use BodySense's stack.
- **Product-surface extensions (`OPTIONAL`)**: Parts 10 and 14. React Native and Next.js are retained as explicit comparison/extension tracks instead of changing the BodySense web architecture just to match the course.

## Part 0 — Web application fundamentals (`DIRECT`)

**Knowledge parity**

- browser/server/database layers;
- HTML and CSS review;
- forms;
- HTTP request/response;
- JavaScript loading and execution;
- JSON;
- traditional page load vs SPA;
- sequence diagrams and reasoning from Network traces.

**BodySense exercise series**

- `BS-FSO-0.1` Inspect one rendered BodySense route and identify semantic HTML landmarks.
- `BS-FSO-0.2` Trace styling ownership from a rendered element to Tailwind/component styling and explain cascade/specificity only where it actually applies.
- `BS-FSO-0.3` Trace a real BodySense form from input state to submit handler to request payload.
- `BS-FSO-0.4` Draw the complete sequence for one non-streaming mutation.
- `BS-FSO-0.5` Draw initial SPA navigation and data hydration for one workbench route.
- `BS-FSO-0.6` Draw a mutation that updates server state and then changes the React projection.

## Part 1 — React fundamentals (`DIRECT`)

**Knowledge parity**

- components, JSX and composition;
- props;
- JavaScript data transformations used in React;
- component state;
- event handlers;
- immutable updates;
- derived values;
- conditional rendering;
- debugging React applications.

**Exercise parity**: `BS-FSO-1.1` through `BS-FSO-1.14`.

The original incremental rhythm is preserved, but each step operates on one small BodySense UI slice. The sequence is:

1. decompose a read-only UI into components;
2. move data through props;
3. replace duplicated primitives with structured data;
4. render a collection correctly with stable identity;
5. compute a derived aggregate without storing duplicate state;
6. introduce one local interaction state;
7. add a second state transition;
8. derive a statistic from state;
9. remove redundant state;
10. extract a reusable presentation component;
11. handle an edge case such as empty/undefined data;
12. add a selection/vote-like interaction against a harmless local fixture;
13. compute a winner/maximum from the collection;
14. debug a deliberately broken variant using React DevTools/console rather than guessing.

## Part 2 — Communicating with the server (`DIRECT`)

**Knowledge parity**

- rendering collections and keys;
- modules;
- controlled forms;
- filtering/search;
- HTTP client calls;
- fetching server data;
- create/update/delete operations;
- error handling and user feedback;
- CSS/UI feedback at the boundary.

**Exercise parity**: `BS-FSO-2.1` through `BS-FSO-2.20`.

The BodySense sequence uses one low-risk CRUD/projection surface and moves from read-only rendering -> form input -> filtering -> GET -> POST -> optimistic/pessimistic update decisions -> PUT/PATCH/DELETE -> error feedback -> cleanup. External-country API exercises are translated into consuming a real BodySense or public read-only endpoint and handling loading/error/empty states.

## Part 3 — Server programming and persistence (`DIRECT`, translated from Node/Express to Go/Gin)

**Knowledge parity**

- REST resources and routing;
- HTTP methods/status codes;
- middleware;
- request logging;
- configuration/environment;
- persistence;
- schema validation;
- database error translation;
- deployment and lint/static quality.

**Exercise parity**: `BS-FSO-3.1` through `BS-FSO-3.22`.

Instead of building a Phonebook backend, the learner traces and extends a narrow BodySense Go resource:

- steps 1-8: route/handler behavior and REST semantics;
- steps 9-11: frontend/backend integration and deployment-shaped run;
- step 12: direct database inspection tool/command;
- steps 13-20: persistence, validation, error mapping, uniqueness and not-found behavior;
- step 21: validate the same flow in the deployed/staging-shaped environment;
- step 22: lint/vet/static-quality gate.

## Part 4 — Backend structure, testing and user administration (`DIRECT`)

**Knowledge parity**

- modular backend structure;
- unit tests and integration tests;
- test helpers;
- API testing;
- users and relationships;
- password hashing;
- token authentication;
- authorization;
- middleware extraction;
- regression repair after auth changes.

**Exercise parity**: `BS-FSO-4.1` through `BS-FSO-4.23`.

BodySense maps these to Go `handler -> service -> repository`, auth/session code, Redis-backed revocation and focused tests. The learner must be able to distinguish authentication, authorization, session revocation, credential rotation and resource ownership.

## Part 5 — React application testing, auth UX, routing and styling (`DIRECT`)

**Knowledge parity**

- frontend login/auth state;
- `props.children` and component composition;
- refs/imperative boundaries where justified;
- component tests;
- mocking boundaries;
- end-to-end testing;
- routing;
- UI libraries/styling and accessibility-oriented polish.

**Exercise parity**: `BS-FSO-5.1` through `BS-FSO-5.31`.

BodySense uses the existing auth and consultation UI. Tests must exercise behavior rather than implementation details, and the E2E segment must operate through the real browser boundary.

## Part 6 — Advanced state management (`DIRECT`)

**Current-course knowledge parity**

- Flux-style unidirectional state reasoning;
- Zustand stores and actions;
- async actions and testing;
- TanStack Query server state;
- React Context + `useReducer`;
- legacy Redux concepts only as comparison knowledge.

**Exercise parity**: `BS-FSO-6.1` through `BS-FSO-6.22`.

BodySense's required result is not “use Zustand everywhere”. Each rep classifies a piece of state as:

```text
URL identity
TanStack Query server state
Zustand presentation state
component-local state
Go durable business state
Python Agent runtime state
```

A solution is correct only if ownership is justified.

## Part 7 — Hooks, build tooling, code organization and refactoring (`DIRECT`)

**Current-course knowledge parity**

- custom hooks;
- hook API design;
- Vite internals and bundling concepts;
- esbuild-level mental model;
- code organization;
- error boundaries;
- single-repository frontend/backend organization;
- class components as legacy-reading knowledge;
- formatting and systematic refactoring of an existing application.

**Exercise parity**: `BS-FSO-7.1` through `BS-FSO-7.20`.

The culminating reps extend existing BodySense functionality without creating a new app. Refactors must preserve behavior with tests before structural change.

## Part 8 — GraphQL (`COMPARE` + optional lab)

**Knowledge parity**

- schema and typed query language;
- resolvers;
- client queries/mutations;
- authentication;
- cache updates;
- subscriptions;
- N+1 problem and batching.

BodySense currently uses REST + SSE. The mandatory lab compares REST/SSE with GraphQL for one BodySense read model and explains whether GraphQL would improve or worsen the boundary. An optional isolated spike may implement a non-production GraphQL facade; production migration is not required.

## Part 9 — TypeScript (`DIRECT`)

**Knowledge parity**

- TypeScript compiler/tooling;
- primitives, unions, narrowing and inference;
- typing backend-like data;
- runtime validation vs static typing;
- React props/state/event types;
- discriminated unions and exhaustive handling;
- safely integrating into an existing typed codebase.

BodySense uses its real contracts and stream event parser. Total TypeScript remains a supplemental short-rep source, while FSO Part 9 supplies the end-to-end typed-application progression.

## Part 10 — React Native (`OPTIONAL`)

Retained knowledge points:

- React Native component model;
- mobile navigation;
- server communication;
- form handling;
- authentication persistence;
- testing mobile UI.

No BodySense production change is required. If a mobile client becomes a real product goal, this part turns into a direct track; until then it is an explicit optional extension rather than a silent omission.

## Part 11 — CI/CD (`DIRECT`)

**Knowledge parity**

- purpose of CI/CD;
- GitHub Actions workflow model;
- lint/test/build gates;
- deployment automation;
- branch conditions;
- versioning/tagging;
- protected main branch;
- notifications and health checks;
- pipeline design for one's own project.

BodySense uses `.github/workflows/ci.yml`, delivery manifests, quality lanes/oracles and release verification as the lab. The exercise is to understand and validate the existing pipeline before changing it.

## Part 12 — Containers (`DIRECT`)

**Knowledge parity**

- images, containers and layers;
- Dockerfiles;
- volumes and ports;
- environment/config injection;
- networks;
- multi-service development;
- Compose orchestration;
- reverse proxy/orchestration basics;
- development vs production container concerns.

BodySense's Docker/Compose setup is the direct laboratory. A learner must be able to bring up infrastructure, explain every network/volume boundary relevant to a request, and debug one controlled failure.

## Part 13 — Relational databases (`DIRECT`, translated from Sequelize to PostgreSQL/GORM)

**Knowledge parity**

- relational modeling;
- ORM trade-offs;
- constraints;
- joins;
- aggregate queries;
- migrations;
- many-to-many relationships;
- transaction-aware application design.

This part merges naturally with TECH SCHOOL's deeper PostgreSQL track. BodySense's longitudinal domain and migration history replace the source exercise database.

## Part 14 — Next.js (`OPTIONAL/COMPARE`)

Retained current-generation concepts:

- App Router mental model;
- server/client component boundary;
- server-side data access and actions;
- framework-managed routing/rendering trade-offs;
- full-stack React framework deployment model.

BodySense currently uses Vite + React Router + separate Go/Python services. The mandatory result is an architecture comparison. A production migration is out of scope unless the product develops a real server-rendering requirement.

## Completion test for the Full Stack Open lane

The lane is complete when the learner can independently:

1. trace one browser interaction through React -> HTTP/SSE -> Go -> DB/Redis/Python -> React;
2. explain state ownership without duplicating server truth in the browser;
3. add a tested React/TypeScript feature;
4. add a tested Go API/domain change;
5. reason about authentication and authorization;
6. validate it in CI/container/deployment-shaped conditions;
7. justify why a source-course technology that BodySense does not use is or is not appropriate here.
