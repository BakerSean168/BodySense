# Learner placement status

> Generated from curriculum ledgers + `ledger/learner-placement.json`. Do not hand-edit this view.
> Placement never infers mastery from production code alone; every assessed level requires recorded learner evidence.

## Active track

**TypeScript contracts and runtime trust** (`typescript-runtime-trust`)

Separate structural static typing from runtime validation, then encode variant/state contracts safely.

Relevant ready nodes including prerequisite closure: **12**; verified: **0**; gaps: **0**; unassessed: **12**.

## Next placement target

[BS-P9-CONCEPT-STRUCTURAL-TYPING](../../exercises/bs-p9-concept-structural-typing.md) · required **L4** · current **unassessed**.

Use the exercise card in placement mode: make the prediction without reading the implementation path, select discriminating evidence, then explain what would falsify the conclusion. Record the observed level only after that learner evidence exists.

## Placement queue

| # | Scope | Exercise | Required | Current | Placement state | Evidence journal |
|---:|---|---|---|---|---|---|
| 1 | track | [BS-P9-CONCEPT-STRUCTURAL-TYPING](../../exercises/bs-p9-concept-structural-typing.md) | L4 | — | UNASSESSED | — |
| 2 | track | [BS-P9-CONCEPT-TYPE-ERASURE](../../exercises/bs-p9-concept-type-erasure.md) | L4 | — | UNASSESSED | — |
| 3 | track | [BS-P9-CONCEPT-UNKNOWN-NARROWING](../../exercises/bs-p9-concept-unknown-narrowing.md) | L4 | — | UNASSESSED | — |
| 4 | track | [BS-P9-CONCEPT-DISCRIMINATED-UNIONS](../../exercises/bs-p9-concept-discriminated-unions.md) | L4 | — | UNASSESSED | — |
| 5 | track | [BS-P9-CONCEPT-EXHAUSTIVE-NARROWING](../../exercises/bs-p9-concept-exhaustive-narrowing.md) | L4 | — | UNASSESSED | — |
| 6 | prerequisite | [BS-FSO-0.1](../../exercises/bs-fso-0-1.md) | L4 | — | UNASSESSED | — |
| 7 | prerequisite | [BS-FSO-0.3](../../exercises/bs-fso-0-3.md) | L4 | — | UNASSESSED | — |
| 8 | prerequisite | [BS-FSO-0.4](../../exercises/bs-fso-0-4.md) | L4 | — | UNASSESSED | — |
| 9 | prerequisite | [BS-FSO-0.5](../../exercises/bs-fso-0-5.md) | L4 | — | UNASSESSED | — |
| 10 | prerequisite | [BS-FSO-2.11](../../exercises/bs-fso-2-11.md) | L4 | — | UNASSESSED | — |
| 11 | track | [BS-P9-CONCEPT-TYPED-SERVER-DATA](../../exercises/bs-p9-concept-typed-server-data.md) | L4 | — | UNASSESSED | — |
| 12 | track | [BS-P9-CONCEPT-SCHEMA-VALIDATION](../../exercises/bs-p9-concept-schema-validation.md) | L4 | — | UNASSESSED | — |

## Available tracks

| Track ID | Track | Nodes | Active |
|---|---|---:|---|
| `web-browser-foundation` | Web/browser request foundation | 8 |  |
| `react-component-hooks` | React component model, async effects and hooks | 20 |  |
| `react-testing-failure-isolation` | React component testing and failure isolation | 10 |  |
| `frontend-routing-build-architecture` | Frontend routing, build and application architecture | 11 |  |
| `typescript-runtime-trust` | TypeScript contracts and runtime trust | 7 | yes |
| `http-auth-web-security` | HTTP, authentication and web security | 15 |  |
| `frontend-state-realtime` | Frontend state, server cache and realtime recovery | 5 |  |
| `go-backend-reliability` | Go backend, database and concurrency reliability | 22 |  |
| `containers-production-delivery` | Containers and production delivery | 12 |  |
| `production-agent-engineering` | Production Agent engineering | 8 |  |

## Recording rule

Use `node scripts/learning/record-placement.mjs <ITEM_ID> <L1|L2|L3|L4|L5> --evidence "..."` only after the learner has actually produced the corresponding evidence. `L1-L3` records a gap and leaves the node exercise-ready; meeting the required gate promotes it to `LEARNER_VERIFIED`.

Prerequisite closure outside the active track: `BS-FSO-0.1`, `BS-FSO-0.3`, `BS-FSO-0.4`, `BS-FSO-0.5`, `BS-FSO-2.11`.
