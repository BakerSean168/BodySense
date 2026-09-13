import {
  appendHealthDocumentReview as appendHealthDocumentReviewOpenApi,
  getHealthDocumentReviewContext,
  getHealthDocumentSource,
} from "@/generated/api/bodysense";
import { AppendHealthDocumentReviewRequest as AppendHealthDocumentReviewRequestSchema } from "@/generated/api/model/appendHealthDocumentReviewRequest.zod";
import { ApiRequestError } from "@/lib/api-client";
import {
  normalizeOpenApiError,
  openApiAuthFetch,
  withOpenApiError,
} from "@/lib/openapi-client";
import type {
  AppendHealthDocumentReviewInput,
  DocumentIndicatorCandidate,
  DocumentIndicatorReviewProjection,
  DocumentIndicatorReviewRecord,
  HealthDocumentReviewContext,
} from "../types/health-document-review.types";

type ReviewContextWire = Awaited<
  ReturnType<typeof getHealthDocumentReviewContext>
>;
type ReviewRecordWire = Awaited<
  ReturnType<typeof appendHealthDocumentReviewOpenApi>
>;
type ReviewProjectionWire = ReviewContextWire["review_candidates"][number];

function toReviewRecord(
  record: ReviewRecordWire,
): DocumentIndicatorReviewRecord {
  return {
    id: record.id,
    extraction_run_id: record.extraction_run_id,
    upload_id: record.upload_id,
    indicator_index: record.indicator_index,
    indicator_id: record.indicator_id,
    action: record.action,
    reviewed_payload: record.reviewed_payload,
    note: record.note,
    created_at: record.created_at,
    idempotency_key: record.idempotency_key,
  };
}

function toCandidate(
  candidate: ReviewProjectionWire["candidate"],
): DocumentIndicatorCandidate {
  return {
    indicator_index: candidate.indicator_index,
    indicator_id: candidate.indicator_id,
    name: candidate.name,
    value: candidate.value,
    unit: candidate.unit,
    reference_range: candidate.reference_range,
    evidence_admissibility: {
      status: candidate.evidence_admissibility.status,
      policy_revision: candidate.evidence_admissibility.policy_revision,
      reason_codes: candidate.evidence_admissibility.reason_codes,
    },
    source_refs: candidate.source_refs,
    source_regions: candidate.source_regions?.map((region) => ({
      source_ref: region.source_ref,
      page_number: region.page_number,
      bbox: region.bbox,
    })),
  };
}

function toProjection(
  projection: ReviewProjectionWire,
): DocumentIndicatorReviewProjection {
  return {
    indicator_index: projection.indicator_index,
    indicator_id: projection.indicator_id,
    candidate: toCandidate(projection.candidate),
    effective_review: projection.effective_review
      ? toReviewRecord(projection.effective_review)
      : undefined,
    history: projection.history?.map(toReviewRecord),
  };
}

function toContext(value: ReviewContextWire): HealthDocumentReviewContext {
  return {
    extraction_run_id: value.extraction_run_id,
    upload_id: value.upload_id,
    review_candidates: value.review_candidates.map(toProjection),
  };
}

export async function fetchHealthDocumentReviewContext(
  uploadId: string,
): Promise<HealthDocumentReviewContext | null> {
  try {
    return toContext(
      await getHealthDocumentReviewContext(
        uploadId,
        undefined,
        openApiAuthFetch,
      ),
    );
  } catch (error) {
    const normalized = normalizeOpenApiError(error);
    if (normalized instanceof ApiRequestError && normalized.status === 404) {
      return null;
    }
    throw normalized;
  }
}

export async function appendHealthDocumentReview(
  uploadId: string,
  runId: string,
  input: AppendHealthDocumentReviewInput,
): Promise<DocumentIndicatorReviewRecord> {
  const response = await withOpenApiError(() =>
    appendHealthDocumentReviewOpenApi(
      uploadId,
      runId,
      AppendHealthDocumentReviewRequestSchema.parse(input),
      undefined,
      openApiAuthFetch,
    ),
  );
  return toReviewRecord(response);
}

export async function fetchHealthDocumentSource(
  uploadId: string,
  runId: string,
): Promise<Blob> {
  return withOpenApiError(() =>
    getHealthDocumentSource(uploadId, runId, undefined, openApiAuthFetch),
  );
}
