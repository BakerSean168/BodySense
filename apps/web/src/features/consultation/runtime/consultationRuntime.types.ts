import type { ChatModelRunResult } from "@assistant-ui/react";
import type { ActiveTurnState } from "./activeTurnReducer";
import type {
  ExtractedInfo,
  Citation,
  SSEMessageCompleted,
  RedFlagEvent,
  PendingInteraction,
} from "../types/consultation";

/** Minimal thread handle used to resume HITL runs under assistant-ui 0.15. */
export type ConsultationThreadController = {
  resumeRun: (config: {
    parentId: string | null;
    stream: () => AsyncGenerator<ChatModelRunResult, void, unknown>;
  }) => void;
  getState: () => { messages: readonly { id?: string }[] };
};
export interface ConsultationAdapterOptions {
  onConversationCreated?: (conversationId: string) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
  onExtractedInfoUpdate?: (info: ExtractedInfo) => void;
  onPhaseChange?: (from: string, to: string) => void;
  onRedFlag?: (flag: RedFlagEvent) => void;
  onCitation?: (citation: Citation) => void;
  onMessageCompleted?: (data: SSEMessageCompleted) => void;
  onInteractionRequired?: (interaction: PendingInteraction) => void;
  /** Called on each dispatch with the full active turn state. */
  onActiveTurnUpdate?: (state: ActiveTurnState) => void;
  /** Called when the stream has fully completed or errored/aborted. */
  onStreamFinished?: () => void;
  onBodyFactCommitted?: () => void;
}

export type ConsultationStreamRun = (
  startRequest: () => Promise<Response>,
  preserveUnstartedState?: boolean,
  resumedInteraction?: PendingInteraction,
) => AsyncGenerator<ChatModelRunResult>;
