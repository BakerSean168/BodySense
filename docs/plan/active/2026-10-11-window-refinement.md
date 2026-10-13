# BodySense 窗口系统优化

> HISTORICAL UI HANDOFF / subordinate to [V2 parallel master](./2026-10-13-v2-parallel-master.md). Original Penpot/UI work is integrated into the unreleased V2 branch for semantic migration and repair. This file preserves source history, not a competing active program. Current engineering status and acceptance belong to the V2 workstreams.


## 范围与约束

用户要求通过 Penpot MCP 系统优化 UI，重点为业务弹窗；Person / Me 保留既定游戏背包式布局和多 Tab，只优化细节。沿用现有绿色视觉体系、真实业务状态和现有原型路径。

## 执行

1. 读取实际 Penpot 文件、业务规格与前端组件，导出当前窗口截图。
2. 保存 Penpot 修改前版本；原位优化人物分类、诊断、训练、助手、历程、设置和辅助窗口，维护现有链接。
3. 统一文字层级、间距、按钮、状态和空态；分别检查桌面和移动端。
4. 用户确认本次仅优化 Penpot；不修改前端。验证设计、状态和原型交互。

## 验收

- 人物分类及背包式内容网格保持，头像可打开身份信息。
- 诊断区分综合结论、病情、依据和不确定性；不将演示数据呈现为用户真实诊断。
- 训练区分已接受计划、建议、执行与安全状态；保留计划切换。
- 助手建议保留明确的确认边界；历程展示真实变化而非技术修订。
- 原型入口、关闭、分类切换、计划切换、执行和返回路径仍可使用。
- 导出截图检查文字溢出、边界、留白、移动触控尺寸。

## 当前证据

- Penpot 文件：`24d9d841-759d-81bc-8008-b6423250c2b8`。
- 当前 Product 页：`ed4c6ec0-8f91-80cb-8008-c1e744bad64b`；历史 V3 manifest 不代表当前文件结构。
- T3 预览被主机 AppArmor 限制；Penpot MCP 已通过配置中的远程端点连接。
- 工作区已有 `apps/web/index.html` 修改，不属于本任务。

## 连接中断检查点

- 已保存 Penpot 版本 `Before systematic window refinement · 2026-10-11`。
- 人物窗口批量修改请求超时，保存状态未知；不得盲目重复提交。
- 随后 MCP 明确报告插件标签页休眠（无心跳）。已请求用户激活设计标签页。
- 恢复后先运行 `/tmp/bodysense-ui/inspect-live.js` 检查当前状态，再按单窗口分批编辑、导出和检查。
- 本地设计草稿脚本在 `/tmp/bodysense-ui/`；诊断、训练、助手、历程草稿尚未执行。

## 已写入 Penpot（2026-10-12）

- 32 个主要业务画板：人物主窗、五类资产、装备填充态、诊断展开/收起态、三份训练计划、助手、身体历程、设置、帮助和训练执行，覆盖对应移动端。
- Foundations 新增 `Window system / Refined foundations`，画板 `844c5dad-396e-8038-8008-c6b7c4f4212f`。
- Components 新增 `Window system / Reusable controls`，画板 `844c5dad-396e-8038-8008-c6b7c8ef5f16`，包含 3 个原生组件。
- 原生 Tokens 集合 `Components/Window` 含 23 个令牌；941 个图形已绑定颜色令牌。
- 人物 Tab 已按原有嵌套覆盖层位置对齐，资产卡片已分组；诊断 6 个状态的展开、收起和关闭连接已补齐。
- 人物、诊断、助手的中间导出截图已查看。第一轮边界检查未发现越界，原生文件校验无错误；发现的移动历程关闭缺失已经修复。

## 尚未完成的最终验证

- 最终全页链接检查超过 MCP 120 秒时限，随后插件失去心跳，最终导出尚未取得。
- 已向用户请求保持 Penpot 前台。若插件卡住，需要刷新并重新连接；不要重复提交已完成的批量设计修改。
- 恢复后用 `reconnect.js` 重建临时板引用，按 `qa-small.js` / `run-qa-small.py` **逐窗口**验证。不要再运行 `final-qa.js` 的全页重复 ID 查找。
- 待执行 `last-polish.js`：将原型建议的原因和处理内容恢复到助手确认卡中，并绑定两个新增收起态的颜色令牌。
- 待执行 `export-final.py`、查看桌面和移动截图、原生 `File.validate()`、保存最终命名版本，再归档此计划。
- 脚本与截图均在 `/tmp/bodysense-ui/`。没有改动前端源代码；`apps/web/index.html` 是任务开始前已有修改。

