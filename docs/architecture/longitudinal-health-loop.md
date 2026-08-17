# BodySense Longitudinal Health Loop Architecture

> Status: Active target architecture  
> Updated: 2026-08-15  
> Domain source: [Longitudinal BodyState Domain Model](./longitudinal-body-state-domain.md)  
> Decision: [ADR 0004](../adr/0004-adopt-longitudinal-body-state-model.md)

## 1. Why the old linear journey model was replaced

The previous Health Journey model treated a user as progressing through a mostly linear sequence:

```text
profile -> consultation -> diagnosis -> plan -> training -> completed
```

That is a useful task-flow abstraction for a one-time workflow, but it does not match BodySense's long-term product behavior.

A user's body does not reach a permanent `completed` state. Symptoms improve, resolve, recur, new concerns appear, activity changes, training affects outcomes, and old hypotheses become stronger or weaker.

Therefore BodySense now models a continuous longitudinal loop centered on `BodyState`.

---

## 2. Core Loop

```text
                    +----------------------+
                    |      BodyState       |
                    | current + history    |
                    +----------+-----------+
                               |
              +----------------+----------------+
              |                |                |
              v                v                v
        Consultation       Diagnosis      Trend / Safety
              |                |
              |                v
              |            Treatment
              |                |
              |                v
              |          Intervention
              |                |
              |                v
              +----------- Outcome
                               |
                               +-------------> BodyState
```

This loop has no user-level terminal `completed` state.

Individual objects may complete:

- a Concern can resolve;
- a Treatment revision can complete;
- an intervention can finish;
- a DiagnosisAnalysis can become stale;
- a training cycle can complete.

The user-level health model continues.

---

## 3. Module Roles

### 3.1 Consultation

Purpose:

- accept natural language;
- teach useful vocabulary just in time;
- collect body/lifestyle facts;
- ask safety and clarification questions;
- run exploratory RAG;
- guide self-tests;
- propose BodyState mutations.

Consultation does not own BodyState.

### 3.2 Posture Analysis

Purpose:

- produce structured posture observations/measurements;
- preserve method/provenance;
- propose accepted observations into BodyState.

It does not directly create Diagnosis truth.

### 3.3 Diagnosis

Purpose:

- analyze one exact BodyState revision plus relevant history;
- organize reasoning by concern;
- produce zero-to-many candidates;
- express supporting evidence, counterevidence, uncertainty, information gaps, and safety constraints;
- preserve immutable historical analyses.

Diagnosis does not mutate historical BodyState to make its conclusion true.

### 3.4 Treatment

Purpose:

- propose/maintain the current accepted intervention strategy;
- pin its source DiagnosisAnalysis and BodyState revision;
- react to material state changes through review rather than silent rewrite.

### 3.5 Training / Daily Tracking

Purpose:

- record intervention execution and adherence;
- collect subjective response and new discomfort;
- produce outcome facts/observations;
- feed accepted changes back into BodyState.

### 3.6 Trend Analysis

Purpose:

- compare current state with temporal history;
- surface improvement, worsening, recurrence, or association with behavior/intervention changes;
- support Diagnosis/Treatment review.

Trend Analysis must not convert temporal association into proven causation without stronger evidence.

### 3.7 Safety Policy

Purpose:

- elevate high-priority safety changes;
- constrain or block normal Diagnosis/Treatment paths when policy requires;
- mark active Treatment for review or pause.

Safety is cross-cutting and can interrupt the ordinary loop.

---

## 4. Business State Versus Runtime State

Longitudinal business state is distinct from Agent runtime state.

### Business state (Go-owned)

```text
BodyState
BodyStateRevision
DiagnosisAnalysis
Candidate user assessment
Treatment / TreatmentRevision
Intervention / Outcome
Safety state
```

### Agent runtime state (Python-owned, ADR 0002)

```text
LangGraph thread state
messages used by model
checkpoints
interrupt/resume
runtime tool sequencing
```

### Public projection/runtime ledger (Go-owned)

```text
runs
runtime_events
thread projections
BodyState projection
Diagnosis/Treatment projections
```

Do not merge these ownership layers.

---

## 5. Trigger Model

The loop is not driven by one global journey stage.

Instead, specific conditions expose actions.

Examples:

