import { AssistantRuntimeProvider } from "@assistant-ui/react";
import { useAssistantChatRuntime } from "../hooks/useAssistantChatRuntime";
import { useRef, useEffect, useState, useCallback } from "react";
import { useChatSSE, type SSEDoneEvent } from "../hooks/useChatSSE";
import type { ExtractedInfo } from "../services/consultationService";

interface AssistantChatPanelProps {
  sessionId: string;
  initialMessages?: ChatMessage[];
  initialExtractedInfo?: ExtractedInfo[];
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
}

/**
 * Chat panel using assistant-ui runtime for state management.
 * Falls back to our custom SSE hook for streaming since assistant-ui's
 * runtime API has complex type requirements.
 */
export function AssistantChatPanel({
  sessionId,
  initialMessages = [],
  initialExtractedInfo = [],
  onExtractedInfoUpdate,
}: AssistantChatPanelProps) {
  // Use assistant-ui runtime for the provider context
  const runtime = useAssistantChatRuntime(sessionId);

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <ChatContent
        sessionId={sessionId}
        initialMessages={initialMessages}
        initialExtractedInfo={initialExtractedInfo}
        onExtractedInfoUpdate={onExtractedInfoUpdate}
      />
    </AssistantRuntimeProvider>
  );
}

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
}

function ChatContent({
  sessionId,
  initialMessages = [],
  initialExtractedInfo = [],
  onExtractedInfoUpdate,
}: {
  sessionId: string;
  initialMessages?: ChatMessage[];
  initialExtractedInfo?: ExtractedInfo[];
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
}) {
  const [messages, setMessages] = useState<ChatMessage[]>(initialMessages);
  const [streamingText, setStreamingText] = useState('');
  const [inputText, setInputText] = useState('');
  const messagesContainerRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom
  useEffect(() => {
    const container = messagesContainerRef.current;
    if (container) {
      container.scrollTo({
        top: container.scrollHeight,
        behavior: 'smooth',
      });
    }
  }, [messages, streamingText]);

  const handleText = useCallback((text: string) => {
    setStreamingText((prev) => prev + text);
  }, []);

  const [localExtractedInfo, setLocalExtractedInfo] = useState<ExtractedInfo[]>(initialExtractedInfo);

  // When sessionId or initialExtractedInfo changes, reset localExtractedInfo
  useEffect(() => {
    setLocalExtractedInfo(initialExtractedInfo);
  }, [sessionId, initialExtractedInfo]);

  const handleExtractedInfo = useCallback(
    (info: ExtractedInfo) => {
      setLocalExtractedInfo((prev) => {
        const updated = [...prev, info];
        onExtractedInfoUpdate?.(updated);
        return updated;
      });
    },
    [onExtractedInfoUpdate],
  );

  const handleDone = useCallback(
    (event: SSEDoneEvent) => {
      if (event.full_text) {
        setMessages((prev) => [...prev, { role: 'assistant', content: event.full_text }]);
      }
      setStreamingText('');
      if (event.extracted_info) {
        onExtractedInfoUpdate?.(event.extracted_info);
      }
    },
    [onExtractedInfoUpdate],
  );

  const { sendMessage, isStreaming, error } = useChatSSE(
    handleText,
    handleExtractedInfo,
    handleDone,
  );

  const handleSend = useCallback(() => {
    if (!inputText.trim() || isStreaming) return;

    // Add user message immediately
    setMessages((prev) => [...prev, { role: 'user', content: inputText }]);
    setStreamingText('');

    // Send to backend
    sendMessage(sessionId, inputText);
    setInputText('');
  }, [inputText, isStreaming, sessionId, sendMessage]);

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
        {messages.length === 0 && !streamingText && (
          <div className="flex items-center justify-center h-full text-gray-400">
            <div className="text-center">
              <p className="text-lg font-medium">开始咨询</p>
              <p className="text-sm mt-1">
                描述你的体态问题，AI 助手会帮助你分析
              </p>
            </div>
          </div>
        )}

        {messages.map((msg, i) => (
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

        {streamingText && (
          <div className="flex justify-start">
            <div className="max-w-[80%] rounded-[20px] px-4 py-3 bg-[#F7F5F0] text-[#1A221E] rounded-bl-[4px] border border-[#E5E3DF]">
              <div className="whitespace-pre-wrap text-sm leading-relaxed font-medium">
                {streamingText}
              </div>
              <span className="inline-block w-1.5 h-3.5 ml-1 bg-[#709a83] animate-pulse" />
            </div>
          </div>
        )}

        {error && (
          <div className="text-center text-red-500 text-sm py-2 font-medium">{error}</div>
        )}
      </div>

      {/* Input area */}
      <div className="flex items-end gap-2 p-4 border-t border-[#E5E3DF] bg-[#FBFBFA]">
        <textarea
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="描述您的症状、体态问题或身体感受..."
          disabled={isStreaming}
          rows={1}
          className="flex-1 resize-none rounded-2xl border border-[#D6D3CD] px-4 py-3 text-sm bg-white
                     focus:outline-none focus:ring-2 focus:ring-primary-600 focus:border-transparent
                     disabled:bg-[#F7F5F0] disabled:text-gray-400
                     placeholder:text-gray-400"
          style={{ maxHeight: '120px' }}
        />
        <button
          onClick={handleSend}
          disabled={isStreaming || !inputText.trim()}
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
