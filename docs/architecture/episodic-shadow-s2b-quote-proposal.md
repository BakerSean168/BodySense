# Episodic S2-B1: verbatim, non-clinical preference proposals (disabled)

Parent: #498 · S2 issue #515 · prerequisite S3 #514 · baseline S2-A #521.

## What is implemented

This is **one narrow S2-B slice**, not general conversational memory.
The future model may propose at most one exact, whole-message user statement
from a reviewed four-phrase vocabulary (brief, bullet, Chinese or detailed
replies). Its output carries only a source-message UUID, verbatim quote and
category; **no free-form summary, medical judgment, provider selection,
storage command or own user identity**.

The canonical Go↔Python private Proto is
`contracts/internal/agent-runtime/bodysense/runtime/v1/runtime.proto`
(`EpisodicShadowGenerationCommand`, `EpisodicShadowDraftProposal`).
Generated Proto remains confined to explicit typed adapters in Go/Python.
The Python PydanticAI service `propose_shadow_quote` requires an explicitly
injected model and `enabled=True`; its only test model is `TestModel`.
There is **no** HTTP route, runtime caller, LangGraph tool, worker,
model resolver, environment enable flag or request from the user's UI.

Python rejects unknown fields, noncontiguous/duplicate/oversized source
ranges, non-user quotes, paraphrases and unapproved text, and abstains on
a source not in the exact allowlist. This is a syntactic safeguard, not a
complete semantic clinical-risk classifier.

Go separately verifies the private Proto proposal, owner, exact source
message ids/seq order, budget and category, and it **constructs the stored
canonical phrase itself**. Go `SaveShadowDraft` checks the entire user
message again against the current owner-locked PostgreSQL source row before
persisting the same `shadow` status as S2-A. It rejects an edited source,
foreign owner, forged request, clinical evidence, asset/multimodal reference,
in-flight Run and stale context cursor. Failed generation does not stop chat
pruning. An empty proposal is no-op, not an empty note. The underlying SQL
source-erasure/finalizer gates remain enforced.

## Verification evidence

- The versioned synthetic shared corpus
  `contracts/internal/agent-runtime/fixtures/episodic-shadow-s2b.v1.json`
  verifies Chinese phrase policy parity in Go and Python.
- 120 synthetic exact-source quotes verify *structural* traceability across
  the two language boundaries. This is **not** the required sampled
  `n>=100` live-model claim-trace audit or the clinical authority gate.
- Python tests cover disabled execution, absent injected model, PydanticAI
  `TestModel`, Pydantic extra-field refusal, Proto unknown-field rejection,
  policy mismatch, duplicate ids and clinical/instruction contamination.
- Disposable PostgreSQL 18 runs verify owner/mismatched source rejection,
  source-text re-read after tampering, default-off writer, idempotence,
  no-op abstention, and existing S3 statement/asset erasure protections.
- Go, Python, protocol generation, code formatting and architecture policy
  checks remain mandatory, as does protected PR/Main CI.

## Explicitly remaining before real-world memory

The S2-B1 vocabulary is too narrow to serve as general episodic context.
It intentionally does **not** summarize health episodes or store any actual
patient content, and there is no model data egress authorization. Remaining
owner decisions and engineering follow-ups in #515/#516/#517:

1. Approve data egress/consent, semantic evidence and nonclinical policy,
   appropriate gateway supplier, retention caps and user-controlled deletion
   tombstones so deleted notes cannot be regenerated.
2. Perform the ADR's full `n>=100` model-output claim traceability audit
   (>=98%, zero untraceable clinical claims), shadow-vs-BodyState diagnosis
   and safety regression, 12-month storage and prune-latency benchmarks.
3. Add a best-effort, durably bounded generation trigger **only after**
   approvals; preserve default-off independence from chat pruning.
4. Ship user-visible inspect/delete controls (#516), then separately
   authorize Chat-only, non-clinical serving/recall (#517) with its own
   performance and quality acceptance. Do not open an S4 reader in S2.

Any model-based attempt to remember arbitrary sentences would bypass this
limited evidence policy and requires a new separately approved contract.
