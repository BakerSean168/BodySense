# Full Stack Open source-course spine

> Generated from the curriculum ledger plus the pinned Part metadata. This file is the learner-facing authority for what “Part N / 第 N 章” means inside the BodySense Master Course.

## Numbering invariant

- `Full Stack Open Part N` / “第 N 章” in FSO context always resolves to the source-course part below.
- A `Study Track` is a topical lens over the ready dependency graph. It is **not** a numbered chapter and must be named as a Track.
- `BS-A1..A8` are BodySense Agent extension modules, not FSO Parts.
- A numbered section such as `§8` inside an architecture document is only a document section.
- If the requested FSO part has no exercise-ready node, coaching must say so and prepare that part; it must not silently substitute a different ready topic.

> **Hard check:** Full Stack Open **Part 8 = GraphQL**. SSE is only relevant there when comparing GraphQL subscriptions/WebSockets with BodySense realtime transport. “Part 8 = SSE event contract design” is invalid navigation.

| FSO Part | Canonical topic | Current exercise records | Ready/verified learning nodes | Learner verified | BodySense note |
|---:|---|---:|---:|---:|---|
| 0 | Fundamentals of Web apps | 6 | 5 | 1 |  |
| 1 | Introduction to React | 14 | 10 | 0 |  |
| 2 | Communicating with server | 20 | 7 | 0 |  |
| 3 | Programming a server with NodeJS and Express | 22 | 5 | 2 | First substantial backend + persistence practice; the source course uses Node/Express and MongoDB. |
| 4 | Testing Express servers, user administration | 23 | 2 | 0 |  |
| 5 | Testing React apps | 31 | 15 | 1 |  |
| 6 | Advanced state management | 22 | 3 | 3 |  |
| 7 | React router, custom hooks, tooling and advanced frontend | 20 | 17 | 2 | Realtime/server-push is introduced as an architecture comparison here. BodySense SSE belongs here or in Agent/runtime work, not as the title of Part 8. |
| 8 | GraphQL | 30 | 12 | 0 | Canonical Part 8 topic. BodySense uses an isolated GraphQL/Apollo Server + Apollo Client lab plus comparisons with REST, TanStack Query and SSE; production migration is not required. |
| 9 | TypeScript | 35 | 7 | 6 | TypeScript/runtime-trust practice. |
| 10 | React Native | 30 | 0 | 0 |  |
| 11 | CI/CD | 24 | 5 | 0 |  |
| 12 | Containers | 25 | 5 | 0 |  |
| 13 | Relational databases | 28 | 8 | 0 | Canonical relational-database part; BodySense maps it to PostgreSQL/GORM/SQL behavior. |
| 14 | Next.js | 26 | 0 | 0 |  |

## Part 8 executable spine

The current learner-facing Part 8 track is `fso-part-8-graphql`. Its core sequence is:

```text
Schema / query selection
-> Apollo Server execution
-> resolver args / request context
-> mutation + domain errors
-> Apollo Client vs TanStack Query
-> variables + normalized cache
-> mutation cache reconciliation
-> auth context
-> subscriptions vs BodySense SSE
-> N+1 / database query cost
```

The source catalog still contains every mapped Part 8 exercise/section. This spine only identifies the high-value executable path; it does not claim that twelve ready cards replace all source-course material.

## Database placement

- Part 3 introduces backend persistence in the original course through MongoDB/Mongoose concepts.
- Part 13 is the dedicated relational-database part. BodySense preserves the concepts while using PostgreSQL/GORM rather than forcing Sequelize into production.
- TECH SCHOOL backend exercises provide the deeper SQL/transaction/lock/isolation path beside FSO Part 13.
