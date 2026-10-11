# BodySense · 窗口与人体桌面交互架构

界面语义见 [UI 与交互规格](../product/interface.md)。

## 空间架构

持续存在的 3D 人体 Canvas 是底层工作桌面。Chat、Person、Diagnosis、TrainingPlan 和 Trainer 是独立浮动业务窗口，可按需打开、拖动、聚焦、收起与恢复；人体点选、涂抹和动作查看是临时工具模式。

窗口状态不决定业务数据权限。选中的部位是当前交互上下文，而不是经过核实的症状或诊断；窗口关闭不丢失待回答问题、已输入内容或训练计划授权目标。

## 数据与事件约束

- Chat 展示真实工具提交结果，承载澄清和训练建议批准。
- Person 展示当前 BodyState 等资料和真实空态。
- Diagnosis 读取唯一的完整当前综合聚合。
- TrainingPlan 读取具体 planId 的当前有效内容；待接受提案与已生效内容分开。
- UI 使用内部版本作一致性验证，但不暴露修订号与版本切换。
- 在移动端调整呈现方式而不改变业务操作语义或清空当前上下文。

对应设计基准为 Penpot **BodySense · Product Design** 的 Foundations / Components / Product 页面。
