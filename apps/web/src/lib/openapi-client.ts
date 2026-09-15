import { authFetch } from "@/features/auth/services/authService";
import { ApiRequestError } from "@/lib/api-client";
import { apiUrl } from "@/lib/api-url";

function requestPath(input: RequestInfo | URL): string {
  if (typeof input === "string") return input;
  const url = input instanceof URL ? input : new URL(input.url);
  return `${url.pathname}${url.search}${url.hash}`;
}

// Generated OpenAPI clients use relative public API paths. This fetch seam keeps
// authentication/refresh/API-origin behavior in the existing auth boundary while
// letting Orval retain its generated response parsing and Zod validation.
export const openApiAuthFetch: typeof globalThis.fetch = async (input, init) =>
  authFetch(requestPath(input), init);

// Public (unauthenticated) generated OpenAPI fetch seam. Session establishment
// is cookie-based: always send credentials and never attach a bearer token.
export const openApiPublicFetch: typeof globalThis.fetch = async (
  input,
  init,
) => fetch(apiUrl(requestPath(input)), { ...init, credentials: "include" });

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

// Orval intentionally throws an Error decorated with status/info for non-2xx
// responses. Generated error aliases are not an application contract, so this
// boundary normalizes them into the stable ApiRequestError consumed by features.
export function normalizeOpenApiError(error: unknown): Error {
  if (!(error instanceof Error) || !isRecord(error)) {
    return error instanceof Error ? error : new Error("Unknown API error");
  }

  const status = typeof error.status === "number" ? error.status : undefined;
  if (status === undefined) return error;

  const info = isRecord(error.info) ? error.info : undefined;
  const detail = info && isRecord(info.error) ? info.error : undefined;
  const code =
    detail && typeof detail.code === "string" ? detail.code : undefined;
  const message =
    detail && typeof detail.message === "string" && detail.message.trim()
      ? detail.message
      : error.message.trim() || `API ${status}`;

  return new ApiRequestError(message, status, code);
}

export async function withOpenApiError<T>(
  request: () => Promise<T>,
): Promise<T> {
  try {
    return await request();
  } catch (error) {
    throw normalizeOpenApiError(error);
  }
}
