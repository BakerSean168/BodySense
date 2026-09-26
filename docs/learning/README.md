# BodySense Master Course

> Status: **curriculum reconstruction in progress**
> Baseline date: 2026-09-08
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

Browse every mapped training point without opening JSON through:

- [`Full Stack Open source-course spine`](./curriculum/views/course-spine.md) — canonical meaning of `Part N / 第 N 章`; **Part 8 is GraphQL**
- [`Full Stack Open catalog`](./curriculum/views/full-stack-open-catalog.md)
- [`TECH SCHOOL backend catalog`](./curriculum/views/techschool-backend-catalog.md)
- [`Agent engineering catalog`](./curriculum/views/agent-engineering-catalog.md)
- [`Concept semantic-audit queue`](./curriculum/views/concept-audit-queue.md)
- [`Pinned-core nested-heading audit`](./curriculum/views/core-subheading-audit.md) — h4-h6 coverage for Parts 0-7, including alternative/removed exercise tracks
- [`Pinned-core targeted prose-risk audit`](./curriculum/views/core-prose-risk-audit.md) — fingerprinted full-fragment review of selected high-risk h3 sections; explicitly non-exhaustive
- [`Exercise-ready study tracks`](./curriculum/views/study-tracks.md) — the recommended learner-facing path through the current ready graph
- [`Learner placement status`](./curriculum/views/placement-status.md) — active track, prerequisite closure, assessed gaps/verification and the next placement target
- [`Learner progress by canonical source`](./curriculum/views/learner-progress.md) — reconciles mastery to canonical FSO Parts regardless of old conversational chapter labels

## Current source-integrity boundary

As of the 2026-09-08 curriculum audit (with the advanced MOOC source snapshot pinned on 2026-09-07):

- Full Stack Open Parts **0-7**: 158 numbered source exercises are individually indexed and semantically mapped to BodySense.
- Full Stack Open Parts **8-14**: the public `courses.mooc.fi` Course Material API is pinned as a metadata-only snapshot: **198 current exercise records and 385 current section headings**. All **198/198 current exercise records are mapped**, and all **385/385 current advanced section-heading review units are dispositioned** against BodySense (`DIRECT` / `COMPARE` / `OPTIONAL`, redundant, or non-engineering).
- Full Stack Open section/concept audit: **664/664 current section-heading review units across Parts 0-14 are dispositioned**. The ledger currently contains **469 explicit section/subheading/prose-derived concept records**; redundant and course-logistics headings are recorded explicitly instead of silently skipped. A separate pinned-core audit dispositioned **196/196 h4-h6 headings**: 118 current exercise detail headings, 4 current alternative exercise variants, 32 nested technical mappings, 6 redundant items, 17 non-engineering headings and 19 headings from a source track explicitly marked removed. A fingerprinted targeted prose-risk layer has also reviewed **10/10 selected high-risk h3 fragments**, exposing 2 previously hidden concepts (`Testing Library` query timing/absence variants and browser security headers). This is deliberately risk-selected and still does **not** claim exhaustive paragraph/example parity.
- BodySense Agent extension: **8/8 modules are now `EXERCISE_READY`**; A3-A6/A8 add executable evidence/admissibility, deterministic authority, qualification/rollout, replay/provenance and production failure-attribution labs.
- The former repository-snapshot records for Parts **8-11** are retained as **104 historical exercises** for comparison and are not counted as current parity.
- TECH SCHOOL Backend #0-#77: 78/78 public README lecture IDs/titles are pinned and mapped. This is **public title-level parity only**, not a claim that paid/video-internal teaching semantics were audited.
- FSO concept coverage is tracked separately from numbered exercises. The core 0-7 section audit now includes explicit mappings for browser/runtime/React/backend/persistence/testing/state/build/security concepts, while redundant and non-engineering sections are dispositioned explicitly rather than silently skipped. Exhaustive paragraph-level semantic parity is still not claimed.

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
│   │   ├── full-stack-open-concept-audit.json
│   │   ├── full-stack-open-core-subheading-audit.json
│   │   ├── full-stack-open-core-prose-risk-audit.json
│   │   ├── learner-placement.json
│   │   ├── techschool-backend.json
│   │   └── bodysense-agent.json
│   ├── placement/
│   │   └── README.md
│   └── views/
│       ├── coverage-status.md
│       ├── prerequisite-spine.md
│       ├── course-spine.md
│       ├── full-stack-open-catalog.md
│       ├── techschool-backend-catalog.md
│       ├── agent-engineering-catalog.md
│       ├── concept-audit-queue.md
│       ├── core-subheading-audit.md
│       ├── core-prose-risk-audit.md
│       ├── study-tracks.md
│       └── placement-status.md
└── exercises/
    └── bs-*.md
