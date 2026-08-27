# Feature Spec: Longitudinal Body Health Workspace

> Status: Active product specification  
> Version: v1  
> Updated: 2026-08-15  
> Domain source: [Longitudinal BodyState Domain Model](./architecture/longitudinal-body-state-domain.md)  
> Decision: [ADR 0004](./adr/0004-adopt-longitudinal-body-state-model.md)

## 1. Feature Positioning

BodySense provides one long-lived health workspace where a user can continuously describe, understand, analyze, and improve their body state over time.

The product is not organized around repeatedly creating consultation documents.

The core experience is:

```text
one long-lived health conversation
+ one live BodyState workbench
+ diagnosis history
+ current treatment
+ temporal trends and safety
```

The system continuously transforms user-provided information, observations, tests, posture analyses, training feedback, and other accepted inputs into a longitudinal BodyState.

Diagnosis and Treatment reason from this durable BodyState rather than treating the raw conversation transcript as the business record.

---

## 2. Primary User Experience

### 2.1 Main Workspace

Desktop layout:

```text
+--------------------------------------------------------------+
| BodySense                                                    |
+-------------------------------+------------------------------+
| Long-lived AI conversation    | My Body State                |
|                               |                              |
| - free-form questions         | Current concerns             |
| - guided health description   | Current facts                |
| - RAG-backed explanations     | Observations                 |
| - self-test guidance          | Recent changes               |
| - safety questions            | AI hypotheses                |
| - diagnosis discussion        | Safety status                |
|                               | Latest diagnosis             |
|                               | Current treatment            |
+-------------------------------+------------------------------+
```

Mobile may use tabs or stacked sections, but the business model remains the same.

### 2.2 No user-managed consultation documents

The user does not need to:

- create a new consultation per body issue;
- manually save an assessment before it becomes durable;
- import an old assessment into a new conversation;
- decide which consultation version is active.

When the user provides accepted body information, the BodyState workbench updates automatically.

### 2.3 One long-lived conversation

The user may continue using the same visible health conversation for months or years.

The implementation may use many Runs, Turns, checkpoints, projections, or internal grouping objects, but these are not user-managed conversations.

---

## 3. First-use Experience

On first use, BodySense should collect enough stable profile context to make consultation useful without forcing the user through a large medical intake form.

Possible onboarding information:

- birth date as the canonical age source; age is derived when needed rather than maintained by the user;
- sex where relevant and voluntarily provided;
- height / weight where useful;
- a natural-language baseline of daily activity and work habits, without requiring an occupation or job title;
- a natural-language baseline of sleep / shift-work patterns, including irregular schedules;
- current exercise type and a lightweight frequency estimate;
- major known injury or surgery history;
- optional posture photos or external reports.

The profile is baseline context, not a second symptom intake. Current symptoms, goals, concerns, and free-form "self description" belong in Consultation and the longitudinal BodyState instead of being duplicated as profile fields.

Time-varying information such as sitting hours, exercise frequency, and sleep pattern should subsequently be represented as timestamped BodyState changes. If a user's general baseline changes, the profile can be edited, but it should not replace the longitudinal history.

The user can immediately begin talking even if onboarding is incomplete.

---

## 4. Consultation Behavior

Consultation is the primary natural-language producer of BodyState information.

### 4.1 User can speak naturally

Example:

> "I sit a lot for work. My left glute feels stiff and sore after sitting, and my lower leg sometimes feels tight."

The user should not need to know anatomy or rehabilitation terminology first.

### 4.2 Just-in-time health vocabulary

When a description is ambiguous, the AI teaches vocabulary at the moment it becomes useful.

Example:

> "When you say the lower leg feels uncomfortable, is it closer to tightness, soreness, sharp pain, numbness, weakness, burning, or something else?"

UI may show compact explanation cards for:

- sagittal / frontal / transverse plane;
- tightness / soreness / sharp pain / dull pain / numbness;
- body landmarks;
- safe self-observation methods.

Educational content is optional support, not a mandatory prerequisite course.

### 4.3 RAG during consultation

Consultation may use knowledge retrieval to:

- explain relevant concepts;
- choose useful follow-up questions;
- provide safe self-test instructions;
- distinguish common patterns;
- provide citations;
- identify knowledge gaps.

RAG content is Evidence, not a user Body Fact.

### 4.4 Structured questions / ask_user

