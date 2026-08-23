# Active Plans

There are currently **no unfinished Thought Forest ingestion, evidence-normalization, or external-evidence admissibility plans**. Previous Agent-platform plans also remain closed. The Thought Forest → BodySense ingestion MVP completed its vertical gate and moved to archive:

- [`../archive/thought-forest-knowledge-ingestion-2026-08-23.md`](../archive/thought-forest-knowledge-ingestion-2026-08-23.md) — completed 2026-08-23; explicit allowlist export, Git/line provenance, BodySense adapter, generated-only dev ingestion and retrieval-safety verification.
- [`../archive/thought-forest-evidence-normalization-2026-08-23.md`](../archive/thought-forest-evidence-normalization-2026-08-23.md) — completed 2026-08-23; Tier-C claim candidates, stable Markdown evidence identity, typed reranking and unpublished retrieval eval.
- [`../archive/thought-forest-external-evidence-admissibility-2026-08-23.md`](../archive/thought-forest-external-evidence-admissibility-2026-08-23.md) — completed 2026-08-23; scoped external references, canonical identity, explicit review manifest and conservative evidence readiness.
- [`../archive/thought-forest-claim-review-publication-2026-08-23.md`](../archive/thought-forest-claim-review-publication-2026-08-23.md) — completed 2026-08-23; exact claim review, reviewed snapshot, transactional publication/rollback, hard visibility gate and overwrite protection.

The Agent-platform migration remains closed, and the bounded Consultation Runtime / Async-RAG hardening audit also remains archived.

The North-Star convergence program completed its final cross-role closeout on 2026-08-21. Historical
implementation plans are preserved under `../archive/` rather than left active after their code has
merged or reached release-gate-ready state.

Completed program families:

- Diagnosis Agent platform — `../archive/2026-08-diagnosis-agent-platform/`
- Treatment Agent platform — `../archive/2026-08-treatment-*/`
- Assessment Agent platform — `../archive/2026-08-assessment-agent-platform/`
- Consultation Agent platform — `../archive/2026-08-consultation-agent-platform/`
- Cross-role Posture / Title / Knowledge closeout — `../archive/2026-08-agent-platform-closeout/`

Authoritative current architecture:

- [`../../architecture/current-longitudinal-system.md`](../../architecture/current-longitudinal-system.md)
- [`../../architecture/model-gateway-routing.md`](../../architecture/model-gateway-routing.md)
- [`../../architecture/agent-platform-role-governance.md`](../../architecture/agent-platform-role-governance.md)

A future model/prompt/tool/schema change that creates a Challenger should start a **new** active plan
with an immutable configuration identity, qualification evidence and the governance required by that
role class. Do not reopen completed migration plans merely to make a new behavior change.
