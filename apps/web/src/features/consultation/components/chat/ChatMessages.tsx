import { useEffect, useMemo, useState } from "react";
import {
  MessagePrimitive,
  useAuiState,
  type ThreadAssistantMessagePart,
} from "@assistant-ui/react";
import { MarkdownTextPrimitive } from "@assistant-ui/react-markdown";
import remarkGfm from "remark-gfm";
import { EXECUTION_LOST_USER_MESSAGE } from "../../runtime/activeTurnReducer";
import { buildAssistantMessagePartsViewModel } from "../../runtime/assistantMessagePartsViewModel";
import { selectLiveAnsweredInteractions } from "../../runtime/activeTurnSelectors";
import {
  parsePendingInteraction,
  readSuggestionHistory,
} from "../../runtime/pendingInteractionProjection";
import { useActiveTurnState } from "../../context/ActiveTurnContext";
import { FailedRunStatusCard } from "../FailedRunStatusCard";
import { CancelledRunStatusCard } from "../CancelledRunStatusCard";
import { StreamingTurnToolCalls } from "../StreamingTurnToolCalls";
import { RedFlagBanner } from "../RedFlagBanner";
import { AskUserStatusCard } from "../AskUserStatusCard";
import {
  Message,
  MessageContent,
  MessageResponse,
} from "@/components/ai-elements/message";
import { Source, SourceList, Sources } from "@/components/ai-elements/sources";

const EMPTY_ASSISTANT_CONTENT: readonly ThreadAssistantMessagePart[] = [];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function readConsultationError(value: unknown): {
  code?: string;
  message?: string;
} | null {
  if (!isRecord(value)) return null;
  const code = typeof value.code === "string" ? value.code : undefined;
  const message = typeof value.message === "string" ? value.message : undefined;
  return code || message ? { code, message } : null;
}

function readHistoricalInteraction(value: unknown) {
  try {
    return parsePendingInteraction(value);
  } catch (error) {
    console.error("Invalid assistant-ui interaction metadata", error);
    return null;
  }
}
export function CustomUserMessage() {
  const customMetadata = useAuiState((state) => state.message.metadata.custom);
  // The Chat header's day picker scrolls to a day's first message by id.
  const messageId = useAuiState((state) => state.message.id);

  // If this message has metadata indicating it's an interaction answer, hide it.
  const isInteractionAnswer = customMetadata.is_interaction_answer === true;
  if (isInteractionAnswer) {
    return null;
  }

  return (
    <MessagePrimitive.Root className="w-full" data-message-id={messageId}>
      <Message from="user">
        <MessageContent>
          <div className="whitespace-pre-wrap">
            <MessagePrimitive.Content />
          </div>
        </MessageContent>
      </Message>
    </MessagePrimitive.Root>
  );
}

/**
 * Renders a completed assistant message using assistant-ui's built-in markdown renderer.
 * Streaming display is handled by StreamingAssistantTurn separately.
 */
