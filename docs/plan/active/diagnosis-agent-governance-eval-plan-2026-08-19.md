# Diagnosis AI Platform North-Star Refactor & Governance Plan

> Status: Active
> Reframed: 2026-08-19
> Design stance: north-star first, migration second; existing internal AI routing is migration input, not an architectural constraint
> Scope: Model Gateway, Agent configuration, Diagnosis eval/qualification, evidence control, safety authority, provenance/replay, promotion
> Product boundary: complete this Diagnosis platform refactor before new Treatment-domain expansion

## 1. Executive decision

This program is **not** an incremental cleanup of the current `llm.json -> models.yaml -> PydanticAI FallbackModel` path.

The target is a clean layered AI platform:

```text
Web
  |
  v
Go Domain/Application
  |  durable truth / authority / policy / persistence
  v
Python Agent Runtime
  |  PydanticAI / LangGraph / tools / typed reasoning
  v
LiteLLM Gateway Service
  |  provider normalization / routing / retry / fallback / rate limit / spend / model telemetry
  v
Physical Providers / Models
```

The current implementation is used only to:

1. preserve proven business semantics;
2. capture a behavioral baseline before cutover;
3. identify data that must migrate;
4. delete or replace obsolete internal boundaries after parity.

### North-star rule

> Existing internal AI implementation is not a protected contract.

Protected are the business invariants:

- BodyState is Go-owned durable health truth;
- Diagnosis consumes an exact BodyState revision;
- active durable safety state can block ordinary Diagnosis;
- Go owns durable analysis/candidate identities and historical immutability;
- Python owns Agent reasoning/runtime, not durable business authority;
- Web remains a projection consumer;
- Diagnosis does not silently create Treatment.

The following are explicitly **retirable**:

```text
use_case="llm.json"
src/config/models.yaml as application routing truth
src/ai/pydantic_model.py provider construction
PydanticAI FallbackModel as BodySense routing layer
legacy AIService / ModelRouter provider policy where superseded
provider credentials inside ai-service
parallel provider-specific model creation paths
```

## 2. North-star ownership model

### 2.1 Go — Domain and Decision Control Plane

Go owns:

```text
BodyState / BodyStateRevision
DiagnosisAnalysis / Candidate identity
SafetyState
AgentDeploymentPolicy / active configuration pointer
DiagnosisDecisionPolicy / SafetyEnvelope
DecisionTrace persistence
configuration/execution provenance persistence
candidate assessment
Treatment eligibility downstream
```

Go does **not** own provider APIs, retry/fallback between LLM vendors, Prompt execution, or Agent tool loops.

### 2.2 Python AI Service — Agent Runtime Plane

Python owns:

```text
PydanticAI Agent definitions
LangGraph consultation runtime
Prompt assembly
structured output schemas
typed run dependencies
EvidenceGap reasoning
Agent tool execution
Evidence acquisition budget enforcement
semantic/output governance
Agent execution metadata collection
```

Python does not own durable health truth or final business authorization.

### 2.3 LiteLLM — Model Gateway Plane

Introduce a dedicated `litellm-gateway` service.

It owns:

```text
provider credentials
provider/model adapters
OpenAI-compatible model API
logical model groups
provider retry / fallback
load balancing / rate limiting
provider health
usage / token / latency / spend telemetry
```

It does **not** own BodySense SafetyEnvelope, Diagnosis eligibility, EvidenceGap meaning, Prompt versions, or final AUTO/ABSTAIN/ESCALATE authority.

### 2.4 Eval Plane

Use Pydantic Evals as the generic dataset/evaluator/report substrate and build only BodySense-specific evaluators and qualification logic on top.

```text
Pydantic Evals
  + BodySense EvalCase metadata
  + deterministic domain evaluators
  + trace/tool evaluators
  + optional calibrated LLM Judge
  + slice / non-inferiority / qualification report
```

