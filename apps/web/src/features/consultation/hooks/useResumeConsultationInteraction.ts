import { useCallback, useRef } from "react";
import { consultationApi } from "../services/consultationService";
import type { PendingInteraction } from "../types/consultation";
import type {
  ConsultationThreadController,
  ConsultationStreamRun,
} from "../runtime/consultationRuntime.types";

/** Retains resume request identity across retries without recreating the runtime. */
export function useResumeConsultationInteraction(
  conversationId: string,
  streamRun: ConsultationStreamRun,
  setIsStreaming: (value: boolean) => void,
) {
  const streamConsultationRunRef = useRef(streamRun);
  // Synchronous runtime callbacks must see the current conversation's runner.
  streamConsultationRunRef.current = streamRun;
  const resumeRequest = useRef<{
    conversationId: string;
    interactionId: string;
    content: string;
    requestId: string;
  } | null>(null);

  return useCallback(
    (
      threadRuntime: ConsultationThreadController,
      interactionId: string,
      answer: unknown,
      interaction?: PendingInteraction,
    ): Promise<void> => {
      const content = JSON.stringify(answer);
      if (
        resumeRequest.current?.conversationId !== conversationId ||
        resumeRequest.current.interactionId !== interactionId ||
        resumeRequest.current.content !== content
      ) {
        resumeRequest.current = {
          conversationId,
          interactionId,
          content,
          requestId: crypto.randomUUID(),
        };
      }
      const requestId = resumeRequest.current.requestId;
      setIsStreaming(true);
      return new Promise<void>((resolve, reject) => {
        try {
          threadRuntime.resumeRun({
            parentId: threadRuntime.getState().messages.at(-1)?.id ?? null,
            stream: async function* () {
              try {
                const runFn = streamConsultationRunRef.current;
                if (!runFn)
                  throw new Error("Conversation runtime is not ready.");
                yield* runFn(
                  () =>
                    consultationApi.resumeInteractionStream(
                      conversationId,
                      interactionId,
                      { requestId, answer },
                    ),
                  true,
                  interaction?.id === interactionId ? interaction : undefined,
                );
                resolve();
              } catch (error) {
                // The pending composer owns retry feedback. An unacknowledged
                // HTTP failure must not fabricate an answered interaction.
                reject(error);
              } finally {
                setIsStreaming(false);
              }
            },
          });
        } catch (error) {
          setIsStreaming(false);
          reject(error);
        }
      });
    },
    [conversationId],
  );
}
