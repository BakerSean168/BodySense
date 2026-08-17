/**
 * Unified API URL helpers for BodySense frontend.
 *
 * Convention:
 *   VITE_API_BASE_URL represents the backend **origin only** (no path prefix).
 *   - Production (same-origin via Caddy reverse-proxy): leave empty or unset.
 *   - Dev: Vite proxy handles /api → localhost:8080, so empty works too.
 *   - External backend for testing: set to "https://other-host.com".
 *
 *   Business endpoint paths keep the **full backend route**, e.g.
 *   `/api/v1/auth/register`, `/api/v1/consultations/...`.
 */

/** Backend origin — empty string for same-origin deployments. */
export const API_BASE_URL: string =
  (import.meta.env.VITE_API_BASE_URL as string) || "";

/** Build a full URL from a backend path. */
export function apiUrl(path: string): string {
  return `${API_BASE_URL}${path}`;
}

/**
 * Parse a Response body intelligently:
 * - If Content-Type is JSON → parse as JSON.
 * - Otherwise → return raw text.
 *
 * This avoids `Unexpected non-whitespace character after JSON` errors when
 * the server returns HTML (e.g. Caddy 404 page) or plain text.
 */
export async function safeJson<T = unknown>(res: Response): Promise<T> {
  const ct = res.headers.get("content-type") || "";
  if (ct.includes("application/json")) {
    return res.json() as Promise<T>;
  }
  const text = await res.text();
  // Best-effort: some servers forget the content-type header.
  if (text.startsWith("{") || text.startsWith("[")) {
    try {
      return JSON.parse(text) as T;
    } catch {
      // fall through
    }
  }
  return text as unknown as T;
}

/**
 * Extract a user-friendly error message from a failed API response.
 *
 * Tries JSON `.message` / `.error` fields first, then falls back to raw text
 * or the HTTP status text.
 */
export async function extractErrorMessage(res: Response): Promise<string> {
  const body: unknown = await safeJson(res);

  if (body && typeof body === "object") {
    const obj = body as Record<string, unknown>;
    const msg = obj.message ?? obj.error;
    if (typeof msg === "string") return msg;
  }

  if (typeof body === "string" && body.length > 0) return body;

  return res.statusText || `Request failed (${res.status})`;
}
