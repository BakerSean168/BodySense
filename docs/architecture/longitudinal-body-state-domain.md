# BodySense Longitudinal BodyState Domain Model

> Status: Architecture source of truth  
> Version: v1  
> Updated: 2026-08-15  
> Decision anchor: [ADR 0004](../adr/0004-adopt-longitudinal-body-state-model.md)  
> Runtime ownership anchor: [ADR 0002](../adr/0002-agent-runtime-ownership.md)

## 1. Purpose

This document defines the target business-domain model for BodySense.

It answers:

- what the product is tracking over months and years;
- which object owns the user's current body truth;
- how conversation, posture analysis, diagnosis, treatment, and training interact;
- how corrections differ from true body-state changes;
- how time, provenance, uncertainty, safety, and history are represented;
- which concepts are authoritative business state versus AI reasoning artifacts;
- which invariants future implementations must preserve.

It intentionally does **not** prescribe the final SQL schema or exact API payload for every object. Storage and transport contracts should be derived from the semantics in this document, not the other way around.

---

## 2. Product North Star

BodySense is not a collection of one-off consultation reports.

It is a long-lived AI-assisted body-state management system:

```text
User talks / uploads / measures / trains
            |
            v
   Longitudinal BodyState
      |              |
      |              +----> Trend / safety reasoning
      |
      +----> DiagnosisAnalysis
                  |
                  v
              Treatment
                  |
                  v
          behavior / outcome
                  |
                  +----------> BodyState
```

The product should feel simple:

- one long-lived health workspace;
- one long-lived conversation surface;
- one current body-state workbench;
- diagnosis history;
- one current treatment view plus history;
- trends and safety changes over time.

The internal system may still use many Turns, Runs, checkpoints, projections, revisions, and domain objects.

Product simplicity does not imply internal state simplicity.

---

## 3. User Mental Model

The user should think:

> "BodySense remembers my body, not just my chat history. I can keep talking to it, correct things, track changes, run new analyses, and see whether my current improvement plan still fits."

The user should **not** need to think in terms of:

- creating a new consultation for every issue;
- saving and importing many consultation documents;
- manually managing BodyState versions;
- generating a separate MedicalRecord after each diagnosis;
- deciding which technical session/checkpoint/run is current.

---

## 4. Core Domain Map

```text
User
 |
 +-- Long-lived Conversation
 |
 +-- BodyState  <-----------------------------------------------+
 |      |                                                       |
 |      +-- Concerns                                            |
 |      +-- Facts                                               |
 |      +-- Observations                                        |
 |      +-- Hypotheses                                          |
 |      +-- Evidence refs                                       |
 |      +-- Safety state                                        |
 |      +-- Current projection                                  |
 |      +-- Revision history                                    |
 |                                                              |
 +-- DiagnosisAnalysis history                                  |
 |      |                                                       |
 |      +-- Concern analyses                                    |
 |      +-- Candidates                                          |
 |      +-- User candidate assessments                          |
 |                                                              |
 +-- Current Treatment + Treatment revisions                    |
 |      |                                                       |
 |      +-- Interventions                                       |
 |      +-- adherence / outcome --------------------------------+
 |
 +-- Optional derived HealthReport exports
```

The center of the domain is `BodyState`, not Conversation and not Diagnosis.

---

## 5. Ownership Boundaries

### 5.1 Go

Go owns durable business truth:

- user ownership and authorization;
- BodyState and BodyState revisions;
- accepted Facts / Observations and their provenance;
- durable Hypothesis records where the product chooses to persist them;
- safety business state;
- DiagnosisAnalysis identity and persistence;
- user assessment of Diagnosis candidates;
- Treatment identity, lifecycle, and accepted revisions;
- intervention/outcome business records;
- public Runtime Event Log and projections.

### 5.2 Python

Python owns AI reasoning/runtime behavior:

- LangGraph Agent Thread runtime truth;
- checkpointed message/tool/interrupt state;
- consultation reasoning;
- knowledge retrieval and AI tool execution where assigned;
- typed proposals for BodyState changes;
- typed Diagnosis reasoning outputs;
- typed Treatment recommendations/reviews;
- safety/governance reasoning adapters.

