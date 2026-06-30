# BodySense Context

## Domain

BodySense is a posture-health AI assistant. It helps users turn body-awareness inputs into safer consultation, diagnosis support, and training guidance.

## Core Terms

- **Conversation**: A durable chat thread owned by a user. It contains ordered messages and may have one active AI run.
- **Consultation Session**: The posture consultation state attached to a conversation. It tracks phase, extracted symptom information, diagnosis candidates, and treatment plan.
- **AI Run**: One model turn for a conversation. It owns idempotency, streaming state, tool audit records, governance review, and completion/failure state.
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
