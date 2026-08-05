# Practice Map

Use `.practice-map/` to persist the user's learning plan inside the active workspace.

## Directory layout

```text
.practice-map/
  current.yaml
  maps/
    ts-generics.md
    react-state-management.md
```

## `current.yaml`

Purpose:

- mark the active map
- stay tiny and easy to edit

Shape:

```yaml
version: 1
current_map: ts-generics
```

Rules:

- Keep `current_map` equal to a map frontmatter `id`.
- Update this file when switching maps.
- Do not embed the whole study state here.

## Map files

Each map lives at `.practice-map/maps/<id>.md`.

Use YAML frontmatter plus Markdown sections.

Recommended frontmatter:

```yaml
id: ts-generics
title: TypeScript Generics
status: active
level: beginner
language: typescript
created_at: 2026-06-05
updated_at: 2026-06-05
```

Lifecycle statuses:

- `planned`: the map exists but practice has not started
- `active`: the map is the current live practice track
- `paused`: the map is intentionally inactive
- `done`: the scoped goal has been achieved
- `archived`: the map is kept for history and should not be resumed by default

Recommended body sections:

- `Goal`: What the user wants to become able to do
- `Why`: Why this topic matters now
- `Milestones`: The ladder of competence
- `Current Focus`: The current narrow target
- `Exercises`: Short reps or assignments
- `Session Log`: Dated notes from coaching sessions
- `Next Step`: The next concrete action

## Update policy

- Preserve any user-added sections and frontmatter keys.
- Prefer appending to `Session Log` instead of rewriting history.
- Update `updated_at` whenever you materially change the map.
- Change `Current Focus` and `Next Step` whenever the learning target shifts.
- Add exercises that are concrete enough to attempt immediately.
- Keep `created_at` stable after first creation.
- On resume, usually move `status` to `active`.
- On explicit completion, set `status` to `done` and make `Next Step` reflect either review maintenance or a follow-on topic.
- On temporary shelving, prefer `paused` over `archived`.

## Naming

- Use lowercase kebab-case ids.
- Keep ids stable once a map is in use.
- Prefer topic-based ids such as `ts-generics`, `sql-joins`, or `react-effects`.

## Multiple maps

Support many map files at once.

When the user asks to:

- create a new topic: add a new map file and usually point `current.yaml` to it
- continue current practice: read `current.yaml` first
- switch topics: update only `current.yaml` unless the user also asks to revise the target map
- review all active topics: scan every file in `maps/`

## Session log format

Append dated notes under `Session Log`.

Recommended shape:

```md
## 2026-06-05

- Worked on generic constraints.
- Learned when `extends` narrows a type parameter.
- Still weak on inferring multiple type parameters.
- Next rep: write two constrained helper functions from scratch.
```

Keep entries short and biased toward learning state, not transcript dumps.

## Transition rules

Use these defaults unless the user asks differently:

- `planned` -> `active`: when the user begins work
- `active` -> `paused`: when the user switches away without finishing
- `active` -> `done`: when the scoped milestones are met
- `paused` -> `active`: when resuming
- `done` -> `active`: only if the user reopens the topic for a new scope
- any state -> `archived`: only when the user wants historical retention without active use