Blocking structured questions should be used when the missing answer materially affects safety or reliable next reasoning.

Non-critical details should normally be collected without unnecessarily interrupting the conversational response.

---

## 5. BodyState Workbench

The right-side workbench is a live projection of the durable BodyState.

### 5.1 Suggested sections

```text
Current body state
- posture findings
- discomfort / symptoms
- negative findings
- movement limitations
- lifestyle / activity
- historical injuries

Observations
- self-tests
- posture analysis
- measurements

Recent changes
- new
- improved
- worsened
- resolved
- corrected

AI hypotheses
- active
- weakened / strengthened
- needs more information

Safety
- current safety status

Latest diagnosis
- freshness
- summary
- open details

Current treatment
- status
- review recommendation
```

### 5.2 User correction

Users can edit structured state directly.

A direct user correction becomes authoritative over stale AI extraction.

When an edit is ambiguous, the UI should distinguish:

```text
The previous information was wrong
vs
The previous information was correct, but my body changed later
```

### 5.3 AI hypotheses are visually distinct

AI hypotheses must not be presented with the same visual semantics as confirmed body facts.

Recommended language:

```text
Body fact / observation: factual/current display
AI hypothesis: "possible direction", "needs validation", or similar
```

---

## 6. Longitudinal Tracking

### 6.1 Changes are automatic

Users do not manually save BodyState versions.

Meaningful accepted updates create internal BodyState revisions automatically.

### 6.2 Current state and history

The main workbench emphasizes current state.

Users can open a history/trend view for questions such as:

- when did this symptom begin?
- has it improved?
- did it fully resolve and later recur?
- what changed around the same time?
- when did the user start running?
- how did sitting time change?
- when did a new safety symptom appear?

### 6.3 Time semantics

The product should be able to represent:

- occurred/observed time;
- recorded time;
- active / resolved lifecycle;
- improving / stable / worsening / fluctuating trend;
- recurrence after a resolved interval.

---

## 7. Diagnosis

### 7.1 Entry point

Diagnosis is a separate analysis action in the health workspace.

The user may request:

- a full current-body analysis;
- later, a focused re-analysis of selected concerns.

Diagnosis does not require a separate chat interface.

### 7.2 Diagnosis input

Diagnosis uses:

```text
exact BodyState revision
+ relevant temporal history
+ relevant observations
+ relevant evidence
+ safety context
```

It does not need the entire raw conversation transcript.

### 7.3 Candidate count

Diagnosis candidates are data-driven.

There is no fixed maximum such as three.

Examples:

- a focused simple case may produce one candidate;
- a complex long-lived full-body state may produce seven or more;
- insufficient information or safety blocking may produce zero candidates with an explicit status.

### 7.4 Organize by concern

For complex analyses, candidates should be grouped by body concern/region rather than displayed as one flat global ranking.

Example:

```text
Head / neck
- Candidate A
- Candidate B

Hip / glute
- Candidate C
- Candidate D
- Candidate E

Knee / ankle
- Candidate F
- Candidate G
```

### 7.5 Candidate explanation

A candidate may present:

- understandable label;
- confidence / fit;
- current impact/severity where appropriate;
- supporting body facts;
- supporting observations;
- supporting evidence/citations;
- counterevidence;
- reasoning summary;
- differential possibilities;
- missing information;
- safety notes.

The UI should explain "why it fits" and, where useful, "what does not fully fit".

### 7.6 User response

Users can independently classify each candidate:

```text
confirmed
unsure
not applicable / currently does not fit
```

Candidates are never deleted because the user did not confirm them.

---

## 8. Diagnosis History

Every DiagnosisAnalysis is historical and immutable after creation.

It records the BodyState revision it analyzed.

The product should let users compare analyses over time:

```text
Previous analysis
- head/neck tendency: moderate
- glute concern: high relevance

Current analysis
- head/neck tendency: mild / improving
- glute concern: weaker evidence
- new right-knee concern
```

Historical diagnosis is the primary high-level way users understand "what the system thought at that time".

There is no separate MedicalRecord required to duplicate this information.

---

## 9. Diagnosis Freshness

The latest analysis can be shown as:

```text
current
may need review
outdated
```

Do not mark analysis outdated just because any BodyState field changed.

Material examples that may require re-analysis:

- new neurological-like complaint;
- major symptom relocation or correction;
- new important observation;
- previously active concern resolves;
- new major concern appears;
- substantial treatment outcome contradicts a previous hypothesis.

