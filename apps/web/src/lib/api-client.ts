import { extractErrorMessage, safeJson } from "./api-url";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
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
    const body = await safeJson(clone);
    if (isRecord(body)) {
      const error = body.error;
      if (isRecord(error)) {
        code = typeof error.code === "string" ? error.code : undefined;
        message = typeof error.message === "string" ? error.message : undefined;
      } else {
        code = typeof body.code === "string" ? body.code : undefined;
        message =
          typeof body.message === "string"
            ? body.message
            : typeof error === "string"
              ? error
              : undefined;
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

export async function expectJson<T>(
  response: Response,
  parse: (input: unknown) => T,
): Promise<T> {
  if (!response.ok) throw await apiErrorFromResponse(response);
  return parse(await safeJson(response));
}

export async function expectEmpty(response: Response): Promise<void> {
  if (!response.ok) throw await apiErrorFromResponse(response);
}

export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim()
    ? error.message
    : fallback;
}
