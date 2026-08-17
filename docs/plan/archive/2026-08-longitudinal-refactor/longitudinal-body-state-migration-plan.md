# Longitudinal BodyState Migration Plan

> Status: Active  
> Created: 2026-08-15  
> Scope: Product/domain migration across Go + Python + React while preserving Agent runtime contracts  
> Domain source: [`docs/architecture/longitudinal-body-state-domain.md`](../../architecture/longitudinal-body-state-domain.md)  
> Decision source: [`docs/adr/0004-adopt-longitudinal-body-state-model.md`](../../adr/0004-adopt-longitudinal-body-state-model.md)

## 1. Executive Decision

BodySense is migrating from a session-centered business model:

```text
ConsultationSession
  -> extracted_info / health_features
  -> Diagnosis
  -> ConfirmedDiagnoses
  -> MedicalRecord
```

into a longitudinal user-centered model:

```text
Long-lived Conversation
      |
      v
Longitudinal BodyState <--------------------------+
      |                                            |
      +--> DiagnosisAnalysis history               |
      |         |                                  |
      |         +--> user candidate assessment     |
      |                                            |
      +--> current Treatment ----------------------+ 
                |                                  |
                +--> intervention / outcome -------+
```

The migration must preserve the already-working Agent runtime architecture:

- LangGraph checkpoint / interrupt / resume;
- request/run idempotency;
- Runtime Event Log and projections;
- StreamEvent v1;
- Go durable business ownership;
- Python Agent Thread runtime ownership;
- Web projection-driven rendering.

The business truth changes; the runtime ownership architecture does not.

### 1.1 Implementation checkpoint — Diagnosis boundary reached (2026-08-15)

The first implementation batch has now been written through the Diagnosis boundary.
This is the deliberate hand-off point where subsequent Diagnosis-agent improvements
(PydanticAI, targeted evidence-gap retrieval, deeper evals) return to the user's
"learn while refactoring" workflow. Treatment migration has **not** started.

Current code state:

| Area | Status | Evidence / notes |
|---|---|---|
| One long-lived product Conversation | Implemented migration semantics | Omitted conversation ID resolves to the user's latest consultation; Web `/consultation` resolves to the existing conversation. Historical conversations remain readable during migration. |
| BodyState aggregate + immutable revisions | Implemented | Migration `000032`; Go models/repository/service; current projection plus revision history. |
| Fact correction / temporal change APIs | Implemented foundation | Explicit correct/temporal endpoints exist. The old InfoPanel remains a compatibility editor and is not the final correction-vs-temporal UX. |
| Observation separate from Fact | Implemented | Durable observation table/API and workbench projection identity. |
| Workbench → BodyState | Implemented compatibility bridge | Existing `health_features` UI now receives BodyState-derived rows with durable item IDs; edits/deletes update/deactivate the exact BodyState item instead of creating a second truth. |
| Consultation → BodyState producer | Implemented | extracted-info events, ask_user answers, user edits and positive safety events can update BodyState. |
| Consultation context reads BodyState | Implemented baseline | Python runtime receives current BodyState + bounded recent revisions; corrected durable state outranks stale transcript text. Historical-message retrieval is still a later context-engineering enhancement. |
| First-class Safety state | Implemented baseline | Positive safety signals persist in BodyState; active `requires_review` safety state creates a `safety_blocked` DiagnosisAnalysis without ordinary candidate generation. |
| Diagnosis consumes exact BodyState revision | Implemented | Go sends exact revision/current state/history; broad Go-side RAG is removed from the new production path. |
| Diagnosis candidate count 0..N | Implemented | Python typed model/prompt no longer caps candidates at 3; completed requires >=1, insufficient/safety states may return zero. |
| Immutable Diagnosis history | Implemented | Migration `000033`; Go-owned analysis/candidate IDs; history API; basic Web Diagnosis history timeline. |
| User candidate assessment | Implemented | `confirmed / unsure / not_applicable` persisted separately; unselected candidates remain in the immutable analysis. |
| PydanticAI Diagnosis Agent | **Learning/refactor next** | Current service still uses the existing AIExecutor/AIService seam plus typed Pydantic output. Do not migrate this silently before the learning pass. |
| Targeted Diagnosis evidence-gap RAG | **Learning/refactor next** | New contract supports evidence references, but tool-driven targeted retrieval is intentionally not implemented in this batch. |
| Treatment / outcome loop | **Not started in this batch** | Legacy treatment route/model remains only for compatibility and becomes the next domain phase after Diagnosis learning. |

Validation status:

