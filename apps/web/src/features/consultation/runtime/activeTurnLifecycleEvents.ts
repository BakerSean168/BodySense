/**
 * Lifecycle event handlers for the active-turn reducer.
 *
 * 会话、运行与消息的生命周期事件；纯状态迁移，副作用通过调用方传入的
 * `effects` 数组交还。
 */

import type { StreamEvent } from "../types/consultation";
import { assertNever } from "./activeTurnHelpers";
import type { ActiveTurnEffect, ActiveTurnState } from "./activeTurnState";
import { EXECUTION_LOST_USER_MESSAGE } from "./activeTurnState";

export type ActiveTurnLifecycleEvent = Extract<
  StreamEvent,
  {
    type:
      | "conversation.created"
      | "run.started"
      | "run.resumed"
      | "run.interrupted"
      | "run.completed"
      | "run.failed"
      | "run.cancelled"
      | "message.persisted"
      | "message.created";
  }
>;

export function applyLifecycleEvent(
  current: ActiveTurnState,
  event: ActiveTurnLifecycleEvent,
  effects: ActiveTurnEffect[],
): ActiveTurnState {
  switch (event.type) {
    case "conversation.created": {
      const conversationId = event.ids.conversation_id || "";
      effects.push({
        type: "conversation_created",
        conversationId,
      });
      return {
        ...current,
        conversationId,
        runId: event.ids.run_id || null,
        status: "streaming",
        error: undefined,
      };
    }

    case "run.started":
    case "run.resumed": {
      return {
        ...current,
        conversationId: event.ids.conversation_id || current.conversationId,
        runId: event.ids.run_id || current.runId,
        status: "streaming",
        pendingInteraction:
          event.type === "run.resumed" ? null : current.pendingInteraction,
        // A new Run is a new turn; a resumed Run keeps this turn's answers.
        answeredInteractions:
          event.type === "run.started"
            ? []
            : (current.answeredInteractions ?? []),
        error: undefined,
      };
    }

    case "run.interrupted": {
      return { ...current, status: "interrupted" };
    }

    case "run.completed": {
      return { ...current, runId: event.ids.run_id || current.runId };
    }

    case "run.failed": {
      const payload = event.payload;
      const executionLost = payload.reason === "execution_lost";
      return {
        ...current,
        runId: event.ids.run_id || current.runId,
        status: "failed",
        pendingInteraction: null,
        error: executionLost
          ? EXECUTION_LOST_USER_MESSAGE
          : payload.error?.message ||
            current.error ||
            "本次执行未能完成，请继续输入后重试。",
      };
    }

    case "run.cancelled": {
      const payload = event.payload;
      return {
        ...current,
        runId: event.ids.run_id || current.runId,
        status: "cancelled",
        error: payload.reason,
        pendingInteraction: null,
      };
    }

    case "message.persisted": {
      const payload = event.payload;
      const messageId = event.ids.message_id || "";
      effects.push({
        type: "message_persisted",
        clientMessageId: payload.client_message_id,
        messageId,
      });
      return { ...current };
    }

    case "message.created": {
      const messageId = event.ids.message_id || "";
      // A resumed Run can own several response segments. Never concatenate the
      // prior segment's text, tools or sources into the new assistant message.
      const newSegment = current.assistantMessageId !== messageId;
      return {
        ...current,
        ...(newSegment
          ? {
              text: "",
              toolCallsById: {},
              citationsByKey: {},
              knowledgeGapsByKey: {},
              finalParts: [],
            }
          : {}),
        assistantMessageId: messageId,
        status: "streaming",
      };
    }

    default:
      return assertNever(event);
  }
}
