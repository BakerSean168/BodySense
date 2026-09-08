---
id: bodysense-fundamentals
title: BodySense Master Course
status: active
level: intermediate
language: go, python, javascript, typescript, react
created_at: 2026-07-13
updated_at: 2026-09-08
---

# Goal

以当前 production-shaped BodySense 主仓库作为唯一长期练习项目，系统覆盖 Full Stack Open 与 TECH SCHOOL Backend Master Class 的知识/训练目标，并补齐 BodySense 特有的 Agent engineering。

课程唯一入口：`docs/learning/README.md`。

课程编号只用于定位来源与练习：

- `FSO-*`：Full Stack Open source point；`BS-FSO-*`：对应 BodySense exercise；
- `TECH-*`：TECH SCHOOL source lecture；`BS-TECH-*`：对应 BodySense exercise；
- `BS-A*`：BodySense Agent extension exercise。

这些编号不是新的平行项目。Phonebook、Blog List、Simple Bank 不作为长期学习代码库。

# Current Focus

**课程重构的 source/exercise/section mapping 已完成，当前进入“高价值 `EXERCISE_READY` 图谱 + 定向 paragraph/example 审计”阶段。正式 placement 只在 ready slice 上进行。**

2026-09-08 本轮后，已可核实的状态：

```text
FSO Parts 0-7 numbered exercises: 158/158 MAPPED
FSO Parts 8-14: current MOOC API source indexed; 198/198 exercise records MAPPED
FSO concept audit: Parts 0-14 = 664/664 current section-heading units dispositioned; 469 explicit section/subheading/prose-derived concepts; pinned core h4-h6 = 196/196 dispositioned; targeted prose-risk = 10/10 selected fragments reviewed, 2 new concepts; exhaustive paragraph/example parity remains separate
FSO historical Parts 8-11: 104 exercise records archived for comparison
TECH public lectures #0-#77: 78/78 title-level MAPPED
Agent A1-A8: 8/8 EXERCISE_READY
EXERCISE_READY: 113
LEARNER_VERIFIED: 0 in the new mastery ledger
```

当前可执行课程已经扩展为 **113-node ready graph**。学习时优先使用 `docs/learning/curriculum/views/study-tracks.md` 的 10 条 learner-facing tracks；机器依赖真相仍由 `prerequisite-spine.md` + ledgers 维护。

下一步优先级：

1. 使用 `study-tracks.md` 维护 113 个 ready 节点的可学习路径，并继续补真正高价值的 readiness；
2. 扩展已建立的 fingerprinted paragraph/example prose-risk 审计；首批 10/10 高风险 core sections 已完成，primary section 与 pinned-core nested h4-h6 heading inventory 也均无 PENDING；
3. 不把 exercise/heading/concept mapping 或已有 production tests 当成 learner mastery；
4. 选择一条 ready track 后再做 placement audit，从第一个低于 L4 的 prerequisite 开始。

# Prior Mastery Evidence

Diagnosis Production Agent 学习已经形成可复用的高阶证据，不因课程重构而重置：

- Go handler/service/repository、constructor DI、DIP / composition root；
- Python `Protocol`、Pydantic/PydanticAI typed boundaries；
- LiteLLM logical routing 与 provider boundary；
- immutable Agent Configuration、qualification、non-inferiority；
- EvidenceGap / acquisition / admissibility；
- SafetyEnvelope / deterministic DecisionAuthority；
- durable Diagnosis domain、DecisionTrace、provenance；
- historical / counterfactual replay；
- behavioral contract 与 production failure attribution；
- shadow / canary / promotion governance。

这些项目将在新的 parity 课程里通过 placement exercise 认定对应 mastery level，而不是重复实现。历史证据入口仍保留：

- `docs/plan/archive/2026-08-diagnosis-agent-platform/diagnosis-agent-governance-eval-plan-2026-08-19.md`；
- Diagnosis general qualification：**7/7**；
- EvidenceGap policy suite：**5/5**；
- v1 -> v2 -> v3 paired non-inferiority：零 critical regression。

