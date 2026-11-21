/**
 * Content event handlers for the active-turn reducer.
 *
 * 累积型内容：流式文本、结构化状态、来源与安全事件；纯状态迁移，副作用
 * 通过调用方传入的 `effects` 数组交还。
 */

import type { RedFlagEvent, StreamEvent } from "../types/consultation";
import { assertNever } from "./activeTurnHelpers";
import type { ActiveTurnEffect, ActiveTurnState } from "./activeTurnState";

export type ActiveTurnContentEvent = Extract<
  StreamEvent,
  {
    type:
      | "message.text.delta"
      | "state.extracted_info.upsert"
      | "state.phase.changed"
      | "source.citation.added"
      | "source.knowledge_gap"
      | "safety.red_flag.detected";
  }
>;

export function applyContentEvent(
  current: ActiveTurnState,
  event: ActiveTurnContentEvent,
  effects: ActiveTurnEffect[],
): ActiveTurnState {
  switch (event.type) {
    case "message.text.delta": {
      const payload = event.payload;
      return { ...current, text: current.text + payload.delta };
    }

    case "state.extracted_info.upsert": {
      const payload = event.payload;
      const info = payload.info;
      const prev = current.extractedInfoByBodyPart[info.body_part] || {};
      effects.push({ type: "extracted_info_updated", info });
      return {
        ...current,
        extractedInfoByBodyPart: {
          ...current.extractedInfoByBodyPart,
          [info.body_part]: { ...prev, ...info },
        },
      };
    }

    case "state.phase.changed": {
      const payload = event.payload;
      effects.push({
        type: "phase_changed",
        from: payload.from || "",
        to: payload.to,
      });
      return current;
    }

    case "source.citation.added": {
      const payload = event.payload;
      const citation = payload.citation;
      const key = citation.title;
      if (current.citationsByKey[key]) return current;
      effects.push({ type: "citation_added", citation });
      return {
        ...current,
        citationsByKey: { ...current.citationsByKey, [key]: citation },
      };
    }

    case "source.knowledge_gap": {
      const payload = event.payload;
      const key = payload.query;
      if (current.knowledgeGapsByKey[key]) return current;
      return {
        ...current,
        knowledgeGapsByKey: {
          ...current.knowledgeGapsByKey,
          [key]: { query: payload.query, message: payload.message },
        },
      };
    }

    case "safety.red_flag.detected": {
      const payload = event.payload;
      const redFlagEvent: RedFlagEvent = {
        has_red_flags: payload.has_red_flags,
        flags: payload.flags,
      };
      effects.push({ type: "red_flag", flags: redFlagEvent });
      return { ...current, redFlag: redFlagEvent };
    }

    default:
      return assertNever(event);
  }
}
