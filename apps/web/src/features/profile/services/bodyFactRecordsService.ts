import {
  correctBodyFactRecord,
  withdrawBodyFactRecord,
  listBodyFactRecords,
  recordBodyFacts,
} from "@/generated/api/bodysense";
import {
  RecordBodyFactsRequest,
  type RecordBodyFactsRequest as RecordBodyFactsInput,
} from "@/generated/api/model/recordBodyFactsRequest.zod";
import { CorrectBodyFactRecordRequest } from "@/generated/api/model/correctBodyFactRecordRequest.zod";
import { WithdrawBodyFactRecordRequest } from "@/generated/api/model/withdrawBodyFactRecordRequest.zod";
import { ListBodyFactRecordsParams } from "@/generated/api/model/listBodyFactRecordsParams.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// Use the generated public contract as the sole browser DTO authority.
// Records are durable BodyState assertions, not the old Profile summary slots.
export type BodyFactRecordsPage = Awaited<
  ReturnType<typeof listBodyFactRecords>
>;
export type BodyFactCommitReceipt = Awaited<ReturnType<typeof recordBodyFacts>>;
export type BodyFactRecordInput = RecordBodyFactsInput;
export type BodyFactRecord = BodyFactRecordsPage["facts"][number];
export type BodyFactCorrectionInput = Parameters<
  typeof correctBodyFactRecord
>[1];
export type BodyFactWithdrawalInput = Parameters<
  typeof withdrawBodyFactRecord
>[1];

export const bodyFactRecordsService = {
  correct: (
    id: string,
    input: BodyFactCorrectionInput,
  ): Promise<BodyFactCommitReceipt> =>
    withOpenApiError(() =>
      correctBodyFactRecord(
        id,
        CorrectBodyFactRecordRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  withdraw: (
    id: string,
    input: BodyFactWithdrawalInput,
  ): Promise<BodyFactCommitReceipt> =>
    withOpenApiError(() =>
      withdrawBodyFactRecord(
        id,
        WithdrawBodyFactRecordRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  list: (beforeRevision?: number): Promise<BodyFactRecordsPage> =>
    withOpenApiError(() =>
      listBodyFactRecords(
        ListBodyFactRecordsParams.parse(
          beforeRevision === undefined
            ? {}
            : { before_revision: beforeRevision },
        ),
        undefined,
        openApiAuthFetch,
      ),
    ),

  record: (input: RecordBodyFactsInput): Promise<BodyFactCommitReceipt> =>
    withOpenApiError(() =>
      // Generated output schemas verify the server's persisted commit receipt.
      // This strict request schema rejects untrusted source/owner/clinical fields.
      recordBodyFacts(
        RecordBodyFactsRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
};