这些是历史学习/工程证据，不会自动写成新的 `LEARNER_VERIFIED`；placement 时仍需确认它们是否满足对应 exercise 的 L4/L5 acceptance contract。

# Session Log

## 2026-09-08 · Frontend architecture/build readiness + 113-node graph

- 将 **16 个 frontend architecture/build/testing concepts** 从 `MAPPED` 提升为 `EXERCISE_READY`：client routing、route params、imperative navigation、route-owned data、form labels、frontend integration boundary、E2E black-box、negative E2E、coverage semantics、transpilation、bundling、Vite dev-vs-prod、Vite config、feature organization、React.memo、monorepo topology。
- 新增第 **10 条 learner-facing track**：`Frontend routing, build and application architecture`；React testing track 同时接入 integration/E2E/coverage，React hooks track 接入 React.memo。
- 每张 card 都要求区分“开发时能跑”和“生产构建/深链/URL ownership/可访问性/黑盒边界真的成立”，避免把 Vite/Router/Playwright 仅当作 API 用法记忆。
- executable graph 由 **97 -> 113 nodes**：FSO 89 + TECH 16 + Agent 8；仍为 **0 LEARNER_VERIFIED**。
- 真实验证：`pnpm nx build @bodysense/web` passed（3670 modules transformed，产出 hashed/code-split chunks；同时观测到 BodyExplorer3D chunk >500kB 的 Vite warning，作为后续 measurement evidence 而非立即重构理由）；routing/service focused suite **3 files / 23 tests passed**；Playwright discovery 成功列出 **10 tests / 6 files**；Nx project inventory 正常。
- 复跑 `pnpm exec playwright test --list` 后 stderr 为空、exit 0，稳定列出 **10 tests / 6 files**；因此这里只证明 E2E 配置与 test discovery 可用，不把 test listing 冒充真实浏览器全链路通过。


## 2026-09-08 · Targeted prose-risk audit + 97-node graph

- 新增 fingerprinted **targeted prose-risk audit**：对 pinned Full Stack Open core 中 10 个高风险 h3 fragment 做完整段落/示例级阅读，source-state SHA-256 为 `ab0d861320e86c30276651c7e59c8177f4529ee72d9943831fb5da1ecfabd845`；**10/10 REVIEWED，0 PENDING**。
- 审计不是“全篇逐段完成率”，而是风险选样的 omission-detection layer；validator 会校验 pinned commit、fragment hash、review disposition、reciprocal concept links 与新暴露概念。
- prose review 找到 **2 个 heading audit 未显式暴露的概念**：Testing Library `getBy/findBy/queryBy` 的同步存在/异步出现/预期缺失语义，以及 CSP/HSTS/nosniff/frame/referrer 等浏览器 security headers；两者均进入 canonical ledger 并直接生成 L4 exercise cards。
- 将 dependency/supply-chain security 也提升为 `EXERCISE_READY`，覆盖 lockfile/frozen install/audit/install scripts/maintainer compromise/升级回归风险。
- executable graph 由 **94 -> 97 nodes**：FSO 73 + TECH 16 + Agent 8；仍为 **0 LEARNER_VERIFIED**。
- 新增 `core-prose-risk-audit.md` generated view、refresh/generator scripts，并把新 ready nodes 纳入现有 9 条 study tracks；track generator 继续 fail closed，禁止 ready node 在 learner-facing map 中失踪。
- 真实验证：Testing Library/React focused suite **3 files / 18 tests passed**，Web TypeScript typecheck passed；`validate-production-proxy.sh` 返回 `PRODUCTION_PROXY_CONTRACT=PASS`；Go refresh/logout/session authority focused suites passed；`pnpm curriculum:check` passed。


## 2026-09-08 · Relational persistence readiness + 94-node graph

