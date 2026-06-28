import { AssistantRuntimeProvider, useThread, useComposerRuntime } from "@assistant-ui/react";
import { useAssistantChatRuntime } from "../hooks/useAssistantChatRuntime";
import { useRef, useEffect, useState, useCallback, useMemo } from "react";
import type { Citation, ConsultationPhase, ExtractedInfo, RedFlagEvent } from "../types/consultation";
import { RedFlagBanner } from "./RedFlagBanner";

interface AssistantChatPanelProps {
  conversationId: string | null;
  initialMessages?: ChatMessage[];
  initialExtractedInfo?: ExtractedInfo[];
  isDraft?: boolean;
  clientDraftId?: string | null;
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
  onPhaseChange?: (phase: ConsultationPhase) => void;
  onCitation?: (citation: Citation) => void;
  onConversationCreated?: (conversationId: string) => void;
  onTitleGenerated?: (title: string) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
}

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
}

/**
 * Chat panel powered by assistant-ui runtime.
 *
 * The runtime drives all SSE streaming via ConsultationChatAdapter.
 * Messages are managed by assistant-ui's thread state. Custom UI renders
 * messages from the thread while the adapter handles callbacks to the
 * parent ConsultationPage for sidebar panels (InfoPanel, DiagnosisPanel).
 */
export function AssistantChatPanel({
  conversationId,
  initialMessages = [],
  initialExtractedInfo: _initialExtractedInfo = [],
  isDraft,
  clientDraftId,
  onExtractedInfoUpdate,
  onPhaseChange,
  onCitation,
  onConversationCreated,
  onTitleGenerated,
  onMessagePersisted,
}: AssistantChatPanelProps) {
  // Accumulate extracted info across multiple SSE events
  const extractedInfoRef = useRef<ExtractedInfo[]>(_initialExtractedInfo);

  // Sync ref when initialExtractedInfo changes (e.g., switching conversations)
  useEffect(() => {
    extractedInfoRef.current = _initialExtractedInfo;
  }, [_initialExtractedInfo]);

  const adapterOptions = useMemo(() => ({
    onExtractedInfoUpdate: (info: ExtractedInfo) => {
      extractedInfoRef.current = [...extractedInfoRef.current, info];
      onExtractedInfoUpdate?.(extractedInfoRef.current);
    },
    onPhaseChange: (_from: string, to: string) => {
      onPhaseChange?.(to as ConsultationPhase);
    },
    onCitation,
    onConversationCreated,
    onTitleGenerated,
    onMessagePersisted,
    isDraft,
    clientDraftId: clientDraftId ?? undefined,
  }), [onExtractedInfoUpdate, onPhaseChange, onCitation, onConversationCreated, onTitleGenerated, onMessagePersisted, isDraft, clientDraftId]);

  const { runtime } = useAssistantChatRuntime(conversationId, adapterOptions);

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <ChatContent
        initialMessages={initialMessages}
      />
    </AssistantRuntimeProvider>
  );
}

/**
 * Inner chat content that reads thread state from the assistant-ui runtime.
 * Uses useThread() to access messages managed by the runtime.
 */
