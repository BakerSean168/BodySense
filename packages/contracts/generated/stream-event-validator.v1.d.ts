import type { BodySenseStreamEventV1 } from "./stream-event.v1";

export interface StreamEventValidationError {
  instancePath: string;
  schemaPath: string;
  keyword: string;
  params: Record<string, unknown>;
  message?: string;
}

export interface StreamEventValidator {
  (data: unknown): data is BodySenseStreamEventV1;
  errors: StreamEventValidationError[] | null;
}

export const validateStreamEvent: StreamEventValidator;
