# P2：AI 输出治理强制关卡（诊断 / 训练计划 / 体态分析）
> ✅ **已完成并归档**（2026-07-29）。实施落地见对应代码与测试；本文件移入 archive 仅作历史记录。


> 文档状态：待实施（源自 [architecture-review-2026-07-26.md](./architecture-review-2026-07-26.md) P2）
> 创建日期：2026-07-26
> 关联：`docs/architecture/ai-output-governance.md`、ADR 0002
> 真值来源：当前代码。文中 `file:line` 为撰写时锚点，实施前以最新代码为准。
> 优先级：🔴 P0 安全属性 —— 应在业务功能扩张前落地

---

## 0. 一句话定位

`AIOutputGuard` 已实现完整（schema / red_flag / faithfulness 三策略），但**没有一处把它当强制关卡**：高风险的诊断与训练计划输出完全不过 Guard，唯一会跑 faithfulness 的 `validate_treatment` 是零调用者的死代码，生效的 `/runtime` 路径下 `governance` 零命中。本方案把治理从"旁路/观察态"升级为"**先治理再落库**的强制关卡"。

---

## 1. 现状盘点（诚实核实）

| 事实 | 位置 | 结论 |
|---|---|---|
| Guard 实现完整 | `services/governance/output_guard.py` | schema/red_flag/faithfulness 三策略，按最高严重度产出 accepted/degraded/rejected |
| **诊断不过 Guard** | `services/diagnosis_service.py:71-188` | 只做 Pydantic schema 校验 + 内联红旗附注，从不调用 `AIOutputGuard` |
| **训练计划不过 Guard** | 同上 `generate_treatment` | 同上；faithfulness 仅作为返回附注字段，不影响是否下发 |
| **`validate_treatment` 死代码** | `output_guard.py:61` | 全仓零调用者 |
| **runtime 路径无治理** | `src/runtime/` | `grep governance` 零命中 —— 生效路径对诊断/训练/文本输出无任何关卡 |
| chat 文本仅观察态 | `services/chat_service.py:86-96` | "observe-only, non-blocking"，且位于**未挂载的旧路径**（见 [P1](./p1-runtime-convergence-cleanup.md)） |
| posture 仅降级不拦截 | `services/posture_analyzer.py:155-159` | 唯一真调用者，但结果只用于降 `overall_confidence`，不阻断 |

**根因**：Guard 建好后，接入点选在了低风险 / 已废弃路径；ADR 0002 重构到 `/runtime` 时没有把 Guard 一起迁移过去。

---

## 2. 设计原则（优雅解法：一个 Seam，而非散落的 if）

治理不应散落在每个 service 里各写一遍，而应是**所有结构化 AI 输出离开 Python 前的唯一收口**。核心是把 Guard 做成一个**装饰器式的守卫函数**，让"生成 → 治理 → 落库/下发"成为不可绕过的三段式。

```text
       ┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
LLM →  │  generate   │ →   │ AIOutputGuard │ →   │ emit / persist  │
       │ (raw output)│     │  (强制关卡)   │     │ (仅 accepted/   │
       └─────────────┘     └──────────────┘     │  degraded 可下发)│
                                  │              └─────────────────┘
                                  ▼ rejected
                           阻断 + 结构化错误事件 + 落审计
```

**三档处置语义**（复用 Guard 现有 verdict）：
- `accepted`：正常下发。
- `degraded`：下发，但强制标注低置信 / 附加安全提示（如 posture 现状），并落审计。
- `rejected`：**不下发原始内容**，改发一个用户可读的安全兜底文案 + `safety.output_rejected` 事件，并落审计表。

---

## 3. 实施方案（分阶段，先堵高风险）

### Phase A：诊断 / 训练计划强制过 Guard（0.5–1 天，🔴 最高优先）

**目标**：`/runtime` 的诊断与训练计划节点前置 Guard，rejected 必须阻断。

1. 在 `src/runtime/consultation_thread.py` 的 `generate_diagnosis` / `generate_treatment` 节点，产出结构化结果后、`writer(...)` 下发前，调用统一守卫：
   ```python
   guarded = await guard_structured_output(
       kind="diagnosis" | "treatment",
       payload=result,
       rag_results=rag_results,   # 供 faithfulness 使用
   )
   if guarded.verdict == "rejected":
       writer(safety_fallback_event(kind, guarded.reasons))
       return   # 不下发原始内容
   writer(guarded.payload)        # accepted 直发；degraded 带标注
   ```
