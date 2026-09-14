# vNext Phase 03 — Public StreamEvent / JSON Schema-first

- Status: IN PROGRESS
- Branch: `refactor/vnext-03-public-stream`
- Parent canonical vNext commit: `169df5c67`
- Canonical authority target: `packages/contracts/schemas/stream-event.v1.schema.json`
- Wire version: StreamEvent v1; no public SSE/JSON wire-version change in this phase

## Goal

Make the public StreamEvent JSON Schema the single static/runtime authority while preserving the existing public wire, then make all Web consumers exhaustive over the generated discriminated union.

Phase 03 does **not** replace the internal Go↔Python Agent protocol. Internal runtime separation and Proto ownership belong to Phase 04.

## STREAM-001 — Repair canonical schema semantics

Status: **COMPLETE ON PHASE BRANCH**

The 2026-09-09 contract-codegen spike proved that the checked-in JSON Schema and the fail-closed handwritten parser agreed on all 34 real fixtures and 10 basic malformed cases, but disagreed on five targeted semantics. The canonical schema was still byte-identical to that spike baseline when Phase 03 started, so the spike-proven candidate delta could be promoted without overwriting later schema work.

The canonical schema now matches the handwritten parser for the five known drifts:

1. `run.failed` accepts either a non-empty durable `reason` or structured `error.message`;
2. `safety.output_reviewed.verdict` is limited to `accepted | degraded | rejected`;
3. optional safety `reasons` is a string array;
4. interaction `question.fields` has at most three entries and each field requires non-empty `key`/`label`;
5. interaction `question.options`, when present, contains strings only.

The same repair also records parser semantics needed for strict schema compilation: parser-required strings become non-empty where already enforced, and required opaque payload members such as `args`, `result`, `info`, `answer`, `citation`, `usage` and job `error` are explicitly declared in `properties` without inventing narrower semantics.

### Permanent evidence

The spike matrix is now repository-owned rather than experimental:

- `stream-events.v1.json`: 34 real events / 33 event variants;
- `stream-events.invalid.v1.json`: 10 malformed boundary cases;
- `stream-events.semantic-probes.v1.json`: the five known semantic-drift probes;
- `stream-event-schema-parity.test.ts`: compares the current handwritten parser with a strict Ajv 2020-12 compile of the canonical schema;
- `check-stream-event-schema.mjs`: makes strict + `strictRequired` schema compilation part of `contracts:lint`, and therefore `contracts:verify`.

Verification on this checkpoint:

```text
public StreamEvent schema strict compile      PASS
TS parser + schema parity                     16/16 PASS
  real fixtures                               34/34 agree and accept
  malformed corpus                            10/10 agree and reject
  semantic probes                             5/5 agree with expected outcome
Go StreamEvent parity                         PASS
Python StreamEvent parity                     7/7 PASS
pnpm nx run @bodysense/contracts:typecheck    PASS
pnpm contracts:verify                         PASS
git diff --check                              PASS
```

The handwritten TypeScript contract and runtime parser remain primary at this checkpoint. STREAM-002 will introduce deterministic generated TypeScript plus a compiled runtime validator in shadow/parity mode before any trust-boundary cutover.

## Next — STREAM-002

- generate the TypeScript discriminated union from the canonical schema;
- compile the canonical schema into a deterministic standalone runtime validator;
- expose both behind the stable `@bodysense/contracts` facade;
- run generated validation in parity/shadow mode against the handwritten parser;
- do not delete the handwritten parser until generated parity and consumer migration are complete.
