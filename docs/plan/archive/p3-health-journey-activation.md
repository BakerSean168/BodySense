# P3：HealthJourney 前端激活（业务主线一等公民化）
> ✅ **已完成并归档**（2026-07-29）。实施落地见对应代码与测试；本文件移入 archive 仅作历史记录。


> 文档状态：待实施（源自 [architecture-review-2026-07-26.md](./architecture-review-2026-07-26.md) P3）
> 创建日期：2026-07-26
> 关联：`docs/architecture/health-journey-workflow.md`（README 标 0%，**实际后端已实现**）
> 真值来源：当前代码。文中 `file:line` 为撰写时锚点，实施前以最新代码为准。
> 优先级：🟠 P1 高杠杆低成本 —— 后端已就绪，缺前端对接

---

## 0. 一句话定位

`HealthJourney` 状态机在 Go 侧**已完整实现**（10 个 stage + 15 个 available_actions + 只读派生 + 单测 + `GET /api/v1/journey` 已挂载），但**全 web 代码搜 `journey` = 0 命中**。前端 Dashboard 是写死的 4 张卡片，唯一的"下一步"逻辑是 `profile===null` 就跳 onboarding。后端算好的引导事实上是**无消费者的死代码**。本方案让前端消费 `available_actions`，把"评估→问诊→计划→跟踪"主线真正一等公民化。

---

## 1. 现状盘点（后端已就绪，前端未接入）

| 事实 | 位置 | 结论 |
|---|---|---|
| **状态机完整实现** | `apps/api/internal/workflow/health_journey.go:39-167` | `GetJourneyState` 从 profile/uploads/consultations/assessments/training_plans 五表派生 stage |
| stage + action 常量齐全 | `apps/api/internal/dto/health_journey.go:5-32` | 10 stage（`profile_incomplete`→…→`completed`）+ 15 action |
| 逐级判定 + available_actions | `health_journey.go:129-167` `determineStage` | 如 `plan_ready`→`[view_treatment, start_training]`、`reassessment_due`→`[view_progress, reassess]` |
| 有单测 | `health_journey_test.go` | 已覆盖 |
| HTTP 已挂载 | `main.go:122-123, 227` → `health_journey_handler.go:20-29` | `GET /api/v1/journey` 可用 |
| **前端 0 调用** | 全 web 代码 | 搜 `journey` 零命中 |
| 前端硬编码下一步 | `DashboardPage.tsx:24-59` | 4 张固定卡片 + `profile===null` 跳转，与实际 stage 无关 |
| 静态路由 | `App.tsx:84-170` | 无基于 stage 的引导 |

**根因**：前后端并行开发，journey 的 DTO 没进 `packages/contracts`，前端缺对接锚点，于是各自实现。

---

## 2. 设计原则（优雅解法：前端不再"猜"，只"读"）

AI Agent 原生产品的核心承诺是**用户不该自己找路**。当前 Dashboard 违背了这一点。优雅解法有两条纪律：

1. **单一来源**：`stage` 与 `available_actions` 只由后端派生，前端从不本地推断"下一步是什么"。前端只负责把 action **映射为 UI**（文案 + 路由 + 图标）。
2. **契约收敛**：`HealthStage` / `HealthAction` 枚举上移到 `packages/contracts`，Go 与 Web 共用一份，杜绝 [P5](./architecture-review-2026-07-26.md) 式的枚举漂移重演。

```text
后端 GetJourneyState ──► { stage, available_actions[], context }
                              │
                     packages/contracts（HealthStage/HealthAction 单一真源）
                              │
前端 useJourney() ──► JourneyGuide 组件：action → { label, route, variant }
                              │
              Dashboard 主 CTA 区 + 路由守卫（stage 不满足则引导）
```

---

## 3. 实施方案

### Phase A：契约收敛 + 前端读取（0.5–1 天）

1. **契约上移**：把 `HealthStage`、`HealthAction` 枚举（现 `dto/health_journey.go`）的**权威定义**迁到 `packages/contracts/src/health-journey.ts`，Go 侧改为与之对齐（值字符串一致），并在 `packages/contracts/src/index.ts` 导出。这是 P5 "业务 DTO 收敛到 contracts" 的第一块试点。
2. **前端 service + hook**：新增 `features/profile/services/journeyService.ts`（`GET /api/v1/journey`）+ `hooks/useJourney.ts`（TanStack Query，随 profile/consultation/training 关键操作后 invalidate）。
3. **action → UI 映射表**：新增 `features/profile/lib/journeyActions.ts`，把 15 个 action 映射为 `{ label, route, variant, icon }`。这是前端唯一"知道 action 长什么样"的地方。
4. **验收**：`useJourney()` 能返回后端 stage 与 actions；映射表覆盖全部 15 个 action（缺一即 TS 编译报错，用 `Record<HealthAction, ...>` 保证穷尽）。

### Phase B：Dashboard 主 CTA 改为 journey 驱动（0.5–1 天）

**目标**：把写死的 4 张卡片改为"当前 stage 的推荐动作优先"。

1. `DashboardPage.tsx`：顶部新增 **JourneyGuide 主区**——渲染当前 stage 的 `available_actions`（首个为 primary CTA）。原有 4 张入口卡片降级为"全部功能"次级区（保留可达性）。
2. 删除 `DashboardPage.tsx:24-28` 的 `profile===null` 硬编码跳转，改由 `stage===profile_incomplete` 的 action 驱动（后端已能判定）。
3. **验收**：新用户 → stage=`profile_incomplete` → 主 CTA "完善档案"；完成评估后 → stage 前进、主 CTA 自动变化，无需前端改代码。

### Phase C（可选）：路由守卫（0.5 天）

1. 对依赖前置条件的页面（如未出评估就进训练），用 journey stage 做软引导（提示 + 引导按钮），而非硬拦截。
2. **验收**：直接访问 `/training` 但 stage 未到 `plan_ready` → 提示"先完成评估"并给引导入口。

---

## 4. 与其它计划的联动

- **[P5 契约收敛](./architecture-review-2026-07-26.md)**：本方案的 Phase A 是"业务 DTO 上移 contracts"的首个落地样板，后续 Assessment/Training DTO 可循此模式。
- **[体态照片分析 P3-B1](./posture-photo-analysis-plan.md)**：journey 的 `context` 未来可纳入"是否已有体态分析"，驱动"去上传体态照片"这类 action。
- **文档校正**：README 架构表须把 HealthJourney 从 **0%** 改为 **"后端已实现，前端本方案接入中"**（见 [索引](./README.md)）。

---

## 5. 落地任务清单

```text
A1 feat(contracts): HealthStage/HealthAction 枚举上移 + 导出
A2 refactor(api): health_journey dto 对齐 contracts 值
A3 feat(web): journeyService + useJourney hook
A4 feat(web): journeyActions 映射表（Record<HealthAction> 穷尽校验）
--- Phase B ---
B1 feat(web): DashboardPage JourneyGuide 主 CTA 区
B2 refactor(web): 删除 profile===null 硬编码跳转，改 stage 驱动
--- Phase C（可选） ---
C1 feat(web): 关键页面 stage 软引导守卫
```

## 6. 风险与回滚

- 纯前端消费 + 契约收敛，对后端零侵入（Go 值不变，仅定义位置迁移）。
- 回滚：隐藏 JourneyGuide 主区、恢复固定卡片即退回原状。
- 注意 `ConsultationPhase` 前端多出的 `completed` 项（P5 已记录的漂移）——本次收敛顺带核对 journey 相关枚举，不要引入新的前端独有值。
