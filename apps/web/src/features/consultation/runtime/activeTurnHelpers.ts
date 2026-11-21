/**
 * Reducer-wide helpers shared by the active-turn event handlers.
 */

import type { ThreadAssistantMessagePart } from "@assistant-ui/react";
import type { ActiveTurnState } from "./activeTurnState";

export function assertNever(event: never): never {
  throw new Error(`Unhandled validated StreamEvent: ${JSON.stringify(event)}`);
}

/** Build final message parts from the accumulated active turn state. */
export function buildFinalMessageParts(
  state: ActiveTurnState,
): ThreadAssistantMessagePart[] {
  const parts: ThreadAssistantMessagePart[] = [];

  if (state.text) {
    parts.push({ type: "text", text: state.text });
  }

  for (const citation of Object.values(state.citationsByKey)) {
    const bodysenseMetadata: Record<string, string> = {};
    if (citation.summary) bodysenseMetadata.summary = citation.summary;
    if (citation.snippet) bodysenseMetadata.snippet = citation.snippet;
    if (citation.source_title)
      bodysenseMetadata.source_title = citation.source_title;
    if (citation.source_author)
      bodysenseMetadata.source_author = citation.source_author;
    parts.push({
      type: "source",
      sourceType: "url",
      id: `src_${crypto.randomUUID().slice(0, 8)}`,
      url: citation.url || "",
      title: citation.title,
      providerMetadata:
        Object.keys(bodysenseMetadata).length > 0
          ? { bodysense: bodysenseMetadata }
          : undefined,
    });
  }

  if (state.redFlag?.has_red_flags) {
    parts.push({ type: "data", name: "red_flag", data: state.redFlag });
  }

  for (const gap of Object.values(state.knowledgeGapsByKey)) {
    parts.push({ type: "data", name: "knowledge_gap", data: gap });
  }

  return parts;
}