```

The JSON ledgers are canonical for source IDs, mapping state, exercise readiness and mastery evidence. Markdown parity files are human-readable views/policy notes and must not invent counts that disagree with the ledgers. Prerequisite edges become authoritative only when `dependency_audit` is `REVIEWED`; unready mappings explicitly keep their dependency graph unmodeled instead of treating source order as proof of prerequisite semantics.

Current advanced FSO source metadata can be refreshed explicitly with:

```bash
pnpm curriculum:refresh-fso-sources
```

Pinned-core nested/prose audit fingerprints can be refreshed only against a checkout at the exact pinned commit:

```bash
pnpm curriculum:refresh-fso-core-subheadings -- /path/to/fullstack-checkout
pnpm curriculum:refresh-fso-core-prose-risk -- /path/to/fullstack-checkout
```

These refresh commands update source fingerprints/inventory only; changed fragments return to `PENDING` until semantic review is repeated. They are intentionally **not** part of ordinary `curriculum:check`, so validation remains deterministic/offline against the committed source snapshots.

Placement is now machine-backed as well:

```bash
pnpm curriculum:select-track -- typescript-runtime-trust
pnpm curriculum:placement
node scripts/learning/record-placement.mjs <ITEM_ID> <L1|L2|L3|L4|L5> --evidence "learner evidence"
pnpm curriculum:check
```

`record-placement.mjs` can be run with `--dry-run` to preview lifecycle/mastery effects. No placement result is written without explicit evidence, and existing production code/tests are never converted into mastery automatically.

## Course numbering and navigation

Inside the BodySense Master Course, source-course numbering and learner-facing tracks are deliberately different namespaces:

```text
Full Stack Open Part 8 = GraphQL
Study Track = topical ready-graph lens, never a numbered chapter
BS-A1..A8 = BodySense Agent extension modules
Architecture doc §8 = document section only
```

If the learner says “进入第八章” while following Full Stack Open, coaching must resolve that to **Part 8 GraphQL**. It must not infer chapter numbers from the ordinal position of a Study Track, from Agent module numbers, or from a numbered architecture section. If a requested Part is not exercise-ready, say so explicitly and prepare that Part rather than silently substituting another ready topic. See [`course-spine.md`](./curriculum/views/course-spine.md).

## Current execution order

Do **not** jump directly to Treatment or placement merely because an old roadmap named it next.

The current order is:

```text
1. Source integrity is pinned; exercise-objective + section-heading mapping is complete across current FSO Parts 0-14
2. Expand the fingerprinted targeted prose-risk audit beyond the first 10/10 reviewed high-risk core sections; primary and nested heading inventories are already fully dispositioned
3. Expand high-value mapped nodes to EXERCISE_READY with reviewed prerequisite closure
4. Use the generated study tracks to choose a coherent ready slice
5. Placement audit only on that ready slice
6. Start from the first prerequisite gap below L4 and continue through the dependency graph
```

The executable curriculum size and current mastery are generated in [`coverage-status.md`](./curriculum/views/coverage-status.md); do not copy those changing counts into this README. Rather than asking the learner to navigate the graph manually, [`study-tracks.md`](./curriculum/views/study-tracks.md) groups ready nodes into named Tracks, including a dedicated `fso-part-8-graphql` track. Track headings are intentionally **not numbered** so “Track 8” can never be confused with “Full Stack Open Part 8”. The track generator fails if any ready node is omitted from the learner-facing map. Placement state is tracked separately in `learner-placement.json`; the current active placement slice is always generated in [`placement-status.md`](./curriculum/views/placement-status.md).
