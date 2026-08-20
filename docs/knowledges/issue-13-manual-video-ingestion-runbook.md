# Issue #13: 单视频手动入库操作手册

## 适用场景

这份文档面向"我要亲手把一个本地视频变成 BodySense 知识库里的正式数据"的场景。

目标不是只跑通脚本，而是让你能手动完成下面整条链路：

1. 准备本地环境
2. 配置 ASR、切分策略与 embedding
3. 跑自动转录与知识切分（可选 AI 精修）
4. 检查产物
5. 正式写库
6. 验证数据库里真的出现了对应的 `source / segments / units / clips`
7. 可选地继续做人工精修并覆盖入库

## 先理解：一个视频不会只变成"一条库数据"

一个视频正式入库后，通常会对应 4 类数据库记录：

1. `knowledge_sources`
   - 1 条
   - 表示这个视频来源本身

2. `knowledge_segments`
   - 多条
   - 表示 ASR 切出来的逐段转录

3. `knowledge_units`
   - 多条
   - 表示真正给搜索和问答使用的知识卡片

4. `knowledge_clips`
   - 多条
   - 表示动作演示或讲解片段

所以"从一个视频到知识库中的一条数据"，更准确地说，是：

- 一个视频先变成一条 `knowledge_sources`
- 再衍生出多条 `knowledge_units`
- 问答真正消费的重点是 `knowledge_units`

## 本手册采用的示例素材

示例视频：

- `C:\Users\baker\Videos\凯圣王\头前移.mp4`

示例问题：

- `problem_slug=forward-head-posture`
- `problem_display_name=头前移`
- `author=凯圣王`

## 前置依赖

请先确认本机具备以下工具：

1. Docker Desktop
2. Go 1.26+
3. Python 3.13+
4. `uv`
5. `ffmpeg`
6. `ffprobe`

### 快速自检命令

```powershell
docker --version
go version
python --version
uv --version
ffmpeg -version
ffprobe -version
```

如果这里有任何一个命令不存在，先补齐环境再继续。

## 目录与关键脚本

本流程最重要的几个入口：

- 自动入库脚本：`apps/ai-service/scripts/ingest_video_source.py`
- 精修入库脚本：`apps/ai-service/scripts/ingest_curated_source.py`
- 视频自动管道：`apps/ai-service/src/rag/video_pipeline.py`
- ASR 提供商：`apps/ai-service/src/rag/asr/`
- 知识切分：`apps/ai-service/src/rag/splitter.py`
- AI 切分：`apps/ai-service/src/rag/ai_splitter.py`
- AI 精修：`apps/ai-service/src/rag/ai_curator.py`
- 正式写库逻辑：`apps/ai-service/src/rag/knowledge_library.py`
- 迁移定义：`apps/api/migrations/000010_create_knowledge_library.up.sql`

## 第 1 步：准备 `.env`

仓库根目录需要有 `.env`。

如果没有，就从 `.env.example` 复制一份：

```powershell
Copy-Item .env.example .env
```

### 当前这条链路最关键的配置

至少确认这些变量是对的：

```env
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=bodysense
DB_USER=bodysense
DB_PASSWORD=bodysense123

REDIS_HOST=localhost
REDIS_PORT=6384
REDIS_PASSWORD=bodysense123

API_PORT=8080
AI_SERVICE_PORT=8100
AI_SERVICE_URL=http://localhost:8100

EMBEDDING_PROVIDER=hashing
EMBEDDING_DIMENSIONS=1536
```

## 非常重要：`EMBEDDING_DIMENSIONS` 必须是 `1536`

当前数据库里的 `knowledge_units.embedding` 列已经固定为：

- `VECTOR(1536)`

所以无论你使用：

1. `hashing`
2. `openrouter`
3. 其他 OpenAI 兼容 embedding

都必须保证最终写入的是 1536 维向量。

如果你配成 384 或 768，入库时会失败。

## 第 2 步：启动数据库和 Redis