- 将 **8 个 relational/persistence concept nodes** 提升为 `EXERCISE_READY`：database layer boundaries、FK/join、relational query、intentional projection、migration/model separation、transactional user-owned insert、direct database inspection、eager-vs-lazy loading。
- 这些节点直接基于 BodySense 当前 Go/PostgreSQL/GORM/migration/repository/service 结构，不强制改成 FSO 的 Sequelize 技术栈；重点是 SQL/约束/ownership/transaction/query-count 等可迁移语义。
- Go backend track 从 14 个显式节点扩展为 **22 个**，把 TECH SCHOOL 的 schema/transaction/lock/isolation/auth/worker 与 FSO relational semantics 合并在同一 prerequisite graph 中。
- executable graph 由 **86 -> 94 nodes**：FSO 70 + TECH 16 + Agent 8；仍为 **0 LEARNER_VERIFIED**。
- 真实验证：`go test ./internal/database ./internal/repository -count=1` passed；service 的 BodyState/HealthWorkspace/Auth/Treatment/Consultation focused suite passed；curriculum validator/status passed。
- 仍遵循规则：已有 repository/service tests 只证明 exercise target 可观察，不自动证明学习者达到 L4。


## 2026-09-08 · React core readiness + 86-node study graph

- 将 **24 个前端高价值 concept nodes** 从 `MAPPED` 提升为 `EXERCISE_READY`：React component/JSX/props/render cycle/useState/event/state ownership/immutable update/queued state/hook rules，browser async/Promise/useEffect/list key/controlled input，Testing Library/user-event/stateful component tests，以及 hooks mental model/custom hooks/useMemo/useCallback/error boundary。
- 为每个节点新增独立 L4 card，要求先预测 render/lifecycle/identity/failure，再用 BodySense 真实组件、hooks、tests 或浏览器证据验证；memoization 明确禁止 cargo-cult 化，production change 仅在测量/回归证据支持时进行。
- 修复一个 stale curriculum target：`FSO-P1-CONCEPT-RENDER-CYCLE` 原指向已不存在的 `runtime/ActiveTurnProvider.tsx`，更新为当前 `context/ActiveTurnContext.tsx`。
- executable graph 由 **62 -> 86 nodes**：FSO 62 + TECH 16 + Agent 8；仍为 **0 LEARNER_VERIFIED**。
- `study-tracks.md` 从 7 条扩为 **9 条**，新增「React component model, async effects and hooks」与「React component testing and failure isolation」；generator 现在会拒绝任何未被 learner-facing track 收录的 ready node。
- 真实验证：React/hook/error-boundary focused suite **6 files / 45 tests passed**，Web TypeScript typecheck passed，`pnpm curriculum:check` passed。
- 这一步扩大的是可执行训练覆盖，不把已有 production code/tests 自动算成掌握。


## 2026-09-08 · Pinned core h4-h6 nested-heading completeness audit

- 发现 core Parts 0~7 原 `source_sections` 有意只索引 h3 teaching headings；虽然 numbered exercises 独立索引，但 h4/h5 的工具链子主题、bonus tests、alternative exercise variants 与 source-removed tracks 仍可能形成可见性盲区。
- 对 pinned Full Stack Open commit `0711aef8a451c4458263e5587ccda85f08fd7a96` 增加独立 h4-h6 inventory，排除 fenced code block 内伪 heading；source fingerprint 为 `f240ac03b800fa109fcf584658022d2f2ac1364e653bf74feab06b5b132969c1`。
- **196/196 nested heading units 全部 dispositioned**：118 covered by current numbered exercise、4 current alternative exercise variants（Part 7 React Query + Context 7.11~7.14）、32 nested technical mappings、6 redundant、17 non-engineering、19 `REMOVED_SOURCE_TRACK`（Part 6 明确标记已从当前课程移除的 Redux exercise material）。
- 新增 5 个此前只藏在 nested heading 中的显式 concepts：test-quality meta-check、minification、source maps、build plugins、polyfills；FSO explicit concepts 由 462 增至 **467**。
- 新增 machine-checkable `full-stack-open-core-subheading-audit.json`、refresh script、validator hard gate 与 generated `core-subheading-audit.md`；若未来 pinned source fingerprint/row count 改变，curriculum check 会拒绝沿用旧完成声明。
- 此审计关闭的是已知 **heading-level** 缺口；仍不宣称 unheaded paragraph/code example/warning 已逐段完成 semantic parity。