Eval must execute the **same Agent configuration path** used by production, with deterministic/test mode and live-provider mode clearly separated.

## 3. Target service topology

```text
                      +-----------------------+
                      |       React Web       |
                      +-----------+-----------+
                                  |
                                  v
                      +-----------------------+
                      |        Go API         |
                      | Domain + Authority    |
                      +-----------+-----------+
                                  |
                     typed internal AI contract
                                  |
                                  v
                      +-----------------------+
                      |    Python AI Service  |
                      | PydanticAI / LangGraph|
                      | tools / RAG / eval    |
                      +-----------+-----------+
                                  |
                       OpenAI-compatible API
                                  |
                                  v
                      +-----------------------+
                      |  LiteLLM Gateway      |
                      | route/retry/fallback  |
                      | provider telemetry    |
                      +-----------+-----------+
                                  |
                +-----------------+-----------------+
                |                 |                 |
                v                 v                 v
              Mimo            OpenRouter        future provider
```

### Docker target

Add:

```text
litellm-gateway
  image: LiteLLM proxy image
  config: docker/litellm/config.yaml
  internal port: 4000
  provider secrets: only here
```

Then:

```text
ai-service
  LITELLM_BASE_URL=http://litellm-gateway:4000
  LITELLM_API_KEY=<internal service key>
```

The AI service should no longer receive provider API keys after migration.

## 4. Model identity hierarchy

Do not collapse business identity, logical model identity and physical deployment.

```text
Business Agent Role
  diagnosis
       |
       v
Agent Configuration
  diag-config-<fingerprint>
       |
       +-- Prompt / Schema / Tool / Evidence / Governance / Decision policy revisions
       |
       v
Logical Model Group
  bodysense-diagnosis
       |
       v
LiteLLM Routing Policy
       |
       +--> mimo/model-x
       +--> openrouter/model-y
       +--> future provider/model-z
```

### Important distinction

- **Agent Configuration** is the qualification/promotion unit.
- **LiteLLM model group** is a model execution/routing unit.
- **Physical provider/model** is execution provenance.
- **Deployment pointer** decides which immutable Agent Configuration receives traffic.

LiteLLM must never become the registry for Prompt/Schema/Safety policy.

## 5. Agent Configuration artifact

Define an immutable repository-versioned manifest, initially file-backed rather than a database product.

Example conceptual shape:

```yaml
id: diag-config-<fingerprint>
role: diagnosis
logical_model: bodysense-diagnosis
model_group_revision: diagnosis-model-group-v1
prompt_revision: diagnosis-prompt-v3
output_schema_revision: diagnosis-output-v2
tool_policy_revision: diagnosis-tools-v2
evidence_policy_revision: diagnosis-evidence-v2
governance_policy_revision: diagnosis-governance-v3
decision_policy_revision: diagnosis-authority-v1
generation:
  temperature: 0.3
  max_tokens: 2048
```

Fingerprint all behavior-significant fields using canonical serialization.

Do not include secrets, hostnames that do not affect behavior, or ephemeral deployment metadata.

### Configuration lifecycle

```text
DRAFT
  -> QUALIFIED
  -> CANDIDATE
  -> PROMOTED
  -> DEPRECATED
  -> RETIRED
```

The first implementation may express lifecycle in versioned repository artifacts + qualification reports rather than a database registry.

## 6. Deployment policy

Separate immutable configuration from mutable routing:

```text
production -> diag-config-007
canary     -> diag-config-008
```

Later rollout can support:

```text
95% -> config-007
 5% -> config-008
```

Assignment must use a stable causal/business key when canary experimentation is enabled, not randomize every HTTP call.

The deployment/control-plane policy belongs with BodySense application governance, not inside LiteLLM, because LiteLLM only sees the model layer and cannot represent full Agent configuration identity.

## 7. Diagnosis runtime north star

