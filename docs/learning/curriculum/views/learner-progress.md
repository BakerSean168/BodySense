# Learner progress by canonical source

> Generated from canonical curriculum ledgers and `learner-placement.json`. Do not hand-edit mastery counts here.
> Progress is keyed by source item IDs and canonical Full Stack Open Parts, not by conversational labels such as “第五章/第六章/第七章”.

Active learning track: **fso-part-8-graphql**.

## Full Stack Open progress

| Part | Canonical topic | Assessed | Verified | Gaps |
|---:|---|---:|---:|---:|
| 0 | Fundamentals of Web apps | 4 | 1 | 3 |
| 1 | Introduction to React | 0 | 0 | 0 |
| 2 | Communicating with server | 1 | 0 | 1 |
| 3 | Programming a server with NodeJS and Express | 4 | 2 | 2 |
| 4 | Testing Express servers, user administration | 2 | 0 | 2 |
| 5 | Testing React apps | 1 | 1 | 0 |
| 6 | Advanced state management | 3 | 3 | 0 |
| 7 | React router, custom hooks, tooling and advanced frontend | 5 | 2 | 3 |
| 8 | GraphQL | 0 | 0 | 0 |
| 9 | TypeScript | 7 | 6 | 1 |
| 10 | React Native | 0 | 0 | 0 |
| 11 | CI/CD | 0 | 0 | 0 |
| 12 | Containers | 0 | 0 | 0 |
| 13 | Relational databases | 0 | 0 | 0 |
| 14 | Next.js | 0 | 0 | 0 |

### Part 0 · Fundamentals of Web apps

- **L3 GAP** · `BS-FSO-0.1` · inspect the semantic HTML structure of a rendered page
- **L4 VERIFIED** · `BS-FSO-0.3` · trace a form from fields through submission
- **L3 GAP** · `BS-FSO-0.4` · model a traditional form mutation as a browser/server sequence
- **L3 GAP** · `BS-FSO-0.5` · model initial single-page-app loading and data fetches

### Part 2 · Communicating with server

- **L3 GAP** · `BS-FSO-2.11` · initialize client state from a backend resource

### Part 3 · Programming a server with NodeJS and Express

- **L4 VERIFIED** · `BS-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY` · HTTP method safety and idempotency as API contract properties
- **L3 GAP** · `BS-P3-CONCEPT-MIDDLEWARE-CHAIN` · HTTP middleware chain and request/response cross-cutting concerns
- **L4 VERIFIED** · `BS-P3-CONCEPT-SAME-ORIGIN-CORS` · browser same-origin policy and CORS response authorization
- **L3 GAP** · `BS-P3-CONCEPT-HTTP-ERROR-TAXONOMY` · HTTP error taxonomy: malformed input, invalid identifiers, absent resources and internal failures

### Part 4 · Testing Express servers, user administration

- **L3 GAP** · `BS-P4-CONCEPT-BEARER-AUTHORIZATION` · bearer token authentication resolves a principal before protected resource mutation
- **L3 GAP** · `BS-P4-CONCEPT-TOKEN-REVOCATION` · token/session revocation, expiry and replay handling are part of authentication authority

### Part 5 · Testing React apps

- **L4 VERIFIED** · `BS-P5-CONCEPT-BROWSER-TOKEN-PERSISTENCE` · browser credential persistence trade-offs: localStorage versus in-memory access token and cookie-backed refresh

### Part 6 · Advanced state management

- **L4 VERIFIED** · `BS-P6-CONCEPT-TANSTACK-QUERY` · TanStack Query owns remote cache, query lifecycle, deduplication and refetch policy
- **L4 VERIFIED** · `BS-P6-CONCEPT-QUERY-MUTATION-INVALIDATION` · TanStack Query mutations synchronize remote changes through precise invalidation or cache updates
- **L4 VERIFIED** · `BS-P6-CONCEPT-STATE-OWNERSHIP-CHOICE` · choose local state, URL, Context, Zustand or TanStack Query based on ownership/lifetime/synchronization

### Part 7 · React router, custom hooks, tooling and advanced frontend

- **L3 GAP** · `BS-P7-CONCEPT-XSS` · cross-site scripting and safe rendering boundaries
- **L3 GAP** · `BS-P7-CONCEPT-DEPENDENCY-SECURITY` · dependency auditing, lockfiles, and supply-chain risk
- **L3 GAP** · `BS-P7-CONCEPT-BROKEN-AUTHZ` · broken authentication/access control and server-side authorization
- **L4 VERIFIED** · `BS-P7-CONCEPT-SERVER-PUSH-SYNC` · client/server synchronization may use polling, WebSockets, SSE or subscriptions depending on direction/reliability needs
- **L4 VERIFIED** · `BS-P7-CONCEPT-SECURITY-HEADERS` · HTTP response security headers provide browser-enforced defense in depth for content execution, framing, MIME handling, referrers and transport

### Part 9 · TypeScript

