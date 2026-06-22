import { AssistantRuntimeProvider } from "@assistant-ui/react";
import { useAssistantChatRuntime } from "../hooks/useAssistantChatRuntime";
import { useRef, useEffect, useState, useCallback } from "react";
import { useChatSSE, type SSEDoneEvent } from "../hooks/useChatSSE";
import type { ExtractedInfo } from "../services/consultationService";

interface AssistantChatPanelProps {
  sessionId: string;
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
}

/**
 * Chat panel using assistant-ui runtime for state management.
 * Falls back to our custom SSE hook for streaming since assistant-ui's
 * runtime API has complex type requirements.
 */
export function AssistantChatPanel({ sessionId, onExtractedInfoUpdate }: AssistantChatPanelProps) {
  // Use assistant-ui runtime for the provider context
  const runtime = useAssistantChatRuntime(sessionId);

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <ChatContent sessionId={sessionId} onExtractedInfoUpdate={onExtractedInfoUpdate} />
    </AssistantRuntimeProvider>
  );
}

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
}

function ChatContent({ sessionId, onExtractedInfoUpdate }: {
  sessionId: string;
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
}) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [streamingText, setStreamingText] = useState('');
  const [inputText, setInputText] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingText]);

  const handleText = useCallback((text: string) => {
    setStreamingText((prev) => prev + text);
  }, []);

  const [localExtractedInfo, setLocalExtractedInfo] = useState<ExtractedInfo[]>([]);

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
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
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
              className={`max-w-[80%] rounded-2xl px-4 py-3 ${
                msg.role === 'user'
                  ? 'bg-blue-600 text-white rounded-br-md'
                  : 'bg-gray-100 text-gray-900 rounded-bl-md'
              }`}
            >
              <div className="whitespace-pre-wrap text-sm leading-relaxed">
                {msg.content}
              </div>
            </div>
          </div>
        ))}

        {streamingText && (
          <div className="flex justify-start">
            <div className="max-w-[80%] rounded-2xl px-4 py-3 bg-gray-100 text-gray-900 rounded-bl-md">
              <div className="whitespace-pre-wrap text-sm leading-relaxed">
                {streamingText}
              </div>
              <span className="inline-block w-2 h-4 ml-1 bg-gray-400 animate-pulse" />
            </div>
          </div>
        )}

        {error && (
          <div className="text-center text-red-500 text-sm py-2">{error}</div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <div className="flex items-end gap-2 p-4 border-t bg-white">
        <textarea
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="描述你的体态问题或不适感受..."
          disabled={isStreaming}
          rows={1}
          className="flex-1 resize-none rounded-xl border border-gray-300 px-4 py-3 text-sm
                     focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent
                     disabled:bg-gray-50 disabled:text-gray-500
                     placeholder:text-gray-400"
          style={{ maxHeight: '120px' }}
        />
        <button
          onClick={handleSend}
          disabled={isStreaming || !inputText.trim()}
          className="flex-shrink-0 rounded-xl bg-blue-600 px-5 py-3 text-sm font-medium text-white
                     hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2
                     disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
        >
          发送
        </button>
      </div>
    </div>
  );
}
