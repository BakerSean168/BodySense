# ADR 0004: Adopt a Longitudinal BodyState as the Core Health Domain Model

## Status

Accepted

## Date

2026-08-15

## Context

BodySense originally evolved around a session-oriented flow:

```text
Consultation
  -> collected consultation state
  -> DiagnosisAnalysis
  -> user confirmation
  -> MedicalRecord
  -> Treatment / Training
```

That model is workable for a one-off consultation product, but it creates unnecessary product and domain complexity for the experience BodySense is actually trying to provide.

The expected user behavior is long-lived and continuous:

- a user may keep using the same health workspace for months or years;
- symptoms, posture findings, lifestyle, training, injuries, and self-observations change over time;
- information can be corrected after the fact without implying that the body itself changed;
- old symptoms can improve, resolve, recur, or be replaced by new concerns;
- posture analysis, training logs, daily check-ins, uploads, and future device data may all contribute to the same health picture;
- Diagnosis should reason over current state plus relevant temporal history rather than over one isolated consultation transcript;
- Treatment should be reviewed against later outcomes and body-state changes.

The previous design also introduced `MedicalRecord` as a separate final aggregate. Under a longitudinal model this duplicates information already owned by the user's current BodyState, BodyState history, DiagnosisAnalysis history, and Treatment history.

A second problem is ownership. A long-lived chat transcript is useful interaction history, but it is not a reliable representation of the user's current body truth. Re-reading an ever-growing transcript is not a viable long-term context strategy and makes correction, provenance, temporal reasoning, and cross-module updates ambiguous.

ADR 0002 remains authoritative for runtime ownership:

- Python owns Agent Thread runtime truth;
- Go owns durable business truth and Runtime Event Log truth;
- Web consumes durable projections and emits user intents.

This ADR defines the business-domain truth that Go must own.

## Decision

### 1. One long-lived user health workspace

At the product level, BodySense presents one long-lived health workspace per user rather than requiring users to create and manage many consultation conversations.

The workspace contains:

- a long-lived AI conversation surface;
- a live BodyState workbench;
- current and historical Diagnosis analyses;
- current Treatment and its revisions;
- temporal trends, safety status, and relevant outcomes.

Technical Runs, Turns, checkpoints, interruptions, projections, and other runtime identities remain internal implementation concepts.

### 2. Conversation is an interaction channel, not health truth

The long-lived Conversation records interaction history.

It is not the authoritative representation of the user's current body state.

The system must not rely on re-reading the full historical transcript to reconstruct current health truth.

### 3. Introduce one user-scoped Longitudinal BodyState aggregate

Each user owns one durable `BodyState` aggregate.

`BodyState` represents both:

- the current projection of what is known about the user's body; and
- the temporal history of meaningful state changes.

It is the shared business boundary between consultation, posture analysis, diagnosis, treatment, training, and future tracking sources.

### 4. BodyState changes are revisioned

Every meaningful committed BodyState mutation produces an immutable `BodyStateRevision`.

A revision records at minimum:

- its monotonic revision identity;
- when it was committed;
- which facts, observations, hypotheses, safety states, or related values changed;
- provenance for those changes.

The current BodyState is a projection of the latest committed state. The system does not require event replay for normal reads.

### 5. Separate Fact, Observation, Hypothesis, and Evidence

These concepts have different epistemic meanings and must not be collapsed into a generic `health_features` bag.

- **Fact** — user-reported or otherwise accepted structured body/lifestyle information.
- **Observation** — an observation or measurement produced by a self-test, posture analysis, external document, sensor, or other observation method.
- **Hypothesis** — an AI-generated explanatory possibility. It is never automatically promoted to fact.
- **Evidence** — knowledge or source material used to support reasoning, including RAG results and external evidence.

### 6. Preserve provenance and temporal semantics

Health information must retain enough provenance to explain where it came from without requiring the complete original conversation transcript to remain the only source of meaning.

The model must distinguish:

- when something occurred or was observed;
- when the system learned or recorded it;
- whether it is currently active, resolved, or inactive;
- whether its trend is improving, stable, worsening, fluctuating, or unknown.

### 7. Correction and temporal change are different operations

The system must distinguish:

- **Correction** — the historical record was wrong, e.g. "I said right side but meant left side";
- **Temporal change** — the previous information was correct at the time, but the body later changed, e.g. "the pain improved".

Correction may supersede a previous statement. Temporal change preserves the previous historical state and records a later state transition.

### 8. Concerns are internal organization, not user-managed documents

`Concern` groups related body-state information such as head/neck, left glute, right knee, or ankle history.

Concerns may become active, monitored, or resolved, but users are not required to create, save, import, or manage them as independent documents.

### 9. Consultation and other modules are BodyState producers

The following may propose or contribute BodyState changes:

- long-lived Consultation Agent;
- posture analysis;
- user edits in the BodyState workbench;
- structured ask/resume answers;
- training and daily check-ins;
- uploaded documents and external records;
- future devices or measurements.

No producer owns BodyState itself.

### 10. Diagnosis consumes an exact BodyState revision

