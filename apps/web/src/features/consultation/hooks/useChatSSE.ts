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

          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
            buffer = lines.pop() || '';

            for (const line of lines) {
              if (line.startsWith('data: ')) {
                try {
                  const data = JSON.parse(line.slice(6));

                  if (data.type === 'text' && data.content) {
                    onText(data.content);
                  } else if (data.type === 'extracted_info' && data.info) {
                    onExtractedInfo(data.info);
                  }
                } catch {
                  // Skip malformed JSON
                }
              } else if (line.startsWith('event: done')) {
                // Next data line is the done event
              }
            }

            // Check for done event in remaining buffer
            if (buffer.includes('event: done')) {
              const doneIdx = buffer.indexOf('event: done');
              const afterDone = buffer.slice(doneIdx);
              const dataLine = afterDone
                .split('\n')
                .find((l) => l.startsWith('data: '));
              if (dataLine) {
                try {
                  const doneData = JSON.parse(dataLine.slice(6));
                  onDone(doneData);
                } catch {
                  // Skip malformed done event
                }
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