```text
Go loads exact BodyStateRevision
        |
        +--> durable SafetyState hard gate
        |
        v
resolve AgentDeploymentPolicy
        |
        v
configuration_id
        |
        v
Python Diagnosis Agent
        |
        +--> Prompt/Schema from immutable config
        +--> PydanticAI typed execution
        +--> EvidenceGap
        |      |
        |      +--> targeted Evidence Acquisition
        |      +--> EvidenceBudget / stop reason
        |
        +--> calls logical model via LiteLLM
        |
        v
DiagnosisExecutionResult
        |
        +--> candidates / hypotheses
        +--> EvidenceGap states
        +--> evidence refs
        +--> governance result
        +--> configuration identity
        +--> execution provenance
        |
        v
Go DiagnosisDecisionPolicy
        |
        +--> SafetyEnvelope
        +--> deny-overrides hard blockers
        +--> final authority decision
        |
        v
ALLOW_NORMAL / ALLOW_DEGRADED / ABSTAIN / ESCALATE / BLOCK
        |
        v
immutable DiagnosisAnalysis + DecisionTrace
```

## 8. Safety architecture

### 8.1 Model output is evidence, not authority

The model may output:

```text
candidate hypotheses
confidence
evidence strength
information/evidence gaps
reasoning summary
```

It may not grant itself normal-delivery authority.

### 8.2 SafetyEnvelope

SafetyEnvelope is a versioned deterministic policy describing the eval-proven region in which a configuration may follow the normal automated path.

Initial envelope should be conservative and based on actually available facts:

```text
configuration approved
AND durable safety state allows ordinary analysis
AND structured contract valid
AND governance != rejected
AND no unresolved critical EvidenceGap
AND analysis status compatible with normal delivery
```

Do not invent a LOW/MEDIUM/HIGH classifier merely to make the architecture look complete. Introduce one only after its own dataset, confusion-matrix requirements and threshold policy exist.

### 8.3 Deny-overrides

Hard blockers are constraints, not votes.

```text
critical_gap = OPEN
confidence = HIGH
judge = EXCELLENT

=> normal delivery remains forbidden
```

## 9. Evidence architecture

Replace free-form `information_gaps: list[str]` as the control mechanism with typed run state.

```text
EvidenceGap
├─ gap_id                  # run-local identity
├─ kind                    # user_information | external_knowledge | conflict | safety
├─ missing_fact
├─ why_needed
├─ criticality             # normal | critical
├─ related_concern_key
├─ acquisition_strategy    # search_knowledge | request_clarification | none
└─ status                  # open | searching | resolved | unresolvable
```

### Evidence acquisition

```text
Gap
  -> acquisition policy
  -> tool action
  -> EvidenceAttempt
  -> update gap
  -> re-evaluate
```

A search call must be attributable to a concrete gap.

A user-information gap cannot be resolved by ordinary knowledge retrieval.

### EvidenceBudget

Budget is explicit run state:

```text
max search calls
max top_k
latency/cost budget where measurable
stop reason
```

Required invariant:

> budget exhausted means stop acquiring evidence; it does not mean the gap is resolved or risk is low.

## 10. Eval and qualification architecture

### 10.1 Use Pydantic Evals

Adopt Pydantic Evals for:

```text
typed Dataset / Case
YAML dataset serialization
custom evaluators
report generation
optional LLMJudge
```

BodySense adds domain-specific metadata and qualification logic.

### 10.2 Dataset taxonomy

```text
Development
Calibration
Holdout
Regression
Challenge
```

Each case carries:

```text
scenario_family_id
case_category
risk_slice
body_state
relevant_history
expected behavioral invariants
forbidden behaviors
expected/forbidden tool behavior
```

Split rules:

```text
stratified coverage
+
grouped by user / episode / scenario_family
```

### 10.3 Evaluator hierarchy

```text
Deterministic first
  schema/status/reference validity/tool trace/governance/decision

Structured semantic second
  hypothesis category/evidence relationship/gap category

LLM Judge last
  nuanced semantic equivalence/overconfidence/reasoning quality
```

