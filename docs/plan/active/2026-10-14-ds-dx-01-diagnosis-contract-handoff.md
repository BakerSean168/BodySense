# DS-DX-01 · Diagnosis V2 read-model and command boundary

Status: **TRACK A CONTRACT CANDIDATE / NO V2 DATABASE OR API READY** (2026-10-14).
Authority: [Diagnosis V2](../../product/domain/diagnosis.md), [acceptance](../../product/acceptance.md), [open questions](../../product/open-questions.md), [Track A](./2026-10-13-v2-domain-schema.md), [gate register](./v2-handoff-state.json).
This document maps approved decisions to engineering and Penpot inputs. It does **not** create a second product authority or imply any of the target DTOs, tables or commands are deployed.

## 1. Identity, ownership and legacy gap

- One durable owner-bound `Diagnosis` per user. `ConditionEntry` is a specific named medical disease/syndrome adopted by the user, not a symptom, BodyState Episode, medical certainty or a second health-fact store. Keep distinct condition IDs when concepts differ even if they cite the same Episode.
- `ConditionEntry.disposition` is only `adopted | inactive`; it does not indicate medical activity or symptom progression. No general condition lifecycle. `assessment.level` is an optional AI `suspected | likely`, never a clinical certification or a user-controlled selection.
- `DiagnosisEvidenceLink` points to versioned BodyFact/Observation/Episode/document evidence, and `SourceClaimRef` preserves precise original clinical wording and its assessment-time validity. Current source availability is queried from the owning domain without rewriting the historical assessment.
- Actual legacy `apps/api/internal/model/diagnosis_analysis.go` and `apps/api/internal/service/diagnosis_application_service.go` are not the target singleton / append-only analysis contracts; existing AI hypotheses and old confidence, lifecycle, title-only or source semantics must be migrated or retired after concrete consumer inventory.

## 2. Target query projections (descriptive contract, not generated API)

```ts
type CurrentDiagnosisView = {
  conditions: Array<{
    conditionId: string;
    medicalName: string;
    conceptRef?: { system: string; code: string; display?: string };
    disposition: "adopted" | "inactive";
    qualifiers?: { regionIds?: string[]; laterality?: string; notes?: string };
    assessment?: { level: "suspected" | "likely"; rationale: string };
    causeAnalysis: { summary: string; rationale: string };
    informationGaps: string[];
    handlingPlan: {
      summary: string;
      actions: string[];
      cautionNotes: string[];
    };
    sourceClaims: Array<{
      claimId: string;
      statement: string;
      sourceKind:
        | "user_suspicion"
        | "agent_analysis"
        | "user_reported_clinical"
        | "clinical_document";
      sourceVersion?: string;
      validityAtAssessment:
        "current" | "corrected" | "withdrawn" | "ineligible";
      currentSourceAvailability:
        "original_available" | "original_unavailable" | "erased" | "unknown";
    }>;
    evidenceLinks: Array<{
      evidenceType:
        | "body_fact"
        | "body_observation"
        | "symptom_episode"
        | "document_evidence";
      evidenceId: string;
      evidenceVersion?: string;
      role: "supporting" | "counter" | "context";
      rationale: string;
    }>;
    reviewRecommended: boolean; // derived hint, not an automatic diagnosis edit
  }>;
  adoptedCount: number; // derived from adopted condition entries, not a second aggregate
  analysis?: {
    runId: string;
    state: "open" | "committed" | "closed_without_change";
    pendingProposal?: {
      proposalId: string;
      changeKind: "create" | "update" | "set_disposition" | "update_relation";
      targetConditionId?: string;
      contentFingerprint: string;
      changeSet?: Array<{ field: string; before: unknown; after: unknown }>;
      proposedDisposition?: "adopted" | "inactive";
      dispositionReason?:
        | "user_withdrawal"
        | "superseded_by_new_conclusion"
        | "no_longer_applicable"
        | "readopted";
      relationChange?: {
        fromConditionId: string;
        toConditionId: string;
        kind: "differential_with" | "supersedes" | "coexisting";
      };
      response: "pending" | "decided_uncommitted" | "superseded" | "expired";
    };
    visibility: "ready" | "stale" | "reconnecting" | "unavailable";
  };
  visibility:
    "loading" | "empty" | "ready" | "partial" | "denied" | "unavailable";
};
```