Python does not own durable BodyState truth.

### 5.3 Web

Web owns interaction and presentation:

- long-lived conversation rendering;
- BodyState workbench rendering;
- user corrections / confirmations / structured answers;
- Diagnosis candidate review;
- Treatment review interactions;
- trend/history presentation.

Web does not invent business truth or runtime resume semantics.

---

## 6. Conversation

### 6.1 Purpose

Conversation records the long-lived interaction history between the user and AI.

It is useful for:

- user-visible history;
- recent conversational continuity;
- provenance navigation;
- retrieval of old discussion details;
- debugging and audit where allowed.

### 6.2 Conversation is not BodyState

Conversation must not be treated as the current body truth.

Reasons:

- old messages can contain corrected information;
- users can describe temporary states;
- AI can make hypotheses that later prove weak;
- a years-long transcript is not feasible as an LLM context window;
- posture analysis and training can change BodyState without ordinary chat text.

### 6.3 Model context strategy

The user sees one long-lived Conversation.

The model receives a dynamically built context such as:

```text
recent conversation turns
+ current BodyState projection
+ relevant BodyState history
+ active safety state
+ active/current Diagnosis context when relevant
+ current Treatment when relevant
+ retrieved historical messages on demand
+ retrieved knowledge on demand
```

Do not use an ever-growing conversation summary as the sole long-term memory strategy.

---

## 7. BodyState Aggregate

### 7.1 Definition

`BodyState` is the durable, user-scoped aggregate representing what BodySense currently knows about the user's body and how that knowledge changed over time.

A user has one BodyState aggregate.

### 7.2 Logical shape

```text
BodyState
├─ user identity reference
├─ current_revision
├─ concerns[]
├─ facts[]
├─ observations[]
├─ hypotheses[]
├─ safety_state
├─ lifestyle/current-context state
└─ temporal history / revisions
```

The exact relational decomposition is an implementation decision.

### 7.3 Producers

BodyState can be updated by proposals/inputs from:

- Consultation Agent;
- direct user edits in the right-side workbench;
- structured ask_user answers;
- posture/photo analysis;
- training logs and check-ins;
- self-tests;
- uploaded external reports;
- future wearables or measurements.

No producer owns the aggregate.

### 7.4 Consumers

Primary consumers include:

- Consultation context construction;
- Diagnosis;
- Treatment generation/review;
- trend analysis;
- safety policies;
- UI current-state projection;
- future health-report export.

---

## 8. BodyStateRevision

### 8.1 Purpose

A `BodyStateRevision` is an immutable record of one meaningful committed state change.

Example:

```text
R142
- added: right-knee downstairs discomfort
- updated: sitting 6h/day -> 4h/day
- changed: left-glute pain active -> resolved
```

### 8.2 Revision identity

A revision should have a monotonic or otherwise totally ordered identity per BodyState.

Diagnosis and Treatment must pin exact revisions.

### 8.3 Revision granularity

A revision is not created per token or per database column write.

A useful rule is:

> one semantically coherent accepted BodyState mutation = one revision.

A single user turn may therefore add/update several items in one revision.

### 8.4 Current projection versus history

Normal product reads should use a current projection.

The system may store compact revision/change history for audit and temporal reasoning without requiring full event-sourcing replay for every read.

---

## 9. Concern

### 9.1 Definition

A `Concern` is an internal grouping for related body-state information.

Examples:

- head/neck posture;
- left glute/leg discomfort;
- right knee after running;
- historical right ankle injury.

### 9.2 User experience

Users are not required to create or manage concerns as documents.

The system may display them as natural body-region sections/cards.

### 9.3 Lifecycle

Suggested semantic states:

```text
active
monitoring
resolved
```

Possible metadata:

```text
first_observed_at
last_updated_at
related_fact_ids
related_observation_ids
related_hypothesis_ids
```

Concern is organizational; it must not become a second source of truth that duplicates its Facts/Observations.

---

## 10. Fact

### 10.1 Definition

A `Fact` is structured information accepted as part of the user's BodyState.

Examples:

