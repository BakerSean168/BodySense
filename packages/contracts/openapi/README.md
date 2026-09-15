# Public REST OpenAPI authority

`bodysense.v1.openapi.yaml` is the canonical browser-facing BodySense REST contract.

The contract is linted for OpenAPI correctness, checked for breaking changes and semantic mutations, and used to generate the Web client/runtime validators, Go transport bindings, and the deterministic Postman API workspace. Handwritten product/domain code must adapt generated transport types at explicit trust boundaries rather than redefining request/response wire schemas.

`tools/contracts/foundation/openapi.yaml` is intentionally only a toolchain smoke contract. It proves the contract toolchain can bootstrap in isolation and is not a source of production routes or domain semantics.
