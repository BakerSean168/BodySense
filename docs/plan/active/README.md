# Active Plans

BodySense currently has active 3D/observability/static-distribution work, one Workbench visual plan, and one parked durability plan:


- [`2026-08-27-vanatome-3d-body-explorer.md`](./2026-08-27-vanatome-3d-body-explorer.md) — READY FOR IMPLEMENTATION; adopt Vanatome as the primary 3D anatomy engine, add BodyRegionOntology, BodyState↔3D linking, anatomy drill-down, Chat context, WebGL fallback, and version-pinned/self-host-ready atlas distribution.
- [`2026-08-27-static-asset-cdn.md`](./2026-08-27-static-asset-cdn.md) — IN IMPLEMENTATION; split public immutable Vite/Vanatome bytes from the private application origin using revision-scoped Cloudflare R2 distribution and an assets-before-HTML release gate.
- [`2026-08-27-observability-foundation.md`](./2026-08-27-observability-foundation.md) — ACTIVE; converge application/browser diagnostics on structured privacy-bounded logging and an OpenTelemetry-compatible observability path.
- [`2026-08-26-workbench-ui-v2.md`](./2026-08-26-workbench-ui-v2.md) — ACTIVE / visual acceptance; single continuous Chat + Workbench shell, graphite visual system, progressive-disclosure content cleanup, and visual-density refinement.
- [`data-durability-backup-2026-08-25.md`](./data-durability-backup-2026-08-25.md) — ACTIVE / PARKED BY OWNER; real off-host PostgreSQL backup + restore drill and durable private user-upload object storage. Repository implementations are preserved, but cloud provisioning/cutover is intentionally postponed.

The broad post-`v0.5.2` production security/runtime/Knowledge closeout has been archived after the 2026-08-25 owner-approved scope split:

- [`../archive/production-security-runtime-knowledge-closeout-2026-08-23.md`](../archive/production-security-runtime-knowledge-closeout-2026-08-23.md) — archived; security/privacy/session/runtime/Knowledge/supply-chain/documentation implementation is closed, while the two real durability acceptance items were moved into the active plan above. Optional Knowledge production publication and Diagnosis promotion were closed with explicit non-rollout/HOLD dispositions rather than being represented as production acceptance.

## Canonical environment roles

- **GCP-dev** — primary development host and production operations control point.
- **GitHub Actions** — CI/release build plane.
- **Alibaba Cloud ACR** — production image registry.
- **Alibaba Cloud ECS (`body.bakersean.top`)** — sole production runtime.
- **Oracle2** — detached from BodySense.
- **DigitalOcean** — retired historical deployment path.

## Authoritative current architecture

- [`../../architecture/current-longitudinal-system.md`](../../architecture/current-longitudinal-system.md)
- [`../../architecture/model-gateway-routing.md`](../../architecture/model-gateway-routing.md)
- [`../../architecture/agent-platform-role-governance.md`](../../architecture/agent-platform-role-governance.md)
- [`../../architecture/deployment-architecture.md`](../../architecture/deployment-architecture.md)
- [`../../architecture/delivery-platform-v3.md`](../../architecture/delivery-platform-v3.md) — adopted production delivery architecture under ADR 0008; v0.9.0 proved affected PR CI, exhaustive main CI, exact-SHA candidates, canonical staging, Draft-first release publication and explicit production selection.
- [`../../architecture/release-lifecycle-v3.md`](../../architecture/release-lifecycle-v3.md) — explicit Prepare Release / Release Publish / Deploy Production authority boundaries.
- [`../../architecture/knowledge-lifecycle-architecture.md`](../../architecture/knowledge-lifecycle-architecture.md)
- [`../../architecture/body-explorer-3d-anatomy.md`](../../architecture/body-explorer-3d-anatomy.md) — target 3D body/anatomy architecture accepted by ADR 0006; implementation pending.
- [`../../architecture/body-region-ontology.md`](../../architecture/body-region-ontology.md) — target canonical body-region semantic boundary for 3D and durable BodyState integration.

A future model/prompt/tool/schema Challenger, Knowledge production-corpus rollout, or other bounded product change should start its own plan rather than being attached to the parked durability plan.