- user works at a computer around 7 hours/day;
- left glute becomes sore after prolonged sitting;
- no current numbness;
- user has a historical right ankle sprain;
- current exercise frequency is 4 times/week.

### 10.2 Fact categories

The exact enum should remain evolvable, but common semantic groups include:

```text
symptom / discomfort
negative finding
lifestyle factor
activity / behavior
injury history
functional limitation
user preference relevant to health/treatment
```

### 10.3 Origin

A Fact should preserve origin such as:

```text
user_reported
user_edited
structured_answer
imported_external_record
system_accepted_derived_value
```

AI inference by itself is not a Fact origin.

### 10.4 Review state

Useful semantic states may include:

```text
unverified
confirmed
corrected
superseded
excluded_from_reasoning
```

Do not conflate review state with temporal lifecycle.

### 10.5 Stable identity

Facts need stable identity so that the system can:

- correct or supersede a specific statement;
- link Diagnosis basis to exact information;
- preserve history;
- explain why a current state exists.

---

## 11. Observation

### 11.1 Definition

An `Observation` records something observed or measured through a method rather than merely reported as a subjective statement.

Examples:

- self-test shows bilateral tightness, left greater than right;
- side-view posture analysis shows ear position anterior to shoulder landmark;
- external report records a measured value;
- future device records range of motion.

### 11.2 Observation shape

Conceptually:

```text
Observation
├─ observation_id
├─ type / method
├─ body region
├─ value
├─ optional unit
├─ condition / test context
├─ observed_at
├─ provenance
└─ quality / confidence metadata where relevant
```

### 11.3 Why separate from Fact

"I feel tight" and "self-test shows a left/right difference" are different epistemic objects.

Diagnosis should be able to weight them differently.

---

## 12. Hypothesis

### 12.1 Definition

A `Hypothesis` is an AI-generated explanatory possibility based on current Facts, Observations, and Evidence.

Example:

```text
Prolonged sitting may contribute to the current glute discomfort pattern.
```

### 12.2 Critical invariant

A Hypothesis never silently becomes a Fact.

### 12.3 Lifecycle

A useful lifecycle may include:

```text
active
strengthened
weakened
unsupported
retired
```

A hypothesis should be able to reference:

```text
supporting_fact_ids
supporting_observation_ids
supporting_evidence_ids
counterevidence_ids
```

This allows later evidence to weaken an earlier explanation without rewriting history.

---

## 13. Evidence

### 13.1 Definition

Evidence is source material used to support reasoning.

Potential source types:

```text
knowledge_base
external_record
self_test_protocol
posture_analysis
measurement
other curated source
```

### 13.2 Evidence is not Body Fact

A knowledge entry saying that prolonged sitting is associated with a pattern does not mean the user has that pattern.

### 13.3 Provenance

Evidence should preserve enough information for traceability:

```text
source identity
source/version where available
retrieved_at
relevant excerpt/summary
metadata
```

### 13.4 Citation

A citation is a presentation of Evidence in a user-facing answer or diagnosis.

Evidence is the domain concept; citation is one UI/contract representation.

---

## 14. Provenance

### 14.1 Goal

The system should be able to answer:

> "Why does BodySense believe this current state item?"

without requiring the full original Conversation to remain the only explanation.

### 14.2 Typical provenance references

```text
conversation message / turn
user edit
ask_user interaction answer
self-test
posture-analysis run
external document
training check-in
measurement
```

### 14.3 Minimal provenance snapshot

For important accepted Facts, retaining a minimal source excerpt or normalized source snapshot can make the object independently understandable even when the original conversation is later unavailable.

Exact retention policy must comply with privacy requirements.

---

## 15. Temporal Semantics

Longitudinal reasoning is a primary feature, not an archival afterthought.

### 15.1 Two clocks

Where relevant, distinguish:

```text
occurred_at / observed_at / valid_from
```

from:

```text
recorded_at
```

Example:

A user reports in 2026 that a right ankle injury happened in 2019. The event occurred in 2019 but was recorded by BodySense in 2026.

### 15.2 Lifecycle state

Prefer a small lifecycle dimension such as:

```text
active
inactive
resolved
```

### 15.3 Trend

Keep trend separate from lifecycle:

```text
improving
stable
worsening
fluctuating
unknown
```

This supports states such as:

```text
active + improving
active + worsening
resolved
```

without a giant temporal enum.

### 15.4 Recurrence

A symptom that resolved and later returns should preserve the historical resolved period.

Implementation may eventually introduce explicit Episodes if needed, but Episode is not yet a required first-class entity.

---

## 16. Correction vs Temporal Change

### 16.1 Correction

Meaning:

> the previous record was inaccurate.

Example:

```text
"I said right glute, but I meant left glute."
```

The corrected item may supersede the old item.

### 16.2 Temporal change

Meaning:

> the previous record was true at the time, but the body later changed.

Example:

```text
"It used to hurt, but it has improved now."
```

The old historical state remains valid; a later state/change is added.

### 16.3 UI implication

When a user edits a historically meaningful item in an ambiguous way, the UI may need to clarify:

```text
Was the old information wrong?
OR
Was it correct then but changed later?
```

This distinction has direct Diagnosis value.

---

## 17. Safety State

Safety cannot be a decorative `red_flags` array attached at the end of a response.

Safety is first-class BodyState/business state.

Possible effects of a new safety concern:

- alter Consultation behavior;
- block or constrain ordinary Diagnosis generation;
- trigger safety-specific guidance/governance;
- mark current Treatment as review-required or paused;
- create high-priority state events/projections.

Safety policy remains deterministic/business-governed where possible; LLM interpretation is advisory and must pass runtime/business validation.

---

## 18. BodyState Mutations

### 18.1 Producers propose, Go commits

Python or other producers may propose typed mutations such as:

```text
AddFact
CorrectFact
ResolveFact
AddObservation
UpdateLifestyleState
AddHypothesis
WeakenHypothesis
SetSafetyState
```

Go validates ownership, revision expectations, schema, and business invariants before committing a new BodyStateRevision.

### 18.2 User edits

Explicit user edits have higher authority than older AI extraction.

A useful priority principle is:

```text
explicit user edit / confirmation
> structured user answer
> current user message extraction
> older AI extraction
> historical summary
```

This is a semantic rule, not a reason to discard provenance.

### 18.3 Concurrency

BodyState updates should use optimistic revision semantics.

Conceptually:

```text
proposal based_on_revision = R140
current revision = R142
=> do not silently overwrite
```

The system may retry, merge non-conflicting changes, or return a conflict requiring review.

---

## 19. DiagnosisAnalysis

### 19.1 Purpose

`DiagnosisAnalysis` is a durable analysis of a specific BodyState snapshot plus relevant temporal context.

It is not the BodyState truth itself.

### 19.2 Input contract

Every analysis pins:

```text
body_state_revision_id
```

It may also record:

```text
scope
relevant_history_window / selected temporal context metadata
retrieved evidence references
model/governance metadata
```

### 19.3 Status

Diagnosis may legitimately produce:

```text
completed
partial
insufficient_information
safety_blocked
```

A successful call does not have to return at least one candidate.

### 19.4 Scope

Diagnosis can support:

```text
full_body
selected concerns
focused re-analysis
```

The initial product may default to full active-body analysis, but the domain should not hard-code that assumption.

---

## 20. Concern-level Diagnosis

For long-lived users, a flat list of many candidates becomes hard to understand.

Prefer conceptual organization:

```text
DiagnosisAnalysis
├─ concern_analyses[]
├─ cross_concern_patterns[]
├─ information_gaps[]
├─ safety_summary
└─ summary
```

A concern analysis may contain:

```text
concern_id
status
candidates[]
missing_information[]
evidence_used[]
```

This lets an analysis contain 2, 8, or 15 candidates without turning the UI into an unstructured ranking list.

---

## 21. DiagnosisCandidate

### 21.1 Candidate count

Candidate count is determined by the body state and reasoning result.

There is no fixed business maximum such as 3.

### 21.2 Conceptual fields

A mature candidate may include:

```text
candidate_id               # assigned by durable application layer
label
related_concern_ids[]
confidence
optional evidence_strength
optional impact/severity
basis_fact_ids[]
basis_observation_ids[]
supporting_evidence_ids[]
counterevidence_ids[]
reasoning_summary
differential[]
missing_information[]
safety_notes[]
```

