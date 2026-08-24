import type {
  ThreadAssistantMessagePart,
  ThreadMessageLike,
} from "@assistant-ui/react";
import type {
  InteractionHistoryItem,
  Message,
  MessagePart,
  PendingInteraction,
  StreamEvent,
} from "../types/consultation";
import {
  INITIAL_ACTIVE_TURN_STATE,
  reduceActiveTurnEvent,
  type ActiveTurnState,
} from "./activeTurnReducer";

type JsonPrimitive = string | number | boolean | null;
type JsonValue = JsonPrimitive | JsonObject | JsonValue[];
type JsonObject = { [key: string]: JsonValue };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isJsonValue(value: unknown): value is JsonValue {
  if (
    value === null ||
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean"
  ) {
    return true;
  }

  if (Array.isArray(value)) {
    return value.every(isJsonValue);
  }

  if (isRecord(value)) {
    return Object.values(value).every(isJsonValue);
  }

  return false;
}

function asJsonObject(value: unknown): JsonObject {
  if (!isRecord(value)) {
    return {};
  }

  const result: JsonObject = {};
  for (const [key, entryValue] of Object.entries(value)) {
    if (isJsonValue(entryValue)) {
      result[key] = entryValue;
    }
  }
  return result;
}

function asProviderMetadata(
  value: unknown,
): Record<string, JsonObject> | undefined {
  if (!isRecord(value)) {
    return undefined;
  }

  const result: Record<string, JsonObject> = {};
  for (const [key, entryValue] of Object.entries(value)) {
    if (!isRecord(entryValue)) {
      continue;
    }
    result[key] = asJsonObject(entryValue);
  }

  return Object.keys(result).length > 0 ? result : undefined;
}

function normalizeAssistantParts(
  message: Message,
): ThreadAssistantMessagePart[] {
  const normalized: ThreadAssistantMessagePart[] = [];
  const runningToolPartIndexById = new Map<string, number>();
  for (const part of message.parts) {
    switch (part.type) {
      case "text": {
        if (!part.text) continue;
        const previous = normalized[normalized.length - 1];
        if (previous?.type === "text") {
          normalized[normalized.length - 1] = {
            ...previous,
            text: `${previous.text}${part.text}`,
          };
        } else {
          normalized.push({ type: "text", text: part.text });
        }
        break;
      }

      case "source": {
        normalized.push({
          type: "source",
          sourceType: "url",
          id: part.id ?? `src-${message.id}-${normalized.length}`,
          title: part.title,
          url: part.url ?? "",
          providerMetadata: asProviderMetadata(part.providerMetadata),
        });
        break;
      }

      case "data": {
        normalized.push({
          type: "data",
          name: part.name,
          data: part.data,
        });
        break;
      }

      case "tool-call": {
        const args = asJsonObject(part.args);
        normalized.push({
          type: "tool-call",
          toolCallId: part.toolCallId,
          toolName: part.toolName,
          args,
          argsText:
            typeof part.argsText === "string"
              ? part.argsText
              : JSON.stringify(args),
        });
        runningToolPartIndexById.set(part.toolCallId, normalized.length - 1);
        break;
      }

      case "tool-result": {
        const toolPartIndex = runningToolPartIndexById.get(part.toolCallId);
        if (toolPartIndex !== undefined) {
          const existing = normalized[toolPartIndex];
          if (existing?.type === "tool-call") {
            normalized[toolPartIndex] = {
              ...existing,
              result: part.result,
            };
            break;
          }
        }

        normalized.push({
          type: "tool-call",
          toolCallId: part.toolCallId,
          toolName: part.toolName,
          args: {},
          argsText: "",
          result: part.result,
        });
        break;
      }
    }
  }

  return normalized;
}

function toAssistantStatus(message: Message): ThreadMessageLike["status"] {
  switch (message.status) {
    case "completed":
      return { type: "complete", reason: "stop" };
    case "failed":
      return {
        type: "incomplete",
        reason: "error",
        error: message.error
          ? {
              code: message.error.code,
              message: message.error.message,
            }
          : { message: "message failed" },
      };
    case "aborted":
      return { type: "incomplete", reason: "cancelled" };
    case "streaming":
    case "submitted":
      return { type: "running" };
    default:
      return { type: "complete", reason: "unknown" };
  }
}

