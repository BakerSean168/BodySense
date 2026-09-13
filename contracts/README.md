# BodySense Contract Authorities

BodySense uses one canonical contract technology per semantic boundary. See ADR 0014 and the vNext engineering reset master plan.

- Public browser HTTP: `packages/contracts/openapi/` — OpenAPI 3.1, introduced in Phase 02.
- Public streaming: `packages/contracts/schemas/stream-event.v1.schema.json` — JSON Schema 2020-12, canonicalized in Phase 03.
- Internal Go/Python Agent runtime: `contracts/internal/agent-runtime/v1/` — Proto + Buf + Protovalidate, introduced in Phase 04.

Generated language-specific artifacts never become domain models. `tools/contracts/foundation/` is deliberately non-serving smoke input used only to prove the pinned generation/governance toolchain before production contracts migrate.