Not every field must ship in the first implementation.

### 21.3 Confidence, evidence strength, and impact are different

Do not overload one field to mean:

- how well the candidate fits;
- how strong the evidence is;
- how severe the user's current impact is.

These are separate dimensions.

### 21.4 Counterevidence

Diagnosis should be able to express both:

- why a candidate fits;
- what current information argues against or weakens it.

This reduces confirmation bias and improves later re-analysis.

---

## 22. User Candidate Assessment

The DiagnosisAnalysis preserves all generated candidates.

User response is a separate durable object/state.

Suggested user-facing states:

```text
confirmed
unsure
not_applicable
```

An unselected candidate must not be silently deleted.

`not_applicable` means the user currently feels it does not fit; it is not equivalent to a clinician-proven false diagnosis.

Historical user candidate assessments remain attached to the corresponding DiagnosisAnalysis.

---

## 23. Diagnosis Freshness / Invalidation

A historical DiagnosisAnalysis remains immutable.

Separately, the current product may classify it as:

```text
fresh
potentially_stale
stale
```

### 23.1 Do not invalidate on every revision

Not every BodyStateRevision affects every Diagnosis.

Example:

- minor sleep update may not invalidate a posture analysis;
- newly reported leg numbness may materially invalidate an old ordinary musculoskeletal analysis.

### 23.2 DiagnosisInvalidationPolicy

The implementation should eventually define an explicit policy using changed concerns/facts/safety signals rather than comparing revision numbers alone.

---

## 24. Treatment

### 24.1 Purpose

Treatment is the current accepted intervention strategy for the user's state.

It is not a terminal artifact.

### 24.2 Input identity

Every accepted Treatment revision should record at least:

```text
source_body_state_revision_id
source_diagnosis_analysis_id
```

### 24.3 Lifecycle

Useful lifecycle states include:

```text
active
review_recommended
paused
superseded
completed
```

### 24.4 Review policy

New BodyState data may recommend review, but the system should not silently rewrite an active intervention.

Material plan changes should follow an explicit review/acceptance path.

---

## 25. Intervention and Outcome

To evaluate whether Diagnosis and Treatment remain useful, the system needs intervention/outcome data.

### 25.1 Intervention examples

```text
exercise prescription
activity change
sitting reduction
mobility routine
sleep habit adjustment
```

### 25.2 Outcome examples

```text
symptom severity/frequency change
movement change
self-test change
posture-analysis change
training adherence
new discomfort
```

### 25.3 Correlation versus causation

The system may observe:

```text
training begins
-> two weeks later symptom decreases
```

This supports a temporal association.

It does not by itself prove causation.

Domain language and AI prompts must preserve this distinction.

---

## 26. Stable Profile, Body Metrics, and Lifestyle Projection

ADR 0007 makes the ownership boundary strict: `UserProfile` is not a health-state aggregate.

### 26.1 Stable Profile / identity

The durable Profile contains only relatively stable identity context:

```text
birth date
sex / gender where relevant and user-provided
account identity/preferences outside this health domain
```

Age is derived from birth date. Height, weight, injury history, activity, sleep, exercise,
nutrition, substance use, and recovery state do **not** belong in `user_profiles`.

### 26.2 Measurements are Observations

Height and weight are BodyState Observations:

```text
anthropometry.height
anthropometry.weight
```

Updating the current measurement retains the old Observation and links the replacement through
`supersedes_observation_id`. BMI is derived from the current projection.

### 26.3 Lifestyle is a taxonomy, not another aggregate

The current lifestyle taxonomy is:

```text
lifestyle.activity
lifestyle.sleep
lifestyle.exercise
lifestyle.nutrition
lifestyle.substances
lifestyle.recovery
```

A user-facing `LifestyleSnapshot` projects the active facts into a stable “生活方式” UI. There is no
separate Lifestyle source-of-truth table.

