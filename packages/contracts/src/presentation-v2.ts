import { z } from "zod";
import {
  personRecordsVM,
  personHabitsVM,
  episodeVM,
  regionSummaryVM,
  bodyStateCommand,
} from "./presentation-v2/bodyState";
import { chatVM, chatCommand } from "./presentation-v2/chat";
import {
  diagnosisVM,
  diagnosisHistoryVM,
  diagnosisConfirmationVM,
  diagnosisCommand,
} from "./presentation-v2/diagnosis";
import {
  trainingPlansVM,
  trainingProposalVM,
  trainerVM,
  safetyVM,
  trainingCommand,
} from "./presentation-v2/training";
import {
  personIdentityVM,
  personEquipmentVM,
  personAssetsVM,
  personErasureVM,
  personAssetCommand,
} from "./presentation-v2/personAssets";
import { id, revision } from "./presentation-v2/common";

export * from "./presentation-v2/common";
export * from "./presentation-v2/bodyState";
export * from "./presentation-v2/chat";
export * from "./presentation-v2/diagnosis";
export * from "./presentation-v2/training";
export * from "./presentation-v2/personAssets";

export const PRESENTATION_V2_VERSION = "2.0.0" as const;
/** Canonical presentation-only schemas; transport binding remains a separate Engineering gate. */
export const presentationV2Queries = {
  "person.records": personRecordsVM,
  "person.habits": personHabitsVM,
  "body.episode": episodeVM,
  "body.region": regionSummaryVM,
  "chat.current": chatVM,
  "diagnosis.current": diagnosisVM,
  "diagnosis.history": diagnosisHistoryVM,
  "diagnosis.confirmation": diagnosisConfirmationVM,
  "training.plans": trainingPlansVM,
  "training.proposal": trainingProposalVM,
  "trainer.current": trainerVM,
  "safety.current": safetyVM,
  "person.identity": personIdentityVM,
  "person.equipment": personEquipmentVM,
  "person.assets": personAssetsVM,
  "person.erasure": personErasureVM,
} as const;
export const presentationV2Command = z.union([
  bodyStateCommand,
  chatCommand,
  diagnosisCommand,
  trainingCommand,
  personAssetCommand,
]);
const cursor = id.optional();
export const presentationV2Query = z.discriminatedUnion("query", [
  z.strictObject({
    query: z.literal("person.records"),
    cursor,
    includeHistory: z.boolean().optional(),
  }),
  z.strictObject({
    query: z.literal("person.habits"),
    cursor,
    includeHistory: z.boolean().optional(),
  }),
  z.strictObject({
    query: z.literal("body.episode"),
    episodeId: id,
    atRevision: revision.optional(),
  }),
  z.strictObject({
    query: z.literal("body.region"),
    regionIds: z.array(id).min(1),
  }),
  z.strictObject({
    query: z.literal("chat.current"),
    afterSequence: z.number().int().nonnegative().optional(),
  }),
  z.strictObject({ query: z.literal("diagnosis.current") }),
  z.strictObject({
    query: z.literal("diagnosis.history"),
    conditionId: id.optional(),
    cursor,
  }),
  z.strictObject({ query: z.literal("diagnosis.confirmation"), runId: id }),
  z.strictObject({ query: z.literal("training.plans"), planId: id.optional() }),
  z.strictObject({ query: z.literal("training.proposal"), proposalId: id }),
  z.strictObject({
    query: z.literal("trainer.current"),
    sessionLogId: id.optional(),
  }),
  z.strictObject({
    query: z.literal("safety.current"),
    planId: id.optional(),
    exerciseId: id.optional(),
  }),
  z.strictObject({ query: z.literal("person.identity") }),
  z.strictObject({ query: z.literal("person.equipment"), cursor }),
  z.strictObject({
    query: z.literal("person.assets"),
    facet: z.enum(["media", "report"]),
    cursor,
  }),
  z.strictObject({ query: z.literal("person.erasure"), jobId: id }),
]);
export type PresentationV2Command = z.infer<typeof presentationV2Command>;
export type PresentationV2Query = z.infer<typeof presentationV2Query>;
