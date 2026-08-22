# Active Plans

There is currently **no unfinished AI Service Agent-platform migration plan**.

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