Onboarding, the Lifestyle editor, and Consultation all converge on the same BodyState taxonomy, but
they do not have the same epistemic authority. Direct onboarding/editor writes are explicit structured
user edits and may become confirmed current facts immediately. Consultation normalization is model-
mediated even when based on explicit user language, so `record_lifestyle_context` persists an
`ai_extracted / unverified / excluded_from_reasoning` candidate. Occupation name or symptoms alone
must never be used to infer it.

The Lifestyle projection exposes reviewable candidates separately from confirmed current state. User
acceptance promotes the candidate and temporally closes the previous confirmed fact in one revision;
rejection keeps the candidate as durable provenance but excluded from reasoning. This preserves both
AI-native conversational capture and the BodyState governance rule that model extraction is not fact
acceptance.

### 26.4 Correction is not temporal change

If a previous fact was incorrect, use `CorrectFact`.

If a previous fact was true and later changed, close its validity interval and create a replacement
linked by `supersedes_fact_id`. This distinction is essential for longitudinal reasoning such as:

```text
sleep regular -> shift work
sitting 8h/day -> walking/standing work
exercise 0 sessions/week -> 4 sessions/week
```

These transitions can support temporal association analysis but never prove causation by themselves.

### 26.5 Health history also stays out of Profile

The onboarding-level injury summary is represented as `history.injury_summary` in BodyState. More
granular injury episodes may be modeled as additional health-history facts without expanding the
Profile schema.

---

## 27. Knowledge Retrieval Strategy

### 27.1 Consultation

Consultation is the primary place for exploratory RAG:

- explaining concepts;
- helping users describe symptoms;
- selecting self-tests;
- asking targeted questions;
- supporting advice with citations.

### 27.2 Diagnosis

Diagnosis should primarily synthesize existing BodyState + Evidence.

It may perform **targeted retrieval** when it finds:

- an evidence gap;
- a new concern not previously covered;
- conflicting information;
- safety-relevant uncertainty.

Diagnosis should not blindly repeat broad RAG over all historical body topics on every run.

### 27.3 Evidence ownership

Whoever performs retrieval must preserve the structured evidence/provenance required for later reasoning and citations.

---

## 28. Long-lived Conversation Context Engineering

A multi-year Conversation cannot be passed wholesale to the model.

The runtime context builder should prefer:

```text
1. current user input
2. recent relevant messages
3. current BodyState projection
4. relevant BodyState temporal events
5. active safety state
6. relevant Diagnosis/Treatment state
7. retrieved old messages only when needed
8. retrieved knowledge only when needed
```

Structured durable state should outrank stale transcript text when the user has explicitly corrected information.

---

## 29. Privacy, Deletion, and Reasoning Exclusion

A long-term health system must distinguish at least three user intents:

### 29.1 Correction

"That information was wrong."

This changes the semantic record while preserving correction history as policy allows.

### 29.2 Exclude from reasoning

"Do not use this information in future AI reasoning."

This is not necessarily the same as deleting the historical record.

### 29.3 Privacy deletion

"Delete this data."

This invokes a privacy/data-retention policy and may require cascading removal or redaction.

Do not implement all three meanings behind a single ambiguous delete action.

---

## 30. Derived Health Reports

`MedicalRecord` is not a core aggregate in this model.

If users need a printable/shareable record, BodySense may generate a derived artifact such as:

```text
HealthReport
```

from:

```text
current or selected BodyStateRevision
+ selected DiagnosisAnalysis history
+ current Treatment summary
+ relevant trends/safety state
```

Such an artifact is an export/presentation snapshot and must not become a second mutable health truth.

---

## 31. UI Projection

A complex domain should still produce a simple workbench.

Example right-side projection:

```text
My Body State
--------------------------------
Head / neck
- forward-head tendency
- trend: improving

Left glute
- occasional soreness after prolonged sitting
- previous period: improved, recently mild recurrence

Right knee
- downstairs discomfort
- onset: around two weeks after starting running

History
- previous right ankle injury

Safety
- current safety status

Recent changes
- sitting: 8h -> 4h
- exercise: 0 -> 4 sessions/week

AI hypotheses
- H1: sitting contribution weakened by later evidence
- H4: recent activity change may be relevant

Latest Diagnosis
- freshness / date / summary

Current Treatment
- active / review recommended / paused
```

