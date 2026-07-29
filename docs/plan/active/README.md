# Active Plans 索引

> 文档状态：当前无进行中的 task spec
> 更新日期：2026-07-29

本目录只保留**尚未完成**的 task spec。已完成计划在 `../archive/`。

---

## 状态

**目前 active 目录为空（仅保留本索引与 `.gitkeep`）。**

2026-07-26 架构审查 W0→W3 与 posture Phase 1–3 已全部落地并归档，详见：

| 主题 | 归档位置 |
|---|---|
| 架构审查总纲 | `archive/architecture-review-2026-07-26.md` |
| P1 运行时收敛 | `archive/p1-runtime-convergence-cleanup.md` |
| P2 输出治理关卡 | `archive/p2-output-governance-gate.md` |
| P3 HealthJourney | `archive/p3-health-journey-activation.md` |
| T0-1 HITL | `archive/t0-hitl-agent-runtime-plan.md` |
| T0-2 事件续传 | `archive/t0-event-sourcing-replay-plan.md` |
| T0-3 契约门禁 | `archive/t0-cross-language-contract-testing-plan.md` |
| final-agent-runtime | `archive/final-agent-runtime-architecture.md` |
| posture 全阶段 | `archive/posture-photo-analysis-plan.md` + implementation-flow |

### 契约门禁

```bash
pnpm nx run @bodysense/contracts:test
```

新计划写入本目录；完成后移入 `../archive/`。
