import { z } from "zod";
import {
  id,
  text,
  instant,
  localDate,
  timeZone,
  sourceRef,
  historicalAdmission,
  sourceAvailability,
  capability,
  resourceSchema,
  mutationBinding,
  exactConsent,
  revision,
} from "./common";

export const profileFields = z.strictObject({
  displayName: text.optional(),
  avatarAssetId: id.optional(),
  birthDate: localDate.optional(),
  gender: z
    .strictObject({ value: text, attribution: z.literal("self_reported") })
    .optional(),
  locale: text.optional(),
  timeZone: timeZone.optional(),
  timeZoneOrigin: z.enum(["user", "inferred", "default"]).optional(),
  unitPreference: z.enum(["metric", "imperial"]).optional(),
});
export const personIdentityVM = resourceSchema(
  z.strictObject({
    profile: profileFields,
    updatedAt: instant,
    canEdit: capability,
  }),
);
export const equipmentCard = z.strictObject({
  equipmentId: id,
  displayName: text,
  accessKind: z.enum(["owned", "borrowed", "facility_access"]),
  availability: z.enum(["available", "unavailable", "retired"]),
  validFrom: instant.optional(),
  validUntil: instant.optional(),
  sourceRef: sourceRef.optional(),
  updatedAt: instant,
  affectedPlanIds: z.array(id),
});
export const personEquipmentVM = resourceSchema(
  z.strictObject({
    equipment: z.array(equipmentCard),
    nextCursor: id.nullable(),
    canEdit: capability,
  }),
);
export const extractionRun = z.strictObject({
  runId: id,
  mechanismVersion: text,
  processingState: z.enum(["queued", "processing", "succeeded", "failed"]),
  retryOfRunId: id.optional(),
  errorMessage: text.optional(),
  canRetry: capability,
});
export const evidenceCandidate = z.strictObject({
  candidateId: id,
  extractionRunId: id,
  sourceLocator: text,
  admissibility: historicalAdmission,
  admissionDecisionId: id.optional(),
  admissionVersion: revision.optional(),
  summary: text.optional(),
});
export const assetCard = z
  .strictObject({
    assetId: id,
    facets: z.array(z.enum(["media", "report"])).min(1),
    originalName: text,
    mimeType: text,
    byteSize: z.number().int().nonnegative(),
    state: z.enum(["available", "deleting", "deleted"]),
    sourceAvailability,
    createdAt: instant,
    deletedAt: instant.optional(),
    extractions: z.array(extractionRun),
    candidates: z.array(evidenceCandidate),
    usage: z.enum(["allowed", "blocked"]),
    canOpenOriginal: capability,
  })
  .superRefine((v, ctx) => {
    if (
      v.sourceAvailability !== "original_available" &&
      v.canOpenOriginal.state !== "disabled"
    )
      ctx.addIssue({
        code: "custom",
        message:
          "Removed originals cannot be presented as currently verifiable files",
      });
    if (
      v.sourceAvailability === "erased" &&
      (v.usage !== "blocked" ||
        v.candidates.length > 0 ||
        v.extractions.length > 0)
    )
      ctx.addIssue({
        code: "custom",
        message:
          "Erased assets must block usage and expose no candidate content",
      });
    if (new Set(v.facets).size !== v.facets.length)
      ctx.addIssue({ code: "custom", message: "Asset facets must be unique" });
  });
