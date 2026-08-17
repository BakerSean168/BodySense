import type { ThreadAssistantMessagePart } from "@assistant-ui/react";
import type {
  Citation,
  RedFlagEvent,
  ToolCallInfo,
} from "../types/consultation";

export interface AssistantMessagePartsViewModel {
  markdown: string;
  toolCalls: ToolCallInfo[];
  citations: Citation[];
  knowledgeGaps: Array<{ query: string; message: string }>;
  redFlag: RedFlagEvent | null;
  hasRenderableContent: boolean;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function buildAssistantMessagePartsViewModel(
  parts: readonly ThreadAssistantMessagePart[],
): AssistantMessagePartsViewModel {
  const markdownParts: string[] = [];
  const toolCalls: ToolCallInfo[] = [];
  const citations: Citation[] = [];
  const knowledgeGaps: Array<{ query: string; message: string }> = [];
  let redFlag: RedFlagEvent | null = null;

  for (const part of parts) {
    switch (part.type) {
      case "text":
        markdownParts.push(part.text);
        break;

      case "tool-call":
        if (part.toolName === "ask_user") {
          break;
        }
        toolCalls.push({
          id: part.toolCallId,
          tool: part.toolName,
          args: part.args,
          result: part.result,
          status: part.result === undefined ? "running" : "completed",
        });
        break;

      case "source": {
        const metadata = isRecord(part.providerMetadata)
          ? part.providerMetadata
          : {};
        const bodysense = isRecord(metadata.bodysense)
          ? metadata.bodysense
          : {};
        citations.push({
          title: part.title ?? part.url,
          source_title:
            typeof bodysense.source_title === "string"
              ? bodysense.source_title
              : undefined,
          source_author:
            typeof bodysense.source_author === "string"
              ? bodysense.source_author
              : undefined,
          summary:
            typeof bodysense.summary === "string"
              ? bodysense.summary
              : undefined,
          snippet:
            typeof bodysense.snippet === "string"
              ? bodysense.snippet
              : undefined,
          url: part.sourceType === "url" ? part.url : undefined,
        } as Citation & { url?: string });
        break;
      }

      case "data":
        if (part.name === "red_flag" && isRecord(part.data)) {
          redFlag = part.data as unknown as RedFlagEvent;
        }
        if (part.name === "knowledge_gap" && isRecord(part.data)) {
          const query = part.data.query;
          const message = part.data.message;
          if (typeof query === "string" && typeof message === "string") {
            knowledgeGaps.push({ query, message });
          }
        }
        break;
    }
  }

  const hasRenderableContent =
    markdownParts.join("").length > 0 ||
    toolCalls.length > 0 ||
    citations.length > 0 ||
    knowledgeGaps.length > 0 ||
    redFlag !== null;

  return {
    markdown: markdownParts.join(""),
    toolCalls,
    citations,
    knowledgeGaps,
    redFlag,
    hasRenderableContent,
  };
}
