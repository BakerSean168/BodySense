# Agent Platform Role Governance

Status: current architecture after the AI Service Agent-platform closeout (2026-08-21)

## 1. One platform, role-appropriate governance

BodySense uses one execution platform for LLM-backed behavior:

```text
Go domain/application authority
  -> repository-known immutable AgentConfiguration id
  -> Python role runtime / typed Agent
  -> manifest-pinned LiteLLM logical model
  -> standalone LiteLLM gateway
  -> physical provider/model
```

"One platform" does **not** mean every component copies the Diagnosis rollout machinery. Governance
is selected from the role's actual blast radius. What is universal is configuration identity,
authority ownership and execution provenance whenever an LLM participates.

## 2. Universal invariants for every LLM-backed role

Every LLM-backed role must satisfy all of these:

1. **Immutable repository configuration** — behavior-significant prompt/tool/schema/governance/model
   group/generation revisions are fingerprinted into an `AgentConfiguration` id.
2. **Go owns deployment selection** — production callers select a repository-known configuration;
   callers cannot submit arbitrary ids and Python does not silently choose a default at Go-owned HTTP
   boundaries.
3. **Python executes the exact manifest** — the manifest's `logical_model` and generation settings
   are passed through the provider-neutral runtime.
4. **LiteLLM alone owns physical routing** — application code never owns provider credentials,
   physical model fallbacks or provider retry chains.
5. **Durable provenance where an artifact is durable** — the durable artifact records enough
   configuration/execution lineage to explain which behavior produced it.
6. **Fail closed on identity mismatch** — Go rejects a response whose returned configuration does not
   match the selected repository-known identity.

## 3. Role classes

### A. Clinical decision / longitudinal Agents

Roles: **Diagnosis, Treatment, Assessment, Consultation**.

These can influence longitudinal health interpretation, automation eligibility or user-facing clinical
reasoning. They receive the strongest governance envelope:

- immutable configuration;
- Go decision authority and durable provenance;
- qualification / behavioral evals;
- frozen-input historical or counterfactual replay where meaningful;
- explicit shadow/canary/promotion/rollback governance for challengers;
- deterministic safety/policy gates outside the model.

Diagnosis and Treatment remain the reference implementations. Assessment adapts replay to derived
reports. Consultation adapts replay/rollout to a multi-turn checkpointed runtime.

#### Consultation execution-identity handshake

Consultation is multi-turn and streaming, so response-end provenance is too late to be the trust boundary. For every start or resume:

- Go selects/pins the expected repository-known Consultation configuration before calling Python.
- Python emits `runtime.agent_configuration` as the first internal event after resolving the manifest and before any message/tool/state output.
- Go validates configuration ID, `role=consultation`, decision-policy revision and logical model against repository registration, then persists the exact identity on the active Run immediately.
- Missing, malformed, duplicate or mismatched handshakes fail closed before ordinary semantic output is accepted.
- The identity is run-local; the shared `consultation.Runtime` never stores request-scoped pending configuration/provenance.
- HITL resume reloads the interrupted source Run configuration and Python verifies it equals the checkpointed manifest before resuming the graph. Current Champion selection cannot replace an already-pinned waiting thread.

Observed completion usage/provider details may enrich provenance but cannot rewrite the immutable configuration identity established by the handshake.

### B. Perception Agent

Role: **Posture**.

Posture converts a user-owned image into a derived observation; it does not become health truth by
itself. Required controls are:

- immutable current `posture-v2` configuration; historical v1 remains known but non-serving;
- required `posture-geometry-v1` subordinate mechanism pinned by MediaPipe version, versioned model URI + SHA256 and canonical threshold SHA256;
- Go-owned Champion pointer;
- exact config pinning on the multipart Go -> Python call;
- build/startup provisioning of the pinned pose model; the request path never downloads a mutable model;
- Python-returned Agent execution + exact geometric mechanism provenance;
- deterministic posture output governance;
- Go validates both Agent identity and mechanism identity and writes a deterministic generation decision trace before persistence;
- direct analysis payload persisted to `user_uploads.analysis_result` (not the HTTP envelope).

A future posture challenger must be added as another immutable manifest and qualified before changing
the Go pointer. A live Diagnosis-style canary is not mandatory merely to generate posture observations;
clinical consumers still apply their own authority/safety rules.

### C. Utility Agent

Role: **Title**.

Conversation title generation is cosmetic and cannot mutate health truth. It therefore requires
identity and auditability, but not clinical replay/rollout machinery:

- immutable `title-v1` configuration;
- Go-owned configuration pointer;
- Python exact-manifest execution through `bodysense-text`;
- Go fail-closed identity verification;
- durable configuration/execution provenance + Go decision trace on the conversation;
- manual rename clears stale AI provenance.

If title generation fails, it may degrade to the normal application fallback rather than escalating a
health workflow.

