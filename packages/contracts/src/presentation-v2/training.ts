import { z } from "zod";
import {
  id,
  text,
  instant,
  revision,
  timeZone,
  localDate,
  sourceRef,
  resourceSchema,
  capability,
  mutationBinding,
  exactConsent,
} from "./common";
import { reviewRecommendation } from "./diagnosis";

export const restrictionScope = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("all_training") }),
  z.strictObject({ kind: z.literal("plan"), planId: id }),
  z.strictObject({
    kind: z.literal("exercise"),
    exerciseIds: z.array(id).min(1),
    planId: id.optional(),
  }),
]);
export const safetyRestriction = z
  .strictObject({
    restrictionId: id,
    riskLevel: z.enum(["advisory", "restricted", "urgent"]),
    reason: text,
    scope: restrictionScope,
    validFrom: instant,
    validUntil: instant.optional(),
    liftPolicy: z.enum([
      "on_expiry",
      "user_reverification",
      "professional_clearance",
    ]),
    liftConditions: z.array(text),
    sourceRefs: z.array(sourceRef).min(1),
    liftedAt: instant.optional(),
    liftBasisDescription: text.optional(),
    effect: z.enum([
      "active",
      "review_due",
      "clearance_pending",
      "lifted",
      "not_yet_effective",
    ]),
  })
  .refine(
    (r) =>
      r.riskLevel !== "urgent" ||
      (r.scope.kind === "all_training" &&
        r.liftPolicy === "professional_clearance"),
    "Urgent requires global block and professional clearance",
  )
  .refine(
    (r) => r.liftPolicy !== "on_expiry" || Boolean(r.validUntil),
    "Expiry lift requires an explicit approved time",
  );
export const trainerGate = z.strictObject({
  result: z.enum(["allow", "warn", "block", "unavailable"]),
  matchedRestrictionIds: z.array(id),
  reasons: z.array(text),
  requiresReview: z.boolean(),
  evaluatedAt: instant.optional(),
  safetyRevision: revision.optional(),
  scopeMatch: z.enum(["verified", "unknown"]),
  freshness: z.enum(["current", "stale", "unavailable"]),
});
export const safetyVM = resourceSchema(
  z.strictObject({
    restrictions: z.array(safetyRestriction),
    gate: trainerGate,
  }),
);
/** Presentation guard only. Go must repeat the safety check before every action. */
export function canRequestExecution(
  gate: z.infer<typeof trainerGate>,
): boolean {
  return (
    gate.freshness === "current" &&
    gate.scopeMatch === "verified" &&
    Boolean(gate.evaluatedAt && gate.safetyRevision) &&
    (gate.result === "allow" || gate.result === "warn")
  );
}
export const exerciseTarget = z
  .discriminatedUnion("kind", [
    z.strictObject({
      kind: z.literal("repetitions"),
      min: z.number().int().positive(),
      max: z.number().int().positive().optional(),
    }),
    z.strictObject({
      kind: z.literal("duration"),
      seconds: z.number().positive(),
    }),
    z.strictObject({
      kind: z.literal("distance"),
      meters: z.number().positive(),
    }),
  ])
  .refine(
    (v) => v.kind !== "repetitions" || v.max === undefined || v.max >= v.min,
    "Maximum must not be less than minimum repetitions",
  );
export const planExercise = z.strictObject({
  planExerciseId: id,
  exerciseId: id,
  name: text,
  sets: z.number().int().positive(),
  target: exerciseTarget,
  restSeconds: z.number().nonnegative().optional(),
  side: z.enum(["left", "right", "both", "alternating"]).optional(),
  loadOrIntensity: text.optional(),
  instructions: text.optional(),
  demoAssetId: id.optional(),
});
export const sessionTemplate = z.strictObject({
  templateId: id,
  title: text,
  stages: z
    .array(
      z.strictObject({
        phase: z.enum(["warmup", "main", "cooldown"]),
        exercises: z.array(planExercise).min(1),
      }),
    )
    .min(1),
});
export const planContent = z
  .strictObject({
    sessions: z.array(sessionTemplate).min(1),
    frequency: z.strictObject({
      sessionsPerWeek: z.number().int().positive().optional(),
      description: text.optional(),
    }),
    schedule: z.discriminatedUnion("mode", [
      z.strictObject({ mode: z.literal("frequency_only") }),
      z.strictObject({
        mode: z.literal("calendar"),
        timeZone,
        occurrences: z.array(
          z.strictObject({ occurrenceId: id, templateId: id, localDate }),
        ),
      }),
    ]),
    constraints: z.array(text),
  })
  .superRefine((v, ctx) => {
    const templates = v.sessions.map((s) => s.templateId);
    const entries = v.sessions.flatMap((s) =>
      s.stages.flatMap((p) => p.exercises.map((e) => e.planExerciseId)),
    );
    if (
      new Set(templates).size !== templates.length ||
      new Set(entries).size !== entries.length
    )
      ctx.addIssue({
        code: "custom",
        message:
          "Template and plan exercise instance IDs must be unique; names are not identities",
      });
    if (
      v.schedule.mode === "calendar" &&
      v.schedule.occurrences.some((o) => !templates.includes(o.templateId))
    )
      ctx.addIssue({
        code: "custom",
        message: "Calendar entries must use this plan's template IDs",
      });
  });