## 2026-09-08 · 62-node executable graph + learner-facing study tracks

- 从已完成的 FSO section concept ledger 中挑选 **30 个高 ROI concept nodes** 提升为 `EXERCISE_READY`：HTTP/middleware/CORS/error taxonomy、auth/revocation/browser token/XSS/BOLA、TanStack Query/state ownership/realtime recovery、TypeScript structural typing/runtime trust、CI/CD provenance/safe deploy、Docker image/Compose/network/volume。
- 每个新增 ready concept 都有独立 L4 card：prediction、failure case、BodySense target、focused verification、explain-back、production-change rule 与 reviewed prerequisite closure。
- 当前 executable graph：**FSO 38 + TECH 16 + Agent 8 = 62 nodes**；仍为 **0 LEARNER_VERIFIED**。
- 新增生成式 `views/study-tracks.md`，将 62-node graph 组织为 7 条重叠但可复用 prerequisite evidence 的学习路径；ledger/prerequisite graph 仍是唯一依赖真相。
- 真实系统验证：Go middleware/auth focused suites passed；Web 5 files / **42 tests passed** + TypeScript typecheck passed；stream-event parser **12/12 passed**；delivery platform **31/31 passed**；`docker compose ... config --quiet` passed。
- 这些测试验证的是课程 exercise target 当前确实可执行/可观察，不自动授予任何 learner mastery。

## 2026-09-08 · Agent extension 8/8 executable readiness

- 将 A3 Evidence/RAG/admissibility、A4 deterministic safety/DecisionAuthority、A5 eval/qualification/rollout、A6 DecisionTrace/replay、A8 production failure attribution 从 `MAPPED` 提升为 `EXERCISE_READY`。
- 补齐 A1~A8 prerequisite closure：A8 作为综合 debugging capstone 依赖 A1~A7；所有 ready prerequisite 均通过 curriculum validator。
- 新增 5 张 L4 exercise cards，要求 prediction -> failure case -> focused test/trace -> explain-back，禁止把“已有测试全绿”当作掌握证明。
- 真实验证：AI service 相关 7 个 test files **31/31 passed**；Go diagnosis decision/rollout/replay/provenance focused suite passed。
- Agent extension 当前 **8/8 EXERCISE_READY, 0/8 LEARNER_VERIFIED**；课程仍等待学习者实际 placement/练习证据。

## 2026-09-08 · FSO current Parts 8~14 section-level semantic audit complete

- 完成 current MOOC P8 GraphQL 51/51 section units：schema/query/resolver/mutation/error、Apollo client/cache、auth context、subscription/WebSocket、N+1 等均以 `COMPARE` 为主映射到当前 REST/SSE/TanStack Query/PostgreSQL 架构，不为课程强行迁移生产技术栈。
- 完成 P9 TypeScript 64/64：structural typing、inference/erasure、unknown/any/assertion、guards/Zod、React props/state/discriminated union/exhaustiveness、utility/distributive types、typed HTTP trust boundary 等形成显式 concept records。
- 完成 P10 React Native 56/56：Expo/native renderer/device storage/navigation/testing/pagination 等保留为 `OPTIONAL/COMPARE`，复用 Web/GraphQL/state/test 已有知识，不强制建设移动端。
- 完成 P11 CI/CD 64/64：CI workflow/runner/reproducibility/gates/deploy health/version provenance/branch protection/supply-chain pinning/metrics/scheduled automation 映射到 BodySense 当前 GitHub Actions 与 production delivery。
- 完成 P12 Containers 42/42：image/container/Dockerfile/Compose/volumes/network/DNS/dev loop/multi-stage/Redis/reverse proxy/orchestration 等映射到现有容器与部署配置。
- 完成 P13 Relational DB 43/43：PostgreSQL/GORM/migrations/constraints/joins/many-to-many/eager-lazy/ORM/query/migration-history 等映射到现有 Go persistence layer。
- 完成 P14 Next.js 65/65：App Router/RSC/Server Actions/static-vs-dynamic/cache revalidation/Auth.js/Route Handlers/Suspense/SEO 等以 `COMPARE` 映射到当前 Vite SPA + Go API，明确何时才值得迁移。
- Full Stack Open 当前 source 的 **664/664 section-heading review units 已全部 dispositioned**；当前 ledger 共 **467 个显式 section/subheading-derived concept records**。这只表示 section-level semantic coverage，不把 heading 覆盖冒充 paragraph/example-level 全量知识 parity。

