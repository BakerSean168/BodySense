# Changelog

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
