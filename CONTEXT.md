# BodySense Context

## Domain

BodySense is a posture-health AI assistant. It helps users turn body-awareness inputs into safer consultation, diagnosis support, and training guidance.

## 现行业务术语（Product Domain）

业务规则以 [`docs/product/`](./docs/product/README.md) 为唯一来源，术语以 [`docs/product/glossary.md`](./docs/product/glossary.md) 为准：

- **BodyState 身体状态**：每位用户一份；明确的本人陈述自动记录，有歧义才 `ask_user`。
- **Diagnosis 综合诊断**：每位用户一份；`ConditionEntry` 病情条目只包含用户在 Chat 病情确认工具中选择「大概率是」或「先保留」的条目。
- **TrainingPlan 训练计划**：每位用户多份完整处方；Agent 建议是提案（接受调整 / 暂不修改）。
- **Person 个人档案**、**Trainer 训练执行**、**Chat**：业务窗口。Penpot 旧名 Assistant / Assessment / Profile / Training / Workflow-Training 分别对应 Chat / Diagnosis / Person / TrainingPlan / Trainer。

下方 Core Terms 描述的是**当前代码**的运行时与旧业务术语（Consultation Session、diagnosis candidates、treatment plan、assessment 属于旧模型），不得覆盖上面的业务规格。

## Core Terms

- **Conversation**: A durable chat thread owned by a user. It contains ordered messages and may have one active AI run.
- **Consultation Session**: The posture consultation state attached to a conversation. It tracks phase, extracted symptom information, diagnosis candidates, and treatment plan.
- **AI Run**: One model turn for a conversation. It owns idempotency, streaming state, tool audit records, governance review, and completion/failure state.
- **Agent Thread**: The durable Agent runtime thread owned by the Python Agent Runtime. It carries checkpointed message state, pending interrupts, and resume identity across turns.
- **Runtime Event Log**: The append-only record in Go that stores the durable public history of runs, tool activity, interrupts, and resumes for audit and replay.
- **Projection**: A read model derived from the Runtime Event Log for a specific consumer such as the Web thread view, pending interaction list, or analytics.
- **Stream Event**: A versioned event envelope shared by Go, Python, and Web. It is the contract for user-visible streaming behavior.
- **Tool Call**: A model-requested action executed by the agent runtime, such as symptom extraction, knowledge search, or asking the user for more information.
- **User Interaction**: A pending human-in-the-loop question raised by the `ask_user` tool and later resumed by the user.
- **Job**: A durable background task such as OCR. Jobs are recoverable through JobRuntime rather than being only in-process goroutines.
- **Knowledge Unit**: A searchable posture-health knowledge item derived from curated or generated sources.
- **Knowledge Publication**: A published batch or version that makes reviewed knowledge units eligible for user-facing search.
- **Health Journey**: The read-only aggregation of profile, uploads, consultation, assessment, and training progress.

## Module Vocabulary

- Runtime Modules should hide orchestration details behind small interfaces.
- Go is the business truth source for conversations, messages, runs, jobs, and user-owned state.
- Python is the AI reasoning and RAG source for model turns, tool execution, and knowledge search.
- Web consumes contracts and renders user workflows; it should not invent backend protocol variants locally.
