# Model Gateway Routing Architecture

Status: current architecture after the AI Service Agent-platform closeout (2026-08-21)

## One model-execution topology

BodySense has one LLM provider-routing topology:

```text
Go business/deployment authority
        |
        | selects a repository-known immutable AgentConfiguration
        v
Python AI service
  - typed PydanticAI Agents where appropriate
  - LangGraph Consultation runtime
  - AIService provider-neutral request/stream transport for utility/offline roles
        |
        | exact manifest logical_model + generation settings
        v
Standalone LiteLLM gateway
  - physical provider credentials
  - physical provider/model mapping
  - provider retry/fallback
        |
        v
Physical providers / models
```

Application code does not construct physical provider candidates or fallback chains. `AIService`
remains as a stable transport facade for Consultation/Knowledge/Title/Posture; its former provider
routing responsibility is retired.

## Logical groups

The repository-owned logical route registry in `src/ai/gateway.py` provides the allowed BodySense
logical groups:

| Business route | LiteLLM logical group | Typical consumer |
| --- | --- | --- |
| `consultation.reply` | `bodysense-consultation` | Consultation runtime + tools |
| `assessment.generate` | `bodysense-structured` | typed Assessment Agent |
| `treatment.proposal` | `bodysense-structured` | typed Treatment Agent |
| `knowledge.curate` | `bodysense-structured` | Knowledge Curator |
| `knowledge.split` | `bodysense-structured` | Knowledge Splitter |
| `conversation.title` | `bodysense-text` | Title Agent |
| `posture.analyze` | `bodysense-posture` | multimodal Posture Agent |

Diagnosis uses its immutable manifest logical group `bodysense-diagnosis`.

## Manifest pinning is authoritative

The logical route registry defines which logical groups are valid for a business route and supplies a
compatibility default for provider-neutral plumbing. It is **not** a second Agent deployment authority.

When an immutable Agent manifest supplies `logical_model`, that exact logical model is authoritative:

```text
Go selects configuration_id
  -> Python resolves exact manifest
  -> AI request carries manifest.logical_model
  -> AIService must preserve it
  -> LiteLLM receives that logical model
```

`AIService.generate()` and `generate_stream()` therefore use an explicitly pinned request model before
falling back to the route default. Regression tests cover both normal and streaming execution. This
closes the historical bug where an apparently pinned manifest could be silently replaced by the route
registry model.

The role/config registrations in Go also record expected decision-policy revision and logical model.
Responses are fail-closed where Go owns a durable artifact or application boundary.

## Internal HTTP configuration authority

The application-facing Python boundaries for Assessment, Consultation turn start, Posture and Title
require `configuration_id`. Knowledge requires exact Splitter/Curator ids whenever those LLM
capabilities are enabled. Production Go callers supply these values from `AgentDeploymentPolicy`.

Python default-manifest helpers are allowed for unit tests/offline experiments; omission at a Go-owned
production boundary is not allowed to silently select a serving configuration.

## Ownership

### Go application / Agent policy owns

- business role and whether an LLM capability is enabled;
- repository-known serving configuration pointer / rollout selection;
- immutable Agent configuration identity expected from Python;
- deterministic decision/safety authority appropriate to the role;
- durable business provenance and replay/rollout records where the role requires them.

### Immutable Agent manifests own

- role;
- logical model / model-group revision;
- prompt revision;
- output-schema revision;
- tool-policy revision;
- governance/decision-policy revision;
- behavior-significant generation settings.

These fields are fingerprinted into the immutable configuration id. Runtime hostnames, credentials and
physical provider choice are intentionally excluded.

### Python runtime owns

- resolving and validating the exact immutable manifest;
- typed reasoning/runtime implementation;
- construction of prompt/tool/model adapters from that manifest;
- returning actual execution provenance (runtime, logical model, physical provider/model when
  available).

### LiteLLM owns

- physical provider/model mapping;
- physical LLM credentials;
- provider retry/fallback;
- gateway transport availability and provider telemetry.

LiteLLM is not allowed to become BodySense safety, clinical or promotion authority.

## Role-appropriate governance

Not every LLM consumer needs the Diagnosis canary machinery. The governing rule is documented in
[`agent-platform-role-governance.md`](./agent-platform-role-governance.md):

- clinical decision/conversational Agents: qualification + replay + rollout appropriate to the role;
- Posture perception: exact config/provenance + deterministic output governance + downstream clinical
  gates;
- Title utility: exact config/provenance, no clinical rollout requirement;
- Knowledge offline content: exact config/lineage + human publication/review authority;
- OCR/ASR/Embedding/Pose: non-LLM mechanisms, not fake Agents.

## Credential isolation

`ai-service` receives only:

```text
LITELLM_BASE_URL
LITELLM_API_KEY
```

for LLM traffic. `MIMO_API_KEY`, `MIMO_BASE_URL`, and `OPENROUTER_API_KEY` are injected into
`litellm-gateway`, not into `ai-service`.

Embedding and ASR credentials are separate non-LLM subsystems and are not covered by the LLM routing
boundary (`EMBEDDING_*`, `ASR_*`).

## Retired implementation

Diagnosis Phase 10 physically deleted the old application-owned provider stack:

```text
src/ai/config.py
src/ai/router.py
src/ai/providers/
src/ai/pydantic_model.py
src/ai/diagnosis_model_boundary.py
src/config/models.yaml
DIAGNOSIS_MODEL_BACKEND
DIAGNOSIS_AGENT_CONFIGURATION_ID compatibility alias
llm.json / llm.text business routing semantics
```

PydanticAI `FallbackModel` is no longer constructed by BodySense. Provider fallback is exercised
through the real LiteLLM container smoke instead.

Keeping `AIService` as a provider-neutral transport is **not** keeping the old provider router. It may
translate BodySense request/stream types to the internal OpenAI-compatible LiteLLM endpoint, but it may
not choose a physical provider or override an immutable manifest model.

## Immutable older Agent configurations

Older Diagnosis/Treatment/Assessment manifests remain resolvable when they are real Champion,
rollback, qualification or replay artifacts. Keeping an immutable historical Agent configuration is
not keeping an old provider-routing stack: all model execution uses the same LiteLLM boundary.

Historical manifests may be retired from serving only through their role's governance process; replay
can continue to resolve them after serving retirement.

## Verification

Structural/release tests fail if retired router artifacts reappear or if `ai-service` Compose receives
physical LLM provider credentials. Current validation also covers:

- manifest logical-model pinning for normal and streaming AIService calls;
- cross-language repository-known configuration identities;
- Posture Go -> Python config pinning + response identity validation;
- Title Go-owned identity + durable provenance;
- Knowledge Go-selected Splitter/Curator ids + provider/model lineage;
- authenticated LiteLLM access and real gateway routing smoke;
- Diagnosis/Treatment qualification/promotion suites;
- full Go/Python/Web/contracts quality gates and PostgreSQL migration replay.

This document is the single model-routing truth for current code.
