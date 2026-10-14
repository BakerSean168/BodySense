import { z } from "zod";
import {
  id,
  text,
  instant,
  revision,
  evidenceRole,
  evidenceSource,
  sourceRef,
  resourceSchema,
  capability,
  mutationBinding,
  exactConsent,
  commitReceipt,
} from "./common";

export const diagnosisAssessment = z.strictObject({
  level: z.enum(["suspected", "likely"]),
  rationale: text,
  informationGaps: z.array(text),
  differentialNotes: z.array(text),
  basedOnRunId: id,
});
export const diagnosisEvidenceLink = z.strictObject({
  evidenceType: z.enum([
    "body_fact",
    "body_observation",
    "symptom_episode",
    "document_evidence",
  ]),
  evidenceId: id,
  evidenceVersion: revision.optional(),
  role: evidenceRole,
  rationale: text,
  analyzedByRunId: id,
  source: evidenceSource,
});
export const clinicalSourceClaim = z
  .strictObject({
    claimId: id,
    sourceKind: z.enum([
      "user_suspicion",
      "agent_analysis",
      "user_reported_clinical",
      "clinical_document",
    ]),
    sourceId: id,
    sourceVersion: revision.optional(),
    statement: text,
    assertedAt: instant.optional(),
    recordedAt: instant,
    validityAtAssessment: z.enum([
      "current",
      "corrected",
      "withdrawn",
      "ineligible",
    ]),
    currentSource: evidenceSource,
  })
  .refine(
    (v) =>
      v.sourceKind !== "clinical_document" ||
      (v.currentSource.attribution === "document" &&
        v.currentSource.historicalAdmission === "admissible"),
    "Clinical document claims require admitted document attribution",
  );
export const conditionContent = z.strictObject({
  medicalName: text,
  conceptRef: z
    .strictObject({ system: text, code: text, display: text.optional() })
    .optional(),
  qualifiers: z
    .strictObject({
      regionIds: z.array(id).optional(),
      laterality: text.optional(),
      notes: text.optional(),
    })
    .optional(),
  assessment: diagnosisAssessment.optional(),
  sourceClaims: z.array(clinicalSourceClaim),
  evidenceLinks: z.array(diagnosisEvidenceLink),
  causeAnalysis: z.strictObject({ summary: text, rationale: text }),
  informationGaps: z.array(text),
  handlingPlan: z.strictObject({
    summary: text,
    actions: z.array(text),
    cautionNotes: z.array(text),
  }),
});
export const conditionEntry = conditionContent.extend({
  conditionId: id,
  disposition: z.enum(["adopted", "inactive"]),
  latestDecisionId: id,
  createdAt: instant,
  updatedAt: instant,
});
const conditionRelation = z.strictObject({
  fromConditionId: id,
  toConditionId: id,
  kind: z.enum(["differential_with", "supersedes", "coexisting"]),
  basisEvidenceRefs: z.array(sourceRef),
});
export const reviewRecommendation = z.strictObject({
  evidenceSetKey: id,
  summary: text,
  evidenceRefs: z.array(sourceRef),
  canRequestReview: capability,
});
export const diagnosisData = z
  .strictObject({
    diagnosisId: id,
    adoptedConditions: z.array(conditionEntry),
    adoptedConditionCount: z.number().int().nonnegative(),
    selectedConditionId: id.optional(),
    relations: z.array(conditionRelation),
    reviewRecommendations: z.array(reviewRecommendation),
    canRequestAnalysis: capability,
  })
  .superRefine((v, ctx) => {
    if (
      v.adoptedConditions.some((c) => c.disposition !== "adopted") ||
      v.adoptedConditionCount !== v.adoptedConditions.length
    )
      ctx.addIssue({
        code: "custom",
        message:
          "Dock count and current selector include adopted conditions only; this query returns the whole current set",
      });
    if (
      new Set(v.adoptedConditions.map((c) => c.conditionId)).size !==
      v.adoptedConditions.length
    )
      ctx.addIssue({
        code: "custom",
        message: "Condition identities must be unique",
      });
    if (
      v.selectedConditionId &&
      !v.adoptedConditions.some((c) => c.conditionId === v.selectedConditionId)
    )
      ctx.addIssue({
        code: "custom",
        message: "Selection must identify a current condition",
      });
  });