export const acceptedPlan = z.strictObject({
  planId: id,
  title: text,
  goal: text,
  relatedConditionIds: z.array(id),
  relatedBodyRefs: z.array(sourceRef),
  status: z.enum(["active", "paused", "completed"]),
  acceptedRevisionId: revision,
  content: planContent,
  reviewRecommendations: z.array(reviewRecommendation),
  gate: trainerGate,
});
export const trainingPlansVM = resourceSchema(
  z.strictObject({
    plans: z.array(acceptedPlan),
    selectedPlanId: id.optional(),
    selectedTemplateId: id.optional(),
  }),
);
const planProposalBase = {
  proposalId: id,
  contentFingerprint: id,
  rationale: text,
  evidenceRefs: z.array(sourceRef),
  createdAt: instant,
  expiresAt: instant,
  state: z.enum(["pending", "accepted", "rejected", "expired"]),
  expiredReason: z
    .enum(["timeout", "base_revision_changed", "relevant_context_changed"])
    .optional(),
  canAccept: capability,
};
export const planProposalCard = z.discriminatedUnion("kind", [
  z.strictObject({
    ...planProposalBase,
    kind: z.literal("create"),
    title: text,
    goal: text,
    relatedConditionIds: z.array(id),
    relatedBodyRefs: z.array(sourceRef),
    preview: planContent,
  }),
  z.strictObject({
    ...planProposalBase,
    kind: z.literal("update"),
    planId: id,
    basedOnRevisionId: revision,
    preview: planContent,
    changes: z
      .array(
        z.strictObject({
          targetId: id,
          fieldLabel: text,
          before: z.string().nullable(),
          after: z.string().nullable(),
        }),
      )
      .min(1),
  }),
]);
export const trainingProposalVM = resourceSchema(
  planProposalCard.refine((v) => {
    const elapsed = Date.parse(v.expiresAt) - Date.parse(v.createdAt);
    return elapsed > 0 && elapsed <= 7 * 24 * 60 * 60 * 1000;
  }, "Training proposals cannot exceed the approved seven-day lifetime"),
);
const actualDose = {
  actualRepetitions: z.number().int().nonnegative().optional(),
  actualDurationSeconds: z.number().nonnegative().optional(),
  actualDistanceMeters: z.number().nonnegative().optional(),
};
const actionBase = {
  actionId: id,
  planExerciseId: id,
  sourceId: id,
  recordedAt: instant,
};
export const sessionAction = z
  .discriminatedUnion("kind", [
    z.strictObject({
      ...actionBase,
      kind: z.literal("completed"),
      scope: z.literal("set"),
      setNumber: z.number().int().positive(),
      ...actualDose,
    }),
    z.strictObject({
      ...actionBase,
      kind: z.literal("skipped"),
      scope: z.enum(["set", "exercise"]),
      setNumber: z.number().int().positive().optional(),
    }),
    z.strictObject({
      ...actionBase,
      kind: z.literal("corrected"),
      correctsActionId: id,
      scope: z.enum(["set", "exercise"]),
      setNumber: z.number().int().positive().optional(),
      ...actualDose,
    }),
  ])
  .refine(
    (a) => a.scope !== "set" || a.setNumber !== undefined,
    "A set action must identify a set",
  );
