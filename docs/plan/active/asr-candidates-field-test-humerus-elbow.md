# ASR 候选实战测试计划：肱骨前移 / 肘外翻

## 目标

对两条新视频进行仅 `ASR` 层的实战测试，验证 4 个候选在 BodySense 中文体态视频场景下的实际表现：

1. `faster-whisper + large-v3-turbo`
2. `faster-whisper + large-v3`
3. `FunASR Paraformer-zh`（官方 llama.cpp / GGUF runtime）
4. `FunASR SenseVoiceSmall`（官方 llama.cpp / GGUF runtime）

测试素材：

- `C:\Users\baker\Videos\凯圣王\肱骨前移.mp4`
- `C:\Users\baker\Videos\凯圣王\肘外翻.mp4`

## 目标产出

1. 每条视频统一产出 `audio.wav`
2. 每个候选统一产出：
   - 原始转录文本
   - 结构化 benchmark JSON
   - 摘样片段，便于人工比读
3. 一份总结文档，覆盖：
   - 耗时
   - 内存 / 资源占用
   - 中文术语表现
   - 在 BodySense 场景下的推荐优先级

## 执行策略

### 阶段 1：统一基准脚本

在 `apps/ai-service/scripts/` 下新增一个可复用脚本，负责：

1. 抽取音频
2. 自动下载所需模型 / runtime
3. 依次跑 4 个候选
4. 统计转录耗时与产物路径

### 阶段 2：候选实际运行方式

#### Whisper 路线

- 使用 `faster-whisper`
- 优先选择 CPU `int8` 模式，避免 4GB 显存成为瓶颈
- 显式指定 `language=zh`

#### FunASR 路线

- 不走 Python `funasr` 包，因为当前 Windows + Python 环境下 `editdistance` 构建失败
- 改走官方 `llama.cpp / GGUF` runtime
- 使用官方 Windows 预编译二进制
- 优先使用 `q8` GGUF 模型，接近官方“体积减半、精度近似”的推荐落地方式

### 阶段 3：人工质检

至少检查以下内容：

1. 标题级专业词：
   - 肱骨前移
   - 肘外翻
2. 解剖 / 动作术语：
   - 肩峰
   - 肩胛
   - 胸锁乳突肌
   - 枕下肌群
   - 肱骨
   - 肘关节
3. 句子流畅度：
   - 是否大面积断句异常
   - 是否出现明显误翻译或英文漂移

## 风险与假设

1. `faster-whisper large-v3-turbo` 的 CTranslate2 模型采用社区转换版本，不是 SYSTRAN 官方仓库内置短名。
2. FunASR Python 包在当前环境不可直接用，不作为本次阻塞项；优先验证官方 GGUF runtime。
3. 如果某个候选在本机无法稳定跑通，也要保留失败记录并写入最终结论。