Judge cannot override deterministic safety contracts.

### 10.4 Critical slices

Promotion requires critical slices to pass independently of overall score.

### 10.5 Non-inferiority

A challenger need not exceed Champion quality if it is demonstrably non-inferior on protected quality/safety dimensions and materially improves an approved objective such as cost or latency.

The margin and decision rule must be declared before holdout results are inspected.

## 11. Interaction and configuration experiments

Configuration changes are not assumed additive.

When attribution matters, support controlled factorial comparison, e.g.:

```text
             Prompt v7   Prompt v8
Model A         A7          A8
Model B         B7          B8
```

Use offline/shadow paired cases to separate:

```text
model main effect
prompt main effect
model x prompt interaction
```

Do not expose a complex factorial debugging experiment directly as a user-facing production canary.

## 12. Audit architecture

### DecisionTrace

Persist structured audit facts, not hidden chain-of-thought.

```text
DecisionTrace
├─ body_state_revision
├─ configuration_id
├─ policy_bundle_revision
├─ durable safety facts
├─ gap results
├─ acquisition attempts / stopping reason
├─ governance verdict
├─ hard blockers
└─ final decision
```

### Configuration provenance

Answers:

> What approved behavior bundle was selected?

### Execution provenance

Answers:

> What actually executed this run?

Capture where reliably available:

```text
logical model group
physical provider/model
fallback path
tool attempts
retrieved evidence IDs/source versions
usage/tokens/latency/cost
```

Unknown fields are recorded as unknown, not invented.

## 13. Replay architecture

### Historical replay

Use preserved historical input + configuration/policy identities + recorded decision inputs to explain/reproduce historical behavioral invariants.

### Counterfactual replay

Run the same frozen case against a new/current configuration to answer:

> What would happen under this configuration today?

### Equality model

```text
Hard invariants      -> deterministic match required
Semantic invariants  -> structured/semantic equivalence
Presentation text    -> may differ
```

Do not require token-for-token LLM reproduction.

## 14. Promotion architecture

```text
DRAFT config
  -> Development eval
  -> Holdout qualification
  -> critical-slice gates
  -> non-inferiority / statistical evidence
  -> CANDIDATE
  -> Shadow
  -> Canary with predeclared stop rules
  -> Progressive promotion
  -> PROMOTED
```

Rollback changes the deployment pointer to the previous immutable configuration; it does not mutate or delete the failed candidate.

## 15. Migration philosophy

This is a strangler only at the **service cutover level**, not a compromise in the target design.

We allow temporary coexistence only to prove parity and rollback safely.

Every compatibility adapter must have a deletion ticket.

### Preserve

```text
Go durable domain model
public product semantics
BodyState revisions
Diagnosis immutable history
candidate assessment semantics
current Web behavior unless intentionally redesigned
```

### Replace aggressively

```text
application-level provider routing
llm.json route semantics
provider credentials in ai-service
local FallbackModel routing
models.yaml as provider truth
parallel provider adapters
ad-hoc eval runner if Pydantic Evals replaces it cleanly
free-form information gap control
implicit safety authorization
```

## 16. Implementation phases

### Phase 0 — Architecture freeze + behavioral baseline

Goal: freeze only business contracts and build eval characterization before structural cutover.

Deliverables:

- ADR: adopt standalone LiteLLM Model Gateway;
- this north-star architecture becomes authoritative for Diagnosis AI platform;
- current Diagnosis behavioral baseline captured with initial Pydantic Evals dataset/evaluators;
- migration/deletion ledger for legacy AI routing files.

No production behavior change yet.

### Phase 1 — Standalone LiteLLM Gateway

Goal: establish the final model/provider boundary first.

Deliverables:

- `litellm-gateway` Docker service;
- `docker/litellm/config.yaml`;
- logical model group `bodysense-diagnosis` preserving the current candidate order initially;
- provider credentials moved from `ai-service` to gateway;
- internal gateway authentication;
- healthcheck and local deploy validation;
- usage/provider telemetry visible for test calls.

