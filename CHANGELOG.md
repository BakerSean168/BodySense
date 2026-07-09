# Changelog

## [0.3.0](https://github.com/T1moooo/BodySense/compare/v0.2.0...v0.3.0) (2026-07-09)


### Features

* **ai,api:** add FaithfulnessPolicy, wire governance observe-only persistence ([afdf961](https://github.com/T1moooo/BodySense/commit/afdf961c850a946e4891b0eeec0ec259c862d4f6))
* **ai,api:** wire ask_user HITL end-to-end path ([4624443](https://github.com/T1moooo/BodySense/commit/4624443f0d5653289cce62abc4be9bb0a98e0ff4))
* **ai:** add agent orchestrator skeleton and update knowledge library policies ([5b6f460](https://github.com/T1moooo/BodySense/commit/5b6f460607dd4a372675a0992d3291f21c41fd47))
* **ai:** add agent runtime module and update consultation tools ([6402795](https://github.com/T1moooo/BodySense/commit/6402795c3daaf197687bb29cb84581166851740d))
* **ai:** add curated specifications and ingest pelvic, shoulder, and thoracic mobility data ([c4e32e4](https://github.com/T1moooo/BodySense/commit/c4e32e492a336b6f677cfea39c4d494c9cd2fbc7))
* **ai:** add ingest_generated_source script to ingest raw generated packs ([8497306](https://github.com/T1moooo/BodySense/commit/849730682b5520ed9400fa406ca4b44b6204484d))
* **ai:** enhance ask_user tool with question normalization and smart options ([212955c](https://github.com/T1moooo/BodySense/commit/212955ccde5f4d5c2f4f462a5a1c11f1ed99bb29))
* **ai:** implement multi-provider router, circuit breaker, and unified AIService ([c2f02c8](https://github.com/T1moooo/BodySense/commit/c2f02c8e55c80a121cdf46a7c8bf72a8b9b3c4d9))
* **ai:** support multimodal assessment and chat stream events ([e7c44f8](https://github.com/T1moooo/BodySense/commit/e7c44f8835c831517594e30a4d4f7ad1fd416d65))
* **ai:** tighten ask_user interruption rules ([188e7fa](https://github.com/T1moooo/BodySense/commit/188e7fa85d4400e73cdf1b785b3be5c2fa97042c))
* **api,web,ai:** add service tests, AskUserCard error state, timed_out transition ([4fd2e32](https://github.com/T1moooo/BodySense/commit/4fd2e32078336457c43cdb534335f653774183b1))
* **api:** add assistant parts builder for structured message construction ([3cec4e4](https://github.com/T1moooo/BodySense/commit/3cec4e49e4bb7b5923ed03b780382b17cc7c6698))
* **api:** add consultation runtime module and refactor ai_client ([94f801f](https://github.com/T1moooo/BodySense/commit/94f801fa481213b6dee62bb4d3ffa31b19e614b5))
* **api:** add database migrations for user uploads and knowledge library ([0253042](https://github.com/T1moooo/BodySense/commit/025304237e747d809c845d55c82813dfd339dff0))
* **api:** add knowledge publication batches, chat runtime, and job runtime enhancements ([caffc99](https://github.com/T1moooo/BodySense/commit/caffc995fe04f07c5e234a56345a35e93d0bd3bf))
* **api:** add KnowledgePublication repository ([b41a438](https://github.com/T1moooo/BodySense/commit/b41a4388ca8a8dada7f9dc978340a15361d7409c))
* **api:** add message metadata and improve interaction handling ([b3c418c](https://github.com/T1moooo/BodySense/commit/b3c418cc7f9668b5d41e2517b7bab6aecd0fd184))
* **api:** converge consultation run runtime ([1b8069b](https://github.com/T1moooo/BodySense/commit/1b8069b41517f9a42f05cc1fbe0afbffb062dbfb))
* **api:** implement database migrations, repositories, and Gin handlers for session redesign ([154dd28](https://github.com/T1moooo/BodySense/commit/154dd289e605d2ae1015fb5d1a94c079ece42071))
* **api:** implement multimodal assessment, log adjustments, and chat SSE flow ([9ac61b6](https://github.com/T1moooo/BodySense/commit/9ac61b6e48b0d56727c2f9638adccb13e9cf9244))
* **api:** persist structured health features ([6f0e7cf](https://github.com/T1moooo/BodySense/commit/6f0e7cf1f5fe4d4a177554c24084b862025ffcab))
* complete ask_user runtime flow and structured health features ([af42132](https://github.com/T1moooo/BodySense/commit/af42132d335762e2f6e4877e269729d77ceaf384))
* converge consultation run architecture ([e0aacb0](https://github.com/T1moooo/BodySense/commit/e0aacb0495e786805689a810515b2fcd28c4c825))
* **docker:** add DigitalOcean production compose ([b286828](https://github.com/T1moooo/BodySense/commit/b286828f0294854e9417b6fc1bf486520743468b))
* **docker:** add resource limits and logging for DO compose ([5f016e1](https://github.com/T1moooo/BodySense/commit/5f016e16f7a177467b1514757de586eb9a172859))
* **web:** add streaming assistant turn and tool call components ([8910c0f](https://github.com/T1moooo/BodySense/commit/8910c0f1d7a4467317ffd9cf409b634fcc56d7a0))
* **web:** consume consultation run streams ([a87b56c](https://github.com/T1moooo/BodySense/commit/a87b56cbfbab35116bdd79e56f1a9a326d6fe772))
* **web:** enhance ConsultationPage with pending interactions and session management ([f877450](https://github.com/T1moooo/BodySense/commit/f877450869a37e7132d531cf6589c1c3ab1ddaf3))
* **web:** extend consultation types and service for new architecture ([32bdd06](https://github.com/T1moooo/BodySense/commit/32bdd06e500326f64a9f22a11a7e596105734d47))
* **web:** integrate @assistant-ui/react runtime, add shadcn/ui components, and redesign sidebar ([39717c7](https://github.com/T1moooo/BodySense/commit/39717c7c820c88d6edbe827d338f79d0dd039da2))
* **web:** integrate onboarding upload, diagnostic dual-sync, and immersive timer player ([6645f19](https://github.com/T1moooo/BodySense/commit/6645f195404af6c4150b9c3f2d33e87de861990a))
* **web:** replay ask_user interactions in thread timeline ([747845a](https://github.com/T1moooo/BodySense/commit/747845add68f89eeac0cb42dc8472c56af3ce48e))
* **web:** surface health features in consultation panel ([251ecbf](https://github.com/T1moooo/BodySense/commit/251ecbf09e1baa6dd04ba328933935aa38582821))
* **web:** update onboarding layouts and profile view/edit components ([47b541f](https://github.com/T1moooo/BodySense/commit/47b541f032484d86cb92ea167c1b3e7ae48a82bd))


### Bug Fixes

* **ai:** add tool call deduplication and improve interaction handling ([f937773](https://github.com/T1moooo/BodySense/commit/f937773516fa9e218c909fde341ecc9aa950fc88))
* **ai:** pool consultation checkpointer connections ([76ae4e1](https://github.com/T1moooo/BodySense/commit/76ae4e142e8dae8d5657987ac9dd6cb8bdcc8178))
* **ai:** prevent duplicate tool call emissions and add tool_call_id to events ([bd05fd1](https://github.com/T1moooo/BodySense/commit/bd05fd18cac29b32bcd4294babd096406ff09b17))
* **ai:** update knowledge ingestion scripts and ASR pipeline ([5795568](https://github.com/T1moooo/BodySense/commit/57955680a7c4091465aba3470688489b628a4ac8))
* **ai:** 修复 ruff E501 过长行报错 ([362352f](https://github.com/T1moooo/BodySense/commit/362352f5feae1b8a2b22068ec8a8c44fc00775f2))
* **api,ai,web:** unify stream event model across Go, Python, and TypeScript ([d84103e](https://github.com/T1moooo/BodySense/commit/d84103e32e42985545ebffd64a6a63a38c65259e))
* **api,ai:** quality fixes for tool error detection, governance output, and resume handler ([774e434](https://github.com/T1moooo/BodySense/commit/774e434e820e873ed9b8b6e22b80cddba8e59bc9))
* **api,web:** update chat handler and remove stale package-lock.json ([ce74641](https://github.com/T1moooo/BodySense/commit/ce7464196cf46bb7fa43305c3580f977a3cff591))
* **api,web:** wire resume to send_message flow, add ASK_USER_ENABLED feature flag ([804808e](https://github.com/T1moooo/BodySense/commit/804808e1151e12ca7862c01955a376d10e48824f))
* **api:** add job idempotency fields, fix OCR TOCTOU race, add waiting_user status ([9dfc0a3](https://github.com/T1moooo/BodySense/commit/9dfc0a3c7c32ad42de595501aee78528fdf20ad3))
* **api:** check and reuse existing empty consultation session ([3debc8f](https://github.com/T1moooo/BodySense/commit/3debc8f19caa7673e6d75625ea8a1416efb4841b))
* **api:** complete implementation audit gaps ([8aa6bf1](https://github.com/T1moooo/BodySense/commit/8aa6bf10cbca134c2d85fcc19618b8d6bf07362d))
* **api:** correct knowledge lifecycle schema gaps from 07a plan ([63043b0](https://github.com/T1moooo/BodySense/commit/63043b01eb746a788c73197b8a367064b1c7fc51))
* **api:** support managed database TLS connections ([0f9c697](https://github.com/T1moooo/BodySense/commit/0f9c697f5a13545df0d16de9e84a2833b9eafd97))
* **deps:** add interaction_id to stream event JSON schema ([c14a780](https://github.com/T1moooo/BodySense/commit/c14a780a97030b4b175dd58184d8750c7947ae6f))
* make workflow checks pass ([c7b3032](https://github.com/T1moooo/BodySense/commit/c7b30327beed391a393b0891476142c6262c781c))
* **web:** prevent duplicate session creation, fix SSE parsing and sync symptom list ([14ddd5b](https://github.com/T1moooo/BodySense/commit/14ddd5bce6769f9374453963bb521bf1c771cd0b))
* **web:** rename Button.tsx to button.tsx to match import casing ([a82a6f4](https://github.com/T1moooo/BodySense/commit/a82a6f4f07c56c89b0e1a87a98e77ca7fe26c18a))
* **web:** resolve CI lint failures ([80eb884](https://github.com/T1moooo/BodySense/commit/80eb88406eb1db73eb82405c81e6b62f9fdf1a2a))
* **web:** update upload step and training page components ([b4a2566](https://github.com/T1moooo/BodySense/commit/b4a2566d1ba4342d3bf807ae70e6cf73de076d7c))
* **web:** use PascalCase Button import to match component filename ([eabfd44](https://github.com/T1moooo/BodySense/commit/eabfd44607006dbc5649ecc08f3b4f4a0068eec5))
* **web:** 修复 ESLint 报错 ([a39ab2e](https://github.com/T1moooo/BodySense/commit/a39ab2ee313d7f316f2453ea66828d9f0c8556e5))


### Performance Improvements

* **web:** 优化认证 store 性能 ([cee8cdc](https://github.com/T1moooo/BodySense/commit/cee8cdc2df2be05da2485723a5fd8504a00d0c3d))
* **web:** 优化问诊工作台渲染性能 ([7932826](https://github.com/T1moooo/BodySense/commit/79328263a3884325499551702b2e96160a032df1))
* **web:** 优化问诊线程查询 hook 性能 ([ffeeac9](https://github.com/T1moooo/BodySense/commit/ffeeac94d69baed29242406f705275059ffc0f9d))
* **web:** 优化问诊页面渲染性能 ([26373b3](https://github.com/T1moooo/BodySense/commit/26373b39c1ac05ee6d9bb55a1cc7608826c10c35))
* **web:** 添加懒加载和骨架屏优化架构 ([2cad847](https://github.com/T1moooo/BodySense/commit/2cad847fde340e737bf68c58d66feaebd83d8690))
* 设置默认的新绘画语义为 `/` ， 同时保留`/new` 。 ([4841713](https://github.com/T1moooo/BodySense/commit/48417135ed07bf9e6f6bce1a36a627fa722f79ec))

## [0.2.0](https://github.com/T1moooo/BodySense/compare/v0.1.0...v0.2.0) (2026-06-23)


### Features

* **ai,api,web:** implement consultation chat with LLM streaming and symptom extraction ([0b7b9f1](https://github.com/T1moooo/BodySense/commit/0b7b9f1874b574c48c8a9a69e2bd251026c4c6ef))
* **ai,api,web:** implement diagnosis analysis and treatment plan generation ([c742594](https://github.com/T1moooo/BodySense/commit/c7425948b5d662f1f0c8bb54f83426598f168186))
* **ai,api,web:** implement health assessment report generation ([9b6f1e2](https://github.com/T1moooo/BodySense/commit/9b6f1e24634583fc9045496b96e8daa93034970b))
* **ai,api,web:** implement progress tracking and reassessment ([2f6283e](https://github.com/T1moooo/BodySense/commit/2f6283e0059907e12e896a65a965bfa067232961))
* **ai,api,web:** Issue [#10](https://github.com/T1moooo/BodySense/issues/10) - 可能性分析 + 方案生成 ([96d563c](https://github.com/T1moooo/BodySense/commit/96d563cb859c7667fed780a304dfe95472a5beb5))
* **ai,api,web:** Issue [#12](https://github.com/T1moooo/BodySense/issues/12) - 进度追踪 + 阶段性复评 ([743b8f5](https://github.com/T1moooo/BodySense/commit/743b8f5f3c47c8b00627cbb7c1e3d26172b6fd6f))
* **ai,api,web:** Issue [#6](https://github.com/T1moooo/BodySense/issues/6) - 咨询工作台 LLM 流式聊天 + 症状提取 ([f1308f6](https://github.com/T1moooo/BodySense/commit/f1308f6cc2534e892ef110d5bac59641dc864ccb))
* **ai,api,web:** Issue [#7](https://github.com/T1moooo/BodySense/issues/7) - 健康评估报告生成 ([3d0f8b3](https://github.com/T1moooo/BodySense/commit/3d0f8b3fbe2984dcdf8ca0d4675fde2886aa0f54))
* **ai,api:** implement issue-13 forward head knowledge pilot with RAG pipeline ([2b5d645](https://github.com/T1moooo/BodySense/commit/2b5d64595ec3aa80fc015eae488ed111bd4b3cde))
* **ai,api:** issue-13 forward head knowledge pilot with RAG pipeline ([2c417ba](https://github.com/T1moooo/BodySense/commit/2c417bac7a78bcc5da906135aa3c4a4a785efc3e))
* **ai:** add mimo model support for embedding and reranking ([0698291](https://github.com/T1moooo/BodySense/commit/069829100300bd8a06cd3ae56a6429be346682d5))
* **ai:** implement RAG infrastructure with pgvector, embedding, and semantic retrieval ([3b9b1e8](https://github.com/T1moooo/BodySense/commit/3b9b1e868fe6144079ae041f35319c5db09b758e)), closes [#3](https://github.com/T1moooo/BodySense/issues/3)
* **api,web:** implement training plan and daily check-in ([16e7daa](https://github.com/T1moooo/BodySense/commit/16e7daadd408b86a4e955cb16539871fecebb493))
* **api,web:** implement user auth with JWT and login/register UI ([19f6306](https://github.com/T1moooo/BodySense/commit/19f6306c742ad38063d4c989c0dd1b45ba773faa))
* **api,web:** Issue [#11](https://github.com/T1moooo/BodySense/issues/11) - 训练计划生成 + 每日打卡 ([55d962c](https://github.com/T1moooo/BodySense/commit/55d962c639637f8a082037442b9a8ed63d829b70))
* **docker:** 搭建开发环境基础设施 ([0b96db4](https://github.com/T1moooo/BodySense/commit/0b96db41ac98f03ea312ce4ab99cd313f0e5932a))
* issue5,ocr + upload... ([3c64431](https://github.com/T1moooo/BodySense/commit/3c64431f963b8b26a41947e655b86c1cca8cef7e))
* issue5,ocr + upload... ([c8d2438](https://github.com/T1moooo/BodySense/commit/c8d243883e861297ea70e7834ccd44b1295b48e6))
* **profile:** implement body info collection and profile management ([92a5430](https://github.com/T1moooo/BodySense/commit/92a54303c97a95ce97f3e3c4f94f9d250dbf1620))
* **profile:** implement body info collection and profile management ([6b56432](https://github.com/T1moooo/BodySense/commit/6b56432587f68c00154ec8fb1c517a214c4981fb)), closes [#4](https://github.com/T1moooo/BodySense/issues/4)
* **web:** implement info panel with body visualization ([755e32b](https://github.com/T1moooo/BodySense/commit/755e32b186963372d77b8b0b32449bb843bd43e5))
* **web:** implement session history page ([311a4f1](https://github.com/T1moooo/BodySense/commit/311a4f14ea2ecf5be1e786785b147e2682dfdac4))
* **web:** Issue [#8](https://github.com/T1moooo/BodySense/issues/8) - 信息面板 + 身体可视化 ([424e22a](https://github.com/T1moooo/BodySense/commit/424e22aa371f900d3abccfcd2a34f98b1152ee3a))
* **web:** Issue [#9](https://github.com/T1moooo/BodySense/issues/9) - 会话保存 + 历史记录 ([f5c3a4b](https://github.com/T1moooo/BodySense/commit/f5c3a4b3d98007790c15c756b8237f8d9d58a9de))
* 搭建开发基础设施并实现用户认证 + JWT 鉴权 ([b40cd47](https://github.com/T1moooo/BodySense/commit/b40cd47b8ac0fa95b5850d518e59237859f4921c))


### Bug Fixes

* **ai,api:** 修复测试、lint 错误、迁移冲突并优化镜像 ([1886aff](https://github.com/T1moooo/BodySense/commit/1886aff7439bc01864c97c4109defc564301c4d1))
* **api,ai:** resolve migration numbering conflict and align embedding dimension to 1536 ([648ee1f](https://github.com/T1moooo/BodySense/commit/648ee1f5c4aa82ba419efe7755444e28928057de))
* resolve Windows asyncio event loop issue and switch to sync psycopg ([d8d9b43](https://github.com/T1moooo/BodySense/commit/d8d9b43ac866dd1d63761891adc03926de12cf1b))
