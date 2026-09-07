# Curriculum Sources, authority and refresh policy

This file records what source material has actually been obtained. Missing semantic review is represented as a mapping gap rather than filled from memory.

## Full Stack Open

- Publisher: University of Helsinki.
- Public course: `https://fullstackopen.com/`.
- License of the public course material: Creative Commons BY-NC-SA 3.0.

### Parts 0-7 — pinned course-repository snapshot

- Indexed repository: `fullstack-hy2020/fullstack-hy2020.github.io`.
- Pinned commit: `0711aef8a451c4458263e5587ccda85f08fd7a96`.
- Current active source inventory: 158 distinct numbered exercises and 279 level-3 section headings for Parts 0-7 under the audit counting rule.
- All 158 numbered exercises have individual semantic BodySense mappings. Section-heading inventory is used for omission detection and is not treated as exhaustive paragraph-level knowledge parity.

### Parts 8-14 — current MOOC Course Material API snapshot

The current Full Stack Open advanced parts are served from `courses.mooc.fi`. A public Course Material API is available without copying the rendered course body into BodySense.

Committed metadata snapshot:

- file: `ledger/full-stack-open-current-mooc.json`;
- retrieval time and source-state SHA-256: recorded inside the snapshot itself and repeated in the generated `views/coverage-status.md`;
- scope: course/chapter/page identifiers, page/section headings, exercise UUIDs and short exercise titles only;
- deliberately excluded: exercise assignment bodies, answers/model solutions and course prose.

| Part | MOOC slug | Current exercise records | Current headings |
|---:|---|---:|---:|
| 8 | `full-stack-open-graphql` | 30 | 51 |
| 9 | `full-stack-open-typescript` | 35 | 64 |
| 10 | `full-stack-open-react-native` | 30 | 56 |
| 11 | `full-stack-open-continuous-integration` | 24 | 64 |
| 12 | `full-stack-open-containers` | 25 | 42 |
| 13 | `full-stack-open-relational-databases` | 28 | 43 |
| 14 | `full-stack-open-nextjs` | 26 | 65 |
| **Total** | | **198** | **385** |

These counts are **platform exercise records**, including warmups or unnamed records where the MOOC platform exposes them. They must not be silently reinterpreted as the number of graded assignments published in prose.

Source-integrity state for Parts 8-14 is `VERIFIED_CURRENT_MOOC_API_INDEX`. The **198 current exercise records have also completed exercise-level semantic review and BodySense mapping**. Exercise readiness, section/prose concept parity and learner mastery remain separate later states.

The older repository snapshot's Parts 8-11 are retained in `historical_items` (104 exercises, 132 headings) for change comparison only; they are not current-course evidence.

Refresh command:

```bash
pnpm curriculum:refresh-fso-sources
```

The refresh script rate-limits API requests, writes a new metadata snapshot/fingerprint, syncs source records without overwriting an existing current exercise's mapping/mastery state, then runs deterministic curriculum validation.

## TECH SCHOOL Backend Master Class

- Public repository: `techschool/simplebank`.
- Pinned public README commit: `97f000fe58ad01a0774179ffa8884ac7784cf263`.
- Public README at that commit lists backend Lecture #0 through Lecture #77.
- Repository code license: MIT.

Authority limit:

```text
VERIFIED_PUBLIC_README_TITLE_ONLY
```

The curriculum may claim 78/78 public lecture ID/title indexing and BodySense title-level mapping. It must **not** claim that all paid/video-internal subtopics, examples or demonstrations were audited unless the course content is separately obtained and reviewed.

## BodySense Agent extension

The Agent extension is project-defined rather than imported from a third-party syllabus. Its source authority is current code/tests + accepted architecture/ADR material, with prior Diagnosis learning evidence retained as historical mastery evidence rather than external source coverage.

## Refresh policy

Before changing a source-completeness claim:

1. pin or fingerprint the exact source state;
2. update source IDs/counts without silently deleting historical items;
3. keep source indexing separate from semantic mapping;
4. only promote a mapped node to `EXERCISE_READY` when target files, failure cases, verification and dependencies are reviewed;
5. never convert source availability or pre-existing production code into learner mastery.

Quarterly source refreshes may add/change mappings, but completed learner evidence is invalidated only if the underlying concept or acceptance contract materially changes.
