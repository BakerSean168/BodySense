import { z } from "zod";
import {
  id,
  text,
  instant,
  revision,
  temporalValue,
  assertionStatus,
  evidenceSource,
  sourceRef,
  capability,
  resourceSchema,
  mutationBinding,
  exactConsent,
} from "./common";

const quantity = z.strictObject({
  rawValue: text,
  rawUnit: text.optional(),
  normalizedValue: z.number().optional(),
  normalizedUnit: text.optional(),
});
const factBase = {
  factId: id,
  statement: text,
  bodyRegionIds: z.array(id),
  source: evidenceSource,
  temporal: temporalValue,
  recordedAt: instant,
  assertionStatus,
  supersedesFactId: id.optional(),
};
export const bodyFactCard = z.discriminatedUnion("kind", [
  z.strictObject({
    ...factBase,
    kind: z.literal("symptom"),
    details: z.strictObject({
      symptomType: text.optional(),
      presence: z.enum(["present", "absent"]),
      laterality: text.optional(),
      severity: quantity.optional(),
      pattern: z.enum(["continuous", "intermittent", "unknown"]).optional(),
      triggers: z.array(text).optional(),
      relievingFactors: z.array(text).optional(),
      occurrenceContext: text.optional(),
    }),
  }),
  z.strictObject({
    ...factBase,
    kind: z.literal("health_history"),
    details: z.strictObject({
      historyKind: text.optional(),
      event: text.optional(),
      outcome: text.optional(),
      sourceContext: text.optional(),
    }),
  }),
  z.strictObject({
    ...factBase,
    kind: z.literal("activity"),
    details: z.strictObject({
      activityType: text.optional(),
      duration: quantity.optional(),
      amount: quantity.optional(),
      intensity: text.optional(),
    }),
  }),
  z.strictObject({
    ...factBase,
    kind: z.literal("lifestyle"),
    details: z.strictObject({
      habitType: text.optional(),
      frequency: text.optional(),
      typicalDuration: quantity.optional(),
    }),
  }),
  z.strictObject({
    ...factBase,
    kind: z.literal("training_feedback"),
    details: z.strictObject({
      sessionLogId: id,
      exerciseId: id.optional(),
      category: text.optional(),
    }),
  }),
]);
export const bodyObservationCard = z
  .strictObject({
    observationId: id,
    kind: text,
    statement: text.optional(),
    value: quantity.optional(),
    method: text.optional(),
    measurementContext: text.optional(),
    source: evidenceSource,
    observedAt: temporalValue.optional(),
    recordedAt: instant,
    assertionStatus,
    supersedesObservationId: id.optional(),
  })
  .refine(
    (v) => Boolean(v.statement || v.value),
    "An observation needs a statement or a measured value",
  )
  .refine(
    (v) =>
      v.source.attribution !== "document" ||
      v.source.historicalAdmission === "admissible",
    "Unadmitted OCR remains an asset candidate, not a BodyObservation",
  );
