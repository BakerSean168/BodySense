import {
  listBodyObservationRecords,
  recordBodyObservation,
} from "@/generated/api/bodysense";
import { ListBodyObservationRecordsParams } from "@/generated/api/model/listBodyObservationRecordsParams.zod";
import {
  RecordBodyObservationRequest,
  type RecordBodyObservationRequest as BodyObservationInput,
} from "@/generated/api/model/recordBodyObservationRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// One source-of-truth: generated OpenAPI/TS/Zod for direct-user observations.
// This adapter neither guesses a normalized clinical measurement nor submits
// device/document provenance. Track B independently owns final visual design.
export type BodyObservationCommand = BodyObservationInput;
export type BodyObservationCommit = Awaited<
  ReturnType<typeof recordBodyObservation>
>;
export type BodyObservationPage = Awaited<
  ReturnType<typeof listBodyObservationRecords>
>;

export const bodyObservationService = {
  record: (input: BodyObservationInput): Promise<BodyObservationCommit> =>
    withOpenApiError(() =>
      recordBodyObservation(
        RecordBodyObservationRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  list: (beforeRevision?: number): Promise<BodyObservationPage> =>
    withOpenApiError(() =>
      listBodyObservationRecords(
        ListBodyObservationRecordsParams.parse(
          beforeRevision === undefined
            ? {}
            : { before_revision: beforeRevision },
        ),
        undefined,
        openApiAuthFetch,
      ),
    ),
};