## 2026-09-08 · FSO core Parts 0~7 section-level concept audit complete

- 完成 Part 3（40/40）Backend 基础语义审计：HTTP server/framework、route/body/method semantics、middleware/CORS、deployment/proxy、database/schema/repository boundaries、environment/secrets、error taxonomy、lint。
- 完成 Part 4（23/23）Backend testing/auth 语义审计：unit/API integration tests、deterministic fixtures、async/await、regression-first refactor、relations/projections、password hashing、bearer auth、token revocation、HTTPS。
- 完成 Part 5（42/42）Frontend testing/router 语义审计：auth UI/token persistence、refs/children、Testing Library、coverage/snapshot、Playwright/E2E fixture/locator/debugging、React Router、UI library/styling trade-offs。
- 完成 Part 6（51/51）State management 语义审计：Flux/Redux/Zustand、server vs client state、Fetch、async store actions、TanStack Query/invalidation、Context、Redux Toolkit/Thunk；重复章节显式链接已有 concept。
- 完成 Part 7（24/24）Advanced React/build/security 语义审计：hooks/memoization、bundling/Vite/esbuild/transpilation、error boundary、monorepo/feature organization、SSE/WebSocket/polling、security，以及 TypeScript/SSR-RSC/Next.js/microservices/serverless 的架构比较。
- core Parts 0~7 的 **279/279 active section-heading review units 已全部 dispositioned**；全课程当前 664 个 section units 中还剩 **385 个 Parts 8~14 current MOOC units** 待审。
- 这些结果只代表 section-level semantic disposition；不会把 heading 数量或 concept mapping 冒充最终 paragraph/prose-level knowledge parity，也不会自动升级 learner mastery。

## 2026-09-07 · FSO Parts 0~2 section-level concept audit complete

- 新增 `full-stack-open-concept-audit.json`，为当前 664 个 active source-section headings 建立独立 disposition 状态：`PENDING / REVIEWED_CONCEPTS_MAPPED / REVIEWED_NON_ENGINEERING / REVIEWED_REDUNDANT`。
- Part 0：33/33 section units dispositioned；14 个 Web 基础 concept records（HTTP GET、browser runtime、DOM、form POST、AJAX、SPA、full-stack boundary 等）+ 19 个 course-logistics 明确标为 non-engineering。
- Part 1：37/37 dispositioned；React component/JS/state/event/hook/debugging/AI coding verification 等 31 个新增 concept records，参考资料章节显式标为 non-engineering。
- Part 2：29/29 dispositioned；collections/keys/modules/controlled form/npm/dev runtime/REST/server mutation/error feedback/initial async loading 等 20 个新增 concept records；重复的 arrays/event/debugging 语义显式链接到前序 concepts，而不是复制或静默忽略。
- 当前 active section audit 总状态：99 个 P0~2 单元已处理；其余 Parts 3~14 继续排队。即使全部 headings 完成，仍需最终 prose-level audit 才能宣称完整 knowledge parity。

## 2026-09-07 · Current FSO Parts 8~14 exercise semantic mapping complete

