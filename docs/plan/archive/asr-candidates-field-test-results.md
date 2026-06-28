# ASR 候选实战测试结果：肱骨前移 / 肘外翻

## 测试目标

对两条新视频只做 `ASR` 层实战测试，验证 4 个候选在 BodySense 中文体态视频场景下的真实表现：

1. `faster-whisper large-v3-turbo`
2. `faster-whisper large-v3`
3. `FunASR Paraformer-zh GGUF`
4. `FunASR SenseVoiceSmall GGUF`

测试视频：

- `C:\Users\baker\Videos\凯圣王\肱骨前移.mp4`
- `C:\Users\baker\Videos\凯圣王\肘外翻.mp4`

产物目录：

- 结果总表：[summary.json](<D:\home\projects\BodySense\apps\ai-service\data\benchmarks\asr_field_test\summary.json>)
- 基准脚本：[benchmark_asr_candidates.py](<D:\home\projects\BodySense\apps\ai-service\scripts\benchmark_asr_candidates.py>)

## 实际运行方式

### Whisper 路线

- 后端：`faster-whisper`
- 模式：CPU `int8`
- 语言参数：`zh`

### FunASR 路线

- 不使用 Python `funasr` 包
- 原因：当前 Windows 环境下 `editdistance` 构建失败
- 改用官方 `llama.cpp / GGUF` runtime
- 模型量化：`q8`
- VAD：`fsmn-vad.gguf`

## 环境说明

- CPU：Intel Core i7-14650HX，16 核 / 24 线程
- 内存：31.78 GB
- 推理设备：全部按本地 CPU 路线测试

## 关键结论

### 1. `large-v3` 明显最重，但质量并没有重到“值回票价”

- `肘外翻`：`306.58s`，RTF `1.38`，峰值内存 `3397.80 MB`
- `肱骨前移`：`725.30s`，RTF `1.63`，峰值内存 `3443.84 MB`

它确实比 `turbo` 更稳一点，但在两条视频里仍然频繁把关键解剖词打坏，例如：

- `肱骨前移` 段里反复出现 `宫骨 / 弓骨 / 公股`
- `肱二头肌长头肌腱` 没有稳定命中
- `桡骨头` 在 `肘外翻` 里也没稳定保住

结论：在这台机器上，`large-v3` 的速度和内存代价太高，不适合作为 BodySense 的默认本地 ASR。

### 2. `large-v3-turbo` 比 `large-v3` 实用，但中文术语仍不够稳

- `肘外翻`：`109.09s`，RTF `0.49`，峰值内存 `1772.30 MB`
- `肱骨前移`：`238.36s`，RTF `0.54`，峰值内存 `1728.62 MB`

优点：

- 比 `large-v3` 快很多
- 文本分段和可读性还可以

问题：

- 在 `肱骨前移` 上仍然把核心词打成 `公骨前移`
- `胸小肌 / 肩袖肌群 / 肱二头肌长头肌腱` 等词稳定性不够
- 模型体积仍然很大，约 `1.55 GB`

结论：如果一定要留在 Whisper 生态里，`turbo` 是比 `large-v3` 更现实的选择，但它仍然不是这组中文视频的最优解。

### 3. `Paraformer` 是速度王，但文本后处理能力弱

- `肘外翻`：`18.22s`，RTF `0.08`，峰值内存 `376.83 MB`
- `肱骨前移`：`26.69s`，RTF `0.06`，峰值内存 `471.85 MB`

优点：

- 四个候选里最快
- 内存也低
- 两条视频的术语命中检查都过了

问题：

- 输出基本不带自然断句和标点
- 文本虽然快，但局部错词依然明显
- 更适合做高速底稿，不太适合直接面向用户展示

结论：如果目标是“批量铺底稿”，`Paraformer` 很强。

### 4. `SenseVoiceSmall` 是这轮测试里最均衡的候选

- `肘外翻`：`15.97s`，RTF `0.07`，峰值内存 `411.99 MB`
- `肱骨前移`：`28.98s`，RTF `0.07`，峰值内存 `454.23 MB`

优点：

- 速度几乎和 `Paraformer` 同级
- 内存控制也很好
- 标点和语句边界比 `Paraformer` 更自然
- 在两条视频上关键术语命中都比两档 Whisper 更稳

问题：

- 仍然会出现局部术语误识别
- 还达不到“完全免精修”

结论：如果要在当前机器和当前视频类型上选一个最适合 BodySense 的本地 ASR 基线，优先推荐 `SenseVoiceSmall GGUF`。

