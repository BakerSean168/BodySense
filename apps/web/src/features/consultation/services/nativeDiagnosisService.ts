import {
  decideNativeDiagnosisProposal,
  getNativeDiagnosis,
  getNativeDiagnosisRun,
  listNativeDiagnosisRuns,
  settleNativeDiagnosisRun,
} from "@/generated/api/bodysense";
import { NativeDiagnosisDecisionRequest } from "@/generated/api/model/nativeDiagnosisDecisionRequest.zod";
import { NativeDiagnosisSettlementRequest } from "@/generated/api/model/nativeDiagnosisSettlementRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// Response shapes for view models; features import them from this adapter only.
export type { NativeDiagnosisConditionOutput as NativeDiagnosisCondition } from "@/generated/api/model/nativeDiagnosisCondition.zod";
export type { NativeDiagnosisDecisionReceiptOutput as NativeDiagnosisDecisionReceipt } from "@/generated/api/model/nativeDiagnosisDecisionReceipt.zod";
export type { NativeDiagnosisProjectionOutput as NativeDiagnosisProjection } from "@/generated/api/model/nativeDiagnosisProjection.zod";
export type { NativeDiagnosisRunProjectionOutput as NativeDiagnosisRun } from "@/generated/api/model/nativeDiagnosisRunProjection.zod";
export type { NativeDiagnosisRunProposalOutput as NativeDiagnosisProposal } from "@/generated/api/model/nativeDiagnosisRunProposal.zod";
export type { NativeDiagnosisSettlementReceiptOutput as NativeDiagnosisSettlementReceipt } from "@/generated/api/model/nativeDiagnosisSettlementReceipt.zod";

export type NativeDiagnosisDecisionCommand = Parameters<
  typeof decideNativeDiagnosisProposal
>[1];
export type NativeDiagnosisSettlementCommand = Parameters<
  typeof settleNativeDiagnosisRun
>[1];

/**
 * V2 Diagnosis decision API. The generated client validates every response
 * (projection, run, decision receipt, settlement receipt) against the public
 * contract; requests are validated here before they leave the browser.
 * A decision receipt records a user choice only; only a settlement receipt
 * with outcome `committed` adopts anything into the Diagnosis.
 */
export const nativeDiagnosisService = {
  current: () =>
    withOpenApiError(() => getNativeDiagnosis(undefined, openApiAuthFetch)),
  latestRun: async () => {
    const page = await withOpenApiError(() =>
      listNativeDiagnosisRuns({ limit: 1 }, undefined, openApiAuthFetch),
    );
    return page.runs[0] ?? null;
  },
  run: (runId: string) =>
    withOpenApiError(() =>
      getNativeDiagnosisRun(runId, undefined, openApiAuthFetch),
    ),
  decide: (runId: string, command: NativeDiagnosisDecisionCommand) =>
    withOpenApiError(() =>
      decideNativeDiagnosisProposal(
        runId,
        NativeDiagnosisDecisionRequest.parse(command),
        undefined,
        openApiAuthFetch,
      ),
    ),
  settle: (runId: string, command: NativeDiagnosisSettlementCommand) =>
    withOpenApiError(() =>
      settleNativeDiagnosisRun(
        runId,
        NativeDiagnosisSettlementRequest.parse(command),
        undefined,
        openApiAuthFetch,
      ),
    ),
};
