# vNext Phase 03 — Public StreamEvent / JSON Schema-first

- Status: COMPLETE
- Branch: `refactor/vnext-03-public-stream`
- Parent canonical vNext commit: `169df5c67`
- Canonical vNext integration: `1125db479` (PR #175 merge commit)
- Canonical authority: `packages/contracts/schemas/stream-event.v1.schema.json`
- Wire version: StreamEvent v1; no public SSE/JSON wire-version change in this phase
- Next: Phase 04 — internal Go/Python runtime reset to Proto IDL (not implemented here)

## Goal

Make the public StreamEvent JSON Schema the single static/runtime authority while preserving the existing public wire, then make all Web consumers exhaustive over the generated discriminated union.

Phase 03 does **not** replace the internal Go↔Python Agent protocol. Internal runtime separation and Proto ownership belong to Phase 04.

## STREAM-001 — Repair canonical schema semantics

Status: **COMPLETE ON PHASE BRANCH**

The 2026-09-09 contract-codegen spike proved that the checked-in JSON Schema and the fail-closed parser agreed on all 34 real fixtures and 10 basic malformed cases, but disagreed on five targeted semantics. The canonical schema was still byte-identical to that spike baseline when Phase 03 started, so the spike-proven candidate delta could be promoted without overwriting later schema work.

The canonical schema now matches the handwritten parser for the five known drifts:

1. `run.failed` accepts either a non-empty durable `reason` or structured `error.message`;
2. `safety.output_reviewed.verdict` is limited to `accepted | degraded | rejected`;
3. optional safety `reasons` is a string array;
4. interaction `question.fields` has at most three entries and each field requires non-empty `key`/`label`;
5. interaction `question.options`, when present, contains strings only.

The same repair also records parser semantics needed for strict schema compilation: parser-required strings become non-empty where already enforced, and required opaque payload members such as `args`, `result`, `info`, `answer`, `citation`, `usage` and job `error` are explicitly declared in `properties` without inventing narrower semantics.

### Permanent evidence

The spike matrix is now repository-owned rather than experimental:

- `stream-events.v1.json`: 35 real events / 34 event variants;
- `stream-events.invalid.v1.json`: 10 malformed boundary cases;
- `stream-events.semantic-probes.v1.json`: the five known semantic-drift probes;
- `stream-event-schema-parity.test.ts`: exercises generated validation and the stable parser facade against the repository-owned fixture matrix;
- `check-stream-event-schema.mjs`: makes strict + `strictRequired` schema compilation part of `contracts:lint`, and therefore `contracts:verify`.

Verification on this checkpoint:

```text
public StreamEvent schema strict compile      PASS
TS parser + schema parity                     16/16 PASS
  real fixtures                               35/35 agree and accept
  malformed corpus                            10/10 agree and reject
  semantic probes                             5/5 agree with expected outcome
Go StreamEvent parity                         PASS
Python StreamEvent parity                     7/7 PASS
pnpm nx run @bodysense/contracts:typecheck    PASS
pnpm contracts:verify                         PASS
git diff --check                              PASS
```

## STREAM-002 — Generated contract and validator cutover

Status: **COMPLETE**

The canonical schema now generates both the public TypeScript union and a deterministic standalone Ajv validator. The generated artifacts are checked in under `packages/contracts/generated/`; `contracts:generate` and `contracts:check-generated` verify deterministic regeneration. Generated public TypeScript contains zero `any`; extensibility remains `unknown` only where the schema intentionally leaves a field open.

The stable `@bodysense/contracts` facade exports the generated event aliases and parser. `parseStreamEvent` is a thin wrapper around the generated validator and exposes validation diagnostics without exposing Ajv. Live SSE parsing and durable run replay both pass through that same parser before reducer or state use.

The active-turn reducer and SSE dispatcher use exhaustive generated-variant switches with explicit no-op cases for valid events that have no current UI projection. Test event builders are parser-backed, and the 35 real fixtures, 10 malformed fixtures, and five semantic probes pass generated-validator/parser parity. Public StreamEvent v1 wire fields remain unchanged; Phase 04 internal runtime/Proto work is not part of this phase.

### Completion evidence

- `pnpm contracts:verify`: PASS;
- `pnpm contracts:check-generated`: PASS after repeated generation;
- generated TypeScript: zero `any` tokens;
- focused contracts and consultation tests: PASS (16 contract parity tests; 68 Web consultation tests);
- full lint: PASS;
- full typecheck: PASS;
- production build: PASS; ConsultationPage remained essentially flat, while the standalone validator is primarily carried by the shared consultation service chunk (about 10.5 KiB gzip increase in the prior comparison);
- isolated atlas-metadata fallback E2E: PASS (1/1, 10.8s); the earlier `net::ERR_NETWORK_CHANGED` failure is classified as infrastructure flake, with no Body Explorer changes;
- full local-deploy validation: PASS; all 10 browser E2E cases passed, including the atlas fallback, and all deployment/database gates passed;
- `git diff --check`: PASS.

Bundle impact is limited to the generated standalone validator in the shared consultation service chunk; the ConsultationPage chunk remained essentially flat in the prior comparison, with about 10.5 KiB gzip added to the shared chunk. Runtime validation was not weakened to reduce bundle size.

## Phase 03 completion acceptance

Status: **COMPLETE**

Canonical vNext integration: `1125db479` (`Merge pull request #175 from BakerSean168/refactor/vnext-03-public-stream`).

Phase 03 closes with the public StreamEvent v1 JSON Schema as the canonical public contract authority, deterministic generated TypeScript and standalone runtime validation, and the same `parseStreamEvent` trust boundary for live SSE and durable replay. Both Web consumers are exhaustive over all 34 public variants, generated TypeScript contains zero `any`, and Phase 04 internal Proto/runtime work remains intentionally unstarted.