function ChatContent({
  initialMessages = [],
}: {
  initialMessages?: ChatMessage[];
}) {
  const thread = useThread();
  const composerRuntime = useComposerRuntime();
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const [inputText, setInputText] = useState('');
  const [redFlags, setRedFlags] = useState<RedFlagEvent | null>(null);
  const [citations, setCitations] = useState<Citation[]>([]);
  const [knowledgeGaps, setKnowledgeGaps] = useState<string[]>([]);

  // Convert initial messages to display format (pure computation, no side effects)
  const displayMessages = useMemo(() => {
    const initial: DisplayMessage[] = initialMessages.map((m) => ({
      role: m.role,
      content: m.content,
    }));

    // Add messages from the runtime thread
    const threadMessages: DisplayMessage[] = [];
    for (const msg of thread.messages) {
      if (msg.role !== 'assistant' && msg.role !== 'user') continue;

      const textParts = msg.content
        .filter((p): p is { type: 'text'; text: string } => p.type === 'text')
        .map((p) => p.text)
        .join('');

      if (textParts) {
        threadMessages.push({ role: msg.role, content: textParts });
      }
    }

    // Merge initial + thread messages, deduplicating by content
    const seen = new Set<string>();
    const merged: DisplayMessage[] = [];
    for (const m of [...initial, ...threadMessages]) {
      const key = `${m.role}:${m.content}`;
      if (!seen.has(key)) {
        seen.add(key);
        merged.push(m);
      }
    }
    return merged;
  }, [initialMessages, thread.messages]);

  // Extract citations, red flags, and knowledge gaps from thread messages.
  // Uses functional setState with shallow comparison to avoid unnecessary
  // re-renders when thread.messages changes due to streaming text updates.
  // Must be in useEffect (not useMemo) because it calls setState.
  useEffect(() => {
    const newCitations: Citation[] = [];
    let newRedFlag: RedFlagEvent | null = null;
    const newGaps: string[] = [];

    for (const msg of thread.messages) {
      if (msg.role !== 'assistant') continue;

      // Extract citations from source parts
      const sourceParts = msg.content.filter((p) => p.type === 'source');
      for (const sp of sourceParts) {
        const source = sp as { type: 'source'; title?: string };
        if (source.title && !newCitations.some((c) => c.title === source.title)) {
          newCitations.push({ title: source.title });
        }
      }

      // Extract red flags from data parts
      const redFlagParts = msg.content.filter(
        (p) => p.type === 'data' && (p as { name: string }).name === 'red_flag',
      );
      for (const rp of redFlagParts) {
        const data = (rp as { data: RedFlagEvent }).data;
        if (data?.has_red_flags) {
          newRedFlag = data;
        }
      }

      // Extract knowledge gaps from data parts
      const gapParts = msg.content.filter(
        (p) => p.type === 'data' && (p as { name: string }).name === 'knowledge_gap',
      );
      for (const gp of gapParts) {
        const data = (gp as { data: { query: string; message: string } }).data;
        if (data?.query && !newGaps.includes(data.query)) {
          newGaps.push(data.query);
        }
      }
    }

    setCitations((prev) => {
      const prevKeys = prev.map((c) => c.title).sort().join(',');
      const newKeys = newCitations.map((c) => c.title).sort().join(',');
      return prevKeys === newKeys ? prev : newCitations;
    });

    setRedFlags((prev) => {
      if (!newRedFlag && !prev) return prev;
      if (!newRedFlag || !prev) return newRedFlag;
      return JSON.stringify(prev) === JSON.stringify(newRedFlag) ? prev : newRedFlag;
    });

    setKnowledgeGaps((prev) => {
      const prevKey = [...prev].sort().join(',');
      const newKey = [...newGaps].sort().join(',');
      return prevKey === newKey ? prev : newGaps;
    });
  }, [thread.messages]);

  // Auto-scroll to bottom
  useEffect(() => {
    const container = messagesContainerRef.current;
    if (container) {
      container.scrollTo({
        top: container.scrollHeight,
        behavior: 'smooth',
      });
    }
  }, [displayMessages]);

  // Get the currently streaming text from the last assistant message
  const streamingText = useMemo(() => {
    const lastMsg = thread.messages[thread.messages.length - 1];
    if (lastMsg?.role !== 'assistant') return '';
    if (lastMsg.status?.type === 'running') {
      return lastMsg.content
        .filter((p): p is { type: 'text'; text: string } => p.type === 'text')
        .map((p) => p.text)
        .join('');
    }
    return '';
  }, [thread.messages]);

  const isRunning = useMemo(() => {
    const lastMsg = thread.messages[thread.messages.length - 1];
    return lastMsg?.role === 'assistant' && lastMsg.status?.type === 'running';
  }, [thread.messages]);

  const handleSend = useCallback(() => {
    if (!inputText.trim() || isRunning) return;
    // Set text in the composer runtime and send
    composerRuntime.setText(inputText.trim());
    composerRuntime.send();
    setInputText('');
    setRedFlags(null);
    setCitations([]);
    setKnowledgeGaps([]);
  }, [inputText, isRunning, composerRuntime]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Messages area */}
      <div ref={messagesContainerRef} className="flex-1 overflow-y-auto p-4 space-y-4">
        {displayMessages.length === 0 && !streamingText && (
          <div className="flex items-center justify-center h-full text-gray-400">
            <div className="text-center">
              <p className="text-lg font-medium">开始咨询</p>
              <p className="text-sm mt-1">
                描述你的体态问题，AI 助手会帮助你分析
              </p>
            </div>
          </div>
        )}

        {displayMessages.map((msg, i) => (
          <div
            key={i}
            className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
          >
            <div
              className={`max-w-[80%] rounded-[20px] px-4 py-3 ${
                msg.role === 'user'
                  ? 'bg-primary-700 text-[#FBFBFA] rounded-br-[4px] shadow-sm shadow-[#2a4b3a]/10'
                  : 'bg-[#F7F5F0] text-[#1A221E] rounded-bl-[4px] border border-[#E5E3DF]'
              }`}
            >
              <div className="whitespace-pre-wrap text-sm leading-relaxed font-medium">
                {msg.content}
              </div>
            </div>
          </div>
        ))}

        {/* Typing indicator: AI is running but no text tokens yet */}
        {isRunning && !streamingText && (
          <div className="flex justify-start">
            <div className="max-w-[80%] rounded-[20px] px-5 py-3.5 bg-[#F7F5F0] text-[#1A221E] rounded-bl-[4px] border border-[#E5E3DF]">
              <div className="flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-[#709a83] animate-bounce [animation-delay:-0.3s]" />
                <span className="w-2 h-2 rounded-full bg-[#709a83] animate-bounce [animation-delay:-0.15s]" />
                <span className="w-2 h-2 rounded-full bg-[#709a83] animate-bounce" />
              </div>
            </div>
          </div>
        )}

        {/* Streaming indicator for messages not yet in displayMessages */}
        {streamingText && !displayMessages.some(
          (m) => m.role === 'assistant' && m.content === streamingText,
        ) && (
          <div className="flex justify-start">
            <div className="max-w-[80%] rounded-[20px] px-4 py-3 bg-[#F7F5F0] text-[#1A221E] rounded-bl-[4px] border border-[#E5E3DF]">
              <div className="whitespace-pre-wrap text-sm leading-relaxed font-medium">
                {streamingText}
              </div>
              <span className="inline-block w-1.5 h-3.5 ml-1 bg-[#709a83] animate-pulse" />
            </div>
          </div>
        )}

        {citations.length > 0 && (
          <div className="flex justify-start">
            <div className="max-w-[80%] rounded-xl px-3 py-2 bg-[#EEF2EE] border border-[#D4DDD4]">
              <p className="text-xs font-semibold text-[#5A7A64] mb-1">参考知识</p>
              <div className="flex flex-wrap gap-1.5">
                {citations.map((c, i) => (
                  <span
                    key={i}
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

        {knowledgeGaps.length > 0 && (
          <div className="flex justify-start">
            <div className="max-w-[80%] rounded-xl px-3 py-2 bg-[#FFF8F0] border border-[#F0D4B0]">
              <div className="flex items-start gap-2">
                <svg className="w-4 h-4 text-[#D4864A] mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div>
                  <p className="text-xs font-semibold text-[#A06030]">知识库提示</p>
                  <p className="text-xs text-[#8B6A4A] mt-0.5">
                    知识库中暂未收录「{knowledgeGaps.join('」「')}」的专项资料，以下建议仅供参考。
                  </p>
                </div>
              </div>
            </div>
          </div>
        )}

        {redFlags && redFlags.has_red_flags && (
          <RedFlagBanner
            redFlags={redFlags.flags}
            onAcknowledge={() => setRedFlags(null)}
          />
        )}
      </div>

      {/* Input area */}
      <div className="flex items-end gap-2 p-4 border-t border-[#E5E3DF] bg-[#FBFBFA]">
        <textarea
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="描述您的症状、体态问题或身体感受..."
          disabled={isRunning}
          rows={1}
          className="flex-1 resize-none rounded-2xl border border-[#D6D3CD] px-4 py-3 text-sm bg-white
                     focus:outline-none focus:ring-2 focus:ring-primary-600 focus:border-transparent
                     disabled:bg-[#F7F5F0] disabled:text-gray-400
                     placeholder:text-gray-400"
          style={{ maxHeight: '120px' }}
        />
        <button
          onClick={handleSend}
          disabled={isRunning || !inputText.trim()}
          className="flex-shrink-0 rounded-full bg-[#CD7B67] px-6 py-3 text-sm font-semibold text-white
                     hover:bg-[#B65E49] focus:outline-none focus:ring-2 focus:ring-primary-600 focus:ring-offset-2
                     disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors duration-300 shadow-sm shadow-[#CD7B67]/15"
        >
          发送
        </button>
      </div>
    </div>
  );
}

interface DisplayMessage {
  role: 'user' | 'assistant';
  content: string;
}
