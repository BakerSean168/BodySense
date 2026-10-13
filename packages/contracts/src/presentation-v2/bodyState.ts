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