export const personAssetsVM = resourceSchema(
  z
    .strictObject({
      facet: z.enum(["media", "report"]),
      assets: z.array(assetCard),
      nextCursor: id.nullable(),
      canUpload: capability,
    })
    .refine(
      (v) =>
        new Set(v.assets.map((a) => a.assetId)).size === v.assets.length &&
        v.assets.every((a) => a.facets.includes(v.facet)),
      "A classification view cannot duplicate asset identity or mix unrelated facets",
    ),
);
export const deletionScope = z.strictObject({
  scopeId: id,
  scopeFingerprint: id,
  assetIds: z.array(id).min(1),
  affected: z.array(
    z.strictObject({
      category: z.enum([
        "original",
        "thumbnail",
        "ocr",
        "index",
        "cache",
        "checkpoint",
        "body_evidence",
        "sensitive_audit",
      ]),
      count: z.number().int().nonnegative(),
    }),
  ),
  retainedSummary: z.array(text),
  independentSourceIdsPreserved: z.array(id),
});
const step = z.strictObject({
  category: text,
  state: z.enum(["pending", "running", "verified", "failed"]),
  explanation: text.optional(),
});
export const erasureJob = z.discriminatedUnion("state", [
  z.strictObject({
    state: z.literal("requested"),
    jobId: id,
    scope: deletionScope,
    newUseBlocked: z.boolean(),
  }),
  z.strictObject({
    state: z.literal("blocked_from_new_use"),
    jobId: id,
    scope: deletionScope,
    newUseBlocked: z.literal(true),
    steps: z.array(step),
  }),
  z.strictObject({
    state: z.literal("propagating"),
    jobId: id,
    scope: deletionScope,
    newUseBlocked: z.literal(true),
    steps: z.array(step),
  }),
  z.strictObject({
    state: z.literal("retryable_failed"),
    jobId: id,
    scope: deletionScope,
    newUseBlocked: z.literal(true),
    steps: z.array(step),
    canRetry: capability,
  }),
  z.strictObject({
    state: z.literal("completed"),
    jobId: id,
    scope: deletionScope,
    newUseBlocked: z.literal(true),
    steps: z.array(step.extend({ state: z.literal("verified") })).min(1),
    verificationReceiptId: id,
    completedAt: instant,
  }),
]);
export const personErasureVM = resourceSchema(
  erasureJob.superRefine((v, ctx) => {
    if (
      v.state === "completed" &&
      v.scope.affected.some(
        (a) =>
          a.count > 0 &&
          !v.steps.some(
            (s) => s.category === a.category && s.state === "verified",
          ),
      )
    )
      ctx.addIssue({
        code: "custom",
        message:
          "Completion must cover every affected category, not just the original object",
      });
  }),
);
const profilePatch = z
  .strictObject({
    displayName: text.nullable().optional(),
    avatarAssetId: id.nullable().optional(),
    birthDate: localDate.nullable().optional(),
    gender: text.nullable().optional(),
    locale: text.nullable().optional(),
    timeZone: timeZone.nullable().optional(),
    unitPreference: z.enum(["metric", "imperial"]).nullable().optional(),
  })
  .refine(
    (v) => Object.keys(v).length > 0,
    "Provide at least one changed field; null explicitly clears it",
  );
export const personAssetCommand = z.discriminatedUnion("operation", [
  z.strictObject({
    operation: z.literal("profile.update"),
    ...mutationBinding,
    fields: profilePatch,
  }),
  z.strictObject({
    operation: z.literal("equipment.create"),
    idempotencyKey: id,
    displayName: text,
    accessKind: z.enum(["owned", "borrowed", "facility_access"]),
    availability: z.enum(["available", "unavailable"]),
    validFrom: instant.optional(),
    validUntil: instant.optional(),
  }),
  z.strictObject({
    operation: z.literal("equipment.update"),
    ...mutationBinding,
    equipmentId: id,
    displayName: text,
    accessKind: z.enum(["owned", "borrowed", "facility_access"]),
    availability: z.enum(["available", "unavailable", "retired"]),
    validFrom: instant.nullable().optional(),
    validUntil: instant.nullable().optional(),
  }),
  z.strictObject({
    operation: z.literal("asset.upload"),
    idempotencyKey: id,
    uploadHandle: id,
    facets: z.array(z.enum(["media", "report"])).min(1),
  }),
  z.strictObject({
    operation: z.literal("asset.retry_extraction"),
    ...mutationBinding,
    assetId: id,
    previousRunId: id,
  }),
  z.strictObject({
    operation: z.literal("asset.request_candidate_review"),
    ...mutationBinding,
    candidateId: id,
    requestText: text,
  }),
  z.strictObject({
    operation: z.literal("asset.preview_deletion"),
    assetId: id,
    mode: z.enum(["original_only", "file_and_related_health_data"]),
  }),
  z.strictObject({
    operation: z.literal("asset.remove_original"),
    ...mutationBinding,
    assetId: id,
    scopeId: id,
    scopeFingerprint: id,
    consent: exactConsent,
  }),
  z.strictObject({
    operation: z.literal("asset.erase_related_health_data"),
    ...mutationBinding,
    assetId: id,
    scopeId: id,
    scopeFingerprint: id,
    consent: exactConsent,
  }),
  z.strictObject({
    operation: z.literal("asset.retry_erasure"),
    idempotencyKey: id,
    jobId: id,
  }),
]);
export type PersonIdentityVM = z.infer<typeof personIdentityVM>;
export type PersonEquipmentVM = z.infer<typeof personEquipmentVM>;
export type PersonAssetsVM = z.infer<typeof personAssetsVM>;
export type PersonErasureVM = z.infer<typeof personErasureVM>;
export type PersonAssetCommand = z.infer<typeof personAssetCommand>;