Acceptance:

- a direct internal OpenAI-compatible call through the gateway reaches the configured model group;
- provider fallback can be characterized without BodySense business logic knowing provider identities.

Implementation checkpoint (2026-08-19): NS-200/NS-210 establishes the pinned v1.97.0 gateway, production provider graph, internal authentication, health checks, and a deterministic container smoke that proves authenticated OpenAI compatibility, usage metadata, and primary-to-fallback behavior. Provider secrets intentionally remain available to the legacy `ai-service` path until Phase 2 switches Diagnosis to the gateway; removing them earlier would break the protected current path. That temporary duplication is a migration bridge, not accepted target state. Live-provider validation requires deployment credentials and is therefore separate from the credential-free deterministic CI smoke.

### Phase 2 — Rebuild Python model boundary around LiteLLM

Goal: PydanticAI talks only to the logical model gateway.

Deliverables:

- one PydanticAI model adapter/client pointing to LiteLLM;
- Diagnosis Agent requests `bodysense-diagnosis` logical model;
- remove PydanticAI `FallbackModel` provider construction;
- remove provider credentials from Python runtime configuration;
- begin deletion of `pydantic_model.py`/`models.yaml` routing responsibilities.

Acceptance:

- Diagnosis runs through PydanticAI -> LiteLLM -> provider;
- no Diagnosis code constructs physical provider clients.

Implementation checkpoint (2026-08-19): new runtime definitions resolve Diagnosis through the `bodysense-diagnosis` logical model on an authenticated OpenAI-compatible LiteLLM endpoint. `DiagnosisRequest.use_case` / `llm.json` was removed from both Go and Python HTTP/application contracts, and production Compose activates the gateway as a health dependency. The container smoke also executes a real PydanticAI request through the gateway and verifies fallback. A deliberately isolated `diagnosis_model_boundary` keeps one migration-only legacy branch when `DIAGNOSIS_MODEL_BACKEND` is absent, protecting older Watchtower-managed servers whose AI image can update before their Compose/runtime files. New Compose files always set `DIAGNOSIS_MODEL_BACKEND=litellm`; the bridge has a Phase 10 deletion condition after runtime synchronization is proven. Legacy physical routing otherwise remains only for non-Diagnosis AI consumers and is tracked for final retirement.

### Phase 3 — Agent Configuration platform

Goal: make full Agent configuration the qualification unit.

Deliverables:

- versioned Diagnosis Agent manifest;
- canonical fingerprint/configuration ID;
- deployment pointer abstraction;
- prompt/schema/tool/evidence/governance/decision revisions explicitly represented;
- config identity added to eval/runtime metadata.

Acceptance:

- changing any behavior-significant revision changes configuration ID;
- provider secrets/runtime host changes do not;
- production can identify exactly which immutable Agent config served a run.

Implementation checkpoint (2026-08-20): NS-400/NS-410 introduces an immutable repository-versioned Diagnosis Agent manifest with canonical SHA-256 **behavior** fingerprinting (`diag-config-f492eb1c0c6676ae`). File-format metadata (`manifest_revision`) and runtime host/provider credentials are excluded from identity, while prompt/schema/tool/evidence/governance/decision/model-group revisions plus generation settings are included. The current evidence/tool and decision revisions are deliberately labeled legacy/pre-envelope so later Phase 5/6 behavior changes necessarily produce a new configuration identity. The mutable deployment pointer is Go-owned via `AgentDeploymentPolicy`; Go sends the selected `configuration_id`, Python resolves that exact manifest, validates that its runtime supports the selected prompt/schema/tool/evidence/governance/model-group revisions, and returns execution provenance on every governance path. Go rejects a response whose configuration identity or role differs from the selected deployment pointer. Eval summaries and the Diagnosis read model expose the same identity; dedicated durable provenance columns remain Phase 7 work.

