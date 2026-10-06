# BodySense Product UX Semantics

> Status: Active design input
> Updated: 2026-10-06
> Depends on: [Product Definition](./product-definition.md)

This document converts BodySense business semantics into user-facing information architecture and interaction rules. It intentionally stops short of choosing one final visual shell.

## 1. IA hypothesis

The leading product IA is **One Body Home + Concern Canvas + contextual capabilities**.

```text
Body Home
├─ active concerns
├─ body spatial overview
├─ cross-concern safety
├─ current priorities / Next
└─ longitudinal summary
     │
     └── Concern Canvas
         ├─ Now
         ├─ Why
         ├─ What to do
         ├─ Change
         ├─ History
         └─ contextual Assistant
```

Diagnosis, Treatment and Progress remain meaningful domain artifacts, but they do not need to be permanent top-level tabs.

The existing **Body / Today / History** model remains a control hypothesis if One Body Home becomes too implicit in usability review.

## 2. Body Home

Body Home answers four questions quickly:

1. What matters in my body now?
2. Has anything important changed?
3. Is there a safety issue or review requirement?
4. What should I do next?

Body Home should avoid becoming a metric dashboard. The body itself and active concerns should carry the strongest spatial hierarchy.

## 3. Concern Canvas

A Concern Canvas is a continuous task scope rather than a stack of module pages.

### Now

- current symptoms/facts;
- relevant observations;
- region/context;
- meaningful recent change;
- safety status;
- missing or pending information.

### Why

- analysis freshness;
- candidate explanations;
- supporting and counter evidence;
- uncertainty;
- missing information;
- evidence/provenance drill-down.

### What to do

- current accepted plan;
- pending proposal/revision;
- interventions;
- constraints;
- review triggers;
- entry to training/action mode.

### Change

- outcome capture;
- improvement/same/worse or structured change;
- retest result;
- plan review trigger;
- longitudinal update.

### History

- meaningful state changes;
- analyses;
- accepted/rejected plan revisions;
- outcomes;
- corrections as corrections rather than fake historical events.

## 4. Next is a capability queue

“Next” is not a permanent page. It is a readiness-driven queue of meaningful actions such as:

- clarify one missing symptom property;
- perform a self-test;
- review a pending observation;
- review a stale analysis;
- accept/reject a treatment proposal;
- complete today's training;
- record an outcome;
- review a safety change.

This lets the UI reflect the user's actual state rather than forcing every user through the same workflow.

## 5. Assistant semantics

The Assistant can:

- ask clarifying questions;
- explain any visible artifact;
- capture new information;
- guide a self-test;
- explain evidence;
- help correct state;
- launch or prepare a product action.

The Assistant should inherit context from the selected body region, concern or artifact. Opening it must not destroy or reset that context.

The design may use a pane, drawer, sheet or focused conversation mode depending on task and viewport. A permanent split pane is not an invariant.

## 6. Artifact semantics

### Analysis artifact

Must expose:

- which BodyState snapshot/revision it used;
- freshness/staleness;
- candidate possibilities;
- supporting/counter evidence;
- missing information;
- safety blocks.

### Plan artifact

Must expose:

- accepted versus proposed status;
- interventions;
- constraints;
- review conditions;
- effective revision;
- entry to execution where relevant.

### Outcome artifact

Must expose:

- what changed;
- when;
- relation to concern/action;
- attribution language that does not overclaim causality.

## 7. Critical interaction patterns

### New concern → clarify → analyze

```text
Body Home
→ add/described concern
→ minimal clarification
→ current concern state
→ readiness check
→ analysis when evidence is sufficient
→ next action / plan
```

### Plan → train → outcome → review

```text
accepted plan
→ focused training/action
→ record result
→ BodyState update
→ review trigger
→ retain plan or propose revision
```

### Correction versus change

```text
user edits existing body information
→ ask intent only when ambiguous
   ├─ correct previous record
   └─ record a new change over time
```

### Safety interrupt

```text
new safety-relevant information
→ global safety guard
→ suppress/qualify inappropriate action
→ explain reason
→ provide safe next action
→ normal flow resumes only when allowed
```

## 8. Responsive modes

Responsive design changes presentation, not product semantics.

### Wide desktop

Can show body spatial surface plus supporting detail and optional Assistant simultaneously when useful.

### Narrow desktop / tablet

May use overlay/drawer for Assistant or secondary evidence. Primary task remains visible and recoverable.

### Mobile

Uses one dominant task surface at a time. Assistant/history/evidence may become sheets or routes, but returning restores the previous concern/task context.

The same terms, status semantics and action authority must be preserved at every width.

## 9. Screen/state matrix

Any candidate intended for implementation must cover at minimum:

- Body Home — default;
- Body Home — empty/new user;
- Concern — default;
- Concern — insufficient information;
- Concern — safety interrupt;
- Analysis — ready;
- Analysis — loading;
- Analysis — stale;
- Analysis — error;
- Analysis — safety blocked;
- Plan — no plan;
- Plan — pending proposal;
- Plan — accepted current plan;
- Plan — review recommended;
- Training — step/focus mode;
- Outcome — improved;
- Outcome — same;
- Outcome — worse;
- History — concern timeline;
- edit — correction or new change;
- Assistant — contextual;
- global workspace loading/error.

The V2c evidence set already covers most of this matrix and remains reusable evidence.

## 10. Design evaluation criteria

A product candidate is stronger when:

- users can explain where they are without naming internal modules;
- a concern feels continuous from Now → Why → Plan → Change;
- safety and authority states are obvious;
- the body is a meaningful spatial index, not decoration;
- complex evidence can become dense without making the default Home dense;
- task modes remain visually distinguishable even with text blurred;
- mobile does not become a different product;
- Assistant context survives open/close/resume transitions.
