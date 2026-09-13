import { recordClientDiagnostic } from "@/generated/api/bodysense";
import { openApiAuthFetch } from "@/lib/openapi-client";

export type ClientDiagnosticCategory =
  "chat.transport" | "body3d.viewer" | "app.runtime";

export type ClientDiagnosticAttributeValue = string | number | boolean | null;

export interface ClientDiagnostic {
  category: ClientDiagnosticCategory;
  event: string;
  severity?: "info" | "warn" | "error";
  code?: string;
  message?: string;
  phase?: string;
  conversationId?: string | null;
  runId?: string | null;
  requestId?: string | null;
  resource?: string | null;
  diagnosticSessionId?: string | null;
  attemptId?: string | null;
  elapsedMs?: number | null;
  attributes?: Record<string, ClientDiagnosticAttributeValue>;
}

/**
 * Creates an opaque operational correlation id. It identifies one browser-side
 * diagnostic session/attempt only; it carries no account or health meaning.
 */
export function createClientDiagnosticId(prefix: string): string {
  const id = globalThis.crypto?.randomUUID?.() ?? fallbackDiagnosticId();
  return `${prefix}-${id}`;
}

/**
 * Best-effort operational telemetry for browser-only failures.
 * Never include consultation text, BodyState content, auth data, or other
 * health data. The OpenAPI boundary and backend sanitizer both reject nested
 * attributes so diagnostics remain flat and safe to index.
 */
export function reportClientDiagnostic(input: ClientDiagnostic): void {
  const payload = {
    schemaVersion: 1 as const,
    category: input.category,
    event: input.event,
    severity: input.severity ?? "info",
    code: input.code,
    message: input.message,
    phase: input.phase,
    conversationId: input.conversationId || undefined,
    runId: input.runId || undefined,
    requestId: input.requestId || undefined,
    resource: input.resource || undefined,
    diagnosticSessionId: input.diagnosticSessionId || undefined,
    attemptId: input.attemptId || undefined,
    elapsedMs:
      typeof input.elapsedMs === "number" && Number.isFinite(input.elapsedMs)
        ? Math.max(0, Math.round(input.elapsedMs * 10) / 10)
        : undefined,
    attributes: input.attributes,
  };

  void recordClientDiagnostic(payload, undefined, openApiAuthFetch).catch(
    () => {
      // Diagnostics must never interfere with the user-facing path they observe.
    },
  );
}

function fallbackDiagnosticId(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}