UI must distinguish `assessment`, user adoption, clinical claim and current source eligibility. Display exact proposal changes rather than a vague accept/decline. A paused Ask User card may preserve a decision without committing Diagnosis; closing the window never means reject/withdraw.

## 3. Target commands and receipts (no guessed transport routes)

| Conceptual command                               | Required checks                                                                                                                                    | Persisted result or refusal                                                                                                                |
| ------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `BeginConditionAnalysis`                         | Authenticated user explicitly requests formal analysis; allowed scope, real evidence IDs and known source revisions                                | Go-created `ConditionAnalysisRun` and stable original Run identity; no Diagnosis mutation for AI preliminary speculation.                  |
| `RecordProposalDecision`                         | Exact run/proposal/fingerprint/toolCallId, owner, evidence versions, pending status                                                                | Append-only accept/reject decision bound to **that exact proposal**. Idempotent replay; no partial ConditionEntry update.                  |
| `SupplementRunEvidence`                          | Source first admitted by its owning BodyState/document domain, then appended to same Run                                                           | Immutable new evidence/hypothesis/proposal events; substantive proposal change supersedes old ID, not in-place rewrite.                    |
| `CommitAnalysisDecisions`                        | All proposals settled; every decided proposal has post-decision evidence recheck; allowed Safety, source admission and `expectedDiagnosisRevision` | One atomic patch and at most one `DiagnosisRevision`; no-op `closed_without_change`; stale/conflicting/safety-blocked input cannot commit. |
| `CorrectConditionMetadata` / `WithdrawCondition` | Exact existing owner-bound condition, explicit user confirmation and current revision                                                              | `ConditionDirectEdit` with bounded metadata-only change or `inactive` + user-withdrawal reason. No independent symptom-course mutation.    |

Go owns all persistent authority, idempotency keys, revision, decision validity and commit receipts. Python can propose, append analysis events and request original Ask User tools but cannot silently commit diagnosis, assert clinician certainty, mutate BodyHypothesis or infer clinical thresholds.

## 4. Required negative acceptance gates

1. Two users cannot read/decide/withdraw one another's condition IDs; same request replay returns one decision/receipt, changed fingerprint or stale evidence is rejected.
2. Multiple pending proposals are confirmed serially. Partial decisions do not update Diagnosis; supplemental relevant evidence triggers selective recheck/reconfirmation before the **single** atomic commit.
3. Serious analysis safety blocking terminates the Run with **zero** DiagnosisRevision even if some proposals were already accepted. Later tool answers and commits reject. Ordinary Trainer restrictions do not automatically become an analysis safety block.
4. A report containing only measurements cannot be promoted to `clinical_document` diagnosis; a user's report of a physician's suggestion preserves uncertainty and is not converted into clinical certainty.
5. A new fact/source withdrawal can raise `reviewRecommended` but cannot rewrite earlier adopted entries; BodyState symptom `ended` never automatically inactivates ConditionEntry.
6. Only explicitly authorized metadata corrections may use direct edit; changing the medical concept, reason, evidence or handling advice requires a formal analysis/decision cycle.
7. No hard-coded Q-17 proposal TTL, no Q-19 UI sign-off before editable Penpot review. Insufficient evidence may close an analysis without inventing a medical-name proposal.

## 5. Implementation handoff and limits

Track C: inventory old Diagnosis and sharing consumers; define additive owner-bound aggregate/revision, append-only Run/Proposal/Decision/Event tables, event replay and idempotency; pin exact OpenAPI/Go/TS/Zod generation and Go-owned authorization/transaction boundaries. Prove native PostgreSQL 18 migration and rollback/replay, owner isolation, stale revisions, duplicate decisions, safety race and all-rejected/no-op analysis. Never rewrite baseline migration or fabricate source admission. Track B: map named source/evidence/disposition and serial proposed-change cards to the **existing** Penpot Product page, including light/dark, desktop/390px and Q-19 discussion. Track D: verify exact-SHA, cross-domain evidence and real browser interaction tests.

Current evidence is **contract review only**, not a production API or migration. `diagnosis.view_model`, `database`, `engineering` and `integration` must remain not-ready until canonical contracts and independent tests exist. Q-17 and Q-19 remain unresolved by their respective owners.
