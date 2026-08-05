# ADR 0003: StreamEvent Contract Versioning

- Status: Accepted
- Date: 2026-07-29
- Deciders: BodySense platform
- Related: T0-3 cross-language contract testing, packages/contracts

## Context

BodySense streams a versioned `StreamEvent` envelope across Go (producer),
Python (producer on `/runtime`), and TypeScript (consumer). The envelope is
closed (`additionalProperties: false` on the top level and on `ids`) and the
`version` field is currently a constant `1`.

Without an explicit evolution rule, teams either:
1. silently break clients by reshaping payloads, or
2. freeze the contract forever and accumulate parallel ad-hoc event types.

## Decision

1. **Single schema, single fixture set** live under `packages/contracts/`:
   - `schemas/stream-event.v1.schema.json`
   - `fixtures/stream-events.v1.json`
   - `fixtures/stream-event-types.v1.json` (required type coverage list)
2. **Additive changes inside v1 are allowed only when**:
   - new *optional* payload fields are introduced, or
   - a new `type` is added with its own payload shape, and
   - the envelope fields (`version/seq/channel/type/ids/payload`) stay stable, and
   - the new type is added to fixtures + the required-types list in the same PR.
3. **Breaking changes require `version: 2`** (new schema file
   `stream-event.v2.schema.json`). Examples of breaking:
   - renaming/removing envelope fields
   - changing `ids` closed key set incompatibly
   - redefining an existing `type`'s payload in a non-backward-compatible way
4. **Dual-read window**: during a v1→v2 migration, producers may emit either
   version; consumers must accept both until v1 traffic is gone. Go/Python
   parity tests load both fixture sets during the window.
5. **CI gate**: `pnpm nx run contracts:test` runs Go + Python fixture/schema
   parity so drift cannot merge silently.

## Consequences

- Event additions (e.g. `state.interaction.expired`, `safety.output_reviewed`)
  are cheap and safe inside v1.
- Payload redesigns of existing types become deliberate version bumps.
- Codegen from schema (optional future) can key off `version`.
