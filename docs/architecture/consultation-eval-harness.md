# Consultation Eval Harness

## 目标

为 `apps/ai-service` 提供一个可重复执行的咨询 Agent 评估入口，把现有的规则测试、红旗检测和 RAG faithfulness 校验串成统一闭环，便于后续迭代、回归和结果复盘。

这套 harness 重点覆盖三类能力：

- `workflow`：意图分类、动作路由、分析前置条件判断
- `red_flags`：高危症状召回与安全兜底
- `faithfulness`：治疗动作是否由检索知识支撑

## 结构

- Case 数据：`apps/ai-service/data/evals/consultation_eval_cases.yaml`
- Runner：`apps/ai-service/src/evals/consultation_eval_runner.py`
- CLI：`apps/ai-service/scripts/run_consultation_eval.py`
- 输出目录：`apps/ai-service/data/benchmarks/consultation-evals/`

生成目录位于 `data/benchmarks/` 下，默认不纳入版本控制，适合持续跑本地对比结果。

## 运行方式

在仓库根目录运行：

```bash
pnpm nx run ai-service:eval
```

或在 `apps/ai-service` 目录内运行：

```bash
uv run python scripts/run_consultation_eval.py
```

执行后会产出：

- `consultation-eval-summary.json`
- `consultation-eval-summary.md`

## 设计取舍

- 先用结构化 case 做稳定回归，不依赖外部模型调用，保证本地和 CI 可重复。
- 红旗检测以高召回为优先，因此对命中类别使用“至少包含预期类别”的校验。
- faithfulness 先复用现有规则校验器，后续可升级为 `tokenization + embedding similarity` 或 `LLM judge`。

## 后续增强方向

- 接入真实运行时 trace，补充 `success rate / latency / interruption rate` 指标
- 加入更复杂的多轮会话 eval，覆盖 `interrupt / resume / tool use`
- 为 RAG 检索补充 `top-k 命中率`、`citation coverage` 和 `rerank` 质量评估
