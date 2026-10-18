import { listSafetyRestrictions } from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

export type { SafetyRestrictionRecordOutput as SafetyRestrictionRecord } from "@/generated/api/model/safetyRestrictionRecord.zod";

/** Go-owned Safety restrictions (read-only for the web). */
export const safetyRestrictionService = {
  list: () =>
    withOpenApiError(() => listSafetyRestrictions(undefined, openApiAuthFetch)),
};

export const safetyRestrictionKeys = {
  all: ["safety", "restrictions"] as const,
};