export function CustomAssistantMessage() {
  const content = useAuiState((state) =>
    state.message.role === "assistant"
      ? state.message.content
      : EMPTY_ASSISTANT_CONTENT,
  );
  const isLast = useAuiState((state) => state.message.isLast);
  const customMetadata = useAuiState((state) => state.message.metadata.custom);
  const activeTurn = useActiveTurnState();

  const viewModel = useMemo(
    () => buildAssistantMessagePartsViewModel(content),
    [content],
  );
  const answeredSuggestions = useMemo(
    () => readSuggestionHistory(customMetadata.body_state_suggestions),
    [customMetadata.body_state_suggestions],
  );
  const [isRedFlagDismissed, setIsRedFlagDismissed] = useState(false);

  useEffect(() => {
    setIsRedFlagDismissed(false);
  }, [content]);

  const isTurnActive =
    activeTurn.status === "streaming" || activeTurn.status === "interrupted";

  const historicalInteraction =
    customMetadata.interaction_history === true
      ? readHistoricalInteraction(customMetadata.interaction)
      : null;
  if (historicalInteraction) {
    // An answered question stays in the thread while the Run resumes (Penpot
    // Chat / Running). Skip it only when the live turn already renders it.
    const shownLive =
      isTurnActive &&
      (activeTurn.pendingInteraction?.id === historicalInteraction.id ||
        selectLiveAnsweredInteractions(activeTurn).some(
          (item) => item.id === historicalInteraction.id,
        ));
    return shownLive ? null : (
      <AskUserStatusCard interaction={historicalInteraction} />
    );
  }

  if (isLast && isTurnActive) {
    return null;
  }

  const consultationStatus =
    typeof customMetadata.consultation_status === "string"
      ? customMetadata.consultation_status
      : undefined;
  const consultationError = readConsultationError(
    customMetadata.consultation_error,
  );

  if (
    consultationStatus === "failed" &&
    consultationError?.code === "execution_lost"
  ) {
    return <FailedRunStatusCard message={EXECUTION_LOST_USER_MESSAGE} />;
  }

  if (consultationStatus === "aborted") {
    return <CancelledRunStatusCard />;
  }

  const suggestionCards = answeredSuggestions.map((interaction) => (
    <AskUserStatusCard key={interaction.id} interaction={interaction} />
  ));

  if (!viewModel.hasRenderableContent) {
    return suggestionCards.length > 0 ? (
      <div className="flex flex-col gap-4">{suggestionCards}</div>
    ) : null;
  }

  return (
    <MessagePrimitive.Root
      className={
        suggestionCards.length > 0 ? "flex w-full flex-col gap-4" : "w-full"
      }
    >
      {suggestionCards}
      <Message from="assistant">
        <MessageContent className="gap-3">
          <StreamingTurnToolCalls toolCalls={viewModel.toolCalls} />

          <MessageResponse className="conversation-prose prose-markdown">
            <MessagePrimitive.Content
              components={{
                Text: () => (
                  <MarkdownTextPrimitive
                    smooth={false}
                    remarkPlugins={[remarkGfm]}
                  />
                ),
                Source: () => null,
                File: () => null,
                Image: ({ image }) => {
                  if (!image) return null;
                  return (
                    <img
                      src={image}
                      alt="用户上传"
                      className="chat-image mt-2 max-h-48 rounded-xl border border-white/10 object-contain"
                    />
                  );
                },
                Reasoning: () => null,
                data: { Fallback: () => null },
                tools: { Fallback: () => null },
              }}
            />
          </MessageResponse>

          <Sources count={viewModel.citations.length}>
            <SourceList>
              {viewModel.citations.map((citation) => (
                <Source
                  key={citation.title}
                  title={
                    citation.summary ||
                    citation.snippet ||
                    citation.content ||
                    ""
                  }
                >
                  <p className="chat-source-title font-medium text-white/78">
                    {citation.title}
                  </p>
                  {citation.summary || citation.snippet ? (
                    <p className="chat-muted mt-0.5 line-clamp-2 text-white/42">
                      {citation.summary || citation.snippet}
                    </p>
                  ) : null}
                </Source>
              ))}
            </SourceList>
          </Sources>

          {viewModel.knowledgeGaps.length > 0 ? (
            <div className="chat-gap rounded-xl border border-amber-300/10 bg-amber-300/[0.045] px-3 py-2.5">
              <p className="chat-gap-title text-xs font-medium text-amber-200/75">
                还缺少一些专项资料
              </p>
              <p className="chat-gap-text mt-1 text-xs leading-5 text-amber-100/48">
                当前知识库暂未覆盖「
                {viewModel.knowledgeGaps.map((gap) => gap.query).join("」「")}
                」。
              </p>
            </div>
          ) : null}

          {viewModel.redFlag?.has_red_flags && !isRedFlagDismissed ? (
            <RedFlagBanner
              redFlags={viewModel.redFlag.flags}
              onAcknowledge={() => setIsRedFlagDismissed(true)}
            />
          ) : null}
        </MessageContent>
      </Message>
    </MessagePrimitive.Root>
  );
}
