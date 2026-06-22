import { useEffect, useRef, useState, useCallback } from 'react';
import { ChatMessage } from './ChatMessage';
import { ChatInput } from './ChatInput';
import { useChatSSE, type SSEDoneEvent } from '../hooks/useChatSSE';
import type {
  ConsultationSession,
  ExtractedInfo,
  ChatMessage as ChatMessageType,
} from '../services/consultationService';

interface ChatPanelProps {
  session: ConsultationSession;
  onSessionUpdate?: (session: ConsultationSession) => void;
  onExtractedInfoUpdate?: (info: ExtractedInfo[]) => void;
}

export function ChatPanel({
  session,
  onSessionUpdate,
  onExtractedInfoUpdate,
}: ChatPanelProps) {
  const [messages, setMessages] = useState<ChatMessageType[]>(session.messages || []);
  const [streamingText, setStreamingText] = useState('');
  const [localExtractedInfo, setLocalExtractedInfo] = useState<ExtractedInfo[]>(
    session.extracted_info || [],
  );
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingText]);

  const handleText = useCallback((text: string) => {
    setStreamingText((prev) => prev + text);
  }, []);

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
      // Add the complete assistant message
      if (event.full_text) {
        setMessages((prev) => [
          ...prev,
          { role: 'assistant', content: event.full_text },
        ]);
      }
      setStreamingText('');

      // Update extracted info with the final state
      if (event.extracted_info) {
        setLocalExtractedInfo(event.extracted_info);
        onExtractedInfoUpdate?.(event.extracted_info);
      }

      // Notify parent of session update
      onSessionUpdate?.({
        ...session,
        messages: [
          ...messages,
          { role: 'assistant', content: event.full_text },
        ],
        extracted_info: event.extracted_info,
      });
    },
    [messages, session, onSessionUpdate, onExtractedInfoUpdate],
  );

  const { sendMessage, isStreaming, error } = useChatSSE(
    handleText,
    handleExtractedInfo,
    handleDone,
  );

  const handleSend = (content: string) => {
    // Add user message immediately
    setMessages((prev) => [...prev, { role: 'user', content }]);
    setStreamingText('');

    // Send to backend
    sendMessage(session.id, content);
  };

  return (
    <div className="flex flex-col h-full">
      {/* Messages area */}
      <div className="flex-1 overflow-y-auto p-4 space-y-1">
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
          <ChatMessage key={i} role={msg.role} content={msg.content} />
        ))}

        {streamingText && (
          <ChatMessage
            role="assistant"
            content={streamingText}
            isStreaming={true}
          />
        )}

        {error && (
          <div className="text-center text-red-500 text-sm py-2">
            {error}
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <ChatInput
        onSend={handleSend}
        disabled={isStreaming || session.status === 'completed'}
        placeholder={
          session.status === 'completed'
            ? '会话已结束'
            : '描述你的体态问题或不适感受...'
        }
      />
    </div>
  );
}