const sessionBase = {
  sessionLogId: id,
  planId: id,
  planRevisionId: revision,
  templateId: id,
  startedAt: instant,
  trainingDate: localDate,
  timeZoneSnapshot: timeZone,
  templateSnapshot: sessionTemplate,
  actions: z.array(sessionAction),
  current: z.strictObject({
    planExerciseId: id,
    setNumber: z.number().int().positive(),
  }),
  verifiedElapsedSeconds: z.number().nonnegative(),
  gate: trainerGate,
  feedback: z
    .strictObject({
      text: z.string(),
      recordedAt: instant,
      bodyStateSourceIds: z.array(id),
    })
    .optional(),
};
export const trainerSession = z.discriminatedUnion("state", [
  z.strictObject({ ...sessionBase, state: z.literal("in_progress") }),
  z.strictObject({
    ...sessionBase,
    state: z.literal("paused"),
    pauseReason: z.enum(["user", "safety_hold", "recovery"]),
  }),
  z.strictObject({
    ...sessionBase,
    state: z.literal("ended"),
    endedAt: instant,
    endedReason: z.enum(["finished", "stopped_early", "safety_stopped"]),
  }),
]);
export const trainerVM = resourceSchema(
  trainerSession.superRefine((v, ctx) => {
    const exercises = v.templateSnapshot.stages.flatMap((s) => s.exercises);
    const current = exercises.find(
      (e) => e.planExerciseId === v.current.planExerciseId,
    );
    if (
      v.templateSnapshot.templateId !== v.templateId ||
      !current ||
      v.current.setNumber > current.sets
    )
      ctx.addIssue({
        code: "custom",
        message: "Session cursor must identify its immutable template snapshot",
      });
    if (
      v.actions.some(
        (a) => !exercises.some((e) => e.planExerciseId === a.planExerciseId),
      )
    )
      ctx.addIssue({
        code: "custom",
        message: "Action cannot target a foreign plan exercise instance",
      });
  }),
);
export const trainingCommand = z.discriminatedUnion("operation", [
  z.strictObject({
    operation: z.literal("training.decide_proposal"),
    idempotencyKey: id,
    proposalId: id,
    consent: exactConsent,
    decision: z.enum(["accept", "reject"]),
    basedOnRevisionId: revision.optional(),
  }),
  z.strictObject({
    operation: z.literal("training.set_status"),
    ...mutationBinding,
    planId: id,
    status: z.enum(["active", "paused", "completed"]),
  }),
  z.strictObject({
    operation: z.literal("training.request_change"),
    ...mutationBinding,
    planId: id,
    targetIds: z.array(id),
    requestText: text,
  }),
  z.strictObject({
    operation: z.literal("trainer.start"),
    idempotencyKey: id,
    planId: id,
    planRevisionId: revision,
    templateId: id,
    plannedOccurrenceId: id.optional(),
    timeZone,
  }),
  z.strictObject({
    operation: z.literal("trainer.pause"),
    ...mutationBinding,
    sessionLogId: id,
  }),
  z.strictObject({
    operation: z.literal("trainer.resume"),
    ...mutationBinding,
    sessionLogId: id,
  }),
  z.strictObject({
    operation: z.literal("trainer.complete_set"),
    ...mutationBinding,
    sessionLogId: id,
    planExerciseId: id,
    setNumber: z.number().int().positive(),
    ...actualDose,
  }),
  z
    .strictObject({
      operation: z.literal("trainer.skip"),
      ...mutationBinding,
      sessionLogId: id,
      planExerciseId: id,
      scope: z.enum(["set", "exercise"]),
      setNumber: z.number().int().positive().optional(),
    })
    .refine(
      (v) => v.scope === "exercise" || v.setNumber !== undefined,
      "Skipping a set must identify it",
    ),
  z.strictObject({
    operation: z.literal("trainer.correct_action"),
    ...mutationBinding,
    sessionLogId: id,
    correctsActionId: id,
    correctionText: text,
  }),
  z.strictObject({
    operation: z.literal("trainer.finish"),
    ...mutationBinding,
    sessionLogId: id,
    reason: z.enum(["finished", "stopped_early"]),
  }),
  z.strictObject({
    operation: z.literal("trainer.feedback"),
    ...mutationBinding,
    sessionLogId: id,
    text,
  }),
  z.strictObject({
    operation: z.literal("safety.submit_reverification"),
    ...mutationBinding,
    restrictionId: id,
    consent: exactConsent,
    sourceRefs: z.array(sourceRef).min(1),
  }),
]);
export type TrainingPlansVM = z.infer<typeof trainingPlansVM>;
export type TrainingProposalVM = z.infer<typeof trainingProposalVM>;
export type TrainerVM = z.infer<typeof trainerVM>;
export type SafetyVM = z.infer<typeof safetyVM>;
export type TrainingCommand = z.infer<typeof trainingCommand>;