- 基于已固定的 current MOOC exercise UUID/title 与逐题 assignment semantic review，将 Parts 8~14 的 **198/198 exercise records** 全部提升为 `MAPPED`。
- 每条记录独立保存 exercise number、原创 concise objective、`DIRECT / COMPARE / OPTIONAL`、BodySense target surface、adaptation task 和 completion-evidence plan；没有使用“范围 = 覆盖”的 shortcut。
- Mode 分布：P8 GraphQL = 26 COMPARE + 4 OPTIONAL；P9 TypeScript = 35 DIRECT；P10 React Native = 30 OPTIONAL；P11 CI = 24 DIRECT；P12 Containers = 23 DIRECT + 2 Mongo-specific COMPARE；P13 Relational DB = 28 DIRECT；P14 Next.js = 26 COMPARE。
- 所有这些节点仍保持 `dependency_audit: UNMODELED`，不会因为 exercise mapping 完成就假装已有完整 prerequisite DAG，也不会自动升级为 `EXERCISE_READY`。
- Full Stack Open 当前**编号/平台 exercise-objective mapping**已覆盖 Parts 0~14；完整 knowledge parity 仍被 section/prose concept audit 和 executable exercise readiness 阻塞。

## 2026-09-07 · Current FSO MOOC source integrity recovered

- 通过 `courses.mooc.fi/api/v0/course-material` 公共 API 获取并固定 Full Stack Open 当前 Parts 8~14 元数据索引，不再把 Part 12/13/14 标为来源不可获取。
- 当前 MOOC source snapshot：198 个 exercise records、385 个 headings；snapshot SHA-256 `4a8093083af985c19e04b03bf876fd148676bd6c2b65327c7628d8b20122689d`。
- 历史 repository snapshot 的 Parts 8~11 共 104 exercise records / 132 headings 移入历史索引，只用于版本对照，不再冒充 current parity。
- 新增可重复刷新脚本 `refresh-fso-mooc-index.mjs` + `sync-fso-current-source.mjs`；普通 curriculum check 保持离线确定性。
- 此步骤只把 Parts 8~14 提升到 `SOURCE_INDEXED`，没有把 source availability 错算成 `MAPPED` / `EXERCISE_READY` / learner mastery。

## 2026-09-07 · Coverage ledger / executable curriculum hardening

- 接受完整性审计结论：上一版是课程蓝图，不能宣称知识/训练点 100% parity。
- 新增 machine-readable ledgers：FSO、TECH SCHOOL、BodySense Agent，明确 `SOURCE_INDEXED -> MAPPED -> EXERCISE_READY -> LEARNER_VERIFIED` 四阶段。
- FSO pinned snapshot Parts 0~11 建立 262 个不同编号记录；Parts 0~7 的 158 个 source exercise 已逐项语义映射，不再使用范围声明代替逐题关系。
- 修正 Part 1 错位：`1.13` 明确对应不可变 vote-like state update，`1.14` 明确对应最大值/最高票派生；调试作为额外能力而非挤占 source id。
- FSO Parts 8~11 明确为 historical snapshot inventory；Parts 12~14 当前 MOOC source 维持 `UNVERIFIED_CURRENT_MOOC`。
- 单独索引 411 个 snapshot section headings，并新增 Promise/Effect、useMemo/React.memo/useCallback、SQL injection、XSS、dependency security、broken auth/access control 等 semantic concept records；明确不把 section count 当作完整语义覆盖。
- TECH SCHOOL 固定 public README commit `97f000fe58ad01a0774179ffa8884ac7784cf263`，78/78 lecture ID/title 映射完成，但 authority 仅为 `VERIFIED_PUBLIC_README_TITLE_ONLY`。
- 建立 27 个首批可执行 exercise cards（包含高价值节点及其 prerequisite closure），覆盖 HTTP/SPA、stale conflict、transaction/lock/isolation、REST/DB error、refresh race、durable jobs、Agent streaming/replay；每个均有 failure case、verification 与 L4 hard gate。
- 新增 curriculum validator 与 generated coverage view；正式 placement 延后到相关 prerequisite slice 达到 `EXERCISE_READY` 之后。

## 2026-09-07 · 课程体系重建为 source-parity BodySense Master Course

