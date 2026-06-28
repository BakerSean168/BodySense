# Issue #13: 头前移自动知识入库实施计划

## 背景

Issue #13 的目标是把合作 UP 主的视频内容转成 BodySense 可检索、可诊断、可引用动作演示的知识库资产。当前仓库原有 `knowledge_entries` 结构过于扁平，不利于把“原视频、转录、知识单元、动作 clip”优雅关联起来，因此本次以重构知识库结构的方式推进。

本计划先以本地素材 `C:\Users\baker\Videos\凯圣王\头前移.mp4` 为试点，跑通从单视频到自动转录、知识单元生成、动作切片、入库的完整流程。

## 本次决策

1. 采用归一化知识库结构：
   - `knowledge_sources`：视频来源
   - `knowledge_segments`：ASR 转录片段
   - `knowledge_units`：可检索的知识单元
   - `knowledge_clips`：动作或讲解视频片段
2. 默认使用本地 `whisper.cpp` + 本地 sentence-transformers 模型，避免依赖外部 API key。
3. 知识单元先由自动转录 + 启发式分段生成，后续可以继续人工精修。
4. 动作 clip 与知识单元一对多关联，供训练页和咨询页直接复用。

## 目录与产物

```
apps/ai-service/
  data/
    .cache/whisper/
    knowledge_sources/
      {source_key}/
        audio.wav
        transcript.raw.jsonl
        transcript.txt
        generated_pack.json
        clips/
  scripts/
    ingest_video_source.py
```

## 单视频工作流

### 阶段 1：自动转录

1. 为 `头前移.mp4` 提取 `audio.wav`
2. 用 `whisper.cpp` 自动输出 `transcript.raw.jsonl`
3. 渲染 `transcript.txt` 供人工快速复核

### 阶段 2：自动知识分段

根据转录文本的关键词和时长启发式生成 `knowledge_units`：

1. `explanation`：原理说明
2. `self_check`：自测与判断
3. `exercise`：改善动作
4. `cause`：影响与成因
5. `warning`：风险提醒

### 阶段 3：动作切片

自动为 `self_check / exercise / warning` 类型单元导出 clip：

- 默认在单元前后各补 1.5 秒
- 结果写入 `clips/`
- clip 元数据同步入库

### 阶段 4：统一入库

生成的 pack 统一写入归一化知识库，而不是旧的扁平表：

- `knowledge_sources`
- `knowledge_segments`
- `knowledge_units`
- `knowledge_clips`

### 阶段 5：质量验收

试点完成标准：

- `头前移.mp4` 成功产出 transcript artifacts
- 自动生成若干 `knowledge_units`
- 至少生成 1 个以上动作或讲解 clip
- 数据成功写入新知识库结构
- 检索问题如“头前移怎么自测”“头前移练什么”能召回对应条目

### 阶段 6：人工精修首条高价值知识源

在 `头前移` 试点跑通后，补一版可直接面向用户展示的精修结构：

1. 补齐 `definition / self_check / impact / muscle_imbalance / cause / warning` 这类解释性卡片
2. 把动作卡片细化到“目标、要点、常见错误、停止条件”
3. 额外沉淀 `habit` 类单元，覆盖办公、看手机、训练中的行为矫正
4. 优化检索意图，让“头前移怎么练”“头前移日常怎么改”“哪些肌肉有问题”能优先命中精修卡

## 头前移试点的自动化边界

这版优先实现“自动转录 + 自动入库 + 自动切片”，而不是直接追求完美医学编辑质量。因此：

1. 自动生成的标题和分段允许后续人工精修
2. 优先保证可检索、可追溯、可回放
3. 后续再叠加人工审核和更强的结构化抽取

## 后续扩展

当 `头前移` 试点跑通后，其余 5 个视频统一复用同一工作流：

- `骨盆前倾`
- `胸椎灵活度`
- `肱骨前移`
- `肩关节`
- `肘外翻`

届时只需要为每个视频新增一个 knowledge pack 目录，而不用再重写脚本。

## 执行命令

```powershell
cd D:\home\projects\BodySense\apps\ai-service
uv run pytest tests\unit\test_video_pipeline.py
uv run python scripts\ingest_video_source.py "C:\Users\baker\Videos\凯圣王\头前移.mp4" --problem-slug forward-head-posture --problem-display-name "头前移" --author "凯圣王" --dry-run
```