export const bodyHypothesisCard = z.strictObject({
  hypothesisId: id,
  statement: text,
  actor: z.literal("user"),
  source: evidenceSource,
  relatedFactIds: z.array(id),
  relatedObservationIds: z.array(id),
  bodyRegionIds: z.array(id),
  assertionStatus,
  supersedesHypothesisId: id.optional(),
  recordedAt: instant,
});
const episodeLink = z.strictObject({
  factId: id,
  role: z.enum(["onset", "observation", "change", "resolution"]),
  associationMethod: z.enum(["explicit", "inferred", "user_corrected"]),
  basisSourceIds: z.array(id).min(1),
});
const episodeProjection = z.discriminatedUnion("state", [
  z.strictObject({
    state: z.literal("unknown"),
    reason: z.enum(["insufficient_evidence", "conflicting_evidence"]),
    basisFactIds: z.array(id),
    stateAsOf: temporalValue.optional(),
  }),
  z.strictObject({
    state: z.literal("ongoing"),
    reason: z.literal("explicit_current_report"),
    basisFactIds: z.array(id).min(1),
    stateAsOf: temporalValue.optional(),
  }),
  z.strictObject({
    state: z.literal("ended"),
    reason: z.literal("explicit_resolution"),
    basisFactIds: z.array(id).min(1),
    stateAsOf: temporalValue.optional(),
    end: temporalValue,
  }),
]);
export const episodeDetail = z.strictObject({
  episodeId: id,
  symptomType: text,
  bodyRegionIds: z.array(id),
  laterality: z.enum([
    "left",
    "right",
    "bilateral",
    "not_applicable",
    "unknown",
  ]),
  identityStatus: z.enum(["current", "superseded", "invalidated"]),
  onset: temporalValue,
  projection: episodeProjection,
  links: z.array(episodeLink),
  basedOnRevision: revision,
  relations: z.array(
    z.strictObject({
      otherEpisodeId: id,
      kind: z.enum([
        "recurrence_reported",
        "similar_symptom",
        "merge_replacement",
        "split_replacement",
      ]),
      basisFactIds: z.array(id),
      authorizationId: id.optional(),
    }),
  ),
  priorSupportedState: episodeProjection.optional(),
  pendingInteractionId: id.optional(),
  history: z.array(
    z.strictObject({
      eventId: id,
      recordedAt: instant,
      eventTime: temporalValue,
      description: text,
      sourceRefs: z.array(sourceRef),
    }),
  ),
});
const derivedGroup = z.strictObject({
  displayKey: id,
  title: text,
  factIds: z.array(id),
  episodeIds: z.array(id),
  observationIds: z.array(id),
  hypothesisIds: z.array(id),
});
export const recordsData = z.strictObject({
  facts: z.array(bodyFactCard),
  observations: z.array(bodyObservationCard),
  hypotheses: z.array(bodyHypothesisCard),
  episodes: z.array(episodeDetail),
  groups: z.array(derivedGroup),
  nextCursor: id.nullable(),
  canRecord: capability,
});
/** Derive a visible list state without claiming an unscanned filtered history is empty. */
export function classifyLoadedBodyList(
  loadedMatchCount: number,
  nextCursor: string | null,
): "populated" | "empty" | "partial_empty" {
  z.number().int().nonnegative().parse(loadedMatchCount);
  id.nullable().parse(nextCursor);
  return loadedMatchCount > 0
    ? "populated"
    : nextCursor === null
      ? "empty"
      : "partial_empty";
}

/**
 * Read-only bridge for the implemented GET /body-state/records transport.
 * It is intentionally NOT a full person.records/person.habits target read-model:
 * the V1 page has no source admissibility, Observation, Hypothesis or Episode joins.
 * Explicitly preserve those absences instead of fabricating resolved sources.
 */
