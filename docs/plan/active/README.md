# Active Plans

BodySense currently has **one parked active durability plan**:

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
- [`../../architecture/knowledge-lifecycle-architecture.md`](../../architecture/knowledge-lifecycle-architecture.md)

A future model/prompt/tool/schema Challenger, Knowledge production-corpus rollout, or other bounded product change should start its own plan rather than being attached to the parked durability plan.
