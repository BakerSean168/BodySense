import type { ChatModelAdapter, ChatModelRunResult } from "@assistant-ui/react";
import { consultationApi } from "../services/consultationService";
import {
  consultationAttachmentBuffer,
  consultationSpatialContextBuffer,
} from "./consultationBuffers";
import type { ConsultationStreamRun } from "./consultationRuntime.types";

export function createConsultationChatAdapter(
  conversationId: string,
  streamRun: ConsultationStreamRun,
  setIsStreaming: (value: boolean) => void,
): ChatModelAdapter {
  return {
    async *run({ messages }): AsyncGenerator<ChatModelRunResult> {
      const lastMessage = messages[messages.length - 1];
      const content = lastMessage.content
        .filter((p): p is { type: "text"; text: string } => p.type === "text")
        .map((p) => p.text)
        .join("");

      const attachments = consultationAttachmentBuffer.next.splice(
        0,
        consultationAttachmentBuffer.next.length,
      );
      const spatialContext = consultationSpatialContextBuffer.next;
      consultationSpatialContextBuffer.next = null;
      const parts: Array<{
        type: string;
        text?: string;
        upload_id?: string;
        mime_type?: string;
        image_url?: string;
      }> = [];
      const text = content.trim();
      if (text) {
        parts.push({ type: "text", text });
      } else if (attachments.length > 0) {
        parts.push({
          type: "text",
          text: "请结合我附上的照片，分析与体态/不适相关的可见信息，并给出谨慎建议。",
        });
      }
      for (const image of attachments.slice(0, 3)) {
        parts.push({
          type: "image",
          upload_id: image.uploadId,
          ...(image.mimeType ? { mime_type: image.mimeType } : {}),
          ...(image.imageUrl ? { image_url: image.imageUrl } : {}),
        });
      }
      if (parts.length === 0) {
        parts.push({ type: "text", text: content || " " });
      }

      setIsStreaming(true);

      yield* streamRun(() =>
        consultationApi.startConsultationRun({
          conversationId: conversationId === "new" ? null : conversationId,
          clientMessageId: `tmp_${crypto.randomUUID()}`,
          requestId: crypto.randomUUID(),
          message: {
            role: "user",
            parts,
            ...(spatialContext
              ? { metadata: { body_explorer_context: spatialContext } }
              : {}),
          },
        }),
      );
    },
  };
}
