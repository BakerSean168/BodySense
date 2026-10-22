/**
 * StreamingAssistantTurn — renders the current assistant turn's streaming content.
 *
 * All data is read from ActiveTurnState via selectors. This component replaces
 * the ad-hoc streaming text display, tool call rendering, and structured event
 * extraction that was previously scattered across AssistantChatPanel.
 */

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  Message,
  MessageContent,
  MessageResponse,
} from "@/components/ai-elements/message";
import { Source, SourceList, Sources } from "@/components/ai-elements/sources";
import {
  useActiveTurnState,
  useActiveTurnActions,
} from "../context/ActiveTurnContext";
import { selectActiveTurnViewModel } from "../runtime/activeTurnSelectors";
import { StreamingTurnToolCalls } from "./StreamingTurnToolCalls";
import { RedFlagBanner } from "./RedFlagBanner";
import { AskUserStatusCard } from "./AskUserStatusCard";
import { AskUserCard } from "./AskUserCard";
import { FailedRunStatusCard } from "./FailedRunStatusCard";
import { CancelledRunStatusCard } from "./CancelledRunStatusCard";

interface StreamingAssistantTurnProps {
  pendingInteractionInComposer?: boolean;
  onInteractionSubmit?: (answer: unknown) => void;
  isInteractionSubmitting?: boolean;
  interactionError?: string | null;
  onInteractionRetry?: () => void;
}

export function StreamingAssistantTurn({
  pendingInteractionInComposer = false,
  onInteractionSubmit,
  isInteractionSubmitting = false,
  interactionError = null,
  onInteractionRetry,
}: StreamingAssistantTurnProps = {}) {
  const state = useActiveTurnState();
  const vm = selectActiveTurnViewModel(state);
  const { dismissRedFlag } = useActiveTurnActions();

  if (
    !vm.hasVisibleContent &&
    !vm.isRunning &&
    !vm.isInterrupted &&
    !vm.isFailed &&
    !vm.isCancelled
  ) {
    return null;
  }

  const shouldRenderTimelineShell =
    vm.hasRenderableContent ||
    (vm.pendingInteraction !== null &&
      vm.pendingInteraction.status === "answered") ||
    (vm.isInterrupted && vm.pendingInteraction !== null) ||
    vm.answeredInteractions.length > 0 ||
    (vm.isRunning && !vm.streamingMarkdown && vm.toolCalls.length === 0) ||
    vm.isFailed ||
    vm.isCancelled;

  if (!shouldRenderTimelineShell) return null;

  return (
    <div className="flex flex-col gap-4">
      {vm.answeredInteractions.map((interaction) => (
        <AskUserStatusCard key={interaction.id} interaction={interaction} />
      ))}

      {vm.pendingInteraction &&
      !(
        pendingInteractionInComposer &&
        vm.pendingInteraction.status === "pending"
      ) ? (
        <AskUserStatusCard interaction={vm.pendingInteraction} />
      ) : null}

      {vm.isFailed ? <FailedRunStatusCard message={vm.error} /> : null}
      {vm.isCancelled ? <CancelledRunStatusCard /> : null}

      {!pendingInteractionInComposer &&
      vm.pendingInteraction?.status === "pending" &&
      onInteractionSubmit ? (
        <div className="w-full max-w-[620px]">
          <AskUserCard
            title={
              vm.pendingInteraction.question.purpose === "symptom_intake"
                ? "补全这条身体记录"
                : "还需要确认一件事"
            }
            question={vm.pendingInteraction.question}
            onSubmit={onInteractionSubmit}
            isSubmitting={isInteractionSubmitting}
            error={interactionError}
            onRetry={onInteractionRetry}
          />
        </div>
      ) : null}

      <StreamingTurnToolCalls toolCalls={vm.toolCalls} />

      {vm.streamingMarkdown ? (
        <Message from="assistant">
          <MessageContent>
            <MessageResponse className="conversation-prose prose-markdown">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {vm.streamingMarkdown}
              </ReactMarkdown>
            </MessageResponse>
          </MessageContent>
        </Message>
      ) : null}

      {vm.isRunning && !vm.streamingMarkdown && vm.toolCalls.length === 0 ? (
        // Penpot Chat / Running: a quiet pill with three graded dots and what
        // is happening; "继续分析" once the user answered a question this turn.
        <div
          className="chat-thinking flex h-9 w-fit items-center gap-1.5 rounded-xl px-3.5 text-[13px] text-white/40"
          role="status"
          aria-label="BodySense 正在思考"
        >
          <span className="chat-thinking-dot size-1.5 animate-pulse rounded-full bg-current [animation-delay:-240ms]" />
          <span className="chat-thinking-dot size-1.5 animate-pulse rounded-full bg-current [animation-delay:-120ms]" />
          <span className="chat-thinking-dot size-1.5 animate-pulse rounded-full bg-current" />
          <span className="chat-thinking-label ml-1">
            {vm.answeredInteractions.length > 0 ||
            vm.pendingInteraction?.status === "answered"
              ? "继续分析"
              : "正在分析"}
          </span>
        </div>
      ) : null}

      <Sources count={vm.citations.length}>
        <SourceList>
          {vm.citations.map((citation) => (
            <Source
              key={citation.title}
              title={citation.summary || citation.content || ""}
            >
              <p className="chat-source-title font-medium text-white/78">
                {citation.title}
              </p>
              {citation.summary ? (
                <p className="chat-muted mt-0.5 line-clamp-2 text-white/42">
                  {citation.summary}
                </p>
              ) : null}
            </Source>
          ))}
        </SourceList>
      </Sources>

      {vm.knowledgeGaps.length > 0 ? (
        <div className="chat-gap rounded-xl border border-amber-300/10 bg-amber-300/[0.045] px-3 py-2.5">
          <p className="chat-gap-title text-xs font-medium text-amber-200/75">
            还缺少一些专项资料
          </p>
          <p className="chat-gap-text mt-1 text-xs leading-5 text-amber-100/48">
            当前知识库暂未覆盖「
            {vm.knowledgeGaps.map((gap) => gap.query).join("」「")}
            」。
          </p>
        </div>
      ) : null}

      {vm.redFlag?.has_red_flags ? (
        <RedFlagBanner
          redFlags={vm.redFlag.flags}
          onAcknowledge={dismissRedFlag}
        />
      ) : null}
    </div>
  );
}
