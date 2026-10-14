import {
  listBodyObservationRecords,
  recordBodyObservation,
  correctBodyObservationRecord,
  withdrawBodyObservationRecord,
} from "@/generated/api/bodysense";
import { ListBodyObservationRecordsParams } from "@/generated/api/model/listBodyObservationRecordsParams.zod";
import {
  CorrectBodyObservationRequest,
  type CorrectBodyObservationRequest as ObservationCorrection,
} from "@/generated/api/model/correctBodyObservationRequest.zod";
import {
  WithdrawBodyObservationRequest,
  type WithdrawBodyObservationRequest as ObservationWithdrawal,
} from "@/generated/api/model/withdrawBodyObservationRequest.zod";
import {
  RecordBodyObservationRequest,
  type RecordBodyObservationRequest as BodyObservationInput,
} from "@/generated/api/model/recordBodyObservationRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// One source-of-truth: generated OpenAPI/TS/Zod for direct-user observations.
// This adapter neither guesses a normalized clinical measurement nor submits
// device/document provenance. Track B independently owns final visual design.
export type BodyObservationCommand = BodyObservationInput;
export type BodyObservationCorrection = ObservationCorrection;
export type BodyObservationWithdrawal = ObservationWithdrawal;
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
  // Exactly one existing observation ID; correction preserves the earlier
  // source and produces a separate version, not a new measurement reading.
  correct: (
    target: string,
    input: ObservationCorrection,
  ): Promise<BodyObservationCommit> =>
    withOpenApiError(() =>
      correctBodyObservationRecord(
        target,
        CorrectBodyObservationRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  withdraw: (
    target: string,
    input: ObservationWithdrawal,
  ): Promise<BodyObservationCommit> =>
    withOpenApiError(() =>
      withdrawBodyObservationRecord(
        target,
        WithdrawBodyObservationRequest.parse(input),
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