export const diagnosisVM = resourceSchema(diagnosisData);
export const diagnosisHistoryVM = resourceSchema(
  z.strictObject({
    entries: z.array(
      z.strictObject({
        at: instant,
        label: text,
        condition: conditionEntry,
        reason: text,
        decisionId: id,
      }),
    ),
    nextCursor: id.nullable(),
  }),
);
const proposalBase = {
  proposalId: id,
  runId: id,
  contentFingerprint: id,
  evidenceRefs: z.array(sourceRef),
  createdAt: instant,
};
const displayChange = z.strictObject({
  fieldLabel: text,
  before: z.string().nullable(),
  after: z.string().nullable(),
});
export const conditionProposalCard = z.discriminatedUnion("changeKind", [
  z.strictObject({
    ...proposalBase,
    changeKind: z.literal("create"),
    content: conditionContent,
  }),
  z.strictObject({
    ...proposalBase,
    changeKind: z.literal("update"),
    targetConditionId: id,
    content: conditionContent,
    changes: z.array(displayChange).min(1),
  }),
  z.strictObject({
    ...proposalBase,
    changeKind: z.literal("set_disposition"),
    targetConditionId: id,
    medicalName: text,
    from: z.enum(["adopted", "inactive"]),
    to: z.enum(["adopted", "inactive"]),
    reason: z.enum([
      "user_withdrawal",
      "superseded_by_new_conclusion",
      "no_longer_applicable",
      "readopted",
    ]),
  }),
  z.strictObject({
    ...proposalBase,
    changeKind: z.literal("update_relation"),
    relation: conditionRelation,
    changes: z.array(displayChange).min(1),
  }),
]);
const proposalDecision = z.strictObject({
  decisionId: id,
  proposalId: id,
  contentFingerprint: id,
  decision: z.enum(["accept", "reject"]),
  decidedAt: instant,
  recheck: z.enum(["not_needed", "pending", "unaffected", "affected"]),
});
const batchBase = {
  runId: id,
  basedOnDiagnosisRevision: revision,
  decisions: z.array(proposalDecision),
};
export const diagnosisConfirmationData = z.discriminatedUnion("state", [
  z.strictObject({
    ...batchBase,
    state: z.literal("awaiting_decision"),
    current: conditionProposalCard,
    position: z.number().int().positive(),
    total: z.number().int().positive(),
    consent: exactConsent,
    diagnosisChanged: z.literal(false),
  }),
  z.strictObject({
    ...batchBase,
    state: z.literal("rechecking"),
    affectedProposalIds: z.array(id),
    diagnosisChanged: z.literal(false),
  }),
  z.strictObject({
    ...batchBase,
    state: z.literal("superseded"),
    oldProposalId: id,
    replacementProposalId: id,
    explanation: text,
    diagnosisChanged: z.literal(false),
  }),
  z.strictObject({
    ...batchBase,
    state: z.literal("expired"),
    reason: z.enum(["timeout", "base_revision_changed", "run_closed"]),
    diagnosisChanged: z.literal(false),
  }),
  z.strictObject({
    ...batchBase,
    state: z.literal("committing"),
    diagnosisChanged: z.literal(false),
  }),
  z.strictObject({
    ...batchBase,
    state: z.literal("committed"),
    receipt: commitReceipt.extend({ domain: z.literal("diagnosis") }),
  }),
  z.strictObject({
    ...batchBase,
    state: z.literal("closed_without_change"),
    reason: z.enum(["insufficient_evidence", "no_accepted_change"]),
    explanation: text,
    diagnosisChanged: z.literal(false),
  }),
  z.strictObject({
    ...batchBase,
    state: z.literal("safety_blocked"),
    explanation: text,
    restrictionIds: z.array(id).min(1),
    canResumeOldRun: z.literal(false),
    diagnosisChanged: z.literal(false),
  }),
]);
export const diagnosisConfirmationVM = resourceSchema(
  diagnosisConfirmationData.superRefine((v, ctx) => {
    if (
      v.state === "awaiting_decision" &&
      (v.current.runId !== v.runId ||
        v.consent.runId !== v.runId ||
        v.consent.contentFingerprint !== v.current.contentFingerprint ||
        v.position > v.total)
    )
      ctx.addIssue({
        code: "custom",
        message: "Confirmation must bind the exact current proposal and Run",
      });
  }),
);
export const diagnosisCommand = z.discriminatedUnion("operation", [
  z.strictObject({
    operation: z.literal("diagnosis.request_analysis"),
    idempotencyKey: id,
    trigger: z.enum(["new_analysis", "partial_update", "full_reanalysis"]),
    conditionIds: z.array(id),
    evidenceRefs: z.array(sourceRef),
    requestMessageId: id,
  }),
  z.strictObject({
    operation: z.literal("diagnosis.decide_proposal"),
    ...mutationBinding,
    proposalId: id,
    consent: exactConsent,
    decision: z.enum(["accept", "reject"]),
  }),
  z.strictObject({
    operation: z.literal("diagnosis.finish_remaining"),
    ...mutationBinding,
    runId: id,
    consent: exactConsent,
  }),
  z.strictObject({
    operation: z.literal("diagnosis.withdraw"),
    ...mutationBinding,
    conditionId: id,
    reason: text,
    consent: exactConsent,
  }),
  z.strictObject({
    operation: z.literal("diagnosis.request_metadata_correction"),
    ...mutationBinding,
    conditionId: id,
    requestText: text,
    consent: exactConsent,
  }),
]);
export type DiagnosisVM = z.infer<typeof diagnosisVM>;
export type DiagnosisConfirmationVM = z.infer<typeof diagnosisConfirmationVM>;
export type DiagnosisCommand = z.infer<typeof diagnosisCommand>;