> **Validated checkpoint.** `@agent-v2` shell access is now available and the
> Diagnosis-boundary batch has completed its local repair/validation pass:
>
> - Go focused packages: pass;
> - `go test ./...`: pass;
> - Python Diagnosis focused tests: 19 passed;
> - Python full suite: 212 passed;
> - touched Diagnosis Python files: Ruff clean and targeted Pyright 0 errors;
> - Web DiagnosisPanel target tests: 9 passed;
> - Web full suite: 138 passed;
> - Web typecheck/build: pass;
> - contracts test: pass;
> - disposable PostgreSQL migration smoke from `000001` through `000033`: pass.
>
> Repository-wide Pyright still contains pre-existing debt outside this migration
> scope; the new/touched Diagnosis surfaces themselves are clean. See
> [`body-state-diagnosis-checkpoint-2026-08-15.md`](./body-state-diagnosis-checkpoint-2026-08-15.md)
> for exact validation evidence and repaired findings.

---

## 2. Superseded Plan

The former active plan:

```text
docs/plan/archive/diagnosis-medical-record-refactor-plan.md
```

is historical.

Its DMR-100 and DMR-101 implementation/learning history remains valid:

- constructor dependency injection;
- consumer-owned `AIExecutor(Protocol)`;
- typed Diagnosis confidence/severity;
- initial typed Diagnosis models;
- run-scoped dependency modeling.

However these former target assumptions are superseded:

```text
Consultation as the durable health container
MedicalRecord as the final artifact
DiagnosisDependencies(extracted_info, profile, conversation_summary, rag_context) as north-star input
Diagnosis candidate count limited to 1..3
one session phase as the health journey state machine
```

Do not continue implementing old DMR tickets mechanically.

---

## 3. Verified Current-system Migration Anchors

The repository already contains useful transitional pieces:

### Consultation runtime

- durable Conversation / Run / Turn / Message/runtime event concepts;
- Python LangGraph thread runtime;
- ask_user interrupt/resume;
- thread projections;
- SSE/public StreamEvent v1.

### Structured health state

Current thread/session projection exposes:

```text
extracted_info
health_features
```

Current `health_features` already distinguishes:

```text
posture_findings
discomforts
negative_findings
movement_limitations
red_flags
user_answers
```

This should be treated as a BodyState precursor, not thrown away.

### Diagnosis

Python's typed Diagnosis models have now been migrated in:

```text
apps/ai-service/src/models/diagnosis.py
```

The obsolete `max_length=3` rule has been removed. `DiagnosisDependencies` now models an exact BodyState revision/current state plus bounded relevant history; the remaining next step is to teach/migrate the execution layer to PydanticAI and targeted evidence-gap retrieval without changing these domain semantics.

### Web workbench

The right panel already supports structured health-feature rendering and user confirm/modify/delete operations.

This is the natural migration surface for the BodyState workbench.

---

## 4. Target Ownership

### Go durable business domain

Go owns:

```text
BodyState
BodyStateRevision
Fact / Observation durable state
Safety business state
DiagnosisAnalysis
UserCandidateAssessment
Treatment / TreatmentRevision
Intervention / Outcome durable records
projections/read models
```

### Python reasoning/runtime

Python owns:

```text
LangGraph Agent Thread
consultation reasoning
RAG/tool reasoning
BodyState mutation proposals
Diagnosis Agent execution
Treatment recommendation/review reasoning
```

Python must not become the durable BodyState database.

### React

React owns:

```text
conversation interaction
BodyState workbench interaction
correction confirmation
Diagnosis review
Treatment review
trend/history presentation
```

React does not infer durable truth from local cache alone.

---

# Phase 0 — Documentation / Contract Baseline

## Objective

Make the new domain language authoritative before production refactors begin.

## Deliverables

- ADR 0004 accepted.
- `longitudinal-body-state-domain.md` is the domain source of truth.
- `feature_spec_longitudinal_body_health.md` is the active product behavior spec.
- `longitudinal-health-loop.md` replaces linear Health Journey semantics.
- old MedicalRecord plan is archived.
- architecture/active-plan indexes point at the new source documents.

## Acceptance

No active product/architecture plan should claim that MedicalRecord is the core terminal artifact or that Diagnosis is capped at three candidates.

---

# Phase 1 — BodyState Domain Foundation

## Objective

Introduce the durable user-scoped BodyState boundary without replacing the entire Consultation runtime at once.

## Why first

Diagnosis, Treatment, and context engineering cannot be migrated reliably until they have one stable durable business input.

## Scope

Define first production contracts for:

```text
BodyState
BodyStateRevision
Fact
Observation
Concern reference/grouping
SafetyState
```

Hypothesis/Evidence persistence may initially be narrower if necessary, but their semantic separation must be preserved.

## Suggested first vertical model

The first implementation should support at minimum:

- user-reported discomfort Fact;
- negative finding Fact;
- lifestyle/activity Fact;
- posture/self-test Observation;
- origin/provenance;
- active/resolved lifecycle;
- correction versus temporal update;
- monotonic revision;
- optimistic `expected_revision` update.

## Out of scope

- full generic clinical ontology;
- explicit Episode table;
- wearable integration;
- advanced automatic causal modeling.

## Tests

Business tests should cover:

1. add a current Fact;
2. correct right -> left as correction;
3. mark old symptom improved/resolved as temporal change without deleting history;
4. add a later recurrence;
5. add an Observation distinct from Fact;
6. stale expected revision rejects/merges safely;
7. explicit user edit outranks stale AI proposal.

## Acceptance

Go can persist and read a current BodyState projection plus immutable revision history independently of `consultation_sessions.health_features`.

---

# Phase 2 — Consultation as a BodyState Producer

## Objective

Make the long-lived Consultation runtime propose and commit BodyState changes instead of treating `health_features` as the final business schema.

## Protected contracts

Preserve:

- LangGraph checkpoint semantics;
- `ask_user` interrupt/resume;
- StreamEvent v1 envelope;
- runtime event persistence/replay;
- thread projection rendering.

## Migration shape

```text
user message / ask_user answer
  -> Python consultation reasoning
  -> typed BodyStateMutationProposal
  -> Go validation + expected revision check
  -> BodyStateRevision commit
  -> BodyState projection event/update
  -> Web right-side workbench
```

## Compatibility

Existing `health_features` may remain as a transitional projection/adapter while Web and contracts migrate.

Do not create a second independently editable business truth.

## Important rules

- AI inference cannot be committed as Fact merely because the model generated it.
- user-edited workbench state is authoritative after server validation;
- provenance is retained;
- structured ask answers can produce Facts but remain linked to the source interaction.

## Acceptance

A correction in the workbench changes BodyState and the next AI turn reads the corrected BodyState rather than stale transcript text.

---

# Phase 3 — BodyState Workbench Projection

## Objective

Turn the current `health_features` right panel into the user-facing current BodyState workbench.

## Target sections

```text
Current Facts
Observations
Recent Changes
AI Hypotheses
Safety
Latest Diagnosis
Current Treatment
```

Not all sections must ship in the first slice.

## UX rules

- hide raw revision/fact IDs by default;
- visually distinguish Fact / Observation / Hypothesis;
- expose correction versus later-change semantics when ambiguity matters;
- allow history expansion;
- refresh from Go durable projections after mutation.

## Acceptance

Users can understand current truth and edit it without managing documents or versions.

---

# Phase 4 — Long-conversation Context Engineering

## Objective

Make one visible long-lived Conversation sustainable without sending the full transcript to the model.

## Context target

```text
current user input
+ recent relevant messages
+ current BodyState
+ relevant temporal history
+ current safety state
+ current Diagnosis/Treatment where relevant
+ retrieved old messages only when needed
+ RAG only when needed
```

## Important boundary

LangGraph checkpoint remains Agent Thread runtime truth.

BodyState remains Go durable business truth.

Python receives/queries explicit business context rather than becoming the durable owner.

## Tests / evals

- model does not repeat questions already answered in BodyState;
- corrected state overrides stale old messages;
- long historical transcript does not need to be fully included;
- old relevant injury can be retrieved when a new concern makes it relevant;
- token/context trace remains observable.

---

# Phase 5 — Diagnosis Consumes BodyStateRevision

## Objective

Replace legacy Diagnosis inputs with an explicit snapshot boundary.

## Target input

Conceptually:

```text
DiagnosisRunInput
├─ body_state_revision
├─ current projection relevant to scope
├─ relevant temporal context
├─ safety context
└─ optional targeted Evidence retrieval capabilities
```

The exact transport shape should avoid copying the entire BodyState if a typed query/read boundary is more appropriate.

## Python model changes

The current target models must be revised:

- remove `max_length=3` from `DiagnosisAgentOutput.candidates`;
- allow zero candidates when status is `insufficient_information` or `safety_blocked`;
- introduce analysis status;
- organize output by concern or otherwise preserve concern references;
- evolve candidate basis from only natural-language text toward Fact/Observation/Evidence references;
- keep durable candidate IDs outside model-generated identity.

## Diagnosis retrieval behavior

