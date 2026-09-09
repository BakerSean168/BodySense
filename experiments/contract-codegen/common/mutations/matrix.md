# Shared mutation matrix

All OpenAPI candidates start from the exact same common spec and apply the same semantic mutations. Record the first stage that fails: lint -> breaking gate -> codegen -> compile/typecheck -> runtime validation -> integration.

| ID | Mutation | Expected compatibility signal |
|---|---|---|
| M1 | `HealthWorkspace.generated_at` -> `generated_time` required rename | breaking before runtime |
| M2 | `WorkspaceAction.priority` integer -> string | breaking before runtime; invalid fixture path remains clear |
| M3 | remove `rejected` from `ReviewBodyStateFactRequest.review_state` | client compatibility warning |
| M4 | make `BodyStateFactInput.concern_key` required | request/client breaking |
| M5 | `conversation_id` nullable -> non-null | response compatibility warning |
| M6 | add optional `HealthWorkspace.contract_revision` string | non-breaking control |
| M7 | add unknown top-level response field to fixture while schema remains closed | runtime policy must be explicit |
| M8 | set `actions[0].priority` to string | runtime validator must fail with path/message |
| M9 | add a representative stream-event variant | evaluated in stream candidate; exhaustive consumers/conformance must react |
| M10 | Proto field-number/wire mutations | Proto candidate only; run FILE/PACKAGE/WIRE_JSON/WIRE modes |
