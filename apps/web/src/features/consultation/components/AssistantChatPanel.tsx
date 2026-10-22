import { useEffect, useLayoutEffect, useMemo, useRef } from "react";
import {
  AssistantRuntimeProvider,
  type ThreadMessageLike,
} from "@assistant-ui/react";
import { useAssistantChatRuntime } from "../hooks/useAssistantChatRuntime";
import {
  ActiveTurnProvider,
  useActiveTurnActions,
  useActiveTurnState,
} from "../context/ActiveTurnContext";
import type {
  Citation,
  ConsultationPhase,
  ConsultationSpatialContext,
  ExtractedInfo,
} from "../types/consultation";
import type { ActiveTurnState } from "../runtime/activeTurnReducer";
import { shouldApplyInitialActiveTurn } from "../runtime/activeTurnSelectors";
import { ChatContent } from "./chat/ChatContent";

function parseConsultationPhase(value: string): ConsultationPhase | null {
  switch (value) {
    case "collecting":
    case "ready_for_analysis":
      return value;
    default:
      return null;
  }
}

interface AssistantChatPanelProps {
  conversationId: string;
  onConversationCreated?: (id: string) => void;
  initialMessages?: ThreadMessageLike[];
  initialActiveTurn?: ActiveTurnState | null;
  initialExtractedInfo?: ExtractedInfo[];
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
  onPhaseChange?: (phase: ConsultationPhase) => void;
  onCitation?: (citation: Citation) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
  onStreamFinished?: () => void;
  spatialContext?: ConsultationSpatialContext | null;
  onClearSpatialContext?: () => void;
  composerFocusKey?: number;
}

/**
 * Bridge that syncs the adapter's active turn state into ActiveTurnContext.
 */
function ActiveTurnBridge({
  bridgeRef,
}: {
  bridgeRef: React.MutableRefObject<((state: ActiveTurnState) => void) | null>;
}) {
  const { hydrateTurn } = useActiveTurnActions();
  // Publish before child passive effects can resume a recovered runtime turn.
  useLayoutEffect(() => {
    bridgeRef.current = hydrateTurn;
    return () => {
      if (bridgeRef.current === hydrateTurn) bridgeRef.current = null;
    };
  }, [bridgeRef, hydrateTurn]);
  return null;
}

/**
 * Chat panel powered by assistant-ui runtime with ActiveTurnState.
 *
 * The runtime drives all SSE streaming via ConsultationChatAdapter.
 * Streaming content is rendered by StreamingAssistantTurn from ActiveTurnState.
 * Historical messages are rendered via assistant-ui's thread state.
 */
export function AssistantChatPanel({
  conversationId,
  initialMessages = [],
  initialActiveTurn = null,
  initialExtractedInfo: _initialExtractedInfo = [],
  onExtractedInfoUpdate,
  onPhaseChange,
  onCitation,
  onMessagePersisted,
  onConversationCreated,
  onStreamFinished,
  spatialContext = null,
  onClearSpatialContext,
  composerFocusKey = 0,
}: AssistantChatPanelProps) {
  const extractedInfoRef = useRef<ExtractedInfo[]>(_initialExtractedInfo);

  useEffect(() => {
    extractedInfoRef.current = _initialExtractedInfo;
  }, [_initialExtractedInfo]);

  const activeTurnRef = useRef<((state: ActiveTurnState) => void) | null>(null);

  const adapterOptions = useMemo(
    () => ({
      onConversationCreated: onConversationCreated
        ? (id: string) => {
            console.debug(
              "[SSE] ⓪ AssistantChatPanel → 桥接 onConversationCreated",
              { id },
            );
            onConversationCreated(id);
          }
        : undefined,
      onExtractedInfoUpdate: (info: ExtractedInfo) => {
        const existing = extractedInfoRef.current;
        const idx = existing.findIndex((e) => e.body_part === info.body_part);
        if (idx >= 0) {
          const updated = [...existing];
          updated[idx] = { ...updated[idx], ...info };
          extractedInfoRef.current = updated;
        } else {
          extractedInfoRef.current = [...existing, info];
        }
        onExtractedInfoUpdate?.(extractedInfoRef.current);
      },
      onPhaseChange: (_from: string, to: string) => {
        const phase = parseConsultationPhase(to);
        if (!phase) {
          console.error(`Ignoring invalid consultation phase: ${to}`);
          return;
        }
        onPhaseChange?.(phase);
      },
      onCitation,
      onMessagePersisted,
      onStreamFinished,
      // Refresh durable projections when a commit arrives, even if the same
      // run then waits for Ask User rather than finishing the stream.
      onBodyFactCommitted: onStreamFinished,
      onActiveTurnUpdate: (state: ActiveTurnState) => {
        activeTurnRef.current?.(state);
      },
    }),
    [
      onConversationCreated,
      onExtractedInfoUpdate,
      onPhaseChange,
      onCitation,
      onMessagePersisted,
      onStreamFinished,
    ],
  );

  const { runtime, resumeInteraction } = useAssistantChatRuntime(
    conversationId,
    initialMessages,
    adapterOptions,
  );

  return (
    <ActiveTurnProvider>
      <ActiveTurnBridge bridgeRef={activeTurnRef} />
      <AssistantRuntimeProvider runtime={runtime}>
        <InitialActiveTurnHydrator initialActiveTurn={initialActiveTurn} />
        <ChatContent
          conversationId={conversationId}
          onResumeInteraction={resumeInteraction}
          spatialContext={spatialContext}
          onClearSpatialContext={onClearSpatialContext}
          composerFocusKey={composerFocusKey}
        />
      </AssistantRuntimeProvider>
    </ActiveTurnProvider>
  );
}

function InitialActiveTurnHydrator({
  initialActiveTurn,
}: {
  initialActiveTurn: ActiveTurnState | null;
}) {
  const { hydrateTurn, resetTurn } = useActiveTurnActions();
  const currentTurn = useActiveTurnState();
  const currentTurnRef = useRef(currentTurn);
  currentTurnRef.current = currentTurn;

  useEffect(() => {
    const current = currentTurnRef.current;
    if (!shouldApplyInitialActiveTurn(current, initialActiveTurn)) {
      return;
    }
    if (initialActiveTurn) {
      hydrateTurn(initialActiveTurn);
      return;
    }
    resetTurn();
  }, [initialActiveTurn, hydrateTurn, resetTurn]);

  return null;
}
