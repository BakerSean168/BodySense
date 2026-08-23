# Active Plans

BodySense currently has **one active production-closeout program**:

- [`production-security-runtime-knowledge-closeout-2026-08-23.md`](./production-security-runtime-knowledge-closeout-2026-08-23.md) — ACTIVE; post-`v0.5.2` security/privacy containment, Consultation process-failure liveness, off-host disaster recovery, Knowledge Lifecycle productionization, production hardening and final governance/documentation convergence.

The completed 2026-08 Agent-platform migration and bounded Consultation Runtime / Async-RAG hardening remain closed. They are historical evidence and must not be reopened merely because this new production-readiness program exists:

- [`../archive/consultation-runtime-rag-hardening-plan-2026-08-22.md`](../archive/consultation-runtime-rag-hardening-plan-2026-08-22.md) — completed 2026-08-23; run identity, stream contracts, cancellation, event ordering, async RAG and Grounding Eval v2 diagnostic hardening.
- Diagnosis Agent platform — `../archive/2026-08-diagnosis-agent-platform/`
- Treatment Agent platform — `../archive/2026-08-treatment-*/`
- Assessment Agent platform — `../archive/2026-08-assessment-agent-platform/`
- Consultation Agent platform — `../archive/2026-08-consultation-agent-platform/`
- Cross-role Posture / Title / Knowledge Agent-platform closeout — `../archive/2026-08-agent-platform-closeout/`

The new Active Plan deliberately preserves the successful North-Star ownership model. It does **not** reintroduce a legacy Journey/MedicalRecord truth, direct provider routing, Oracle2 deployment, or a second Treatment/Training authority.

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
- [`../../architecture/deployment-architecture.md`](../../architecture/deployment-architecture.md) — topology correction is an explicit ticket in the active production-closeout plan because this document still contains stale Oracle2 wording at plan creation time.

A future model/prompt/tool/schema Challenger should still start its own bounded governance plan when appropriate. Do not overload this production-closeout program with unrelated product features.