## 性能总表

说明：下表使用的是“模型已就绪后的单次推理耗时”，不包含首次下载模型的冷启动时间。

| 视频 | 候选 | 耗时 | 音频时长 | RTF | 峰值内存 | 模型体积 |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| 肱骨前移 | `fw_turbo` | 238.36s | 444.31s | 0.54 | 1728.62 MB | 1546.54 MB |
| 肱骨前移 | `fw_large_v3` | 725.30s | 444.31s | 1.63 | 3443.84 MB | 2947.65 MB |
| 肱骨前移 | `funasr_paraformer` | 26.69s | 444.31s | 0.06 | 471.85 MB | 225.95 MB |
| 肱骨前移 | `funasr_sensevoice` | 28.98s | 444.31s | 0.07 | 454.23 MB | 242.43 MB |
| 肘外翻 | `fw_turbo` | 109.09s | 222.63s | 0.49 | 1772.30 MB | 1546.54 MB |
| 肘外翻 | `fw_large_v3` | 306.58s | 222.63s | 1.38 | 3397.80 MB | 2947.65 MB |
| 肘外翻 | `funasr_paraformer` | 18.22s | 222.63s | 0.08 | 376.83 MB | 225.95 MB |
| 肘外翻 | `funasr_sensevoice` | 15.97s | 222.63s | 0.07 | 411.99 MB | 242.43 MB |

## 术语命中检查

### 肘外翻视频

检查词：

- `肘外翻`
- `桡骨头`
- `肱骨`
- `肘管`
- `鹰嘴`

结果：

- `funasr_paraformer`：全部命中
- `funasr_sensevoice`：全部命中
- `fw_large_v3`：`桡骨头` 未稳定命中
- `fw_turbo`：`桡骨头` 未稳定命中

### 肱骨前移视频

检查词：

- `肱骨前移`
- `肩胛骨`
- `胸小肌`
- `肩袖肌群`
- `肱二头肌长头肌腱`

结果：

- `funasr_paraformer`：全部命中
- `funasr_sensevoice`：全部命中
- `fw_large_v3`：`肱二头肌长头肌腱` 未稳定命中
- `fw_turbo`：`胸小肌 / 肩袖肌群 / 肱二头肌长头肌腱` 未稳定命中

## 冷启动与工程层面的实际情况

### FunASR 路线更顺

- 官方 Windows runtime 二进制很小
- GGUF 模型下载和复用简单
- 不依赖 Python 运行时

### Whisper 路线踩到了两个现实问题

1. 模型太大  
   `turbo` 约 `1.55 GB`，`large-v3` 约 `2.95 GB`
2. 社区 `turbo` CT2 模型首次自动下载一度卡住  
   最后通过“直接按文件下载 `model.bin` 等文件”才稳定跑通

这两个点说明 Whisper 路线不是不能用，而是实际工程体验明显不如 FunASR GGUF 路线顺手。

## 最终建议

### 默认生产候选

优先选 `FunASR SenseVoiceSmall GGUF`。

理由：

- 速度快
- 内存低
- 文本比 `Paraformer` 更像可直接读的稿子
- 在这两条中文体态视频上，关键术语稳定性优于两档 Whisper

### 次优候选

`FunASR Paraformer-zh GGUF`

适合场景：

- 批量快速跑底稿
- 更在乎吞吐量
- 后面本来就会有人或规则层做文本清洗

### 不建议作为当前默认基线

- `faster-whisper large-v3`
- `faster-whisper large-v3-turbo`

不是因为它们完全不能用，而是因为在这台本地机器和这类中文视频上：

- 更慢
- 更吃内存
- 术语优势并不明显

## 对 BodySense 的直接落地建议

1. 下一阶段把 `SenseVoiceSmall GGUF` 接成新的默认 ASR
2. 保留 `Paraformer` 作为“快速批量转录”后备模式
3. 在 ASR 后增加一个体态术语纠错层
   例如统一校正：
   - `肱骨前移`
   - `肩袖肌群`
   - `肱二头肌长头肌腱`
   - `桡骨头`
   - `肱骨外上髁`
4. 暂时不要把 `large-v3` 当默认路线推进

## 附：本次测试里最有价值的工程结论

这轮测试最重要的结论不是“哪个模型更先进”，而是：

> 对 BodySense 这种中文体态知识库场景，模型是否“更适合中文术语 + 本地运行更顺 + 足够快”，比它是不是 Whisper 家族更重要。

从这个角度看，这次实战里 `SenseVoiceSmall` 的综合表现最好。
