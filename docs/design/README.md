# BodySense · 产品设计

产品交互由业务领域语义和可编辑 Penpot 设计共同约束。界面必须让用户容易理解**现在身体怎样、当前综合判断是什么、正在执行哪个训练计划，以及如何通过 Agent 继续操作**。

## 产品语义

- [产品需求](../product/requirements.md)
- [领域契约](../product/domain.md)
- [Agent 交互](../product/agent-workflows.md)
- [UI 交互规格](../product/interface.md)
- [验收矩阵](../product/acceptance.md)
- [视觉 Foundations](./foundations.md)

## 设计文件

Penpot：**BodySense · Product Design**

文件 ID：24d9d841-759d-81bc-8008-b6423250c2b8

| Page | 用途 |
| --- | --- |
| Foundations | 字体、色彩、尺寸、响应式、交互规则 |
| Components | 可复用按钮、窗体、状态、工具、人体控件 |
| Product | 桌面和移动端人体场景、业务窗口及交互状态 |

[桌面 Viewer](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) ·
[移动 Viewer](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=0)

## 交互约束

1. 3D 人体 Canvas 持续作为底层桌面；业务窗口位于其上，可以打开、拖动、收起和恢复。
2. Chat 负责表达、追问、诊断请求及训练变更授权，不预设常驻固定侧栏。
3. Person 使用当前原型的 Profile / Records / Equipment / Media / Reports / Habits 分类；真实数据替换演示内容。
4. Diagnosis 以单一当前综合结果展示可独立展开的病情；TrainingPlan 显示目标计划当前有效内容。
5. 关键澄清和 Agent 建议通过交互工具处理；所有接受、拒绝、取消、错误都有清晰反馈。
6. 部位选择或参考动画只是交互上下文，不自动变为健康事实。
7. 桌面、窄屏与移动端保持相同业务语义；确保可访问性、操作焦点、状态恢复及安全提示。
8. 对照实际 Penpot 组件和流程验证 UI，不仅依赖静态截图。