- **L4 VERIFIED** · `BS-P9-CONCEPT-STRUCTURAL-TYPING` · TypeScript compatibility is structural: values satisfy required shape regardless of nominal declaration identity
- **L4 VERIFIED** · `BS-P9-CONCEPT-TYPE-ERASURE` · TypeScript type information is erased at runtime and cannot validate untrusted values by itself
- **L4 VERIFIED** · `BS-P9-CONCEPT-UNKNOWN-NARROWING` · unknown is the safe top type for uncertain values and must be narrowed before operations
- **L3 GAP** · `BS-P9-CONCEPT-DISCRIMINATED-UNIONS` · discriminated unions model variant-specific data while preserving shared fields and safe narrowing
- **L4 VERIFIED** · `BS-P9-CONCEPT-EXHAUSTIVE-NARROWING` · control-flow narrowing plus exhaustive switch checks make unhandled union variants visible at compile time
- **L4 VERIFIED** · `BS-P9-CONCEPT-TYPED-SERVER-DATA` · typing an HTTP client response does not validate the network payload; runtime parsing is required at trust boundaries
- **L4 VERIFIED** · `BS-P9-CONCEPT-SCHEMA-VALIDATION` · schema validators provide runtime parsing/error reporting and can align inferred static types with trusted parsed output

## TECH SCHOOL Backend

Assessed: **4** · verified: **4** · gaps: **0**.

- **L4 VERIFIED** · `BS-TECH-20` · How to create and verify JWT & PASETO token in Golang
- **L4 VERIFIED** · `BS-TECH-21` · Implement login user API that returns PASETO or JWT access token in Go
- **L4 VERIFIED** · `BS-TECH-22` · Implement authentication middleware and authorization rules in Golang using Gin
- **L4 VERIFIED** · `BS-TECH-37` · How to manage user session with refresh token - Golang

## BodySense Agent extension

Assessed: **1** · verified: **1** · gaps: **0**.

- **L4 VERIFIED** · `BS-A7` · Streaming, interrupt/resume and HITL

## Recent assessment timeline

> Times are Asia/Shanghai. This timeline is useful for reconciling old conversational chapter labels with the canonical item IDs that actually received mastery evidence.

- 2026-09-09 13:01 · **L4** · `FSO-P9-CONCEPT-STRUCTURAL-TYPING` · placement
- 2026-09-09 13:54 · **L4** · `FSO-P9-CONCEPT-TYPE-ERASURE` · placement
- 2026-09-09 15:05 · **L4** · `FSO-P9-CONCEPT-UNKNOWN-NARROWING` · placement
- 2026-09-09 17:43 · **L4** · `FSO-P9-CONCEPT-EXHAUSTIVE-NARROWING` · placement
- 2026-09-10 09:43 · **L4** · `FSO-0.3` · placement
- 2026-09-10 10:47 · **L3** · `FSO-0.1` · prior-evidence-review
- 2026-09-10 10:47 · **L3** · `FSO-0.4` · placement
- 2026-09-10 10:47 · **L3** · `FSO-P9-CONCEPT-DISCRIMINATED-UNIONS` · prior-evidence-review
- 2026-09-10 11:37 · **L3** · `FSO-0.5` · placement
- 2026-09-10 12:14 · **L3** · `FSO-2.11` · placement
- 2026-09-10 12:33 · **L4** · `FSO-P9-CONCEPT-TYPED-SERVER-DATA` · placement
- 2026-09-10 12:46 · **L4** · `FSO-P9-CONCEPT-SCHEMA-VALIDATION` · placement
- 2026-09-10 13:03 · **L4** · `FSO-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY` · placement
- 2026-09-10 13:19 · **L3** · `FSO-P3-CONCEPT-MIDDLEWARE-CHAIN` · placement
- 2026-09-10 13:51 · **L4** · `FSO-P3-CONCEPT-SAME-ORIGIN-CORS` · placement
- 2026-09-10 14:00 · **L3** · `FSO-P3-CONCEPT-HTTP-ERROR-TAXONOMY` · placement
- 2026-09-10 14:22 · **L4** · `TECH-20` · placement
- 2026-09-10 14:34 · **L4** · `TECH-21` · placement
- 2026-09-10 14:58 · **L4** · `TECH-22` · placement
- 2026-09-10 15:13 · **L4** · `TECH-37` · placement
- 2026-09-10 15:31 · **L3** · `FSO-P4-CONCEPT-BEARER-AUTHORIZATION` · placement
- 2026-09-10 15:31 · **L3** · `FSO-P4-CONCEPT-TOKEN-REVOCATION` · placement
- 2026-09-10 16:09 · **L4** · `FSO-P5-CONCEPT-BROWSER-TOKEN-PERSISTENCE` · placement
- 2026-09-10 16:33 · **L3** · `FSO-P7-CONCEPT-BROKEN-AUTHZ` · placement
- 2026-09-10 16:33 · **L3** · `FSO-P7-CONCEPT-DEPENDENCY-SECURITY` · placement
- 2026-09-10 18:04 · **L4** · `FSO-P7-CONCEPT-SECURITY-HEADERS` · placement
- 2026-09-10 18:08 · **L3** · `FSO-P7-CONCEPT-XSS` · placement
- 2026-09-10 19:19 · **L4** · `FSO-P6-CONCEPT-STATE-OWNERSHIP-CHOICE` · placement
- 2026-09-10 19:32 · **L4** · `FSO-P6-CONCEPT-TANSTACK-QUERY` · placement
- 2026-09-10 20:48 · **L4** · `FSO-P6-CONCEPT-QUERY-MUTATION-INVALIDATION` · placement
- 2026-09-10 21:02 · **L4** · `FSO-P7-CONCEPT-SERVER-PUSH-SYNC` · placement
- 2026-09-10 22:05 · **L4** · `BS-A7` · placement

## Reconciliation rule

If an old conversation called a study block “Chapter N” but the recorded item IDs belong to another canonical Part, keep the mastery evidence on those canonical IDs and treat the old chapter label as a navigation error. Do **not** duplicate the evidence onto the incorrectly named Part and do **not** force the learner to repeat verified work.
