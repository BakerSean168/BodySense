# Public stream schemas

`stream-event.v1.schema.json` is the existing public stream schema candidate. Phase 03 will repair the semantic drift identified by the 2026-09-09 spike, make this schema the single public event authority, and generate both static TypeScript and runtime validation from it.

Until that migration is complete, the checked-in handwritten parser remains the active production trust boundary. Phase 01 deliberately does not switch production event behavior.
