import {
  recordBodyHypothesis,
  correctBodyHypothesisRecord,
  withdrawBodyHypothesisRecord,
  listDerivedConcernGroups,
} from "@/generated/api/bodysense";
import { RecordBodyHypothesisRequest } from "@/generated/api/model/recordBodyHypothesisRequest.zod";
import { CorrectBodyHypothesisRequest } from "@/generated/api/model/correctBodyHypothesisRequest.zod";
import { WithdrawBodyHypothesisRequest } from "@/generated/api/model/withdrawBodyHypothesisRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

export type HypothesisInput = Parameters<typeof recordBodyHypothesis>[0];
export type HypothesisCorrection = Parameters<
  typeof correctBodyHypothesisRecord
>[1];
export type HypothesisWithdrawal = Parameters<
  typeof withdrawBodyHypothesisRecord
>[1];
// Generated clients validate receipts and read responses, as well as these requests.
export const bodyHypothesisService = {
  record: (input: HypothesisInput) =>
    withOpenApiError(() =>
      recordBodyHypothesis(
        RecordBodyHypothesisRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  correct: (id: string, input: HypothesisCorrection) =>
    withOpenApiError(() =>
      correctBodyHypothesisRecord(
        id,
        CorrectBodyHypothesisRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  withdraw: (id: string, input: HypothesisWithdrawal) =>
    withOpenApiError(() =>
      withdrawBodyHypothesisRecord(
        id,
        WithdrawBodyHypothesisRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  groups: () =>
    withOpenApiError(() =>
      listDerivedConcernGroups(undefined, openApiAuthFetch),
    ),
};
