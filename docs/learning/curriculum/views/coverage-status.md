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
| 0 | 6 | 6 | 5 | 0 | pinned course-repository snapshot |
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

Advanced Parts 8-14: **198/198 current exercise records semantically mapped** after indexing from the public courses.mooc.fi Course Material API. Snapshot retrieval: `2026-09-07T15:30:35.638Z`; source-state SHA-256: `4a8093083af985c19e04b03bf876fd148676bd6c2b65327c7628d8b20122689d`. Mapping is complete at exercise-objective level; exercise readiness and concept-level parity remain separate.

The previous repository snapshot's Parts 8-11 are retained only as **104 historical exercise records** for comparison; they are not counted as current-course parity.

Current source-section inventory: **279 core headings (Parts 0-7)** + **385 current MOOC headings (Parts 8-14)**. **228 explicit section-derived concepts** have already been semantically decomposed and mapped.

Section semantic-audit disposition: **279/664** reviewed; **227** concept-mapped, **29** explicitly redundant, **23** non-engineering/logistics, **385** pending. Heading review improves omission detection but does not equal exhaustive paragraph-level prose parity.

The MOOC metadata snapshot intentionally stores only identifiers, page/chapter metadata, headings and short exercise titles; it does not copy exercise assignments, answers or course prose.

## TECH SCHOOL Backend Master Class

Public README baseline: `97f000fe58ad01a0774179ffa8884ac7784cf263`.

- Public lecture IDs/titles indexed: **78/78**.
- Title-level BodySense mappings: **78/78**.
- Exercise-ready lecture reps: **16/78**.
- Learner-verified lecture reps: **0/78**.

This does **not** claim that paid/video-internal teaching semantics were audited. The source authority is the public README title/index only.

## BodySense Agent extension

- Project-defined modules mapped: **8/8**.
- Exercise-ready: **3/8**.
- Learner-verified: **0/8**.

## Exercise-ready spine

There are currently **27** executable cards and **0** learner-verified cards.

- `BS-FSO-0.1` -> [card](../../exercises/bs-fso-0-1.md)
- `BS-FSO-0.3` -> [card](../../exercises/bs-fso-0-3.md)
- `BS-FSO-0.4` -> [card](../../exercises/bs-fso-0-4.md)
- `BS-FSO-0.5` -> [card](../../exercises/bs-fso-0-5.md)
- `BS-FSO-0.6` -> [card](../../exercises/bs-fso-0-6.md)
- `BS-FSO-2.11` -> [card](../../exercises/bs-fso-2-11.md)
- `BS-FSO-2.17` -> [card](../../exercises/bs-fso-2-17.md)
- `BS-FSO-3.1` -> [card](../../exercises/bs-fso-3-1.md)
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
- `BS-A7` -> [card](../../exercises/bs-a7.md)

The next curriculum milestone is concept-level semantic audit plus expansion of `EXERCISE_READY` coverage while preserving prerequisite closure. Placement assessment starts only on ready prerequisite slices.