export const boundedBodyFactCard = z.strictObject({
  factId: id,
  kind: z.enum([
    "symptom",
    "health_history",
    "activity",
    "lifestyle",
    "training_feedback",
  ]),
  statement: text,
  bodyRegionIds: z.array(id),
  sourceEvidenceId: id,
  recordedAt: instant,
  temporal: z.strictObject({
    precision: z.enum(["exact", "day", "range", "approximate", "unknown"]),
    start: z.string().optional(),
    end: z.string().optional(),
    originalExpression: z.string().optional(),
    timeZone: z.string().optional(),
  }),
  assertionStatus,
  supersedesFactId: id.optional(),
});
export const boundedBodyFactVM = z
  .strictObject({
    contract: z.literal("bodyfact-ledger-page/v1"),
    query: z.enum(["person.records", "person.habits"]),
    state: z.enum([
      "unauthenticated",
      "loading",
      "unavailable",
      "empty",
      "partial_empty",
      "ready",
      "stale",
    ]),
    currentRevision: z.number().int().nonnegative().nullable(),
    hasMore: z.boolean(),
    pagesDisagree: z.boolean(),
    records: z.array(boundedBodyFactCard),
    missingSections: z.tuple([
      z.literal("source_metadata"),
      z.literal("observations"),
      z.literal("hypotheses"),
      z.literal("episodes"),
      z.literal("concern_groups"),
    ]),
  })
  .superRefine((v, ctx) => {
    if (
      (v.state === "unauthenticated" ||
        v.state === "loading" ||
        v.state === "unavailable") &&
      v.records.length
    )
      ctx.addIssue({
        code: "custom",
        message: "No data should appear for unowned or never loaded ledger",
      });
    if (v.state === "unauthenticated" && v.currentRevision !== null)
      ctx.addIssue({
        code: "custom",
        message: "Do not leak another owner's revision",
      });
    if (v.state === "empty" && (v.records.length || v.hasMore))
      ctx.addIssue({
        code: "custom",
        message: "Empty requires an exhausted scan",
      });
    if (v.state === "partial_empty" && (v.records.length || !v.hasMore))
      ctx.addIssue({
        code: "custom",
        message: "Partial-empty requires remaining pages",
      });
    if ((v.state === "stale") !== v.pagesDisagree)
      ctx.addIssue({
        code: "custom",
        message: "Stale revision conflict must stay visible",
      });
    if (
      v.query === "person.habits" &&
      v.records.some((r) => r.kind !== "lifestyle")
    )
      ctx.addIssue({
        code: "custom",
        message:
          "Habits is a lifestyle-filtered ledger, never another data owner",
      });
  });
export type BoundedBodyFactVM = z.infer<typeof boundedBodyFactVM>;

export const personRecordsVM = resourceSchema(recordsData);
export const personHabitsVM = resourceSchema(
  z.strictObject({
    habits: z.array(bodyFactCard.options[3]),
    nextCursor: id.nullable(),
    canRecord: capability,
  }),
);
export const episodeVM = resourceSchema(episodeDetail);
export const regionSummaryVM = resourceSchema(
  z.strictObject({
    regionIds: z.array(id).min(1),
    selectionOnly: z.literal(true),
    factIds: z.array(id),
    episodeIds: z.array(id),
    observationIds: z.array(id),
    hypothesisIds: z.array(id),
    adoptedConditionIds: z.array(id),
    pendingInteractionId: id.optional(),
  }),
);
export const bodyStateCommand = z.discriminatedUnion("operation", [
  z.strictObject({
    operation: z.literal("body.record_statement"),
    idempotencyKey: id,
    statement: text,
    bodyRegionIds: z.array(id),
    temporal: temporalValue.optional(),
  }),
  z.strictObject({
    operation: z.literal("body.correct_assertion"),
    ...mutationBinding,
    targetKind: z.enum(["fact", "observation", "hypothesis"]),
    targetId: id,
    replacementStatement: text,
    reason: text,
    consent: exactConsent.optional(),
  }),
  z.strictObject({
    operation: z.literal("body.withdraw_assertion"),
    ...mutationBinding,
    targetKind: z.enum(["fact", "observation", "hypothesis"]),
    targetId: id,
    reason: text,
    consent: exactConsent.optional(),
  }),
  z.strictObject({
    operation: z.literal("body.record_temporal_change"),
    ...mutationBinding,
    statement: text,
    episodeId: id.optional(),
    temporal: temporalValue,
  }),
  z.strictObject({
    operation: z.literal("body.change_episode_links"),
    ...mutationBinding,
    proposalId: id,
    consent: exactConsent,
  }),
]);
export type PersonRecordsVM = z.infer<typeof personRecordsVM>;
export type PersonHabitsVM = z.infer<typeof personHabitsVM>;
export type EpisodeVM = z.infer<typeof episodeVM>;
export type RegionSummaryVM = z.infer<typeof regionSummaryVM>;
export type BodyStateCommand = z.infer<typeof bodyStateCommand>;
