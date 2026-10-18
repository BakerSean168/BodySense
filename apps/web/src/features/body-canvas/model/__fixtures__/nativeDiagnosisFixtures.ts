import type { NativeDiagnosisConditionOutput } from "@/generated/api/model/nativeDiagnosisCondition.zod";
import type { NativeDiagnosisDecisionReceiptOutput } from "@/generated/api/model/nativeDiagnosisDecisionReceipt.zod";
import type { NativeDiagnosisProjectionOutput } from "@/generated/api/model/nativeDiagnosisProjection.zod";
import type { NativeDiagnosisRunProjectionOutput } from "@/generated/api/model/nativeDiagnosisRunProjection.zod";
import type { NativeDiagnosisRunProposalOutput } from "@/generated/api/model/nativeDiagnosisRunProposal.zod";
import type { NativeDiagnosisSettlementReceiptOutput } from "@/generated/api/model/nativeDiagnosisSettlementReceipt.zod";

export const USER_ID = "00000000-0000-4000-8000-000000000001";
export const RUN_ID = "00000000-0000-4000-8000-0000000000a1";
export const PROPOSAL_A = "00000000-0000-4000-8000-0000000000b1";
export const PROPOSAL_B = "00000000-0000-4000-8000-0000000000b2";
export const CONDITION_ID = "00000000-0000-4000-8000-0000000000c1";
export const FINGERPRINT_A = "a".repeat(64);
export const FINGERPRINT_B = "b".repeat(64);
const EVIDENCE_ID = "00000000-0000-4000-8000-0000000000e1";
const AT = "2026-10-18T03:00:00Z";

let counter = 0;
export function uuid(): string {
  counter += 1;
  return `00000000-0000-4000-9000-${counter.toString(16).padStart(12, "0")}`;
}

export function makeCondition(
  overrides: Partial<NativeDiagnosisConditionOutput> = {},
): NativeDiagnosisConditionOutput {
  return {
    conditionId: CONDITION_ID,
    medicalName: "肩峰下撞击",
    assessment: {
      level: "likely",
      rationale: "抬臂疼痛与夜间痛",
      informationGaps: [],
      differentialNotes: ["需排除肩袖撕裂"],
      basedOnRunId: RUN_ID,
    },
    sourceClaimRefs: [],
    evidenceLinks: [
      {
        evidenceType: "body_fact",
        evidenceId: EVIDENCE_ID,
        evidenceVersion: "1",
        role: "supporting",
        rationale: "用户描述",
        analyzedByRunId: RUN_ID,
      },
    ],
    causeAnalysis: { summary: "肩胛控制不足", rationale: "久坐与抬臂代偿" },
    informationGaps: ["是否做过影像检查"],
    handlingPlan: { summary: "", actions: [], cautionNotes: [] },
    disposition: "adopted",
    latestDecisionId: uuid(),
    committedRevision: 2,
    createdAt: AT,
    updatedAt: AT,
    ...overrides,
  };
}

export function makeDiagnosis(
  overrides: Partial<NativeDiagnosisProjectionOutput> = {},
): NativeDiagnosisProjectionOutput {
  return {
    userId: USER_ID,
    currentRevision: 2,
    conditionEntries: [],
    ...overrides,
  };
}

export function makeProposal(
  overrides: Partial<NativeDiagnosisRunProposalOutput> = {},
): NativeDiagnosisRunProposalOutput {
  return {
    proposalId: PROPOSAL_A,
    runId: RUN_ID,
    changeKind: "create",
    content: {
      proposedEntry: {
        medicalName: "上交叉综合征",
        assessment: { level: "suspected" },
        causeAnalysis: { summary: "长期低头伏案" },
      },
    },
    contentFingerprint: FINGERPRINT_A,
    baseRevision: 2,
    evidenceRefs: [],
    counterEvidenceRefs: [],
    createdAt: AT,
    state: "pending",
    ...overrides,
  };
}

export function makeRun(
  overrides: Partial<NativeDiagnosisRunProjectionOutput> = {},
): NativeDiagnosisRunProjectionOutput {
  return {
    runId: RUN_ID,
    trigger: "new_analysis",
    basedOnBodyStateRevision: 5,
    basedOnDiagnosisRevision: 2,
    startedAt: AT,
    state: "open",
    currentDiagnosisRevision: 2,
    requestMessageId: uuid(),
    requestEvidenceId: uuid(),
    requestSourceVersion: 1,
    scope: { conditionIds: [], evidenceRefs: [], description: "全身" },
    inputEvidenceRefs: [],
    agentHypotheses: [],
    proposals: [
      makeProposal(),
      makeProposal({
        proposalId: PROPOSAL_B,
        contentFingerprint: FINGERPRINT_B,
        content: { proposedEntry: { medicalName: "胸椎活动受限" } },
      }),
    ],
    events: [],
    ...overrides,
  };
}

export function makeDecisionReceipt(
  overrides: Partial<NativeDiagnosisDecisionReceiptOutput> = {},
): NativeDiagnosisDecisionReceiptOutput {
  return {
    decisionId: uuid(),
    proposalId: PROPOSAL_A,
    runId: RUN_ID,
    contentFingerprint: FINGERPRINT_A,
    decidedEvidenceRefs: [],
    decision: "accept",
    expectedRevision: 2,
    toolCallId: "web:diagnosis-window:test",
    idempotencyKey: "key-a",
    receiptId: uuid(),
    decidedAt: AT,
    ...overrides,
  };
}

export function makeSettlementReceipt(
  overrides: Partial<NativeDiagnosisSettlementReceiptOutput> = {},
): NativeDiagnosisSettlementReceiptOutput {
  return {
    settlementId: uuid(),
    runId: RUN_ID,
    expectedRevision: 2,
    revision: 3,
    outcome: "committed",
    diagnosisRevisionId: uuid(),
    conditionIds: [CONDITION_ID],
    decisionIds: [],
    bodyStateRevision: 5,
    idempotencyKey: "settle-key",
    toolCallId: "web:diagnosis-window:test",
    receiptId: uuid(),
    settledAt: AT,
    ...overrides,
  };
}