### Phase 4 — Pydantic Evals Diagnosis qualification system

Goal: create a first-class eval platform before further behavioral changes.

Deliverables:

- typed Pydantic Evals dataset;
- YAML + schema;
- deterministic BodySense evaluators;
- tool/trace evaluators;
- split/slice metadata;
- development/holdout/regression/challenge suites;
- current Champion baseline report;
- optional calibrated semantic Judge only where deterministic grading cannot work.

Acceptance:

- one command evaluates an immutable configuration and outputs slice-aware qualification evidence.

Implementation checkpoint (2026-08-20): NS-500 upgrades the three-case characterization smoke into `diagnosis_qualification_v1`: seven schema-validated Pydantic Evals cases spanning development/holdout/regression/challenge, with multi-slice taxonomy and four hard `critical-safety` cases. Deterministic evaluators cover response contract, expected status, candidate policy, no-Treatment side effects, exact Agent configuration provenance, and Agent/tool trace behavior. Safety cases prove the model is bypassed entirely; normal cases prove the tool surface declared by the evaluated immutable configuration without introducing artificial tool calls. Qualification emits dataset/configuration fingerprints, split/slice/evaluator evidence and hard-gate reasons. A paired non-inferiority comparator uses the same dataset fingerprint, blocks any critical regression, enforces a predeclared 0.02 margin, and distinguishes non-inferiority from promotion eligibility. The current Champion (`diag-config-f492eb1c0c6676ae`) is committed as machine-readable evidence and passes 7/7 with all required splits and critical slices green. Semantic LLM judging remains intentionally absent until a criterion exists that deterministic grading cannot express.

### Phase 5 — EvidenceGap runtime redesign

Goal: replace model-driven bare search with a controlled evidence acquisition loop.

Deliverables:

- typed EvidenceGap;
- structured EvidenceAttempt;
- acquisition policy;
- EvidenceBudget;
- stopping reasons;
- retrieval only through a gap-aware tool contract;
- regression cases for no-search/user-info/critical-gap/budget-exhausted behavior.

Acceptance:

- every search explains why it exists;
- user facts are never fabricated from external RAG;
- critical unresolved gaps survive budget exhaustion.

Implementation checkpoint (2026-08-20): NS-600 introduces a cumulative immutable Challenger, `diag-config-20fbfc23ca09cbab`, while leaving the v1 Champion manifest and Go serving pointer unchanged. The Challenger binds `diagnosis-prompt-v4-evidence-gap`, `diagnosis-evidence-acquisition-tools-v2`, and `diagnosis-evidence-gap-v2`. The only v2 retrieval tool is `acquire_evidence(EvidenceGap)`: each gap carries identity, source kind, description, rationale, criticality and (only for external knowledge) a targeted query. `user_fact` structurally forbids a query. `DiagnosisEvidenceAcquirer` owns a finite two-search/five-results-per-search `EvidenceBudget`, emits typed `EvidenceAttempt` records and explicit stopping reasons, and prevents searches after exhaustion. Critical unresolved gaps are merged back into final `information_gaps` by runtime code even if the model omits them. Legacy Diagnosis `rag_context` / `rag_results` HTTP inputs were deleted and the internal request now forbids extra fields, so v2 cannot bypass the acquisition policy through preloaded RAG. `DiagnosisService` also no longer accepts a preconstructed Agent: only the model is injectable and the Agent is always built from the exact manifest, preserving configuration/execution identity. The dedicated EvidenceGap Pydantic Evals policy suite passes 5/5 (no-gap, user-fact, targeted external search, zero-budget critical gap, and budget-exhausted second critical gap). On the same general Diagnosis dataset fingerprint, v1 and v2 both pass 7/7; paired comparison reports `+0.000` pass-rate delta, zero critical regressions, non-inferior=YES and promotion-eligible=YES. Actual traffic promotion is intentionally deferred to Phase 9 so this refactor does not bypass the Shadow/Canary/Promotion governance it is building.

