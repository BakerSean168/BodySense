# BodySense Master Course

> Status: active
> Baseline date: 2026-09-07
> Learning workspace: the production-shaped BodySense repository itself
> Progress source of truth: `.practice-map/maps/bodysense-fundamentals.md`

## Purpose

This is the only maintained BodySense learning curriculum.

The older topic-by-topic local tutorials have been retired. The curriculum now uses two mature public courses as the external syllabus backbone:

1. **Full Stack Open** — full-stack/Web pedagogy, incremental exercises, debugging, React, server communication, testing, state management, TypeScript, CI/CD, containers and databases.
2. **TECH SCHOOL Backend Master Class** — Go/PostgreSQL backend depth, transactions, locking, isolation, testing, authentication, background work, production delivery and server hardening.

BodySense itself replaces Phonebook, Blog List and Simple Bank as the primary practice application.

## What "1:1 parity" means

The goal is **coverage parity, not text/code copying**.

Every source knowledge point or training point must end in exactly one of three states:

- `DIRECT` — reproduced as an original BodySense exercise against current code.
- `COMPARE` — learned by comparing the source technology/design with the BodySense production choice; no forced migration.
- `OPTIONAL` — explicitly retained as an extension when the source topic targets a different product surface (for example React Native or Next.js).

Nothing is silently skipped.

We do not copy course explanations, solution code, or exercise wording. Full Stack Open is CC BY-NC-SA; TECH SCHOOL's public Simple Bank repository is MIT, while video/course material has its own rights. This curriculum therefore records attribution and source coverage while using newly written BodySense exercises.

## Course files

- [`curriculum/01-full-stack-open-parity.md`](./curriculum/01-full-stack-open-parity.md) — Full Stack Open coverage map and BodySense exercise families.
- [`curriculum/02-techschool-backend-parity.md`](./curriculum/02-techschool-backend-parity.md) — TECH SCHOOL lectures #0-#77 mapped one-by-one.
- [`curriculum/03-bodysense-agent-extension.md`](./curriculum/03-bodysense-agent-extension.md) — Agent engineering that neither source course covers deeply enough.
- [`curriculum/04-learning-protocol.md`](./curriculum/04-learning-protocol.md) — how every exercise is performed and accepted.
- [`curriculum/SOURCES.md`](./curriculum/SOURCES.md) — source snapshots, attribution, and refresh policy.

## Curriculum order

```text
Foundation / browser / HTTP
  -> React + TypeScript
  -> server communication + state
  -> Go API architecture
  -> PostgreSQL schema + migrations
  -> transactions / locks / isolation
  -> testing / validation / auth
  -> streaming / async / jobs
  -> CI / containers / release / recovery
  -> typed Agent / RAG / eval / safety / replay
  -> independent BodySense vertical slice
```

The order is intentionally not identical to either source course. Coverage is one-to-one, but sequencing is optimized for the existing BodySense architecture.

## Mastery rule

A topic is not complete because an AI agent changed code successfully. Completion requires evidence that the learner can reason about it.

For each exercise, produce at least two of:

- a request/data/state/ownership diagram;
- a prediction before running the code or test;
- a test written or substantially modified by the learner;
- a failure analysis identifying the first violated contract;
- a small production improvement with verification evidence;
- an explain-back without reading the answer.

Production code is modified only when the exercise reveals a real gap. Otherwise the exercise ends with understanding + proof.

## Current starting point

Existing learning progress is preserved rather than reset. Diagnosis Agent work already completed remains completed. The next active production-shaped learning slice continues from Treatment, while this new curriculum becomes the canonical knowledge map for future sessions.