2. 新增 `guard_structured_output`（`src/runtime/governance.py` 或复用 `services/governance/`）作为 runtime 侧唯一入口，内部委托现有 `AIOutputGuard`。**不要在节点里直接 new Guard**——保持单一 Seam。
3. 删除 `output_guard.py:61` 死代码 `validate_treatment`（其职责并入统一入口），或将其重命名为被真正调用的实现。
4. **验收**：构造一个"引用了 RAG 里不存在的治疗动作"的 treatment → faithfulness 判 rejected → 前端收到安全兜底而非幻觉方案；单测覆盖 accepted/degraded/rejected 三分支。

### Phase B：审计落库（0.5 天）

**目标**：每次治理结论可追溯（`ai-output-governance.md` 规划的 `ai_output_reviews` 表）。

1. Go 侧新增 `ai_output_reviews`（run_id, kind, verdict, reasons jsonb, created_at），治理结论作为 runtime event（`safety.output_reviewed`）由 Go 落库——**复用现有 Runtime Event Log 通道**，不新建写路径。
2. Python 在 Guard 后 emit `safety.output_reviewed` 事件，Go 的 `ShouldPersistEvent` 白名单已含 `safety.*`（`runtime_event_service.go:31-60`），天然落库。
3. **验收**：一次问诊产生的诊断/训练治理结论可在 `runtime_events` 查到，verdict 与前端展示一致。

### Phase C：posture 对齐同一 Seam（0.5 天）

**目标**：把 `posture_analyzer.py:155` 的"仅降级"接入统一守卫语义，rejected 也能阻断（如检出严重结构性异常但模型强行给训练建议时）。

1. `posture_analyzer` 改调 `guard_structured_output(kind="posture", ...)`，degraded 保持现有降 confidence 行为，新增 rejected 分支。
2. **验收**：posture 输出与诊断/训练走同一治理语义，无第二套 if。

---

## 4. 与其它计划的联动

- **[P1 运行时收敛](./p1-runtime-convergence-cleanup.md)**：本方案只在 `/runtime` 生效路径接入 Guard；旧路径 `chat_service.py` 的 observe-only 治理随 P1 一并删除，不再维护两套。
- **[契约测试 T0-3](./t0-cross-language-contract-testing-plan.md)**：新增 `safety.output_rejected` / `safety.output_reviewed` 事件类型，必须同步 `packages/contracts` schema+fixtures 并过三方 parity。
- **[体态照片分析](./posture-photo-analysis-plan.md)**：Phase C 与 posture 的治理已有代码收口一致。

---

## 5. 落地任务清单

```text
A1 feat(ai): runtime/governance.py 统一 guard_structured_output 入口（委托 AIOutputGuard）
A2 feat(ai): consultation_thread 诊断/训练节点前置 Guard，rejected 阻断 + 安全兜底事件
A3 refactor(ai): 删除 output_guard.py 死代码 validate_treatment（职责并入统一入口）
A4 test(ai): 治理三分支（accepted/degraded/rejected）单测
--- Phase B ---
B1 feat(api): ai_output_reviews 表 + 迁移
B2 feat(ai): emit safety.output_reviewed 事件（Go 经白名单落库）
B3 feat(contracts): safety.output_reviewed / safety.output_rejected schema+fixtures（三方 parity）
--- Phase C ---
C1 refactor(ai): posture_analyzer 接入统一 guard 语义（保留 degraded，新增 rejected）
```

## 6. 风险与回滚

- Phase A 是安全属性，rejected 阻断可能误伤（Guard 误判）。对策：先以 `degraded`（标注但下发）为默认，观察一段时间的 rejected 命中率再收紧为真阻断；faithfulness 当前是子串匹配（`faithfulness_checker.py` 自述 MVP 局限），**不要**一上来就用它做硬阻断，先用 schema + red_flag 两类做硬关卡，faithfulness 先降级标注。
- 回滚：守卫入口加开关，rejected 退化为 degraded 即恢复"不阻断"行为，事件类型新增不影响旧客户端。
