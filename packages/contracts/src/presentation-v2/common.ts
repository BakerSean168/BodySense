import { z } from "zod";

/** Target presentation contract: not an HTTP DTO or a second domain owner. */
export const id = z.string().min(1);
export const text = z.string().min(1);
export const instant = z.string().datetime({ offset: true });
export const revision = z.string().min(1);
export const localDate = z.iso.date();
export const timeZone = z.string().refine((value) => {
  try {
    new Intl.DateTimeFormat("en", { timeZone: value });
    return true;
  } catch {
    return false;
  }
}, "Expected an IANA time zone");
export const versionedRef = z.strictObject({
  id,
  version: revision.optional(),
});
export const queryName = z.enum([
  "person.records",
  "person.habits",
  "body.episode",
  "body.region",
  "chat.current",
  "diagnosis.current",
  "diagnosis.history",
  "diagnosis.confirmation",
  "training.plans",
  "training.proposal",
  "trainer.current",
  "safety.current",
  "person.identity",
  "person.equipment",
  "person.assets",
  "person.erasure",
]);
export const domainName = z.enum([
  "body_state",
  "conversation",
  "diagnosis",
  "training_plan",
  "trainer",
  "profile",
  "equipment",
  "user_asset",
  "safety",
]);
export const temporalValue = z
  .strictObject({
    precision: z.enum(["exact", "day", "range", "approximate", "unknown"]),
    start: text.optional(),
    end: text.optional(),
    originalExpression: text.optional(),
    timeZone: timeZone.optional(),
  })
  .refine(
    (v) => v.precision !== "unknown" || (!v.start && !v.end),
    "Unknown event time cannot borrow recordedAt",
  );
export const assertionStatus = z.enum(["asserted", "superseded", "withdrawn"]);
export const evidenceRole = z.enum(["supporting", "counter", "context"]);
export const historicalAdmission = z.enum([
  "admissible",
  "needs_review",
  "rejected",
]);
export const sourceAvailability = z.enum([
  "original_available",
  "original_unavailable",
  "erased",
]);
export const sourceRef = z.strictObject({
  sourceId: id,
  kind: z.enum([
    "user_statement_evidence",
    "user_asset",
    "evidence_candidate",
    "body_fact",
    "body_observation",
    "symptom_episode",
    "training_session",
    "direct_user_action",
  ]),
  version: revision.optional(),
  recordedAt: instant,
  observedAt: temporalValue.optional(),
  locator: text.optional(),
});
export const evidenceSource = z
  .strictObject({
    ref: sourceRef,
    attribution: z.enum([
      "user_statement",
      "user_measurement",
      "self_test",
      "device",
      "document",
      "structured_observation",
    ]),
    label: text,
    excerpt: text.optional(),
    historicalAdmission: historicalAdmission.optional(),
    availability: sourceAvailability,
    currentValidity: z.enum([
      "current",
      "corrected",
      "withdrawn",
      "ineligible",
    ]),
    usage: z.enum(["allowed", "blocked"]),
  })
  .refine(
    (v) => v.availability !== "erased" || (v.usage === "blocked" && !v.excerpt),
    "Erased sources must be blocked and contain no sensitive excerpt",
  );
export const capability = z.discriminatedUnion("state", [
  z.strictObject({ state: z.literal("enabled") }),
  z.strictObject({ state: z.literal("disabled"), reason: text }),
]);
export const queryMeta = z.strictObject({
  fetchedAt: instant,
  sourceVersions: z.array(
    z.strictObject({ domain: domainName, targetId: id, revision }),
  ),
});
export const recoverableError = z.strictObject({
  code: z.enum([
    "network",
    "rate_limited",
    "quota_exhausted",
    "unavailable",
    "invalid_input",
    "not_found",
    "conflict",
    "expired",
    "forbidden",
  ]),
  message: text,
  retry: z.enum(["same_request", "refresh_then_review", "sign_in", "none"]),
  retryAfter: instant.optional(),
});

/** Empty is an authenticated successful query, never loading or forbidden. */
export function resourceSchema<T extends z.ZodType>(data: T) {
  return z.discriminatedUnion("state", [
    z.strictObject({ state: z.literal("loading") }),
    z.strictObject({
      state: z.literal("empty"),
      meta: queryMeta,
      explanation: text,
    }),
    z.strictObject({ state: z.literal("ready"), meta: queryMeta, data }),
    z.strictObject({
      state: z.literal("partial"),
      meta: queryMeta,
      data,
      unavailableSections: z.array(text).min(1),
    }),
    z.strictObject({
      state: z.literal("stale"),
      meta: queryMeta,
      data,
      reason: text,
    }),
    z.strictObject({
      state: z.literal("offline"),
      lastKnown: data.optional(),
      lastFetchedAt: instant.optional(),
    }),
    z.strictObject({
      state: z.literal("recovering"),
      lastKnown: data.optional(),
    }),
    z.strictObject({
      state: z.literal("failed"),
      error: recoverableError,
      lastKnown: data.optional(),
    }),
    z.strictObject({ state: z.literal("permission_denied"), message: text }),
  ]);
}
export const commitReceipt = z.strictObject({
  receiptId: id,
  domain: domainName,
  targetId: id,
  revision,
  eventIds: z.array(id).min(1),
  committedAt: instant,
});
export const commandOutcome = z.discriminatedUnion("state", [
  z.strictObject({ state: z.literal("pending"), requestId: id }),
  z.strictObject({
    state: z.literal("committed"),
    receipt: commitReceipt,
    refresh: z.array(queryName).min(1),
  }),
  z.strictObject({
    state: z.literal("unchanged"),
    requestId: id,
    explanation: text,
  }),
  z.strictObject({
    state: z.literal("decision_recorded"),
    decisionId: id,
    runId: id,
    proposalId: id,
    diagnosisChanged: z.literal(false),
  }),
  z.strictObject({
    state: z.literal("conflict"),
    error: recoverableError,
    refresh: z.array(queryName).min(1),
  }),
  z.strictObject({ state: z.literal("rejected"), error: recoverableError }),
  z.strictObject({
    state: z.literal("failed"),
    error: recoverableError,
    retainDraft: z.literal(true),
  }),
]);
export const mutationBinding = {
  idempotencyKey: id,
  expectedRevision: revision,
};
export const exactConsent = z.strictObject({
  interactionId: id,
  runId: id,
  toolCallId: id,
  contentFingerprint: id,
  sourceVersions: z.array(versionedRef),
});
export type EvidenceSourceVM = z.infer<typeof evidenceSource>;
export type CommandOutcomeVM = z.infer<typeof commandOutcome>;
export type QueryName = z.infer<typeof queryName>;
