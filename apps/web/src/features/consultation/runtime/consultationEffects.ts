import type { ActiveTurnEffect } from "./activeTurnReducer";
import type { ConsultationAdapterOptions } from "./consultationRuntime.types";

export function applyConsultationEffects(
  effects: ActiveTurnEffect[],
  optionsRef: { current: ConsultationAdapterOptions },
) {
  for (const effect of effects) {
    switch (effect.type) {
      case "conversation_created":
        console.debug(
          "[SSE] ④ applyEffects → 触发 onConversationCreated 回调",
          {
            conversationId: effect.conversationId,
            hasCallback: !!optionsRef.current.onConversationCreated,
          },
        );
        optionsRef.current.onConversationCreated?.(effect.conversationId);
        break;
      case "message_persisted":
        optionsRef.current.onMessagePersisted?.(
          effect.clientMessageId,
          effect.messageId,
        );
        break;
      case "extracted_info_updated":
        optionsRef.current.onExtractedInfoUpdate?.(effect.info);
        break;
      case "phase_changed":
        optionsRef.current.onPhaseChange?.(effect.from, effect.to);
        break;
      case "red_flag":
        optionsRef.current.onRedFlag?.(effect.flags);
        break;
      case "citation_added":
        optionsRef.current.onCitation?.(effect.citation);
        break;
      case "interaction_required":
        optionsRef.current.onInteractionRequired?.(effect.interaction);
        break;
      case "interaction_answered":
        break;
      case "message_completed":
        optionsRef.current.onMessageCompleted?.(effect.data);
        break;
      case "stream_error":
        break;
      default:
        break;
    }
  }
}
