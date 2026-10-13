import { z } from "zod";
import {
  id,
  text,
  instant,
  revision,
  capability,
  recoverableError,
  resourceSchema,
  commandOutcome,
} from "./common";

const choice = z.strictObject({
  value: id,
  label: text,
  description: text.optional(),
});
const questionBase = {
  questionId: id,
  label: text,
  required: z.boolean(),
  help: text.optional(),
};
export const interactionQuestion = z.discriminatedUnion("kind", [
  z.strictObject({
    ...questionBase,
    kind: z.literal("single_choice"),
    choices: z.array(choice).min(1),
    allowExplanation: z.boolean(),
  }),
  z.strictObject({
    ...questionBase,
    kind: z.literal("multiple_choice"),
    choices: z.array(choice).min(1),
    allowExplanation: z.boolean(),
  }),
  z.strictObject({ ...questionBase, kind: z.literal("text") }),
  z.strictObject({
    ...questionBase,
    kind: z.literal("number"),
    unit: text.optional(),
  }),
  z.strictObject({
    ...questionBase,
    kind: z.literal("asset"),
    allowedMimeTypes: z.array(text),
  }),
  z.strictObject({ ...questionBase, kind: z.literal("body_region") }),
]);
export const answerValue = z.discriminatedUnion("kind", [
  z.strictObject({
    kind: z.literal("single_choice"),
    value: id,
    explanation: text.optional(),
  }),
  z.strictObject({
    kind: z.literal("multiple_choice"),
    values: z.array(id),
    explanation: text.optional(),
  }),
  z.strictObject({ kind: z.literal("text"), value: text }),
  z.strictObject({ kind: z.literal("number"), value: z.number() }),
  z.strictObject({ kind: z.literal("asset"), assetIds: z.array(id).min(1) }),
  z.strictObject({
    kind: z.literal("body_region"),
    regionIds: z.array(id).min(1),
  }),
]);
export const answerEntry = z.strictObject({
  questionId: id,
  answer: answerValue,
});
export const interactionBinding = {
  interactionId: id,
  runId: id,
  toolCallId: id,
  interactionRevision: revision,
};
export const pendingInteraction = z.strictObject({
  ...interactionBinding,
  purpose: z.enum([
    "clarification",
    "diagnosis_confirmation",
    "plan_confirmation",
    "safety_reverification",
    "profile_edit",
    "asset_erasure",
  ]),
  title: text,
  questions: z.array(interactionQuestion).min(1),
  draftAnswers: z.array(answerEntry),
  state: z.enum([
    "ready",
    "submitting",
    "failed",
    "stale",
    "expired",
    "recovering",
  ]),
  error: recoverableError.optional(),
  proposalId: id.optional(),
  contentFingerprint: id.optional(),
  canAnswer: capability,
});
export const chatRun = z.discriminatedUnion("state", [
  z.strictObject({ state: z.literal("idle") }),
  z.strictObject({
    state: z.literal("running"),
    runId: id,
    startedAt: instant,
    steps: z.array(
      z.strictObject({
        stepId: id,
        label: text,
        state: z.enum(["pending", "running", "completed", "failed"]),
      }),
    ),
  }),
  z.strictObject({
    state: z.literal("waiting_for_user"),
    runId: id,
    interactionId: id,
  }),
  z.strictObject({
    state: z.literal("failed"),
    runId: id,
    error: recoverableError,
  }),
  z.strictObject({ state: z.literal("cancelled"), runId: id }),
]);
export const chatComposer = z.discriminatedUnion("kind", [
  z.strictObject({
    kind: z.literal("text"),
    draft: z.string(),
    assetIds: z.array(id),
    selectedRegionIds: z.array(id),
    canSend: capability,
  }),
  z.strictObject({
    kind: z.literal("run_progress"),
    runId: id,
    canStop: capability,
  }),
  z.strictObject({
    kind: z.literal("interaction"),
    interaction: pendingInteraction,
    canStop: capability,
  }),
]);
export const messageItem = z.strictObject({
  messageId: id,
  seq: z.number().int().nonnegative(),
  role: z.enum(["user", "assistant"]),
  createdAt: instant,
  runId: id.optional(),
  text: z.string(),
  assetIds: z.array(id),
  outcomes: z.array(commandOutcome),
  delivery: z.enum(["sending", "delivered", "failed", "streaming"]),
});
export const chatData = z
  .strictObject({
    conversationId: id,
    messages: z.array(messageItem),
    run: chatRun,
    composer: chatComposer,
    lastActivityAt: instant.optional(),
    connection: z.enum(["online", "reconnecting", "offline"]),
    history: z.strictObject({
      mode: z.literal("rolling"),
      earlierMessagesPruned: z.boolean(),
      earliestAvailableAt: instant.optional(),
      durableFactsPreserved: z.literal(true),
    }),
  })
  .superRefine((v, ctx) => {
    if (v.run.state === "waiting_for_user") {
      if (
        v.composer.kind !== "interaction" ||
        v.composer.interaction.runId !== v.run.runId ||
        v.composer.interaction.interactionId !== v.run.interactionId
      )
        ctx.addIssue({
          code: "custom",
          message:
            "Ask User replaces the entire composer and resumes the original Run",
        });
    } else if (v.composer.kind === "interaction")
      ctx.addIssue({
        code: "custom",
        message: "An interaction requires waiting_for_user",
      });
    if (
      v.run.state === "running" &&
      (v.composer.kind !== "run_progress" || v.composer.runId !== v.run.runId)
    )
      ctx.addIssue({
        code: "custom",
        message: "A running Chat cannot send a second ordinary message",
      });
    if (v.composer.kind === "run_progress" && v.run.state !== "running")
      ctx.addIssue({
        code: "custom",
        message: "Progress composer requires an active Run",
      });
    if (
      v.composer.kind === "text" &&
      v.connection !== "online" &&
      v.composer.canSend.state === "enabled"
    )
      ctx.addIssue({
        code: "custom",
        message: "Offline draft must not present an available send",
      });
    if (v.messages.some((m, i) => i > 0 && m.seq <= v.messages[i - 1]!.seq))
      ctx.addIssue({
        code: "custom",
        message:
          "Timeline sequence must remain strictly increasing after pruning",
      });
  });
export const chatVM = resourceSchema(chatData);
export const chatCommand = z.discriminatedUnion("operation", [
  z.strictObject({
    operation: z.literal("chat.send"),
    idempotencyKey: id,
    conversationId: id.optional(),
    text,
    assetIds: z.array(id),
    selectedRegionIds: z.array(id),
  }),
  z.strictObject({
    operation: z.literal("chat.answer"),
    idempotencyKey: id,
    ...interactionBinding,
    answers: z.array(answerEntry).min(1),
  }),
  z.strictObject({
    operation: z.literal("chat.cancel_run"),
    idempotencyKey: id,
    runId: id,
    expectedRevision: revision,
  }),
  z.strictObject({
    operation: z.literal("chat.recover"),
    conversationId: id,
    afterSequence: z.number().int().nonnegative().optional(),
  }),
]);
export type ChatVM = z.infer<typeof chatVM>;
export type PendingInteractionVM = z.infer<typeof pendingInteraction>;
export type ChatCommand = z.infer<typeof chatCommand>;
