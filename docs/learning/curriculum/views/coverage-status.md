# BodySense Master Course · Coverage Status

> Generated from machine-readable ledgers. Do not hand-edit counts in this file.
> Baseline date: 2026-09-07

## What the numbers mean

Lifecycle is monotonic:

`SOURCE_INDEXED -> MAPPED -> EXERCISE_READY -> LEARNER_VERIFIED`

- **SOURCE_INDEXED**: source item has a stable identifier/location, but no coverage claim beyond that source inventory.
- **MAPPED**: source objective has a BodySense DIRECT / COMPARE / OPTIONAL mapping.
- **EXERCISE_READY**: the exercise has concrete targets, prediction, task, failure case, verification evidence and an L4 gate.
- **LEARNER_VERIFIED**: the learner has actually met the required mastery level with recorded evidence.

These states intentionally separate source indexing, curriculum design, executable practice and actual learning.

## Full Stack Open

Snapshot exercise inventory for Parts 0-11: **262** distinct numbered exercises.

| Part | Indexed exercises | Mapped | Exercise ready | Learner verified | Source authority |
|---:|---:|---:|---:|---:|---|
| 0 | 6 | 6 | 5 | 0 | current-site snapshot |
| 1 | 14 | 14 | 0 | 0 | current-site snapshot |
| 2 | 20 | 20 | 2 | 0 | current-site snapshot |
| 3 | 22 | 22 | 1 | 0 | current-site snapshot |
| 4 | 23 | 23 | 0 | 0 | current-site snapshot |
| 5 | 31 | 31 | 0 | 0 | current-site snapshot |
| 6 | 22 | 22 | 0 | 0 | current-site snapshot |
| 7 | 20 | 20 | 0 | 0 | current-site snapshot |
| 8 | 26 | 0 | 0 | 0 | historical snapshot; current MOOC pending |
| 9 | 30 | 0 | 0 | 0 | historical snapshot; current MOOC pending |
| 10 | 27 | 0 | 0 | 0 | historical snapshot; current MOOC pending |
| 11 | 21 | 0 | 0 | 0 | historical snapshot; current MOOC pending |

Current-site core (Parts 0-7): **158/158 source exercises semantically mapped**. This is mapping coverage, not completed learning.

Parts 8-11: **104 historical exercise IDs indexed** from commit `0711aef8a451c4458263e5587ccda85f08fd7a96`; current MOOC parity remains unverified and therefore is not reported as current-course coverage.

Parts 12-14: current MOOC source is explicitly recorded as **UNVERIFIED_CURRENT_MOOC** until a reproducible source snapshot is obtained.

Concept inventory: **411 source section headings indexed** from the snapshot; **10 high-risk/previously-missed concepts semantically decomposed and mapped** (async runtime, Promise, Effect lifecycle, memoization/reference stability, injection, XSS, dependency security, broken auth/access control). Exhaustive paragraph-level concept parity is **not yet claimed**.

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

The next implementation milestone is to expand **EXERCISE_READY** coverage from this spine while continuing the current-MOOC source audit. Placement assessment starts only after the prerequisite slice being assessed is exercise-ready.
