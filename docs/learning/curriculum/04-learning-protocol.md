# BodySense Learning Protocol

This protocol separates curriculum completeness from learner progress and defines how a source-course point becomes a real BodySense exercise.

## 1. Two independent state machines

### Curriculum lifecycle

```text
SOURCE_INDEXED
  -> MAPPED
  -> EXERCISE_READY
  -> LEARNER_VERIFIED
```

- `SOURCE_INDEXED`: stable source identity/location is known.
- `MAPPED`: source objective is assigned `DIRECT`, `COMPARE` or `OPTIONAL` and has a BodySense adaptation.
- `EXERCISE_READY`: a concrete exercise card exists and passes curriculum validation.
- `LEARNER_VERIFIED`: learner mastery evidence meets the exercise gate.

### Mastery level

```text
L1 Recognize
-> L2 Explain
-> L3 Predict
-> L4 Verify
-> L5 Change
```

Do not use `L1/L2` for historical project phases. Old phrases such as “L1 Diagnosis / L2 Treatment” are historical roadmap labels only; current mastery levels always mean Recognize/Explain/Predict/Verify/Change.

## 2. Exercise card contract

An `EXERCISE_READY` card must contain:

```text
Source point
Concept
Prerequisites
BodySense target files
Prediction before reading/running
Task
Failure case
Verification command/evidence
Explain-back questions
Production change rule
L4/L5 acceptance gate
```

The machine ledger must point to the card and the validator must confirm its target paths exist.

## 3. The learning loop

### Step 1 — Locate

Use the source point to identify the smallest real BodySense surface that demonstrates the concept. Executable truth order:

```text
code/tests/config
-> accepted ADR/current architecture docs
-> historical plans
```

### Step 2 — Predict

Write what should happen before asking AI to explain it. Include at least one failure prediction.

### Step 3 — Trace

Follow the actual call/data/state path and name ownership at each boundary.

### Step 4 — Verify or falsify

At least one observation must be capable of proving the prediction wrong:

- characterization test;
- malformed input;
- race/retry/disconnect simulation;
- browser/network/database trace;
- isolated comparison spike;
- performance/rerender measurement when the concept is an optimization.

### Step 5 — Repair only a real gap

Do not refactor production merely to use a source-course technique. Change product code only when the exercise demonstrates a defect, missing invariant, missing observability, or justified structural improvement. Adding a focused regression/characterization test can itself be the useful production-quality improvement.

### Step 6 — Explain back

Without reading the prepared answer, explain:

1. the engineering problem;
2. how BodySense currently solves it;
3. the source-course alternative when different;
4. the important failure mode;
5. why the selected evidence proves the behavior.

## 4. L4 is a hard gate for core material

For core Full Stack Open, TECH SCHOOL and Agent engineering material, completion requires at least `L4 Verify`.

The learner must independently design or select a test/trace/experiment that distinguishes correct behavior from a relevant failure case and explain why that evidence is discriminating.

Examples of evidence that are useful but insufficient alone:

- a diagram;
- a prediction;
- AI-generated explanation;
- tests that were already green but the learner cannot explain;
- a successful AI-generated code change.

Independent vertical delivery requires `L5 Change`.

## 5. Placement audit rule

Placement is performed only against `EXERCISE_READY` prerequisite nodes. Existing project work can satisfy an exercise, but only if there is concrete evidence that the learner reached the same mastery gate.

Placement outcome per exercise:

```text
unassessed
L1/L2/L3 gap
L4 verified
L5 verified
```

A previous AI implementation is not automatically learner evidence. Conversely, a previously documented prediction/test/review can be reused; the learner is not forced to rewrite code just to re-earn credit.

Machine recording path:

```text
select active track
-> generate placement-status.md
-> assess the next ready node
-> record L1..L5 + concrete evidence
-> update canonical ledger mastery/current lifecycle
-> regenerate/validate
```

`ledger/learner-placement.json` stores the active track and the latest placement journal row per assessed item. The source ledgers remain canonical for `mastery.current`, evidence and `LEARNER_VERIFIED`. The placement validator rejects journal/ledger drift, ghost mastery without a journal row, and verification below the required gate. Mastery recording is monotonic through the normal command; a downgrade requires an explicit manual correction because it invalidates earlier evidence rather than representing ordinary study progress.

## 6. Dependency rule

Dependency claims are promoted with exercise readiness rather than invented globally. A mapped-but-not-ready record uses `dependency_audit: UNMODELED` and does not pretend that simple source ordering is a verified prerequisite graph.

Every ready exercise uses `dependency_audit: REVIEWED` and lists its concrete prerequisites. The curriculum validator rejects unknown/self dependencies, cycles, and any ready node that depends on a non-ready node. The generated prerequisite view therefore shows a closed, currently executable dependency spine.

The dependency graph is allowed to merge source courses. For example:

```text
HTTP mutation tracing
  -> stale-client conflict handling

migration/repository basics
  -> transaction
  -> row lock/deadlock
  -> isolation

JWT/auth middleware
  -> refresh rotation/replay

Agent typed boundary + runtime ownership
  -> streaming/HITL/replay
```


## 6.5 Learner-facing study tracks

The generated prerequisite graph is the dependency truth, but it is not the most ergonomic study interface once dozens of nodes are ready. Use [`views/study-tracks.md`](./views/study-tracks.md) as the learner-facing lens over that graph.

A track is **not** a second curriculum source of truth. It may overlap other tracks and it may depend on ready prerequisites outside the track. The generator checks that every listed node is currently `EXERCISE_READY` / `LEARNER_VERIFIED`; the ledgers and prerequisite graph remain canonical.

Recommended rule:

```text
choose one track
-> resolve its external prerequisite closure once
-> placement-audit the ready nodes
-> start at the first node below L4
-> reuse verified prerequisite evidence across overlapping tracks
```

This keeps study order coherent without forcing a single global linear course sequence.

## 7. AI coaching policy

Default learning mode:

```text
Goal
-> relevant concept/file
-> small hint
-> learner attempt
-> review
-> deeper hint
-> direct solution only when blocked or explicitly requested
```

A shipping session may use direct Agent implementation, but that session does not grant mastery automatically.

## 8. Session closeout

Update `.practice-map/maps/bodysense-fundamentals.md` with:

- source/exercise IDs covered;
- mastery level actually reached;
- files traced or changed;
- commands/tests actually run;
- one corrected misconception or confirmed prediction;
- next unlocked prerequisite node.
