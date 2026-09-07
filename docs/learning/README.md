# BodySense Master Course

> Status: **curriculum reconstruction in progress**
> Baseline date: 2026-09-07
> Practice application: the production-shaped BodySense repository itself
> Coverage source of truth: `docs/learning/curriculum/ledger/*.json`
> Learner-progress source of truth: `.practice-map/maps/bodysense-fundamentals.md`

## Purpose

BodySense is the single long-lived practice project. Phonebook, Blog List and Simple Bank are source-course teaching domains, not additional repositories that must be maintained.

The external syllabus backbone is:

1. **Full Stack Open** for browser/Web/React/server communication/testing/state/TypeScript/CI/container/database pedagogy.
2. **TECH SCHOOL Backend Master Class** for Go/PostgreSQL depth: schema, transactions, locks, isolation, API design, testing, auth, async work and production hardening.
3. **BodySense Agent Engineering** for typed Agents, runtime ownership, evidence, safety, evaluation, replay, HITL and production debugging that the two external courses do not cover deeply enough.

## Coverage is a lifecycle, not one percentage

Every source item moves through four different states:

```text
SOURCE_INDEXED
  -> MAPPED
  -> EXERCISE_READY
  -> LEARNER_VERIFIED
```

- `SOURCE_INDEXED`: the source item has a stable identifier/location. No learning-coverage claim is implied yet.
- `MAPPED`: the source objective has a `DIRECT`, `COMPARE` or `OPTIONAL` BodySense mapping.
- `EXERCISE_READY`: the rep has concrete target files, prerequisites, prediction, task, failure case, verification evidence and acceptance criteria.
- `LEARNER_VERIFIED`: the learner actually reached the required mastery with recorded evidence.

This separation prevents four different facts from being conflated:

```text
source exists
!= mapping exists
!= exercise is executable
!= learner has mastered it
```

Current generated counts are in [`curriculum/views/coverage-status.md`](./curriculum/views/coverage-status.md).

## Current source-integrity boundary

As of the pinned 2026-09-07 audit:

- Full Stack Open Parts **0-7**: 158 numbered source exercises are individually indexed and semantically mapped to BodySense.
- Full Stack Open Parts **8-14**: the public `courses.mooc.fi` Course Material API has been indexed into a metadata-only snapshot: **198 current exercise records and 385 current section headings**. These are source-integrity records, not yet semantic mappings.
- The former repository-snapshot records for Parts **8-11** are retained as **104 historical exercises** for comparison and are not counted as current parity.
- TECH SCHOOL Backend #0-#77: 78/78 public README lecture IDs/titles are pinned and mapped. This is **public title-level parity only**, not a claim that paid/video-internal teaching semantics were audited.
- FSO concept coverage is tracked separately from numbered exercises. Source section headings are indexed and high-risk previously-missed concepts (Promises/Effects, memoization, XSS/injection, dependency security, access control) now have explicit records; exhaustive paragraph-level semantic parity is not yet claimed.

Exact source commits and unresolved source gaps are recorded in [`curriculum/SOURCES.md`](./curriculum/SOURCES.md).

## Mapping modes

- `DIRECT`: reproduce the engineering objective against current BodySense.
- `COMPARE`: preserve the source design/technology as a structured comparison or isolated spike; do not force a production migration.
- `OPTIONAL`: explicitly retain a different product-surface extension such as React Native/Next.js.

Nothing should be silently omitted. A source gap remains a visible source gap instead of being filled by guesswork.

## Mastery gate

Evidence artifacts are **not** interchangeable with mastery.

Core exercises require at least **L4 Verify**:

- `L1 Recognize`: locate the concept.
- `L2 Explain`: explain the path/ownership.
- `L3 Predict`: predict normal and failure behavior before execution.
- `L4 Verify`: design/select a test, trace or experiment that can prove or falsify the prediction.
- `L5 Change`: safely modify the boundary while preserving invariants.

A diagram plus a prediction can be useful evidence, but by themselves do not pass an L4 core exercise. Independent delivery/capstone work requires **L5**.

See [`curriculum/04-learning-protocol.md`](./curriculum/04-learning-protocol.md).

## Repository layout

```text
docs/learning/
├── README.md
├── curriculum/
│   ├── SOURCES.md
│   ├── 01-full-stack-open-parity.md
│   ├── 02-techschool-backend-parity.md
│   ├── 03-bodysense-agent-extension.md
│   ├── 04-learning-protocol.md
│   ├── ledger/
│   │   ├── schema-v1.json
│   │   ├── full-stack-open.json
│   │   ├── full-stack-open-current-mooc.json
│   │   ├── techschool-backend.json
│   │   └── bodysense-agent.json
│   └── views/
│       ├── coverage-status.md
│       └── prerequisite-spine.md
└── exercises/
    └── bs-*.md
```

The JSON ledgers are canonical for source IDs, mapping state, exercise readiness and mastery evidence. Markdown parity files are human-readable views/policy notes and must not invent counts that disagree with the ledgers. Prerequisite edges become authoritative only when `dependency_audit` is `REVIEWED`; unready mappings explicitly keep their dependency graph unmodeled instead of treating source order as proof of prerequisite semantics.

Current advanced FSO source metadata can be refreshed explicitly with:

```bash
pnpm curriculum:refresh-fso-sources
```

This is intentionally **not** part of ordinary `curriculum:check`, so validation remains deterministic/offline against the committed source snapshot.

## Current execution order

Do **not** jump directly to Treatment or placement merely because an old roadmap named it next.

The current order is:

```text
1. Source integrity / source gaps
2. Source -> BodySense semantic mapping (current advanced MOOC Parts 8-14 are the largest remaining mapping gap)
3. Make the prerequisite slice EXERCISE_READY
4. Placement audit on that ready slice
5. Start from the first prerequisite gap below L4
6. Continue through the dependency graph
```

The first executable spine is intentionally small and high-value: HTTP/SPA tracing, stale mutation handling, transaction/lock/isolation, REST/error boundaries, refresh-token races, durable jobs and streaming/replay. It is listed in the generated coverage view.
