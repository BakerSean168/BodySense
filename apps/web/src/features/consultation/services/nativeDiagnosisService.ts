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
