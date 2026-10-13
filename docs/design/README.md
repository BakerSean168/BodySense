# BodySense · 产品设计

产品交互由业务规格和可编辑 Penpot 设计共同约束：**docs/product 决定业务语义，Penpot Product 页决定视觉与交互形态**。界面必须让用户容易理解**现在身体怎样、当前综合诊断是什么、正在执行哪个训练计划，以及如何通过 Agent 继续操作**。

## 当前设计文件（视觉基准）

| 项 | 值 |
| --- | --- |
| Penpot 文件 | **BodySense · Product Design** |
| 文件 ID | `24d9d841-759d-81bc-8008-b6423250c2b8` |
| Product 页 ID（当前） | `ed4c6ec0-8f91-80cb-8008-c1e744bad64b` |
| 其他页面 | Foundations（字体、色彩、尺寸、响应式、交互规则）· Components（按钮、窗体、状态、工具、人体控件） |

[桌面 Viewer](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) ·
[移动 Viewer](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=0)

> **已失效**：旧 Body Canvas V3 交接文档及其 `penpot-manifest.json` 引用的页面 ID `536d2a99-67da-80a5-8008-c113cf18702a`（Product）、`536d2a99-67da-80a5-8008-c112dce0b60c`（Foundation）、`536d2a99-67da-80a5-8008-c1132f6f6c7c`（Component）不再是基准。相关文件已移至 [archive/design/body-canvas-v3](../archive/design/body-canvas-v3/README.md)，仅供追溯。

## 设计稿命名对照

Product 页仍使用部分旧窗口名，阅读时按下表换算（完整对照见 [术语表 §3](../product/glossary.md#3-penpot--旧代码名称对照)）：

| Penpot 名称 | 规格术语 |
| --- | --- |
| Assistant | Chat |
| Assessment | Diagnosis |
| Profile | Person |
| Training | TrainingPlan |
| Workflow/Training * | Trainer |

- Workflow/Assessment 的「上传 → 证据 → 结果」流程（Input / Evidence / Evidence Empty / Upload Error / Result）是**旧流程**：上传资料归 Person 的 Media / Reports，分析在 Chat 中发起。
- Workflow/Assessment **Safety Hold** 保留，作为安全阻断状态。
- 旧 UI 架构（问诊会话、单一 Treatment 及其修订、按问诊生成的诊断候选、Assessment 报告）不得覆盖 docs/product 的新规格。

## 产品语义

- [产品需求](../product/requirements.md)
- [领域契约](../product/domain/README.md)
- [Agent 交互](../product/agent-workflows.md)
- [UI 交互规格](../product/interface.md)
- [验收矩阵](../product/acceptance.md)
- [术语表](../product/glossary.md)
- [视觉 Foundations](./foundations.md)

## V2 前端设计实施与交接（未完成最终验收）

- [五大业务领域 ViewModel 契约](./v2/viewmodels.md)：16 项目标查询与命令边界；Records/Habits 已有真实 BodyFact bounded Adapter。
- [Penpot V2 实际写入检查点](./v2/penpot-handoff-checkpoint.md)：42 个状态画板与 14 个可编辑复用组件的形状证据、哪些交互已验证和哪些门禁仍未完成。
- [Penpot 画板/组件精确 ID 清单](./v2/penpot-scene-manifest.json) 和 [六组业务 Flow 条件契约](./v2/penpot-flow-contract.json)：设计恢复时使用已有源文件与授权业务状态，不重建一套图片原型。
- 本地检查：`node scripts/design/validate-v2-penpot-handoff.mjs && node --test scripts/design/validate-v2-penpot-handoff.test.mjs`。通过表示引用和业务流*规格*自洽，不等于 Penpot 真实 Flow/Viewer 已验收。

## 交互约束

1. 3D 人体 Canvas 持续作为底层桌面；业务窗口位于其上，可以打开、拖动、收起和恢复。
2. Chat 负责表达、追问、分析请求、病情确认及训练变更授权，不预设常驻固定侧栏。
3. Person 使用当前原型的五个分类标签页 Records / Equipment / Media / Reports / Habits；身份（Profile）显示在窗口标题区与 Identity Popover，不是分类标签页（[R-PS-04](../product/requirements.md#26-person-个人档案与当前状态)）；真实数据替换演示内容。
4. Diagnosis 以单一当前综合诊断展示可独立展开的病情，只显示用户确认过的条目；TrainingPlan 显示目标计划当前有效内容。
5. 关键澄清和 Agent 建议通过交互工具处理；所有接受、拒绝、取消、错误都有清晰反馈。
6. 部位选择或参考动画只是交互上下文，不自动变为健康事实。
7. 桌面、窄屏与移动端保持相同业务语义；确保可访问性、操作焦点、状态恢复及安全提示。
8. 对照实际 Penpot 组件和流程验证 UI，不仅依赖静态截图。