The UI does not need to show raw Fact IDs or revision internals by default.

---

## 32. Core Business Invariants

These rules should guide schema, API, runtime, and tests:

1. One user owns one durable BodyState aggregate.
2. Conversation is an input/history surface, not current health truth.
3. Meaningful committed changes create immutable BodyState revisions.
4. Diagnosis and Treatment pin exact BodyState revisions.
5. Historical Diagnosis/Treatment artifacts are never silently rewritten.
6. Fact, Observation, Hypothesis, and Evidence remain distinct.
7. AI inference cannot silently become Fact.
8. Correction and temporal change are distinct.
9. User edits outrank stale AI extraction for current truth.
10. Candidate count is not capped by a business constant.
11. Diagnosis may validly return zero candidates with an explicit status.
12. Full DiagnosisAnalysis is stored independently from user candidate assessment.
13. Unselected candidates are not deleted.
14. New safety findings may block/review Diagnosis and Treatment.
15. Not every BodyState revision invalidates current Diagnosis/Treatment.
16. Correlation is not represented as proven causation.
17. MedicalRecord is not a core source of truth.
18. Go owns durable business truth; Python owns Agent runtime reasoning; Web consumes projections.

---

## 33. Deliberately Open Design Questions

These are intentionally **not** fixed yet because locking them prematurely would create unnecessary migration cost:

- whether `Concern` becomes its own relational table;
- whether recurrence eventually needs an explicit `Episode` entity;
- exact Fact category enums;
- exact Observation schema per test/device type;
- exact confidence/evidence-strength scales;
- whether Hypotheses are all durable or only selected/high-value ones;
- exact Diagnosis freshness algorithm;
- exact automatic/manual Treatment review UX;
- exact HealthReport/export format;
- exact data-retention implementation for minimal provenance excerpts.

Future decisions must preserve the invariants above.

---

## 34. Migration Interpretation of Current Models

Existing repository concepts should be interpreted as migration-era pieces:

```text
consultation_sessions.extracted_info
    -> narrow precursor to BodyState Facts

consultation_sessions.health_features
    -> precursor to BodyState Fact / Observation projection

Consultation phase
    -> workflow/runtime state, not the user's health lifecycle

DiagnosisDependencies(extracted_info, profile, conversation_summary, rag_context)
    -> transitional Diagnosis input, not the target domain boundary

DiagnosisAgentOutput candidates max 3
    -> obsolete target constraint, to be removed in implementation

consultation_sessions.diagnosis / treatment_plan
    -> overloaded legacy fields to be migrated
```

This document defines the target semantics; migration should preserve existing runtime contracts until their replacement vertical slice is ready.

---

## 35. Target Plugin-style Relationships

The architecture should converge toward modules that interact through BodyState rather than directly mutating each other:

```text
Consultation -----------+
Posture Analysis -------+
Training Check-ins -----+
External Records -------+----> BodyState
Future Measurements ----+
                              |
                 +------------+-------------+
                 |            |             |
                 v            v             v
             Diagnosis   Trend Analysis   Safety Policy
                 |
                 v
              Treatment
                 |
                 v
           Intervention
                 |
                 v
              Outcome
                 |
                 +--------------------------> BodyState
```

This is the primary extensibility boundary for future BodySense development.

---

## 36. Definition of Domain-model Success

The model is working when BodySense can correctly express all of the following without special-case document workflows:

- a user corrects "right" to "left" without pretending the body changed sides;
- an old symptom improves and later recurs while preserving the resolved interval;
- a new running habit precedes a new knee complaint without asserting causation;
- an old AI hypothesis becomes weaker when later evidence contradicts it;
- a historical injury is learned years later with distinct occurred/recorded times;
- Diagnosis compares current state against meaningful trends;
- eight Diagnosis candidates can be organized coherently by concern;
- user marks some candidates confirmed, some unsure, and some not applicable without deleting any;
- Treatment is reviewed when material body changes occur;
- training outcomes feed back into BodyState;
- a multi-year conversation remains usable because the model reads structured current state and relevant history rather than every old message;
- the user sees one coherent health workspace instead of managing internal system artifacts.
