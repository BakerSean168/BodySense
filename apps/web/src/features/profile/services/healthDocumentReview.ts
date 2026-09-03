import { apiUrl, extractErrorMessage, safeJson } from "@/lib/api-url";
import type {
  AppendHealthDocumentReviewInput,
  DocumentIndicatorReviewRecord,
  HealthDocumentReviewContext,
} from "../types/health-document-review.types";

function authHeaders(accessToken: string): HeadersInit {
  return { Authorization: `Bearer ${accessToken}` };
}

export async function fetchHealthDocumentReviewContext(
  uploadId: string,
  accessToken: string,
): Promise<HealthDocumentReviewContext | null> {
  const response = await fetch(
    apiUrl(`/api/v1/uploads/${encodeURIComponent(uploadId)}/health-document-review`),
    { headers: authHeaders(accessToken) },
  );
  if (response.status === 404) return null;
  if (!response.ok) throw new Error(await extractErrorMessage(response));
  return safeJson<HealthDocumentReviewContext>(response);
}

export async function appendHealthDocumentReview(
  uploadId: string,
  runId: string,
  input: AppendHealthDocumentReviewInput,
  accessToken: string,
): Promise<DocumentIndicatorReviewRecord> {
  const response = await fetch(
    apiUrl(
      `/api/v1/uploads/${encodeURIComponent(uploadId)}/extractions/${encodeURIComponent(runId)}/reviews`,
    ),
    {
      method: "POST",
      headers: {
        ...authHeaders(accessToken),
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
  );
  if (!response.ok) throw new Error(await extractErrorMessage(response));
  return safeJson<DocumentIndicatorReviewRecord>(response);
}

export async function fetchHealthDocumentSource(
  uploadId: string,
  runId: string,
  accessToken: string,
): Promise<Blob> {
  const response = await fetch(
    apiUrl(
      `/api/v1/uploads/${encodeURIComponent(uploadId)}/extractions/${encodeURIComponent(runId)}/source`,
    ),
    { headers: authHeaders(accessToken) },
  );
  if (!response.ok) throw new Error(await extractErrorMessage(response));
  return response.blob();
}
