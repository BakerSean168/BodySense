# BodySense Postman workspace assets

BodySense does **not** maintain a second hand-written REST specification in Postman.

The contract authority is:

```text
packages/contracts/openapi/bodysense.v1.openapi.yaml
```

The public collection in `postman/collections/` is a deterministic generated projection of that OpenAPI document. Regenerate it with:

```bash
pnpm postman:generate
```

Verify that committed assets are fresh and that every canonical OpenAPI operation has exactly one generated Postman request with:

```bash
pnpm postman:verify
```

With Postman CLI installed, validate the current Postman file formats and a deterministic v2.1 -> Native Git v3 migration with:

```bash
pnpm postman:validate-native
```

## Environments

Committed environment files intentionally contain no credentials.

- `BodySense Dev.environment.json` uses `http://127.0.0.1:20101`.
- `BodySense Production.environment.json` uses `https://body.bakersean.top`.
- `BodySense Staging.environment.json` leaves `baseUrl` empty because staging has no stable public repository-owned origin. Fill it locally for the staging runtime you are using.
- `bearerToken` is always committed empty and marked as a secret variable. Populate it only in your local/private Postman environment.

## Public vs internal APIs

This workspace represents the browser-facing Go REST API only. Python AI-service routes such as `/runtime/*`, `/api/diagnosis/*`, `/api/treatment/*`, `/api/assessment/*`, and other service-local routes are internal boundaries and are intentionally **not** mixed into the public collection.

Their authority remains in the service/runtime contracts and implementation; adding them manually to this public Postman collection would create a second, misleading public contract.

## Native Git / Postman Cloud binding

The repository intentionally does not contain a fabricated `.postman/resources.yaml` workspace ID. Current Postman Native Git workspace metadata requires a real non-empty cloud workspace ID, and creating or connecting one requires Postman credentials.

After authenticating the Postman CLI, create or connect the BodySense workspace from the repository root using the current `postman workspace` commands. Postman will write the real `.postman/resources.yaml`; that metadata can then be committed. Never commit `POSTMAN_API_KEY`, access tokens, passwords, or bearer tokens.

Until that one-time cloud binding is performed, the collection and environment assets remain fully repo-native, reproducible, CLI-lintable, and importable without a cloud account.
