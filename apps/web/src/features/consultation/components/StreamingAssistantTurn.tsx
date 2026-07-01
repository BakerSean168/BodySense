/**
 * StreamingAssistantTurn — renders the current assistant turn's streaming content.
 *
 * All data is read from ActiveTurnState via selectors. This component replaces
 * the ad-hoc streaming text display, tool call rendering, and structured event
 * extraction that was previously scattered across AssistantChatPanel.
 *
 * Interaction resume (ask_user) is rendered by AssistantChatPanel's input area.
 */

import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useActiveTurnState, useActiveTurnActions } from '../context/ActiveTurnContext';
import { selectActiveTurnViewModel } from '../runtime/activeTurnSelectors';
import { StreamingTurnToolCalls } from './StreamingTurnToolCalls';
import { RedFlagBanner } from './RedFlagBanner';


export function StreamingAssistantTurn() {
  const state = useActiveTurnState();
  const vm = selectActiveTurnViewModel(state);
  const { dismissRedFlag } = useActiveTurnActions();

  // Don't render if there's nothing to show
  if (!vm.hasVisibleContent && !vm.isRunning && !vm.isInterrupted) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3">
      {/* Tool calls */}
      <StreamingTurnToolCalls toolCalls={vm.toolCalls} />

      {/* Streaming markdown text */}
      {vm.streamingMarkdown && (
        <div className="flex justify-start">
          <div className="max-w-[80%] rounded-[20px] px-4 py-3 bg-[#F7F5F0] text-[#1A221E] rounded-bl-[4px] border border-[#E5E3DF]">
            <div className="text-sm leading-relaxed font-medium prose-markdown">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {vm.streamingMarkdown}
              </ReactMarkdown>
            </div>
          </div>
        </div>
      )}

      {/* Loading indicator when running but no text yet */}
      {vm.isRunning && !vm.streamingMarkdown && vm.toolCalls.length === 0 && (
        <div className="flex justify-start">
          <div className="max-w-[80%] rounded-[20px] px-4 py-3 bg-[#F7F5F0] rounded-bl-[4px] border border-[#E5E3DF]">
            <div className="flex items-center gap-1.5 py-1 px-1">
              <span className="w-2 h-2 rounded-full bg-[#709a83] animate-bounce [animation-delay:-0.3s]" />
              <span className="w-2 h-2 rounded-full bg-[#709a83] animate-bounce [animation-delay:-0.15s]" />
              <span className="w-2 h-2 rounded-full bg-[#709a83] animate-bounce" />
            </div>
          </div>
        </div>
      )}

      {/* Citations */}
      {vm.citations.length > 0 && (
        <div className="flex justify-start">
          <div className="max-w-[80%] rounded-xl px-3 py-2 bg-[#EEF2EE] border border-[#D4DDD4]">
            <p className="text-xs font-semibold text-[#5A7A64] mb-1">参考知识</p>
            <div className="flex flex-wrap gap-1.5">
              {vm.citations.map((c) => (
                <span
                  key={c.title}
                  className="inline-block rounded-full bg-white px-2.5 py-0.5 text-xs text-[#3D5A47] border border-[#C8D8CC]"
                  title={c.summary || c.content || ''}
                >
                  {c.title}
                </span>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Knowledge gaps */}
      {vm.knowledgeGaps.length > 0 && (
        <div className="flex justify-start">
          <div className="max-w-[80%] rounded-xl px-3 py-2 bg-[#FFF8F0] border border-[#F0D4B0]">
            <div className="flex items-start gap-2">
              <svg className="w-4 h-4 text-[#D4864A] mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <div>
                <p className="text-xs font-semibold text-[#A06030]">知识库提示</p>
                <p className="text-xs text-[#8B6A4A] mt-0.5">
                  知识库中暂未收录「{vm.knowledgeGaps.map((g) => g.query).join('」「')}」的专项资料，以下建议仅供参考。
                </p>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Red flag */}
      {vm.redFlag?.has_red_flags && (
        <RedFlagBanner
          redFlags={vm.redFlag.flags}
          onAcknowledge={dismissRedFlag}
        />
      )}

      {/* Ask user card is now rendered in AssistantChatPanel's input area */}
    </div>
  );
}
