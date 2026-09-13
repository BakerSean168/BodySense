# Internal Agent Runtime v1

Phase 04 will define the canonical Go <-> Python Agent runtime Proto contract here.

This contract is private to the runtime boundary and is intentionally distinct from the public JSON `StreamEvent` contract. Proto adoption is initially an IDL/codegen/validation decision; HTTP/NDJSON remains the transport unless a later workload-specific decision changes it.