- 退役旧 `docs/learning/00~06` topic tutorials 与第一版 `07-bodysense-open.md`，不再维护本地自创课程和外部课程两套结构。
- 新唯一入口为 `docs/learning/README.md`。
- Full Stack Open 采用 coverage parity：Parts 0~7 建立 `BS-FSO-*` 直接训练序列；Parts 8/9/11/12/13 做直接或比较迁移；React Native/Next.js 明确保留为 optional/compare，不静默跳过。
- TECH SCHOOL Backend Master Class 的 backend lecture #0~#77 已逐条映射到 BodySense，使用 `DIRECT` 或 `COMPARE` 标记，避免为了课程强制引入 sqlc/PASETO/gRPC/Asynq/Kubernetes。
- 新增 BodySense Agent extension，覆盖 typed Agent、runtime ownership、evidence、safety、eval、replay、HITL 与 failure attribution。
- 课程原则从“复制教程项目”改为“source objective -> BodySense prediction/trace/test -> 只有真实缺口才改 production”。
- 旧 Diagnosis 学习成果保留为 prior mastery evidence；这是当时的计划记录。当前规则已升级为：先让对应 prerequisite slice 达到 `EXERCISE_READY`，再做 placement，不机械从零重做已掌握内容。

## 2026-08-22 · Diagnosis 学习阶段完成与知识体系收口

- 将历史路线的 Diagnosis 阶段（旧编号 `L1`）从 80% 更新为 **100% 完成**。
- 复核 Diagnosis Agent Platform 归档计划，确认 configuration qualification、EvidenceGap runtime、DecisionAuthority、DecisionTrace/provenance、Replay、Shadow/Canary/Promotion 与统一 LiteLLM routing 已形成完整 production-shaped 闭环。
- 完成 EvidenceGap / Evidence Acquisition 深层 ownership 学习：模型负责 proposal/semantic reasoning，runtime 负责事实记录/验证，policy 负责 authority。
- 完成 Evidence Admissibility 学习：`retrieved != admissible != gap resolved`。
- 完成 Durable Diagnosis Domain Model：immutable Analysis、Candidate vs Hypothesis、Evidence/Gap/Attempt durable boundary、historical truth vs current applicability。
- 完成 SafetyEnvelope / deterministic DecisionAuthority：confidence/Judge 不能覆盖 hard blocker；unknown/malformed facts fail closed。
- 完成 DecisionTrace、Configuration/Execution Provenance、Historical/Counterfactual Replay 与 Behavioral Contract。
- 完成 production failure attribution：从 Input/Context 到 Delivery 查找第一个 contract violation。
- Thought Forest 已按原子知识 + 综合 MOC 重新整理；统一入口为 `BodySense Diagnosis Agent Architecture`。
- 当时下一阶段切换到历史路线的 **Treatment 阶段（旧编号 `L2`）**；由于当前代码已经有 production-shaped Treatment Agent 基础，学习目标改为理解/验证 Treatment 的 domain、safety、proposal/action authority 与 durable outcome ownership，而不是从 legacy migration 重新开始。

## 2026-08-18 · Oracle Two 统一学习工作区

- 确认主仓库与 learning snapshot 均已迁到 Oracle Two。
- 对比发现 main 的 Diagnosis production 实现已经比 snapshot 更成熟；因此不复制 snapshot source code。
- 从 snapshot 仅迁移缺失的 Diagnosis Agent/model 学习与回归测试到 main。
- 在 Oracle Two 安装 CPython 3.13.15，并用 uv 建立 `apps/ai-service/.venv`。
- 补齐开发测试需要的 dev + OCR extras。
- 新增 `apps/ai-service/pyrightconfig.json`，让 Pyright 直接识别项目 `.venv`。
- Diagnosis focused tests 扩展到 14 条并全部通过；Ruff clean；Pyright 0 errors。
- 当时曾新增 `docs/learning/00-unified-roadmap.md`；该旧路线已于 2026-09-07 随 Master Course 重构退役。
- learning snapshot 完成知识迁移后计划退役；主仓库成为唯一学习与实施工作区。
