import type { ThreadAssistantMessagePart, ThreadMessageLike } from '@assistant-ui/react';
import type { Message, MessagePart } from '../types/consultation';
import { INITIAL_ACTIVE_TURN_STATE, type ActiveTurnState } from './activeTurnReducer';

type ToolCallPartLike = Extract<MessagePart, { type: 'tool-call' | 'tool_call' }>;
type ToolResultPartLike = Extract<MessagePart, { type: 'tool-result' | 'tool_result' }>;
type JsonPrimitive = string | number | boolean | null;
type JsonValue = JsonPrimitive | JsonObject | JsonValue[];
type JsonObject = { [key: string]: JsonValue };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isJsonValue(value: unknown): value is JsonValue {
  if (
    value === null ||
    typeof value === 'string' ||
    typeof value === 'number' ||
    typeof value === 'boolean'
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

function getToolName(part: ToolCallPartLike | ToolResultPartLike): string {
  const toolName = 'toolName' in part ? part.toolName : undefined;
  return toolName ?? part.tool ?? 'unknown_tool';
}

function getToolCallId(part: ToolCallPartLike | ToolResultPartLike): string | undefined {
  const toolCallId = 'toolCallId' in part ? part.toolCallId : undefined;
  return toolCallId ?? part.tool_call_id ?? undefined;
}

function normalizeAssistantParts(
  message: Message,
): ThreadAssistantMessagePart[] {
  const normalized: ThreadAssistantMessagePart[] = [];
  const runningToolPartIndexById = new Map<string, number>();
  let legacyToolCounter = 0;

  const findLatestOpenToolByName = (toolName: string): number | undefined => {
    for (let index = normalized.length - 1; index >= 0; index -= 1) {
      const part = normalized[index];
      if (part?.type !== 'tool-call') continue;
      if (part.toolName !== toolName) continue;
      if (part.result !== undefined) continue;
      return index;
    }
    return undefined;
  };

  for (const part of message.parts) {
    switch (part.type) {
      case 'text': {
        if (!part.text) continue;
        const previous = normalized[normalized.length - 1];
        if (previous?.type === 'text') {
          normalized[normalized.length - 1] = {
            ...previous,
            text: `${previous.text}${part.text}`,
          };
        } else {
          normalized.push({ type: 'text', text: part.text });
        }
        break;
      }

      case 'source': {
        normalized.push({
          type: 'source',
          sourceType: 'url',
          id: part.id ?? `src-${message.id}-${normalized.length}`,
          title: part.title,
          url: part.url ?? '',
          providerMetadata: asProviderMetadata(part.providerMetadata),
        });
        break;
      }

      case 'data': {
        normalized.push({
          type: 'data',
          name: part.name,
          data: part.data,
        });
        break;
      }

      case 'tool-call':
      case 'tool_call': {
        const toolName = getToolName(part);
        const toolCallId =
          getToolCallId(part) ?? `legacy-tool-${message.id}-${legacyToolCounter++}`;
        const args = asJsonObject(part.args);
        normalized.push({
          type: 'tool-call',
          toolCallId,
          toolName,
          args,
          argsText:
            typeof part.argsText === 'string' ? part.argsText : JSON.stringify(args),
        });
        runningToolPartIndexById.set(toolCallId, normalized.length - 1);
        break;
      }

      case 'tool-result':
      case 'tool_result': {
        const toolName = getToolName(part);
        const toolCallId = getToolCallId(part);
        const exactIndex = toolCallId
          ? runningToolPartIndexById.get(toolCallId)
          : undefined;
        const fallbackIndex =
          exactIndex ?? findLatestOpenToolByName(toolName);

        if (fallbackIndex !== undefined) {
          const existing = normalized[fallbackIndex];
          if (existing?.type === 'tool-call') {
            normalized[fallbackIndex] = {
              ...existing,
              result: part.result,
            };
            break;
          }
        }

        normalized.push({
          type: 'tool-call',
          toolCallId:
            toolCallId ?? `legacy-tool-result-${message.id}-${legacyToolCounter++}`,
          toolName,
          args: {},
          argsText: '',
          result: part.result,
        });
        break;
      }
    }
  }

  return normalized;
}

function toAssistantStatus(message: Message): ThreadMessageLike['status'] {
  switch (message.status) {
    case 'completed':
      return { type: 'complete', reason: 'stop' };
    case 'failed':
      return {
        type: 'incomplete',
        reason: 'error',
        error: message.error
          ? {
              code: message.error.code,
              message: message.error.message,
            }
          : { message: 'message failed' },
      };
    case 'aborted':
      return { type: 'incomplete', reason: 'cancelled' };
    case 'streaming':
    case 'submitted':
      return { type: 'running' };
    default:
      return { type: 'complete', reason: 'unknown' };
  }
}

export function toInitialThreadMessage(message: Message): ThreadMessageLike {
  if (message.role === 'assistant') {
    return {
      id: message.id,
      role: 'assistant',
      content: normalizeAssistantParts(message),
      status: toAssistantStatus(message),
      createdAt: new Date(message.created_at),
      metadata: {
        custom: {
          message_id: message.id,
          consultation_status: message.status,
        },
      },
    };
  }

  return {
    id: message.id,
    role: message.role === 'system' ? 'system' : 'user',
    content:
      message.content_text ||
      message.parts
        .filter((part): part is Extract<MessagePart, { type: 'text' }> => part.type === 'text')
        .map((part) => part.text)
        .join(''),
    createdAt: new Date(message.created_at),
    metadata: {
      custom: {
        message_id: message.id,
        consultation_status: message.status,
      },
    },
  };
}

export function toInitialThreadMessages(messages: readonly Message[]): ThreadMessageLike[] {
  return messages.map(toInitialThreadMessage);
}

export interface InterruptedTurnSeed {
  activeTurn: ActiveTurnState;
  consumedMessageId: string | null;
}

export function buildInterruptedTurnSeed(
  messages: readonly Message[],
  pendingInteractions: ReadonlyArray<{
    id: string;
    run_id: string;
    conversation_id: string;
    tool_call_id: string;
    tool_name: string;
    question: {
      question: string;
      reason?: string;
      answer_type: 'text' | 'single_choice' | 'multi_choice' | 'number' | 'date';
      options?: string[];
      required?: boolean;
      context?: string;
    };
    status: 'pending' | 'answered' | 'cancelled';
    created_at: string;
  }> | undefined,
): InterruptedTurnSeed | null {
  const pending = [...(pendingInteractions ?? [])]
    .filter((interaction) => interaction.status === 'pending')
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))[0];

  if (!pending) {
    return null;
  }

  const sourceMessage =
    [...messages]
      .reverse()
      .find((message) => message.role === 'assistant' && message.status === 'aborted') ??
    null;

  const activeTurn: ActiveTurnState = {
    ...INITIAL_ACTIVE_TURN_STATE,
    toolCallsById: {},
    citationsByKey: {},
    knowledgeGapsByKey: {},
    extractedInfoByBodyPart: {},
    finalParts: [],
    runId: pending.run_id,
    conversationId: pending.conversation_id,
    assistantMessageId: sourceMessage?.id ?? null,
    status: 'interrupted',
    pendingInteraction: pending,
  };

  if (!sourceMessage) {
    return {
      activeTurn,
      consumedMessageId: null,
    };
  }

  const normalizedParts = normalizeAssistantParts(sourceMessage);
  for (const part of normalizedParts) {
    switch (part.type) {
      case 'text':
        activeTurn.text += part.text;
        break;

      case 'tool-call':
        if (part.toolName === 'ask_user') {
          break;
        }
        activeTurn.toolCallsById[part.toolCallId] = {
          id: part.toolCallId,
          tool: part.toolName,
          args: part.args,
          result: part.result,
          status: part.result === undefined ? 'running' : 'completed',
        };
        break;

      case 'source': {
        const metadata = isRecord(part.providerMetadata) ? part.providerMetadata : {};
        const bodysense = isRecord(metadata.bodysense) ? metadata.bodysense : {};
        const title = part.title ?? (part.sourceType === 'url' ? part.url : '') ?? 'Untitled source';
        activeTurn.citationsByKey[title] = {
          title,
          summary:
            typeof bodysense.summary === 'string' ? bodysense.summary : undefined,
          snippet:
            typeof bodysense.snippet === 'string' ? bodysense.snippet : undefined,
          source_title:
            typeof bodysense.source_title === 'string'
              ? bodysense.source_title
              : undefined,
          source_author:
            typeof bodysense.source_author === 'string'
              ? bodysense.source_author
              : undefined,
        };
        break;
      }

      case 'data':
        if (part.name === 'knowledge_gap' && isRecord(part.data)) {
          const query = part.data.query;
          const message = part.data.message;
          if (typeof query === 'string' && typeof message === 'string') {
            activeTurn.knowledgeGapsByKey[query] = { query, message };
          }
        }
        if (part.name === 'red_flag' && isRecord(part.data)) {
          activeTurn.redFlag = part.data as unknown as ActiveTurnState['redFlag'];
        }
        break;
    }
  }

  return {
    activeTurn,
    consumedMessageId: sourceMessage.id,
  };
}