## 2026-10-12 续跑收尾（设计修改和结构验收完成）

- 原 T3 Codex Astra 运行已中断，避免多执行者并发写入；Oracle2 Penpot 编辑器连接已恢复。
- 完成 `last-polish.js`，修复助手确认卡的细节文案，并将两个诊断收起态绑定 46 处颜色令牌。
- 将两个匿名 `Board` 恢复为 `Desktop/Mobile / Window / Assessment / Collapsed`，修复诊断状态间的无效引用。
- 修正诊断的 6 个桌面/移动状态窗口：关闭按钮关闭当前 overlay，“与助手复核更新”打开对应身体助手 overlay。
- 将 8 张原本位于 63,000～90,650px 远处的训练/诊断变体移入 9,000～11,040px 状态画板区；该区 10 张画板未检测到重叠。
- Person 身体记录空态精简：Desktop 720→625px、Mobile 650→575px，空态面板统一高度 250px，保留原有 Tab 和点击区域。
- 对主要业务窗口的 30 张画板完成分窗检查；对两个新增诊断收起态补充检查；修复后相关检查均无越界、空热点、失效引用或过小热点。
- 成功导出并人工查看人物、装备、诊断、训练、移动端和历程窗口，以及精简后的双端人物窗口；Penpot 原生 `File.validate()` 返回空问题列表。
- 最终命名版本已保存并从版本列表核实：`BodySense · Window and Person UI refinement · QA 2026-10-12`。
- 尚未进行浏览器中的逐步点击式原型播放回归；上述验收是结构及静态视觉验收，不应表述为端到端交互播放通过。
- 本轮未修改 BodySense 前端代码；原有 `apps/web/index.html` 工作区修改在任务开始前已存在。

## 2026-10-12 顶栏极简调整（已实施并检查）

- 统一缩短独立窗口顶栏：桌面 56px、移动 52px。涉及 74 张独立顶栏画板：22 张 Window/训练执行与 Person 主窗，18 张器材选择/表单/结果，30 张通用工作流，以及 4 张安全提示和今日训练状态。
- Person 主窗仅保留 Avatar、昵称与关闭控件，去除旧说明文案 `我的身体档案 · 点击完善个人信息`；保留头像与昵称的原点击热点及分类 Tab。
- 其他窗口顶栏移除左侧图标、标题、说明、步骤提示，只保留关闭按钮；业务正文、动作、反馈和结果区整体上移。嵌套于 Person 的分类内容沿用 Person 主窗顶栏，不重复添加关闭按钮。
- 关闭热点至少 44×44px，原有原型交互引用未重建。器材参数的嵌套字段与工作流底部反馈区同步平移，防止保留旧顶栏空白。
- 74/74 张目标画板的结构检验通过：无残留顶栏左侧文本、无越界图形、关闭热点可用且未超出顶部、Person 的头像/昵称存在且旧描述已移除。
- 导出并人工检查桌面与移动 Person、训练、器材表单、评估输入、人物资料编辑和安全提示的实际效果。Penpot `File.validate()` 无错误。
- 已创建并在版本历史核实版本 `BodySense · Minimal headers across forms · 2026-10-12`；修改前版本 `BodySense · Before minimal headers · 2026-10-12` 保留用于对照。
- 当前验收为画板结构 + 静态视觉复查，不等于所有原型流程均已在播放模式下逐步点击验证。本轮不修改前端源码。

## 2026-10-12 训练计划拖动栏合并（已实施）

- 按最新交互要求，将训练计划选择器直接并入窗口最顶部、包含关闭按钮的拖动栏；不是在拖动栏下面添加第二层 Header。
- 顶部采用 Penpot 原生 Flex Row：Left = 固定宽度计划选择器（最多 10 个汉字，超长省略）+ `3次/周 · 20分钟/次` 类简略信息；Right = 原窗口关闭按钮。Desktop 56px / Mobile 52px。
- 将计划切换从旧页面跳转改为 `open-overlay` 下拉浮层，绑定现存 Desktop/Mobile Plan Picker，保留各计划项的切换动作。
- 删除原 88px 计划摘要大卡片及“调整建议须经你确认……”提示条。
- 训练动作重组到单独的 `Training / Content / Scroll viewport` 裁切容器，顶部 Header 与底部 Footer 标记 `fixedWhenScrolling`；当前 6 个动作在 552px 窗口中完整可见。注意 Penpot 当前原型 API 不提供可配置的嵌套垂直滚动行为，不能据此宣称鼠标滚轮的局部滚动已被验证。
- 6 张训练窗口（桌面/移动 × 肩胛、臀腿、功能三份计划）全部通过结构检查，原有开始训练/助手调整/关闭交互继续保留；Penpot `File.validate()` 0 错误。
- 已导出并人工核对桌面和移动版视觉效果。保存并核实命名版本 `BodySense · Training selector in dragbar · 2026-10-12`。
- 本轮仅修改 Penpot 原型；未更新 React 前端实现。

