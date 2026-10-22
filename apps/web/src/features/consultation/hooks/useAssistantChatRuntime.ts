import { useRef, useState } from "react";
import { useLocalRuntime, type ThreadMessageLike } from "@assistant-ui/react";
import type {
  ConsultationAdapterOptions,
  ConsultationStreamRun,
} from "../runtime/consultationRuntime.types";
import { createConsultationChatAdapter } from "../runtime/createConsultationChatAdapter";
import { streamConsultationRun } from "../runtime/streamConsultationRun";
import { useResumeConsultationInteraction } from "./useResumeConsultationInteraction";
export type {
  ConsultationAdapterOptions,
  ConsultationThreadController,
} from "../runtime/consultationRuntime.types";
export type { ConsultationImageAttachment } from "../runtime/consultationBuffers";
export {
  consultationAttachmentBuffer,
  consultationSpatialContextBuffer,
} from "../runtime/consultationBuffers";

export function useAssistantChatRuntime(
  conversationId: string,
  initialMessages: ThreadMessageLike[] = [],
  options: ConsultationAdapterOptions = {},
) {
  const [isStreaming, setIsStreaming] = useState(false);
  const optionsRef = useRef(options);
  // Runtime events can arrive between render and effects; retain current callbacks.
  optionsRef.current = options;
  const streamRun: ConsultationStreamRun = (
    startRequest,
    preserveUnstartedState,
    interaction,
  ) =>
    streamConsultationRun(
      { conversationId, optionsRef, setIsStreaming },
      startRequest,
      preserveUnstartedState,
      interaction,
    );
  const runtime = useLocalRuntime(
    createConsultationChatAdapter(conversationId, streamRun, setIsStreaming),
    { initialMessages },
  );
  const resumeInteraction = useResumeConsultationInteraction(
    conversationId,
    streamRun,
    setIsStreaming,
  );
  return { runtime, isStreaming, resumeInteraction };
}
