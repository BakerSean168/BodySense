import type { ClientDiagnosticAttributeValue } from "@/lib/clientDiagnostics";
export type ViewerDiagnosticReporter = (input: {
  event: string;
  severity?: "info" | "warn" | "error";
  code?: string;
  message?: string;
  phase: string;
  resource?: string | null;
  attributes?: Record<string, ClientDiagnosticAttributeValue>;
}) => void;