## 2026-10-12 Diagnosis 角标和极简顶栏（已实施）

- 最新 UI 规则：Diagnosis 当前病情条目总数不出现在诊断窗口，而是显示在底部 Diagnosis Dock 图标右上角；原型用数字 `2`，正式业务数据源应取当前 `Diagnosis.ConditionEntry.length`，该数字不是未读通知。
- 更新三个 Dock 状态：Desktop Open、Desktop Active、Mobile Active。将原箱形图标替换为复用的 FileText 报告图标，旧 6px 圆点改为 20px 绿色计数角标，保留原 Dock 点击交互和桌面快捷键 D。
- 6 张诊断窗口状态画板统一使用最上方拖动栏的 3 个控件：左病情下拉选择器、右 AI 对话图标按钮、右关闭按钮；Desktop 56px、Mobile 52px，Flex 左右布局。不再展示计数、旧的大摘要卡或底部整行 AI 操作按钮。
- 新增 Desktop/Mobile × 第一/第二病情四张病情选择下拉浮层，能够通过已有两种诊断状态画板切换病情；AI 图标保留原有关闭诊断后打开助手窗口的操作。
- 诊断内容改为当前病情的“分析说明 / 处理方向 / 仍需明确”三个高密度区块，保留待核实状态和非临床诊断免责声明。正文为独立裁切容器，顶部固定；Penpot 嵌套滚轮滚动本身仍需播放模式验证，不以裁切代替真实滚动验收。
- 6/6 张窗口结构检查通过，3/3 张 Dock 图标检查通过，4 张下拉浮层均存在；Penpot `File.validate()` 返回空错误。已导出并检查桌面/移动诊断窗口、下拉浮层、Dock 图标。
- 修改前版本 `BodySense · Before diagnosis toolbar and dock badge · 2026-10-12`；最终版本 `BodySense · Diagnosis selector and Dock count badge · 2026-10-12`，已保存且在版本历史核实。
- 本轮仅更新 Penpot 设计；未更新 React 源码。

## 2026-10-12 Chat 窗口顶部时间（Penpot 已实施）

- 在原有 Chat/Assistant 顶部窗口栏左侧添加单行次级时间文案 `今天 12:32`，不新增工具栏或双行标题。桌面端顶栏 56px、移动端 52px；右侧保持原有 44×44px 关闭按钮和交互。
- 保留正文 `综合分析 · 待确认` 状态 chip、所有聊天气泡、复核建议卡和底部输入/发送区的原有位置与动作，不修改窗口整体高度。
- 为时间文本标记语义数据绑定 `conversation.lastMessageAt`；`今天 12:32` 只是 Penpot 演示用静态时间，正式产品需从会话消息时间获取并根据日期格式化，不应硬编码当前时间。
- Desktop / Window / Assistant、Mobile / Window / Assistant 两张画板结构 QA 通过；导出并人工检查两端截图。Penpot `File.validate()` 为 0 错误。
- 修改前命名版本：`BodySense · Before Chat header time · 2026-10-12`；实施后版本：`BodySense · Chat header conversation time · 2026-10-12`，已在版本历史核实。
- 本轮未修改 BodySense 前端源码。

## 2026-10-12 Chat 顶部完整日期占位（修正）

- 将桌面和移动 Chat 顶栏的 `今天 12:32` 改为完整日期 `2026年10月12日`，保持左侧单行、12px 次级文字和紧凑 Header，不增加新控件。
- 日期是暂时的顶部信息占位，业务语义从 `conversation.lastMessageAt` 修正为 `system.currentDate`，显示格式 `YYYY年M月D日`；目前 Penpot 内是静态示例，并非已经实现动态日期。
- 两张画板检查通过：日期没有与关闭按钮重叠，正文状态、关闭、发送等原交互不变；原生文件校验 0 错误。
- 已保存命名版本 `BodySense · Chat header full date placeholder · 2026-10-12`；本次未修改前端源代码。