最简单的方式，是直接用项目现成的 dev compose：

```powershell
docker compose -f docker/docker-compose.yml --profile dev up -d postgres-dev redis-dev
```

### 检查容器是否正常

```powershell
docker compose -f docker/docker-compose.yml --profile dev ps
```

你至少应该看到：

1. `postgres-dev`
2. `redis-dev`

处于运行状态。

## 第 3 步：跑一次 API，自动完成数据库迁移

知识库表不是 ingestion 脚本自动建的，而是由 Go API 启动时自动跑 migration。

最稳妥的方式是把 API 也拉起来一次：

```powershell
docker compose -f docker/docker-compose.yml --profile dev up -d api
```

### 为什么要这一步

因为 `apps/api/cmd/server/main.go` 启动时会调用：

- `database.RunMigrations(dbCfg)

把全部 migration 应用到数据库里，包括：

- `knowledge_sources`
- `knowledge_segments`
- `knowledge_units`
- `knowledge_clips`

### 检查 API 是否启动成功

```powershell
Invoke-RestMethod http://localhost:8080/api/health
```

看到类似：

```json
{
  "status": "ok",
  "service": "bodysense-api",
  "db": "ok",
  "redis": "ok"
}
```

就说明迁移基本已经跑好了。

## 第 4 步：安装 ai-service 依赖

进入 AI 服务目录：

```powershell
Set-Location D:\home\projects\BodySense\apps\ai-service
```

### 最小安装

如果你只是要跑视频自动入库，通常先执行：

```powershell
uv sync
```

### 如果你准备用本地 transformer embedding

再补装 `ml` 额外依赖：

```powershell
uv sync --extra ml
```

### 如果你还要跑 OCR 路由

才需要：

```powershell
uv sync --extra ocr
```

当前"视频入库"本身不依赖 OCR。

## 第 5 步：把根目录 `.env` 注入当前 PowerShell 会话

这一点非常容易踩坑。

`ingest_video_source.py` 这个脚本本身**不会自动加载根目录 `.env`**，所以你要先把环境变量注入当前 shell。

在仓库根目录执行：

```powershell
Set-Location D:\home\projects\BodySense
Get-Content .env | ForEach-Object {
  if ($_ -match '^(.*?)=(.*)$') {
    [System.Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process')
  }
}
```

如果你想显式覆盖数据库连接，也可以再补一条：

```powershell
$env:DATABASE_URL = 'postgresql://bodysense:bodysense123@127.0.0.1:5432/bodysense'
```

## 第 6 步：选择 ASR provider

当前自动入库支持三条 ASR 路径。

## 方案 A：`whisper.cpp`（本地，离线）

适合：

1. 你想先用最简单、最稳定的离线方案
2. 想控制模型大小
3. 允许后续再做更多人工精修

### 支持的模型

`asr/whisper_cpp.py` 当前内置支持：

1. `ggml-tiny.bin`
2. `ggml-base.bin`
3. `ggml-small.bin`

### 模型怎么选

- `ggml-tiny.bin`
  - 最快
  - 质量最低
  - 只适合快速冒烟测试

- `ggml-base.bin`
  - 速度和质量比较均衡
  - 适合第一轮批量自动底稿

- `ggml-small.bin`
  - 更慢
  - 质量通常更好
  - 适合需要减少后续人工修正时使用

### 配置方式

```powershell
--transcript-provider whisper.cpp --whisper-model ggml-base.bin
```

## 方案 B：`funasr_sensevoice`（本地，离线）

适合：

1. 中文语音场景更强
2. 希望降低中文错字率
3. 可以接受首次运行自动下载 runtime 和模型

### 当前内置模型

当前代码内置支持：

- `sensevoice-small-q8.gguf`

### 配置方式

```powershell
--transcript-provider funasr_sensevoice --transcript-model sensevoice-small-q8.gguf
```

### 首次运行会自动下载什么

1. FunASR Windows runtime
2. `sensevoice-small-q8.gguf`

缓存目录位于：

- `apps/ai-service/data/.cache/funasr_runtime/`
- `apps/ai-service/data/.cache/funasr_models/`

## 方案 C：`asr_api`（远程 API）

适合：

1. 不想占用本地算力
2. 追求更高的转录质量
3. 有可用的 ASR API 服务（MiMo、OpenAI、LocalAI 等）

### 配置方式

在 `.env` 中配置：

```env
ASR_PROVIDER=asr_api
ASR_API_KEY=your-api-key
ASR_API_BASE_URL=https://token-plan-cn.xiaomimimo.com/v1
ASR_API_MODEL=mimo-v2.5-asr
```

然后脚本参数：

```powershell
--transcript-provider asr_api
```

### 支持的 API 服务

任何 OpenAI-compatible 的 `/v1/audio/transcriptions` 接口都可以，包括：

- MiMo ASR（`mimo-v2.5-asr`）
- OpenAI Whisper API（`whisper-1`）
- LocalAI（自部署）
- faster-whisper-server

## 第 7 步：选择知识切分策略

当前支持两种切分策略：

### 方案 A：`heuristic`（默认，零成本）

关键词规则 + 滑动窗口，不需要 LLM，零外部依赖。

```powershell
--splitter-provider heuristic
```

或通过环境变量：

```env
SPLITTER_PROVIDER=heuristic
```

### 方案 B：`llm`（LLM 语义切分）

用 LLM 做语义理解，切分质量更高，标题/摘要更自然。需要配置 `LITELLM_API_KEY` 和 `LITELLM_BASE_URL`。

```powershell
--splitter-provider llm
```

如果 LLM 调用失败，会自动 fallback 到启发式切分（不会阻塞流程）。

## 第 8 步：可选，启用 AI 精修

AI 精修会对每个知识单元做：

1. 润色标题和摘要
2. 丰富正文结构（Markdown 格式）
3. 补充标签
4. 质量评分（低于 0.6 的自动标记为 `ai_flagged`）

```powershell
--ai-refine
```

需要配置 `LITELLM_API_KEY` 和 `LITELLM_BASE_URL`。失败的单元会保留原样，不会阻塞流程。

## 第 9 步：先做 dry-run，不直接写库

正式入库前，强烈建议先跑一遍 dry-run。

### 示例 1：最简模式（本地 whisper + 启发式切分）

```powershell
Set-Location D:\home\projects\BodySense\apps\ai-service
uv run python scripts\ingest_video_source.py `
  "C:\Users\baker\Videos\凯圣王\头前移.mp4" `
  --problem-slug forward-head-posture `
  --problem-display-name "头前移" `
  --author "凯圣王" `
  --dry-run
```

### 示例 2：MiMo ASR + LLM 切分 + AI 精修

```powershell
Set-Location D:\home\projects\BodySense\apps\ai-service
uv run python scripts\ingest_video_source.py `
  "C:\Users\baker\Videos\凯圣王\头前移.mp4" `
  --problem-slug forward-head-posture `
  --problem-display-name "头前移" `
  --author "凯圣王" `
  --transcript-provider asr_api `
  --splitter-provider llm `
  --ai-refine `
  --dry-run
```

### 示例 3：SenseVoice 本地

```powershell
Set-Location D:\home\projects\BodySense\apps\ai-service
uv run python scripts\ingest_video_source.py `
  "C:\Users\baker\Videos\凯圣王\头前移.mp4" `
  --problem-slug forward-head-posture `
  --problem-display-name "头前移" `
  --author "凯圣王" `
  --transcript-provider funasr_sensevoice `
  --transcript-model sensevoice-small-q8.gguf `
  --dry-run
```

### dry-run 成功后，你应该看到

终端会打印类似：

1. `Source key: ...`
2. `Artifact dir: ...`
3. `Transcript segments: ...`
4. `Knowledge units: ...`
5. `Clips: ...`

## 第 10 步：检查自动生成产物

dry-run 成功后，到这个目录检查：

- `apps/ai-service/data/knowledge_sources/{source_key}/`

以头前移为例，典型目录会是：

- `apps/ai-service/data/knowledge_sources/凯圣王-forward-head-posture-头前移/`

### 重点检查 4 类文件

1. `audio.wav`
2. `transcript.raw.jsonl`
3. `transcript.txt`
4. `generated_pack.json`

如果开启 clip 导出，还要看：

5. `clips/*.mp4`

### 人工检查重点

#### `transcript.txt`

重点看：

1. 是否能大致听懂原句
2. 专业术语错字是否过多
3. 时间轴是否合理

#### `generated_pack.json`

重点看：

1. `source.source_key` 是否符合预期
2. `units` 是否已经切成多个知识块
3. `unit_type` 是否大致合理
4. `clips` 是否真的对应了 `self_check / exercise / warning`
5. 如果启用了 AI 精修，检查 `review_status` 是否有 `ai_flagged` 的单元

如果这一层已经很差，不要急着写库，先换 ASR provider 或换模型。

## 第 11 步：正式把自动生成结果写库

确认产物没问题后，去掉 `--dry-run` 再执行一次。

### `whisper.cpp` 示例

```powershell
Set-Location D:\home\projects\BodySense\apps\ai-service
uv run python scripts\ingest_video_source.py `
  "C:\Users\baker\Videos\凯圣王\头前移.mp4" `
  --problem-slug forward-head-posture `
  --problem-display-name "头前移" `
  --author "凯圣王" `
  --transcript-provider whisper.cpp `
  --whisper-model ggml-base.bin `
  --overwrite-source
```

### MiMo ASR + LLM 切分 + AI 精修 示例

```powershell
Set-Location D:\home\projects\BodySense\apps\ai-service
uv run python scripts\ingest_video_source.py `
  "C:\Users\baker\Videos\凯圣王\头前移.mp4" `
  --problem-slug forward-head-posture `
  --problem-display-name "头前移" `
  --author "凯圣王" `
  --transcript-provider asr_api `
  --splitter-provider llm `
  --ai-refine `
  --overwrite-source
```

### `--overwrite-source` 是做什么的

如果同一个 `source_key` 已经存在：

- 不加它：脚本会返回 `already_exists`
- 加了它：会删掉旧来源，再写入新版本

这对你切换 ASR 模型重新跑时特别有用。

## 第 12 步：验证数据库里真的有数据

建议从 3 个层面验证：

1. 查 API
2. 查 SQL
3. 查具体某一条 `knowledge_units`

## 验证 A：查知识库统计

如果 API 还在运行，可以直接调用：

```powershell
Invoke-RestMethod http://localhost:8080/api/knowledge/stats
```

你应该能看到：

- `knowledge_sources`
- `knowledge_segments`
- `knowledge_units`
- `knowledge_clips`

四张表的数量。

## 验证 B：查 sources 列表

```powershell
Invoke-RestMethod http://localhost:8080/api/knowledge/sources
```

确认新视频的：

1. `source_key`
2. `title`
3. `author`
4. `problem_slug`

都已经出现。

## 验证 C：直接查 PostgreSQL

进入 Postgres 容器：

```powershell
docker exec -it bodysense-local-postgres-dev-1 psql -U bodysense -d bodysense
```

如果你的容器名不同，先执行：

```powershell
docker ps --format "{{.Names}}"
```

找到实际名字再替换。

### 1. 查 source 记录

```sql
SELECT id, source_key, title, author, problem_slug, transcript_provider, transcript_model
FROM knowledge_sources
WHERE source_key = '凯圣王-forward-head-posture-头前移';
```

### 2. 查 segment 数量

```sql
SELECT COUNT(*) AS segment_count
FROM knowledge_segments
WHERE source_id = (
  SELECT id
  FROM knowledge_sources
  WHERE source_key = '凯圣王-forward-head-posture-头前移'
);
```

### 3. 查 unit 列表

```sql
SELECT id, unit_key, unit_type, title, review_status
FROM knowledge_units
WHERE source_id = (
  SELECT id
  FROM knowledge_sources
  WHERE source_key = '凯圣王-forward-head-posture-头前移'
)
ORDER BY id;
```

### 4. 查 clip 列表

```sql
SELECT id, clip_key, clip_type, title, file_path
FROM knowledge_clips
WHERE source_id = (
  SELECT id
  FROM knowledge_sources
  WHERE source_key = '凯圣王-forward-head-posture-头前移'
)
ORDER BY id;
```

## 第 13 步：追踪"某一条知识数据"是怎么来的

如果你想从数据库里的一条知识卡片，反查它来自视频哪一段，可以这样查。

### 先找到某一条 unit

```sql
SELECT id, unit_key, title, source_start_sec, source_end_sec, evidence_segment_indices
FROM knowledge_units
WHERE unit_key = 'fhp-self-check';
```

你会得到：

1. 这条知识卡片的时间范围
2. 它引用了哪些 `segment_index`

### 再查对应 segment 文本

```sql
SELECT segment_index, start_sec, end_sec, transcript
FROM knowledge_segments
WHERE source_id = (
  SELECT source_id
  FROM knowledge_units
  WHERE unit_key = 'fhp-self-check'
)
AND segment_index = ANY(
  SELECT evidence_segment_indices
  FROM knowledge_units
  WHERE unit_key = 'fhp-self-check'
)
ORDER BY segment_index;
```

这一步就能让你把：

- 一条 `knowledge_units`
- 对应的原始转录证据
- 对应的视频时间段

三者串起来。

## 第 14 步：验证搜索是否能命中

如果你已经正式写库，可以直接搜索验证。

### 例子：搜索"头前移怎么自测"

```powershell
$body = @{
  query = "头前移怎么自测"
  top_k = 5
} | ConvertTo-Json

Invoke-RestMethod `
  -Uri http://localhost:8080/api/knowledge/search `
  -Method Post `
  -ContentType "application/json" `
  -Body $body
```

预期：

1. 会返回 `results`
2. 前几条里应出现 `self_check` 或 `definition`
3. 如果该条目绑定了 clip，还会带回对应 `clips`

## 第 15 步：可选，把自动包升级成精修包

如果自动结果可读性一般，但整体结构已经对了，就不要重新从零做。

正确姿势是：

1. 保留当前 `generated_pack.json`
2. 新建或修改 `curated_sources/{problem_slug}/xxx.json`
3. 再通过精修脚本覆盖入库

### 精修脚本示例

```powershell
Set-Location D:\home\projects\BodySense\apps\ai-service
uv run python scripts\ingest_curated_source.py `
  "D:\home\projects\BodySense\apps\ai-service\data\knowledge_sources\凯圣王-forward-head-posture-头前移\generated_pack.json" `
  "D:\home\projects\BodySense\apps\ai-service\data\curated_sources\forward-head-posture\kai-sheng-wang-head-forward-v1.json" `
  --overwrite-source
```

### 精修版会做什么

1. 保留原始 `transcript_segments`
2. 用人工撰写的 `units` 替换自动单元
3. 用人工指定的 clip 替换自动 clip
4. 把 `review_status` 提升为 `curated`

这才是最终更适合给问答链路使用的版本。

## 推荐的人工验收清单

每次正式写库前，建议人工过一遍下面 8 项：

1. `source_key` 是否清晰且唯一
2. `problem_slug` 是否标准化
3. `transcript.txt` 是否可读
4. `generated_pack.json` 的 `units` 数量是否合理
5. `unit_type` 是否基本符合内容
6. `clips` 是否真的能看懂动作
7. `knowledge/search` 是否能命中正确主题
8. 是否需要进入 `curated_pack` 精修阶段

## 常见问题排查

### 1. 脚本能跑，但一写库就报错

先看 3 个地方：

1. `DATABASE_URL` 是否指向正确库
2. migration 是否已经执行
3. `EMBEDDING_DIMENSIONS` 是否是 `1536`

### 2. 首次跑 `SenseVoice` 很慢

正常。

因为它会自动下载：

1. runtime
2. 模型文件

第二次通常会快很多。

### 3. `whisper.cpp` 结果错字很多

优先尝试：

1. 从 `ggml-base.bin` 升到 `ggml-small.bin`
2. 改用 `funasr_sensevoice`
3. 改用 `asr_api`（如 MiMo ASR）
4. 进入人工精修流程

### 4. 生成了 pack，但 `knowledge/search` 命中很差

通常是以下原因之一：

1. 自动标题太差
2. `unit_type` 不稳定
3. embedding 质量不够
4. 还没做精修版覆盖

### 5. 为什么我搜到了条目，但问答回答还是一般

因为"能搜到"只代表技术链路通了，不代表知识质量够好。

问答体验最终主要取决于：

1. 精修后的 `title`
2. `summary`
3. `body_markdown`
4. 风险边界
5. 动作卡片质量

### 6. LLM 切分或 AI 精修失败了怎么办

不用担心，两个都有降级策略：

- LLM 切分失败 → 自动 fallback 到启发式切分（日志有 warning）
- AI 精修某个单元失败 → 该单元保留原样，不影响其他单元

检查日志中的 warning 信息，通常是 API key 或网络问题。

## 一条最短可执行路径

如果你只想最快从零跑通一次，最短路径是：

1. `Copy-Item .env.example .env`
2. 把 `.env` 中 `EMBEDDING_DIMENSIONS` 设成 `1536`
3. `docker compose -f docker/docker-compose.yml --profile dev up -d postgres-dev redis-dev api`
4. `cd apps/ai-service`
5. `uv sync`
6. 在 PowerShell 会话里加载根目录 `.env`
7. 先执行 `ingest_video_source.py ... --dry-run`
8. 检查 `transcript.txt` 和 `generated_pack.json`
9. 去掉 `--dry-run` 再正式写库
10. 用 `/api/knowledge/stats`、`/api/knowledge/sources` 和 SQL 三层验证

## 全部参数速查

```powershell
uv run python scripts\ingest_video_source.py <video_path> `
  --problem-slug <slug> `
  --problem-display-name <中文名> `
  --author <作者> `
  --source-title <标题，默认取文件名> `
  --language zh `
  --transcript-provider whisper.cpp `      # whisper.cpp | funasr_sensevoice | asr_api
  --whisper-model ggml-base.bin `          # whisper.cpp 专用
  --transcript-model <模型名> `            # funasr_sensevoice 专用
  --splitter-provider heuristic `          # heuristic | llm
  --ai-refine `                            # 启用 AI 精修
  --force-transcribe `                     # 强制重新转录
  --no-export-clips `                      # 跳过 clip 导出
  --dry-run `                              # 只生成产物，不写库
  --overwrite-source                       # 覆盖已有的 source
```

## 相关入口

- 操作背景文档：`docs/knowledges/issue-13-video-knowledge-pipeline.md`
- 自动入库脚本：`apps/ai-service/scripts/ingest_video_source.py`
- 精修入库脚本：`apps/ai-service/scripts/ingest_curated_source.py`
- 视频自动管道：`apps/ai-service/src/rag/video_pipeline.py`
- ASR 子系统：`apps/ai-service/src/rag/asr/`
- 知识切分：`apps/ai-service/src/rag/splitter.py`
- AI 切分：`apps/ai-service/src/rag/ai_splitter.py`
- AI 精修：`apps/ai-service/src/rag/ai_curator.py`
- 知识库写入：`apps/ai-service/src/rag/knowledge_library.py`
- 头前移精修 spec：`apps/ai-service/data/curated_sources/forward-head-posture/kai-sheng-wang-head-forward-v1.json`