```text
BodyState has enough relevant data
-> diagnosis can be requested

material BodyState change after Diagnosis
-> diagnosis review/re-analysis recommended

new safety concern
-> ordinary diagnosis/treatment constrained

Diagnosis accepted + user reviews candidates
-> treatment generation/review available

current treatment + enough new outcomes
-> treatment review available
```

This is closer to capability/readiness policies than a single rank-based state machine.

---

## 6. Readiness Policies

Readiness should be scoped by action.

Examples:

### DiagnosisReadiness

Inputs:

- target concerns;
- current Facts/Observations;
- safety state;
- missing high-value information.

Output:

```text
ready
missing_information
blocked_by_safety
```

### TreatmentReadiness

Inputs:

- current DiagnosisAnalysis;
- user candidate assessment;
- relevant BodyState revision;
- safety state;
- constraints/preferences.

### ReviewReadiness

Inputs:

- changed concerns/facts;
- new outcomes;
- current Diagnosis/Treatment source revisions;
- safety changes.

These policies are durable business rules owned by Go, while Python may provide advisory reasoning inputs.

---

## 7. Diagnosis History as the User's Analytical Timeline

Instead of generating a separate MedicalRecord after each diagnosis, BodySense keeps immutable DiagnosisAnalysis history.

Each analysis records:

```text
analysis id
body_state_revision_id
scope
concern analyses
all candidates
user assessment state
safety/governance metadata
created_at
```

The user can compare:

```text
Aug analysis
Oct analysis
Nov analysis
```

and understand what changed between them.

This analytical history, together with BodyState history, replaces the need for a separate MedicalRecord aggregate.

---

## 8. Treatment History

The user normally sees one current Treatment.

Internally:

```text
T1 revision 1
T1 revision 2
T2 ...
```

Each accepted revision records its source state/analysis.

A Treatment can be:

```text
active
review_recommended
paused
superseded
completed
```

The system can explain:

> "This plan was created from Diagnosis D8 based on BodyState R142. BodyState has changed materially since then."

---

## 9. Example Longitudinal Loop

```text
Day 1
User reports left-glute soreness after sitting
-> BodyState R10

Day 3
Self-test shows bilateral tightness, left > right
-> BodyState R13

Day 5
Diagnosis D1 analyzes R13
-> candidates across glute/leg concern

Day 6
User confirms some candidates, marks another unsure
-> user assessment saved against D1

Day 7
Treatment T1 accepted
-> based on D1 + R13

Week 2
Sitting time falls, symptoms improve
-> R21 / R22
-> temporal association available

Week 4
Running begins, right-knee discomfort appears
-> R30
-> D1 potentially stale
-> T1 review recommended

Week 5
Focused RAG for new knee evidence gap
-> Diagnosis D2 based on R32

Week 8
Old glute symptom recurs despite lower sitting time
-> R44
-> earlier sitting hypothesis weakened
-> diagnosis/treatment review
```

The user never needed to create or import a new consultation document.

---

## 10. UI Navigation Implication

The primary navigation can derive from durable capabilities rather than linear stage.

Possible actions:

```text
continue conversation
review current body state
view recent changes
run diagnosis
compare diagnosis history
review current treatment
open today's training
submit feedback
review safety notice
```

The Web should not infer these actions from ad-hoc local state. Go should project actionable/readiness state where the rule is business-critical.

---

## 11. Deprecated Linear Concepts

The following should not be treated as final product truth:

```text
one consultation session = one health journey episode
consulting -> diagnosis_ready -> record_ready -> completed
MedicalRecord as final artifact
one final confirmed diagnosis that overwrites analysis
one global monotonic phase for the user's entire health journey
```

Runtime phases may still exist for a particular Run/operation, but they do not represent the user's longitudinal health lifecycle.

---

## 12. Success Criteria

The loop architecture is successful when:

- new health information can enter through multiple producer modules;
- BodyState remains the single durable health truth;
- diagnosis can use current plus historical state;
- treatment can be reviewed from later outcomes;
- a resolved concern can recur without erasing its history;
- a new concern does not require a new user-managed conversation;
- safety changes can immediately constrain ordinary actions;
- the system can explain which state/analysis each intervention was based on;
- the UI remains simpler than the internal domain model.