Each `DiagnosisAnalysis` must record the exact `BodyStateRevision` it analyzed.

Diagnosis may also consume a bounded, relevant temporal context derived from BodyState history.

Diagnosis must not depend on a floating `latest` state after creation, and historical DiagnosisAnalysis objects are never silently rewritten when BodyState later changes.

### 11. Diagnosis candidate count is data-driven

There is no business rule limiting Diagnosis to three candidates.

Diagnosis covers all relevant active concerns in scope and may produce zero to many candidates depending on the evidence.

A valid analysis may also be:

- partial;
- insufficient-information;
- safety-blocked.

Candidates should be organized by concern when that improves clarity.

### 12. Diagnosis analysis and user assessment are separate facts

A `DiagnosisAnalysis` preserves the complete set of generated candidates.

User reactions such as:

- confirmed;
- unsure;
- not applicable / currently does not fit;

are stored separately and do not delete unselected candidates.

### 13. Diagnosis may become stale

A DiagnosisAnalysis can remain historically valid while no longer representing the current state.

A separate invalidation policy determines whether later BodyState changes make an analysis fresh, potentially stale, or stale.

Not every BodyState revision invalidates Diagnosis.

### 14. Treatment is a revisioned intervention, not a static terminal artifact

Treatment consumes an explicit DiagnosisAnalysis and BodyState revision.

Treatment has lifecycle states such as active, review-recommended, paused, superseded, or completed.

AI must not silently rewrite an active intervention merely because new information appears; material changes require an explicit review/acceptance path.

### 15. Outcomes feed back into BodyState

Training adherence, symptom change, activity change, self-tests, and other outcomes may update BodyState.

Temporal association may be recorded, but correlation must not be represented as proven causation without an appropriate evidence basis.

### 16. Remove MedicalRecord as a core aggregate

BodySense will not use a separate `MedicalRecord` aggregate as the normal destination of Consultation or Diagnosis.

The durable health history is represented by:

- BodyState and BodyStateRevision history;
- DiagnosisAnalysis history and user assessment states;
- Treatment revisions and outcome history.

If a future user needs an exportable or shareable report, the system may generate a derived `HealthReport` / export snapshot from existing durable state. Such a report is a presentation artifact, not a new source of business truth.

### 17. Go remains durable business owner

Consistent with ADR 0002:

- Go owns durable BodyState, BodyStateRevision, DiagnosisAnalysis, Treatment, safety state, and their business transitions;
- Python reasons over explicit input snapshots and proposes typed outputs/changes;
- Python does not become the durable BodyState database;
- Web renders projections and submits user edits/intents.

## Consequences

### Positive

- Product UX becomes substantially simpler: users do not manage many consultations or assessment documents.
- Long-term health changes become first-class data instead of being inferred from chat history.
- Diagnosis can reason about progression, recurrence, behavior changes, and intervention outcomes.
- The same BodyState boundary supports consultation, posture analysis, training, future sensors, and other producers.
- Historical Diagnosis remains reproducible because each analysis pins an exact BodyState revision.
- AI hypotheses cannot accidentally become durable user facts.
- The system can support current-state views and historical trend views without introducing a parallel MedicalRecord aggregate.

### Negative / Cost

- Existing `consultation_sessions.extracted_info`, `health_features`, Diagnosis request shapes, and session phase semantics become migration-era representations rather than the final domain model.
- BodyState requires careful concurrency, revision, provenance, correction, and temporal semantics.
- Long-lived conversations require retrieval/context engineering rather than complete-transcript prompting.
- Existing Diagnosis/MedicalRecord implementation plans must be superseded and rewritten.
- Existing linear Health Journey stage models must move toward a continuous loop rather than a terminal `completed` user journey.

## Invariants

The following are considered architecture invariants unless replaced by a later ADR:

1. One user has one durable Longitudinal BodyState aggregate.
2. Conversation is not the authoritative health-state model.
3. Meaningful durable BodyState changes create immutable revisions.
4. Fact, Observation, Hypothesis, and Evidence have distinct semantics.
5. AI hypotheses never silently become user/body facts.
6. Correction and temporal change are distinct operations.
7. Historical analyses and interventions are not rewritten by later state.
8. Diagnosis pins an exact BodyState revision.
9. Diagnosis candidate count has no fixed business maximum.
10. Full Diagnosis output and user candidate assessment are stored separately.
11. Treatment pins its source DiagnosisAnalysis and BodyState revision.
12. Correlation is not automatically represented as causation.
13. MedicalRecord is not a core aggregate.
14. Go owns durable business truth; Python owns Agent runtime reasoning; Web consumes projections.

## Follow-up

- Define the full Longitudinal BodyState domain vocabulary and lifecycle.
- Replace the old Consultation -> MedicalRecord feature specification.
- Replace the linear Health Journey workflow with a longitudinal health loop.
- Create a migration plan from `consultation_sessions` / `health_features` / legacy Diagnosis to BodyState-based contracts.
- Remove the current hard `DiagnosisAgentOutput.candidates <= 3` target constraint during the corresponding implementation ticket.
