import { useCallback, useRef, useState } from 'react';
import { useAuthStore } from '@/stores/authStore';
import type { ExtractedInfo } from '../services/consultationService';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

export interface SSEMessage {
  type: 'text' | 'extracted_info';
  content?: string;
  info?: ExtractedInfo;
}

export interface SSEDoneEvent {
  session_id: string;
  full_text: string;
  extracted_info: ExtractedInfo[];
}

interface UseChatSSEReturn {
  sendMessage: (sessionId: string, content: string) => void;
  isStreaming: boolean;
  error: string | null;
  abort: () => void;
}

/**
 * Hook for streaming chat messages via SSE.
 * Calls onText for each text chunk, onExtractedInfo for extracted data,
 * and onDone when the stream completes.
 */
export function useChatSSE(
  onText: (text: string) => void,
  onExtractedInfo: (info: ExtractedInfo) => void,
  onDone: (event: SSEDoneEvent) => void,
  onError?: (error: string) => void,
): UseChatSSEReturn {
  const [isStreaming, setIsStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  const abort = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
      setIsStreaming(false);
    }
  }, []);

  const sendMessage = useCallback(
    (sessionId: string, content: string) => {
      // Abort any existing stream
      abort();

      const controller = new AbortController();
      abortControllerRef.current = controller;
      setIsStreaming(true);
      setError(null);

      const { accessToken } = useAuthStore.getState();

      fetch(`${API_BASE_URL}/api/v1/consultation/${sessionId}/message`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
        },
        body: JSON.stringify({ content }),
        signal: controller.signal,
      })
        .then(async (response) => {
          if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.error || `HTTP ${response.status}`);
          }

          const reader = response.body?.getReader();
          if (!reader) {
            throw new Error('No response body');
          }

          const decoder = new TextDecoder();
          let buffer = '';
          let currentEvent = 'message';

          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
            buffer = lines.pop() || '';

            for (const line of lines) {
              const trimmed = line.trim();
              if (trimmed.startsWith('event:')) {
                currentEvent = trimmed.slice(6).trim();
              } else if (trimmed.startsWith('data:')) {
                const dataStr = trimmed.slice(5).trim();
                try {
                  const data = JSON.parse(dataStr);
                  if (currentEvent === 'done') {
                    onDone(data);
                  } else {
                    if (data.type === 'text' && data.content) {
                      onText(data.content);
                    } else if (data.type === 'extracted_info' && data.info) {
                      onExtractedInfo(data.info);
                    }
                  }
                } catch {
                  // Skip malformed JSON
                }
                currentEvent = 'message';
              } else if (trimmed === '') {
                currentEvent = 'message';
              }
            }
          }

          // Process final remaining buffer
          if (buffer.trim()) {
            const trimmed = buffer.trim();
            if (trimmed.startsWith('event:')) {
              currentEvent = trimmed.slice(6).trim();
            } else if (trimmed.startsWith('data:')) {
              const dataStr = trimmed.slice(5).trim();
              try {
                const data = JSON.parse(dataStr);
                if (currentEvent === 'done') {
                  onDone(data);
                } else {
                  if (data.type === 'text' && data.content) {
                    onText(data.content);
                  } else if (data.type === 'extracted_info' && data.info) {
                    onExtractedInfo(data.info);
                  }
                }
              } catch {
                // Ignore malformed final JSON; the stream may end mid-frame.
              }
            }
          }

          setIsStreaming(false);
        })
        .catch((err) => {
          if (err.name !== 'AbortError') {
            const msg = err.message || 'Stream failed';
            setError(msg);
            onError?.(msg);
          }
          setIsStreaming(false);
        });
    },
    [abort, onText, onExtractedInfo, onDone, onError],
  );

  return { sendMessage, isStreaming, error, abort };
}
