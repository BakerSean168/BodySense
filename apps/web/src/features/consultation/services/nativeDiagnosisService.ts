import {
  createNativeDiagnosisAnalysisRequest,
  decideNativeDiagnosisProposal,
  dismissNativeDiagnosisReviewPrompt,
  getNativeDiagnosis,
  getNativeDiagnosisRun,
  listNativeDiagnosisRuns,
  settleNativeDiagnosisRun,
  startNativeDiagnosisRun,
} from "@/generated/api/bodysense";
import { NativeDiagnosisAnalysisRequestCreate } from "@/generated/api/model/nativeDiagnosisAnalysisRequestCreate.zod";
import { NativeDiagnosisDecisionRequest } from "@/generated/api/model/nativeDiagnosisDecisionRequest.zod";
import { NativeDiagnosisReviewPromptDismissRequest } from "@/generated/api/model/nativeDiagnosisReviewPromptDismissRequest.zod";
import { NativeDiagnosisRunStartRequest } from "@/generated/api/model/nativeDiagnosisRunStartRequest.zod";
import { NativeDiagnosisSettlementRequest } from "@/generated/api/model/nativeDiagnosisSettlementRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// Response shapes for view models; features import them from this adapter only.
export type { NativeDiagnosisAnalysisRequestOutput as NativeDiagnosisAnalysisRequest } from "@/generated/api/model/nativeDiagnosisAnalysisRequest.zod";
export type { NativeDiagnosisConditionOutput as NativeDiagnosisCondition } from "@/generated/api/model/nativeDiagnosisCondition.zod";
export type { NativeDiagnosisDecisionReceiptOutput as NativeDiagnosisDecisionReceipt } from "@/generated/api/model/nativeDiagnosisDecisionReceipt.zod";
export type { NativeDiagnosisProjectionOutput as NativeDiagnosisProjection } from "@/generated/api/model/nativeDiagnosisProjection.zod";
export type { NativeDiagnosisReviewPromptDismissalOutput as NativeDiagnosisReviewPromptDismissal } from "@/generated/api/model/nativeDiagnosisReviewPromptDismissal.zod";
export type { NativeDiagnosisRunProjectionOutput as NativeDiagnosisRun } from "@/generated/api/model/nativeDiagnosisRunProjection.zod";
export type { NativeDiagnosisRunProposalOutput as NativeDiagnosisProposal } from "@/generated/api/model/nativeDiagnosisRunProposal.zod";
export type { NativeDiagnosisSettlementReceiptOutput as NativeDiagnosisSettlementReceipt } from "@/generated/api/model/nativeDiagnosisSettlementReceipt.zod";

export type NativeDiagnosisDecisionCommand = Parameters<
  typeof decideNativeDiagnosisProposal
>[1];
export type NativeDiagnosisSettlementCommand = Parameters<
  typeof settleNativeDiagnosisRun
>[1];
export type NativeDiagnosisStartCommand = Parameters<
  typeof startNativeDiagnosisRun
>[0];
export type NativeDiagnosisReviewPromptDismissCommand = Parameters<
  typeof dismissNativeDiagnosisReviewPrompt
>[1];

/** Diagnosis History reads at most this many recent formal runs. */
export const DIAGNOSIS_HISTORY_RUNS = 10;

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
  /** Recent settled runs with their decisions, newest first (History). */
  history: async () => {
    const page = await withOpenApiError(() =>
      listNativeDiagnosisRuns(
        { limit: DIAGNOSIS_HISTORY_RUNS },
        undefined,
        openApiAuthFetch,
      ),
    );
    return Promise.all(
      page.runs
        .filter((run) => run.state !== "open")
        .map((run) =>
          withOpenApiError(() =>
            getNativeDiagnosisRun(run.runId, undefined, openApiAuthFetch),
          ),
        ),
    );
  },
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
  /** Binds the user's own chat message, verbatim, as the request statement. */
  bindRequest: (messageId: string) =>
    withOpenApiError(() =>
      createNativeDiagnosisAnalysisRequest(
        NativeDiagnosisAnalysisRequestCreate.parse({ messageId }),
        undefined,
        openApiAuthFetch,
      ),
    ),
  /** Starts a formal run; its proposals stay proposals until settled. */
  start: (command: NativeDiagnosisStartCommand) =>
    withOpenApiError(() =>
      startNativeDiagnosisRun(
        NativeDiagnosisRunStartRequest.parse(command),
        undefined,
        openApiAuthFetch,
      ),
    ),
  /** §3.6 "暂不处理": a UI record only; the Condition never changes. */
  dismissReviewPrompt: (
    conditionId: string,
    command: NativeDiagnosisReviewPromptDismissCommand,
  ) =>
    withOpenApiError(() =>
      dismissNativeDiagnosisReviewPrompt(
        conditionId,
        NativeDiagnosisReviewPromptDismissRequest.parse(command),
        undefined,
        openApiAuthFetch,
      ),
    ),
};