### Phase 6 — Deterministic DecisionAuthority / SafetyEnvelope

Goal: move final normal-delivery authority into Go.

Deliverables:

- pure versioned `DiagnosisDecisionPolicy`;
- minimal eval-backed SafetyEnvelope;
- deny-overrides composition;
- internal outcomes: allow-normal / allow-degraded / abstain / escalate / block;
- malformed/unknown policy facts fail closed.

Acceptance:

- high confidence/Judge score cannot override a hard blocker;
- Python cannot self-authorize outside the envelope.

Implementation checkpoint (2026-08-20): NS-700 adds cumulative Challenger `diag-config-5a4a13627e14b4cf`, identical to the Phase-5 EvidenceGap Agent except that its decision revision is `diagnosis-decision-policy-v1`. The Go control plane now accepts only repository-known immutable Diagnosis configuration IDs and binds each to an expected decision revision; unknown IDs fail startup instead of becoming unqualified traffic. `DiagnosisDecisionPolicy` is a pure deterministic deny-overrides function with outcomes `allow-normal`, `allow-degraded`, `abstain`, `escalate`, and `block`. Its minimal SafetyEnvelope uses durable BodyState safety state plus structured runtime facts (status, Python governance as evidence, new red flags, candidate cardinality, unresolved critical EvidenceGaps). Candidate confidence/Judge score is not an authority input. Unsupported policy revisions, unknown enum values, contradictory safety state and malformed required facts fail closed. For v3, Python rejection no longer self-terminates the HTTP path: Go evaluates the policy, suppresses ordinary candidate delivery for block/escalate/abstain, persists the authorized result, and exposes `decision_authority` on the read model. Pre-envelope v1/v2 behavior remains reachable only for current serving/replay compatibility until Phase 9/10. The versioned Go policy fixture covers nine safety/quality states, including high-confidence hard-block cases. v3 also remains 7/7 on the unchanged Diagnosis Pydantic Evals dataset and is paired non-inferior to v2 with `+0.000` pass-rate delta and zero critical regressions. Production serving remains v1 pending Phase 9 promotion.

### Phase 7 — DecisionTrace + provenance persistence

Goal: make every new Diagnosis auditable.

Deliverables:

- additive persistence schema;
- DecisionTrace;
- configuration provenance;
- execution provenance;
- provider/model/fallback metadata from LiteLLM where available;
- evidence/tool attempt identities;
- reload/detail tests.

Implementation checkpoint (2026-08-20): NS-710/NS-800 adds additive migration `000035_add_diagnosis_decision_trace` and promotes Diagnosis audit metadata from temporary `RawOutput` parsing into first-class immutable columns: indexed `agent_configuration_id`, full `agent_configuration`, `decision_trace`, `execution_provenance`, and `evidence_acquisition_trace`. `diagnosis-decision-trace-v1` ties the exact BodyState revision to configuration, runtime execution/bypass, typed EvidenceAttempts/evidence identities, Python governance evidence, and the Go authority decision. PydanticAI run metadata now records only actually observed fields (`gateway_reported_model`, provider adapter, run/conversation IDs and usage); absent physical-provider/fallback metadata is not invented. Both Python and Go pre-agent safety gates emit explicit bypass provenance. Historical rows are backfilled only from provenance already present in immutable raw output, while new writes build the durable trace at persistence time. The public/detail read model now reads provenance from dedicated columns rather than reparsing raw output.

### Phase 8 — Replay / configuration comparison

Goal: make failures reusable as regression evidence.

Deliverables:

- historical replay mode;
- counterfactual replay against selected config;
- hard/semantic/presentation comparison layers;
- export historical failure to Regression Dataset.

### Phase 9 — Promotion / Shadow / Canary

Goal: prove the complete release governance loop with one real Challenger.

Deliverables:

