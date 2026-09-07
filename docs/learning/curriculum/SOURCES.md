# Curriculum Sources, authority and refresh policy

This file records what source material has actually been obtained. Missing source material is represented as a source gap rather than reconstructed from memory.

## Full Stack Open

- Publisher: University of Helsinki.
- Public course: `https://fullstackopen.com/`
- License of the public course material: Creative Commons BY-NC-SA 3.0.
- Indexed repository: `fullstack-hy2020/fullstack-hy2020.github.io`.
- Pinned indexed commit: `0711aef8a451c4458263e5587ccda85f08fd7a96` (used in the 2026-09-07 audit).

### Authority matrix

| Parts | Source obtained | What may be claimed |
|---|---|---|
| 0-7 | full English snapshot content at pinned commit | numbered exercise index + reviewed source-exercise semantics can be mapped |
| 8-11 | numbered exercise/content snapshot exists at pinned commit, but official course now routes these advanced parts through MOOC course instances | historical snapshot inventory only until current MOOC material is pinned/verified |
| 12-13 | pinned repository files are migration notices to the MOOC platform rather than the full course body | current source remains `UNVERIFIED_CURRENT_MOOC`; no exercise/knowledge completeness claim |
| 14 | current course index points to a Next.js MOOC instance; reproducible full body not captured by this audit | current source remains `UNVERIFIED_CURRENT_MOOC` |

The machine ledger contains 262 distinct numbered snapshot exercises for Parts 0-11 using the audit's de-duplication rule. Parts 0-7 account for 158 of them.

The ledger also indexes source section headings separately. Section-heading inventory improves omission detection but does not automatically equal exhaustive semantic knowledge-point decomposition.

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

1. pin the exact source commit/version/URL;
2. update the corresponding ledger source metadata;
3. validate source IDs/counts without silently deleting old items;
4. separately update semantic mapping state;
5. only then promote exercises to `EXERCISE_READY`;
6. never convert source availability into learner mastery.

Quarterly source refreshes may append/update mappings, but completed learner evidence is invalidated only if the underlying concept or acceptance contract materially changes.
