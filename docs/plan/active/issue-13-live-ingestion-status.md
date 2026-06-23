# Issue 13 Live Ingestion Status

更新日期：2026-06-23

## 当前已正式入库的来源

1. `凯圣王-forward-head-posture-头前移`
2. `凯圣王-humerus-anterior-glide-肱骨前移`
3. `凯圣王-cubitus-valgus-肘外翻`

## 当前库内统计

- `knowledge_sources`: 3
- `knowledge_segments`: 229
- `knowledge_units`: 45
- `knowledge_clips`: 25

## 本轮新增结果

### 肱骨前移

- 转写 provider：`funasr_sensevoice`
- 模型：`sensevoice-small-q8.gguf`
- 正式入库结果：
  - `segments`: 35
  - `units`: 21
  - `clips`: 14

### 肘外翻

- 转写 provider：`funasr_sensevoice`
- 模型：`sensevoice-small-q8.gguf`
- 正式入库结果：
  - `segments`: 15
  - `units`: 9
  - `clips`: 6

## 现在是否可用于 AI 问答

可以。

判断标准：

1. 数据已经写入 `knowledge_sources / knowledge_segments / knowledge_units / knowledge_clips`
2. 检索层可以枚举 3 个 `source`
3. 新来源已经能被搜索命中，并能返回关联动作片段

## 当前质量结论

技术链路已经打通，但知识质量还没有达到“高可信临床科普”的精修状态。

当前短板主要有 3 个：

1. 自动生成的标题和摘要仍然带有 ASR 噪声
   - 例如会混入口头禅、错别字、错误术语切分
   - `肱骨前移`、`肘外翻` 的自动标题明显不如 `头前移 curated_pack`

2. 检索存在跨病种串扰
   - 对 `肱骨前移` 的问句，可能会把 `头前移` 的“什么是”定义排到更前面
   - 说明当前 `hashing embedding + 自动单元命名` 还不够稳

3. 自动结构化不等于专家级结构化
   - 现在已经有“定义 / 自测 / 影响 / 动作”这类单元
   - 但“肌肉失衡、受力机制、禁忌、动作步骤、适用人群”仍需人工精修

## 下一步建议

1. 优先把 `肱骨前移` 做成 `curated_pack`
   - 人工修正定义
   - 补齐肌肉失衡与受力机制
   - 精修动作标题、动作步骤、适用场景、注意事项

2. 再把 `肘外翻` 做成第二个 `curated_pack`
   - 形成可复用的精修模板

3. 在精修完成前，问答侧对新来源加上更强过滤
   - 优先按 `problem_slug` 过滤
   - 对“什么是 / 怎么练 / 风险 / 自测”做更强意图路由

4. 精修稳定后，再批量跑剩余视频
   - 先自动转写
   - 再半自动结构化
   - 最后人工审核入库