---

## 10. Treatment

### 10.1 Current treatment view

The normal user experience shows one current accepted improvement/treatment plan.

Historical revisions are available when needed but not presented as many user-managed documents.

### 10.2 Generation inputs

Treatment should reference:

```text
BodyState revision
DiagnosisAnalysis
user constraints/preferences
relevant evidence
```

### 10.3 Review lifecycle

Suggested states:

```text
active
review recommended
paused
superseded
completed
```

New BodyState changes do not silently rewrite the active plan.

Material changes lead to a review flow.

### 10.4 Safety

Safety concerns can block or pause ordinary treatment recommendations.

---

## 11. Training / Intervention Feedback Loop

Training is not a terminal screen after Diagnosis.

It is part of the feedback loop:

```text
Treatment
  -> exercise / habit intervention
  -> adherence / feedback
  -> outcome
  -> BodyState changes
  -> diagnosis/treatment review when needed
```

Inputs may include:

- completed training;
- skipped training;
- perceived difficulty;
- new discomfort;
- symptom improvement;
- exercise volume;
- self-test change;
- posture-analysis change.

The system may record temporal association but should not claim causation solely because two events occurred in sequence.

---

## 12. Safety Experience

Safety has higher priority than ordinary posture reasoning.

When a materially concerning symptom is reported, BodySense may:

- ask targeted safety questions;
- present appropriate safety guidance;
- block ordinary Diagnosis result generation;
- mark current Treatment for review or pause;
- recommend professional evaluation when warranted by policy.

The UI should make this state clear without mixing it into an ordinary low-priority feature card.

---

## 13. History and Reports

### 13.1 No MedicalRecord aggregate

The product does not require a separate MedicalRecord after every Diagnosis.

History is represented by:

- BodyState change history;
- DiagnosisAnalysis history;
- candidate user-assessment history;
- Treatment revisions;
- training/intervention outcomes.

### 13.2 Optional HealthReport export

If the user later needs to share or download a summary, BodySense can derive a `HealthReport` from selected current/historical data.

The report is an export artifact, not a mutable source of truth.

---

## 14. Current Page Set

The target user-facing navigation can remain small:

1. **Health Workspace** — main long-lived conversation + BodyState.
2. **Diagnosis History** — historical analyses and comparison.
3. **Treatment / Training** — current plan, execution, adherence, feedback.
4. **Body Trends** — body-state history, activity/symptom trends, observations.
5. **Profile / Data** — relatively stable identity/profile, uploads, privacy controls.

These may initially be tabs/subroutes rather than five completely separate product sections.

---

## 15. Protected Runtime Behavior

This product redesign must not accidentally break the existing runtime architecture that already works:

- request/run idempotency;
- Conversation / Turn / Run identity;
- StreamEvent v1;
- Runtime Event Log durability and replay;
- thread projection;
- LangGraph checkpointing;
- ask_user interrupt / resume;
- Go durable business ownership;
- Web projection consumption.

The business model changes; the runtime ownership decision in ADR 0002 remains.

---

## 16. MVP Migration Interpretation

Current repository features are transitional:

```text
health_features
  -> initial workbench projection, later mapped into BodyState concepts

extracted_info
  -> narrow symptom precursor, eventually retired

consultation phase
  -> workflow state, not long-term health state

legacy Diagnosis
  -> migrate to BodyStateRevision-based DiagnosisAnalysis

legacy treatment_plan
  -> migrate to current revisioned Treatment model
```

The migration should preserve existing user-visible consultation streaming while replacing business truth incrementally.

---

## 17. Success Criteria

The feature is successful when:

- the user can keep using one health workspace without managing consultation documents;
- direct conversation and right-panel edits update the same durable BodyState;
- corrected information no longer pollutes current AI context;
- true historical changes remain visible rather than being overwritten;
- the system can represent improvement, resolution, recurrence, and new concerns;
- Diagnosis can analyze all relevant concerns without a fixed candidate limit;
- historical Diagnosis remains tied to the exact state used at the time;
- unconfirmed candidates remain available in historical analysis;
- Treatment is reviewable as BodyState changes;
- training outcomes feed back into longitudinal state;
- a multi-year conversation remains context-efficient;
- users can understand their current body state and how it changed without learning internal data/versioning concepts.