- predeclared qualification gates;
- non-inferiority margin;
- paired configuration comparison;
- interaction experiment when attribution requires it;
- shadow path;
- stable canary assignment;
- predeclared stop/rollback rules;
- progressive promotion record.

### Phase 10 — Legacy AI routing retirement

Goal: delete superseded architecture instead of carrying two truths.

Delete/retire when no callers remain:

```text
Diagnosis `llm.json` semantics
Python physical provider construction
PydanticAI FallbackModel route layer
provider secrets in ai-service
obsolete models.yaml routing
legacy ModelRouter/AIService responsibilities superseded by LiteLLM + typed Agents
obsolete custom eval framework surfaces superseded by Pydantic Evals
```

Run repository-wide call-site search before deletion.

## 17. First execution ticket sequence

```text
NS-000  Write ADR + legacy deletion ledger + baseline behavior map
NS-100  Introduce Pydantic Evals Diagnosis baseline
NS-200  Add standalone LiteLLM gateway service
NS-210  Prove gateway provider/model group routing + telemetry
NS-300  Rebuild PydanticAI model adapter against LiteLLM
NS-310  Cut Diagnosis off physical provider construction
NS-400  Introduce immutable AgentConfiguration manifest/fingerprint
NS-410  Add deployment pointer + runtime configuration identity
NS-500  Expand eval to qualification/slices/non-inferiority
NS-600  Redesign EvidenceGap/acquisition/budget
NS-700  Add Go SafetyEnvelope/DecisionAuthority
NS-710  Persist DecisionTrace
NS-800  Capture execution provenance via runtime/gateway metadata
NS-810  Historical/counterfactual replay
NS-900  Shadow/canary/promotion exercise
NS-990  Delete legacy routing and archive plan
```

Each ticket receives exact file paths and tests after the immediately preceding architectural seam exists. No ticket may preserve a legacy internal API merely because it is already present; compatibility must have a stated consumer and deletion condition.

## 18. Verification strategy

Every structural batch validates from narrow to broad:

```text
focused unit/contract tests
-> Python/Go package tests
-> Pydantic Eval deterministic suite
-> affected integration tests
-> Docker prod-like composition including LiteLLM
-> repository lint/typecheck/test/build
-> pnpm validate:local-deploy
```

Gateway-specific verification must include:

```text
health
logical model resolution
provider credential isolation
retry/fallback characterization
usage/latency metadata
AI service cannot reach provider directly after cutover
```

## 19. Architecture quality gates

The refactor is not complete if any of these remain true:

- Diagnosis still sends `llm.json` as business intent;
- Python Agent code knows physical provider fallback chains;
- provider credentials still live in ai-service after gateway cutover;
- Agent configuration identity excludes Prompt/Schema/Policy versions;
- LiteLLM is incorrectly used as BodySense safety authority;
- eval only reports overall averages;
- free-form model confidence can grant normal-delivery authority;
- EvidenceGap search can loop without a bounded budget;
- historical analyses cannot explain configuration/policy/provenance;
- old and new model routing stacks remain indefinitely in parallel.

## 20. Definition of done

Diagnosis AI platform refactor is complete when:

```text
Business Role
  -> Immutable Agent Configuration
  -> PydanticAI Agent Runtime
  -> Standalone LiteLLM Gateway
  -> Physical Provider/Model
```

is the sole model execution architecture, and:

- configuration qualification is backed by Pydantic Evals;
- critical slice gates and non-inferiority are supported;
- EvidenceGap acquisition is typed and bounded;
- SafetyEnvelope is deterministic and Go-owned;
- DecisionTrace and configuration/execution provenance are durable;
- historical and counterfactual replay are distinct and usable;
- Champion/Challenger promotion can be demonstrated end to end;
- legacy provider/router paths are deleted, not merely deprecated;
- full local deployment validation is green;
- architecture docs and ADRs match the resulting implementation;
- only then may Treatment build on this platform as a new Agent role/configuration family.
