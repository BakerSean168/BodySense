# BodySense LiteLLM 1.104.2 — staged rollout

Scope: Development and staging Compose image defaults plus deterministic gateway
contract test. **Production stays pinned to v1.102.1** until an independent
release/rollback review.

Verified on 2026-10-10:

- The 1.104.2 image from ghcr.io/berriai/litellm starts with existing GCP
  BodySense Staging and Dev settings in disposable, unexposed Docker canaries.
  Each canary can list eight configured models using its existing authorized key.
  No upstream model inference was made in these tests.
- The hermetic gateway contract test exercises mock fallback, auth rejection,
  PydanticAI, four logical AI routing groups and streaming. It never requires a
  paid inference or network provider credentials.
- The container image defaults on dev/staging are version-pinned and do not
  change the production image path or credential configuration.

Compatibility gates before live service migration:

1. Keep LITELLM_MASTER_KEY unchanged and ensure it is not an unset,
   empty or known unsafe demonstration key.
2. Verify /health/liveliness, authorized GET /v1/models, and unauthenticated
   request rejection. Do not log or display secret values.
3. Preserve the previous v1.102.1 image locally and have the exact
   deployment Compose config available for rollback.
4. Check for old Admin UI sessions, budget-exceeded retry assumptions
   (422 versus 429), stdio MCP server dependencies, and active Prisma-backed
   database migrations before switching production.
5. For this checkout only, use its locally-pinned Docker Compose files; GCP's
   generated staging runtime may need redeployment from the tracked source.
   Do not treat a successful development canary as evidence that production
   consumer sessions have been tested.

Reference: https://docs.litellm.ai/release_notes/v1.104.0/v1-104-0