export function toInitialThreadMessage(message: Message): ThreadMessageLike {
  const sourceMetadata = isRecord(message.metadata) ? message.metadata : {};

  if (message.role === "assistant") {
    return {
      id: message.id,
      role: "assistant",
      content: normalizeAssistantParts(message),
      status: toAssistantStatus(message),
      createdAt: new Date(message.created_at),
      metadata: {
        custom: {
          message_id: message.id,
          consultation_status: message.status,
          consultation_error: message.error ?? undefined,
          ...sourceMetadata,
        },
      },
    };
  }

  return {
    id: message.id,
    role: message.role === "system" ? "system" : "user",
    content:
      message.content_text ||
      message.parts
        .filter(
          (part): part is Extract<MessagePart, { type: "text" }> =>
            part.type === "text",
        )
        .map((part) => part.text)
        .join(""),
    createdAt: new Date(message.created_at),
    metadata: {
      custom: {
        message_id: message.id,
        consultation_status: message.status,
        ...sourceMetadata,
      },
    },
  };
}

export function toInitialThreadMessages(
  messages: readonly Message[],
): ThreadMessageLike[] {
  return messages.map(toInitialThreadMessage);
}

function toInteractionHistoryMessage(
  interaction: InteractionHistoryItem,
): ThreadMessageLike {
  return {
    id: `interaction-${interaction.id}`,
    role: "assistant",
    content: [],
    status: { type: "complete", reason: "stop" },
    createdAt: new Date(interaction.answered_at || interaction.created_at),
    metadata: {
      custom: {
        interaction_history: true,
        interaction,
      },
    },
  };
}

export function toInitialThreadTimeline(
  messages: readonly Message[],
  interactionHistory: readonly InteractionHistoryItem[] = [],
): ThreadMessageLike[] {
  const timeline = [
    ...messages.map(toInitialThreadMessage),
    ...interactionHistory
      .filter((interaction) => interaction.status !== "pending")
      .map(toInteractionHistoryMessage),
  ];

  timeline.sort((a, b) => {
    const aTime = a.createdAt instanceof Date ? a.createdAt.getTime() : 0;
    const bTime = b.createdAt instanceof Date ? b.createdAt.getTime() : 0;
    return aTime - bTime;
  });

  return timeline;
}

export interface ActiveTurnSeed {
  activeTurn: ActiveTurnState;
  consumedMessageId: string | null;
}

export function buildActiveTurnSeedFromRuntimeEvents(
  events: readonly StreamEvent[] | undefined,
  pendingInteractions?: readonly PendingInteraction[],
): ActiveTurnSeed | null {
  if (!events || events.length === 0) {
    return null;
  }

  let activeTurn = { ...INITIAL_ACTIVE_TURN_STATE };
  const sortedEvents = [...events].sort((a, b) => a.seq - b.seq);
  for (const event of sortedEvents) {
    activeTurn = reduceActiveTurnEvent(activeTurn, event).state;
  }

  if (
    activeTurn.status !== "streaming" &&
    activeTurn.status !== "interrupted"
  ) {
    return null;
  }

  const projectedPending = selectProjectedPendingInteraction(
    pendingInteractions,
    activeTurn.pendingInteraction?.id ?? null,
    activeTurn.runId,
  );
  if (projectedPending) {
    activeTurn = {
      ...activeTurn,
      pendingInteraction: projectedPending,
      status:
        projectedPending.status === "pending"
          ? "interrupted"
          : activeTurn.status,
    };
  }

  return {
    activeTurn,
    consumedMessageId: activeTurn.assistantMessageId,
  };
}

function selectProjectedPendingInteraction(
  pendingInteractions: readonly PendingInteraction[] | undefined,
  interactionId: string | null,
  runId: string | null,
): PendingInteraction | null {
  const pending = pendingInteractions ?? [];
  if (pending.length === 0) {
    return null;
  }

  if (interactionId) {
    const exact = pending.find(
      (interaction) => interaction.id === interactionId,
    );
    if (exact) {
      return exact;
    }
  }

  if (runId) {
    const byRun = pending
      .filter((interaction) => interaction.run_id === runId)
      .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))[0];
    if (byRun) {
      return byRun;
    }
  }

  return (
    pending
      .filter((interaction) => interaction.status === "pending")
      .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))[0] ??
    null
  );
}