### D. Offline content-pipeline Agents

Roles: **Knowledge Splitter, Knowledge Curator**.

These Agents transform source material into candidate knowledge artifacts. Their final authority is the
**knowledge publication / human review gate**, not an online production canary. Required controls are:

- immutable Splitter/Curator manifests;
- Go-selected ids whenever the caller enables LLM splitting/refinement;
- callers may choose the capability (`heuristic` vs `llm`, refinement on/off) but cannot choose an
  arbitrary immutable config id;
- exact manifest pinning for every LLM call;
- provider/model/call/fallback lineage recorded in `knowledge_sources.metadata.agent_execution`;
- Go validates the returned Agent identities before reporting ingestion success;
- heuristic splitting remains explicitly non-LLM and is recorded as such by absence of LLM execution
  lineage;
- publication/review remains the gate that determines whether generated content can become trusted
  retrieval material.

This is stronger and more domain-correct than pretending an offline source-ingestion batch is a live
user canary.

### E. Non-LLM mechanisms

Components: **OCR, ASR, embedding generation, geometric pose estimation**.

These are tools/mechanisms, not reasoning Agents. They do not receive fake `AgentConfiguration`
identities merely for platform uniformity. They still need their own version/provider provenance where
that matters, input validation, credential isolation and deterministic error handling.

For OCR, extraction completion and downstream evidence authority are explicitly separate. `ocr-indicator-admissibility-v1` may mark a parsed report value `admissible`, `needs_review` or `rejected`; current Assessment v4 only catalogs the first class and Go revalidates the exact policy revision before durable projection. This admission policy does not remove the separate requirement to version the OCR mechanism implementation itself.

If one of these mechanisms later gains LLM reasoning behavior, the LLM portion becomes an Agent role
and must enter the universal invariants above.

## 4. Current role matrix

| Role / mechanism   | Class                     | Immutable config |    Go selection     | Exact manifest execution |    Durable lineage     | Replay / rollout authority                              |
| ------------------ | ------------------------- | :--------------: | :-----------------: | :----------------------: | :--------------------: | ------------------------------------------------------- |
| Diagnosis          | clinical decision         |        ✅        |         ✅          |            ✅            |           ✅           | full qualification + replay + rollout                   |
| Treatment          | clinical decision         |        ✅        |         ✅          |            ✅            |           ✅           | full qualification + replay + rollout                   |
| Assessment         | clinical derived decision |        ✅        |         ✅          |            ✅            |           ✅           | qualification + replay + rollout                        |
| Consultation       | clinical conversational   |        ✅        |         ✅          |            ✅            |           ✅           | multi-turn replay + rollout                             |
| Posture            | perception                |        ✅        |         ✅          |            ✅            |           ✅           | config qualification/pointer; downstream clinical gates |
| Title              | utility                   |        ✅        |         ✅          |            ✅            |           ✅           | no clinical rollout required                            |
| Knowledge Splitter | offline content           |        ✅        | ✅ when LLM enabled |            ✅            |   ✅ source metadata   | publication/review gate                                 |
| Knowledge Curator  | offline content           |        ✅        |   ✅ when enabled   |            ✅            |   ✅ source metadata   | publication/review gate                                 |
| OCR                | mechanism                 |       n/a        |         n/a         |           n/a            |  mechanism provenance  | n/a                                                     |
| ASR                | mechanism                 |       n/a        |         n/a         |           n/a            |    source metadata     | n/a                                                     |
| Embedding          | mechanism                 |       n/a        |         n/a         |           n/a            | index/model provenance | n/a                                                     |

## 5. Internal HTTP authority rule

For Go-owned runtime boundaries, `configuration_id` is required. The following must never silently
fall back to a Python default when called by the application:

- Assessment generation;
- Consultation turn start;
- Posture analysis;
- Title generation;
- LLM Knowledge splitting/refinement.

Python-side default manifest helpers may remain useful to unit tests, offline tooling or explicit local
experiments, but they are not a production deployment authority.

## 6. Provider-neutral `AIService` is not legacy routing

`AIService` remains for the roles that need a small request/stream transport abstraction. It is valid
only because it now obeys an explicit manifest `logical_model` when one is present and sends traffic to
LiteLLM. It must not choose physical providers, construct provider fallbacks, or replace Go deployment
selection.

This distinction closes the historical ambiguity between "remove the old router" and "remove every
shared transport facade". The former is required; the latter is neither required nor desirable.

## 7. Definition of done for a new LLM role

A new role is not production-ready until:

1. its class above is declared;
2. a repository-versioned immutable manifest exists;
3. Go has a repository-known selection/policy boundary when the role is application-invoked;
4. Python executes the exact manifest through LiteLLM;
5. identity mismatch is rejected;
6. durable output includes role-appropriate lineage;
7. class-appropriate eval/safety/review gates exist;
8. release validation covers the new boundary.
