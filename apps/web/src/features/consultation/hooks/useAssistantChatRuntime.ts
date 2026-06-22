import { useLocalRuntime, type ChatModelAdapter } from "@assistant-ui/react";
import { useAuthStore } from "@/stores/authStore";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

/**
 * Custom ChatModelAdapter that connects to our SSE backend.
 * This bridges our existing consultation chat API with assistant-ui's runtime.
 */
function createConsultationChatAdapter(sessionId: string) {
  const adapter: ChatModelAdapter = {
    async run(options) {
      const { messages, abortSignal } = options;
      const { accessToken } = useAuthStore.getState();

      // Get the last user message content
      const lastMessage = messages[messages.length - 1];
      let userContent = '';
      if (lastMessage?.role === 'user' && 'content' in lastMessage) {
        const content = lastMessage.content as Array<{ type: string; text?: string }>;
        userContent = content
          .filter((p) => p.type === 'text')
          .map((p) => p.text || '')
          .join('');
      }

      // Call our SSE endpoint
      const response = await fetch(`${API_BASE_URL}/api/v1/consultation/${sessionId}/message`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
        },
        body: JSON.stringify({ content: userContent }),
        signal: abortSignal,
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const reader = response.body?.getReader();
      if (!reader) throw new Error('No response body');

      const decoder = new TextDecoder();
      let buffer = '';
      let accumulatedText = '';

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
                accumulatedText += data.content;
              }
            } catch {
              // Skip malformed JSON
            }
          }
        }
      }

      // Return final result with proper status type
      return {
        content: [{
          type: 'text' as const,
          text: accumulatedText,
        }],
        status: { type: 'complete' as const, reason: 'stop' as const },
      };
    },
  };

  return adapter;
}

/**
 * Hook to create an assistant-ui runtime for consultation chat.
 */
export function useAssistantChatRuntime(sessionId: string) {
  const adapter = createConsultationChatAdapter(sessionId);
  return useLocalRuntime(adapter);
}