Default:

- synthesize existing BodyState + Evidence.

Targeted RAG only for:

- new uncovered concern;
- evidence gap;
- conflict;
- safety-relevant uncertainty.

Do not broad-search every old body issue on every diagnosis run.

## Acceptance

A long-lived user with seven relevant candidates receives seven structured candidates if that is what the analysis supports.

---

# Phase 6 — Durable Diagnosis History and User Candidate Assessment

## Objective

Persist immutable Diagnosis analyses and separate user interpretation from AI analysis.

## Durable objects

```text
DiagnosisAnalysis
UserCandidateAssessment
```

Suggested candidate user states:

```text
confirmed
unsure
not_applicable
```

## Invariants

- unselected candidates are never deleted;
- later BodyState changes never rewrite historical Diagnosis;
- each analysis pins exact source BodyState revision;
- user assessment cannot fabricate candidate content/identity;
- new analyses can supersede current analytical relevance without deleting old analyses.

## Freshness

Introduce a DiagnosisInvalidationPolicy rather than using `current_revision != source_revision` as the only rule.

## Acceptance

Web can display and compare historical Diagnosis analyses and their candidate assessment states after reload.

---

# Phase 7 — Treatment as a Revisioned Intervention

## Objective

Migrate legacy `treatment_plan` into a current accepted Treatment with explicit source analysis/state and review lifecycle.

## Inputs

```text
BodyStateRevision
DiagnosisAnalysis
user constraints/preferences
safety state
relevant Evidence
```

## Lifecycle

```text
active
review_recommended
paused
superseded
completed
```

## Invariant

AI recommendations do not silently mutate an accepted active Treatment.

## Acceptance

A material new BodyState change can mark Treatment for review without destroying the prior accepted plan.

---

# Phase 8 — Intervention / Outcome Feedback

## Objective

Close the long-term product loop.

## Producers

- training check-ins;
- adherence;
- user feedback;
- daily activity changes;
- repeated self-tests;
- posture re-analysis.

## Outputs

Accepted outcomes update BodyState.

Trend analysis can correlate:

```text
intervention timing
behavior change
symptom change
```

without asserting unsupported causality.

## Acceptance

Diagnosis and Treatment review can use meaningful trend/outcome information rather than only the latest symptom snapshot.

---

# Phase 9 — Legacy Domain Retirement

## Objective

Remove obsolete session-centered business truth only after all active paths use BodyState.

## Candidates to retire/migrate

```text
consultation_sessions.extracted_info as authoritative health truth
consultation_sessions.health_features as authoritative health truth
consultation_sessions.diagnosis overloaded semantics
consultation_sessions.treatment_plan overloaded semantics
single linear consultation phase as health journey truth
legacy MedicalRecord implementation/spec paths
Diagnosis candidate max=3 target constraint
```

## Important

Do not delete runtime/session concepts required by LangGraph/Conversation execution simply because the business health model moved to BodyState.

---

# Phase 10 — Hardening

## Areas

### Concurrency

- optimistic expected revision;
- retry/merge strategy;
- multiple devices.

### Privacy

Distinguish:

```text
correct information
exclude from future AI reasoning
privacy delete
```

### Evals

Diagnosis evals:

- zero candidate insufficient case;
- 1 candidate focused case;
- 7+ candidate multi-concern case;
- counterevidence;
- stale analysis;
- safety block;
- targeted RAG gap.

Consultation evals:

- correction vs temporal change;
- user edit authority;
- long-conversation context retrieval;
- Fact vs Hypothesis separation.

Treatment evals:

- review after material change;
- safety pause;
- correlation language.

---

## 5. Recommended Implementation Order

Use vertical batches rather than rebuilding every schema first.

```text
Batch 0  Documentation baseline
  -> Batch A  BodyState core + revision
  -> Batch B  Consultation producer + workbench projection
  -> Batch C  Long-conversation context
  -> Batch D  Diagnosis on BodyStateRevision
  -> Batch E  Diagnosis history + user assessment
  -> Batch F  Treatment migration
  -> Batch G  Outcome feedback loop
  -> Batch H  Legacy retirement + hardening
```

Each batch must keep the primary existing Consultation runtime operational.

---

## 6. First Implementation Ticket Candidates

Do not start all of these simultaneously.

### BDS-001 — Add BodyState domain vocabulary and persistence skeleton

Goal:

- one BodyState per user;
- monotonic revision;
- current projection read;
- no Consultation integration yet.

### BDS-002 — Implement first Fact mutation path

Goal:

- add/update/correct/resolve a discomfort/lifestyle Fact;
- preserve provenance and revision.

### BDS-003 — Add Observation path

Goal:

- persist self-test/posture Observation separately from Fact.

### BDS-004 — Map current health_features into BodyState migration adapter

Goal:

- current Consultation can feed the new domain without breaking SSE/interrupt runtime.

### BDS-005 — BodyState workbench read projection

Goal:

- Web reads current BodyState from Go durable projection.

### BDS-006 — Diagnosis model correction before PydanticAI integration

Goal:

- remove obsolete 1..3 limit;
- define Diagnosis analysis status;
- align typed output with BodyState-based direction without yet rewriting full Diagnosis execution.

The earlier DI/Protocol learning work remains useful and should be preserved.

---

## 7. Protected Contracts

Until intentionally migrated, protect:

```text
ADR 0002 ownership model
POST /api/v1/consultation-runs
Run / Turn / Conversation identity
request idempotency
StreamEvent v1 envelope
runtime_events durability / replay
thread projection
ask_user interrupt / resume
Python LangGraph checkpointing
Go authorization/business persistence ownership
governance accepted/degraded/rejected semantics
```

Changes to public contracts must have characterization/target tests before cutover.

---

## 8. Explicit Non-goals of the First Migration

- full medical ontology;
- replacing LangGraph;
- replacing assistant-ui/runtime projections;
- wearable/device ingestion;
- full causal inference engine;
- sophisticated Episode entity;
- automatic medical diagnosis claims;
- rebuilding the entire Training product before BodyState exists;
- preserving every obsolete internal field indefinitely.

---

## 9. Verification Strategy

Because this migration crosses domain/business/runtime boundaries, validation should proceed from narrow to wide.

### Go

- BodyState domain/service/repository unit tests;
- revision conflict tests;
- API/persistence integration tests;
- projection reload tests.

### Python

- typed model tests;
- BodyState mutation proposal tests;
- Diagnosis output tests including 7+ candidates;
- safety/governance evals;
- long-context retrieval evals.

### Web

- BodyState workbench state rendering;
- user correction flow;
- reload from durable projection;
- Diagnosis grouped multi-candidate rendering;
- stale/review indicators.

### End-to-end

1. user reports a symptom;
2. BodyState updates;
3. user corrects side;
4. current context reflects correction;
5. user later reports improvement without deleting history;
6. Diagnosis pins a revision;
7. state changes materially;
8. old analysis remains history but is marked review/stale appropriately;
9. treatment/outcome feeds back into BodyState;
10. refresh/replay preserves both conversation runtime and business state.

---

## 10. Risk Ledger

### R1 — BodyState becomes an untyped JSON bag

Containment:

- enforce semantic object boundaries;
- avoid prematurely dumping all `health_features` into one JSON schema;
- add typed mutation commands/tests.

### R2 — Duplicate truth during migration

Containment:

- explicitly mark `health_features` as transitional projection/adapter;
- choose one write owner per vertical slice;
- retire old write path after cutover.

### R3 — Python checkpoint and Go BodyState diverge

Containment:

- preserve ADR 0002 distinction;
- pass committed BodyState revision/context to Agent runs;
- Python proposes changes, Go commits business truth.

### R4 — Every revision invalidates Diagnosis/Treatment

Containment:

- implement material-change policies;
- track changed concerns/fact references;
- avoid revision-number-only invalidation.

### R5 — Timeline complexity leaks into UI

Containment:

- current projection first;
- history/details on demand;
- keep raw revision/event mechanics internal.

### R6 — Legacy DMR implementation history is lost

Containment:

- keep archived plan;
- reuse existing DI/Protocol/typed-model work where compatible;
- rewrite only target semantics that were superseded.

---

## 11. Definition of Done for the Migration Program

The migration is complete when:

- one user-scoped BodyState is the durable health source of truth;
- Conversation no longer carries authoritative health state in session-only fields;
- BodyState preserves meaningful temporal history and provenance;
- Consultation/Posture/Training can contribute to BodyState through explicit boundaries;
- long-lived Conversation context uses BodyState + relevant history rather than full transcript replay;
- Diagnosis pins BodyState revisions and supports data-driven candidate count;
- user candidate assessment is separate from immutable DiagnosisAnalysis;
- Diagnosis history replaces the need for MedicalRecord as a core artifact;
- current Treatment pins its source analysis/state and supports review lifecycle;
- outcomes can feed back into BodyState;
- safety is first-class state;
- obsolete MedicalRecord/session-diagnosis target paths are retired;
- ADR 0002 runtime ownership remains intact.
