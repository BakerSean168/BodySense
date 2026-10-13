import {
  listBodyFactRecords,
  recordBodyFacts,
} from "@/generated/api/bodysense";
import {
  RecordBodyFactsRequest,
  type RecordBodyFactsRequest as RecordBodyFactsInput,
} from "@/generated/api/model/recordBodyFactsRequest.zod";
import { ListBodyFactRecordsParams } from "@/generated/api/model/listBodyFactRecordsParams.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// Use the generated public contract as the sole browser DTO authority.
// Records are durable BodyState assertions, not the old Profile summary slots.
export type BodyFactRecordsPage = Awaited<
  ReturnType<typeof listBodyFactRecords>
>;
export type BodyFactCommitReceipt = Awaited<ReturnType<typeof recordBodyFacts>>;

export const bodyFactRecordsService = {
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
