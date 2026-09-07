# BodySense Learning Protocol

This file defines how a source-course point becomes a real BodySense learning exercise.

## 1. Exercise template

Every `BS-*` exercise should be written/run with these fields:

```text
Source point
Concept
BodySense target files
Prediction before reading/running
Task
Failure case
Verification command/evidence
Explain-back questions
Production change: none | justified change
```

## 2. The six-step loop

### Step 1 — Locate

Use the source point only to know what concept to study. Find the smallest real BodySense surface that exhibits it.

### Step 2 — Predict

Before asking AI for an explanation, write what you think happens. A wrong prediction is useful evidence of the actual knowledge gap.

### Step 3 — Trace

Follow the real call/data/state path. Prefer executable truth in this order:

```text
code/tests/config
-> accepted ADR/current architecture docs
-> historical plans
```

### Step 4 — Break or test

Do at least one:

- write a characterization test;
- add a malformed input;
- simulate a race/retry/disconnect;
- inspect a real network/DB trace;
- construct a small isolated spike for a comparison topic.

### Step 5 — Repair only a real gap

Do not refactor production merely to use a technique mentioned by a course. Change production code only if the existing behavior or architecture has a demonstrated deficiency.

### Step 6 — Explain back

Without looking at the source answer, explain:

1. what problem the concept solves;
2. how BodySense currently solves it;
3. what alternative the source course uses;
4. the important failure mode;
5. how the tests prove the behavior.

## 3. AI coaching policy

Default learning mode is progressive hints:

```text
Goal
-> relevant concept/file
-> small hint
-> learner attempt
-> review
-> deeper hint
-> direct solution only when blocked or explicitly requested
```

AI may implement directly in a shipping session, but that session does not count as a completed learning exercise until the learner later performs the explain/test evidence.

## 4. Mastery levels

- `L1 Recognize` — can locate the concept in BodySense.
- `L2 Explain` — can explain the path/ownership in their own words.
- `L3 Predict` — can predict normal and failure behavior before execution.
- `L4 Verify` — can design tests/traces that prove the behavior.
- `L5 Change` — can safely modify the boundary and preserve invariants.

Core topics require at least `L4`; independent delivery topics require `L5`.

## 5. What does not count

These are not completion evidence by themselves:

- AI generated a correct answer;
- all tests were already green before the learner understood why;
- reading a tutorial without touching the real code;
- copying a source exercise into another toy repository;
- changing libraries just to match the course stack.

## 6. Session closeout

After a meaningful session, update `.practice-map/maps/bodysense-fundamentals.md` with:

- source points covered;
- mastery level reached;
- files traced/changed;
- tests/commands actually run;
- one misconception corrected;
- next smallest rep.
