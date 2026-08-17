import { extractErrorMessage, safeJson } from "./api-url";

interface StructuredErrorBody {
  error?: string | { code?: string; message?: string };
  message?: string;
  code?: string;
}

export class ApiRequestError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "ApiRequestError";
    this.status = status;
    this.code = code;
  }
}

export async function apiErrorFromResponse(
  response: Response,
): Promise<ApiRequestError> {
  const clone =
    typeof response.clone === "function" ? response.clone() : response;
  let code: string | undefined;
  let message: string | undefined;

  try {
    const body = await safeJson<StructuredErrorBody>(clone);
    if (body && typeof body === "object") {
      if (typeof body.error === "object" && body.error) {
        code = body.error.code;
        message = body.error.message;
      } else {
        code = body.code;
        message =
          body.message ??
          (typeof body.error === "string" ? body.error : undefined);
      }
    }
  } catch {
    // The shared fallback below handles non-JSON responses.
  }

  const fallback = message || (await extractErrorMessage(response));
  const normalizedMessage = message
    ? message
    : `API ${response.status}: ${fallback}`;
  return new ApiRequestError(normalizedMessage, response.status, code);
}

export async function expectJson<T>(response: Response): Promise<T> {
  if (!response.ok) throw await apiErrorFromResponse(response);
  return safeJson<T>(response);
}

export async function expectEmpty(response: Response): Promise<void> {
  if (!response.ok) throw await apiErrorFromResponse(response);
}

export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim()
    ? error.message
    : fallback;
}
