# BodySense Agent Engineering Extension

Full Stack Open and TECH SCHOOL provide a strong web/backend foundation, but BodySense also contains an AI runtime whose production concerns are not covered deeply enough by either source. This extension is mandatory for understanding BodySense as a whole.

## A1 — Typed Agent boundary

Learn:

- Pydantic models and runtime validation;
- PydanticAI typed dependencies/output;
- model proposal vs application authority;
- stable logical-model routing through LiteLLM.

Lab: trace one Agent execution from immutable configuration selection through Python output validation to Go persistence/decision authority.

## A2 — Runtime ownership and durable truth

Learn:

- ephemeral runtime state vs durable domain state;
- Go ownership vs Python checkpoint ownership;
- BodyState revision pinning;
- immutable historical analysis vs current applicability.

Lab: take one diagnosis/treatment artifact and list every field by owner, lifetime and mutability.

## A3 — Evidence, RAG and admissibility

Learn:

- retrieval vs evidence;
- provenance;
- EvidenceGap;
- admissibility and conflict;
- user facts vs external knowledge;
- targeted retrieval and budgets.

Lab: construct a case where retrieval succeeds but evidence must still be rejected, then identify the deterministic gate.

## A4 — Safety and decision authority

Learn:

- proposer/reasoner vs verifier vs authority;
- safety envelope;
- deny-overrides policy;
- abstain/escalate/block;
- fail-closed handling of unknown or malformed safety facts.

Lab: predict whether three adversarial cases are authorized before running the policy tests.

## A5 — Evaluation and configuration qualification

Learn:

- immutable Agent Configuration as qualification unit;
- deterministic evaluators;
- holdout/regression/challenge slices;
- paired non-inferiority;
- critical regression gates;
- interaction effects between model, prompt and tools.

Lab: inspect one qualification dataset and explain what evidence would be sufficient to promote or block a challenger.

## A6 — Decision trace and replay

Learn:

- observability trace vs decision trace;
- configuration provenance vs execution provenance;
- historical replay;
- counterfactual replay;
- current re-analysis;
- behavioral contract vs token-identical output.

Lab: take one frozen case and write what must remain invariant under historical replay versus what may vary under counterfactual replay.

## A7 — Streaming, interrupt/resume and HITL

Learn:

- public event ledger;
- sequence ownership;
- transport disconnect vs business cancellation;
- LangGraph checkpoint/thread ownership;
- ask-user interrupt/resume;
- idempotent replay and deduplication.

Lab: draw a timeline for disconnect -> continued execution -> reconnect/recovery and another for interrupt -> answer -> resume.

## A8 — Agent production debugging

Learn to find the first contract violation along:

```text
Input
-> Configuration
-> Reasoning proposal
-> Evidence path
-> Runtime facts
-> Decision authority
-> Persistence
-> Delivery
```

Lab: analyze a known/fabricated failure without defaulting to “the model was wrong”. Produce a failure-attribution report naming the earliest broken contract.

## Extension completion test

The learner can safely change an Agent-backed BodySense feature only when they can state:

- what the model is allowed to propose;
- what deterministic runtime/policy must verify;
- who owns durable truth;
- how evidence is admitted;
- how the configuration is qualified;
- how a production failure can be replayed and attributed.
