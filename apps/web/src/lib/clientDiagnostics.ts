import { authFetch } from "@/features/auth/services/authService";

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
 * health data. The backend also rejects nested attributes to keep the schema
 * flat and indexable by an OTel/Loki pipeline.
 */
export function reportClientDiagnostic(input: ClientDiagnostic): void {
  const payload = {
    schemaVersion: 1,
    ...input,
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
  };

  void authFetch("/api/v1/client-diagnostics", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  }).catch(() => {
    // Diagnostics must never interfere with the user-facing path they observe.
  });
}

function fallbackDiagnosticId(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}
