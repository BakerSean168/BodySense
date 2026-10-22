import type { ConsultationSpatialContext } from "../types/consultation";

/** Ephemeral image attachments for the next user turn (Phase 3-B2).
 * ChatInput pushes upload_ids here; the model adapter drains them when
 * composing the StartRun parts. Not a global store — single in-flight turn.
 */
export type ConsultationImageAttachment = {
  uploadId: string;
  mimeType?: string;
  imageUrl?: string;
};

export const consultationAttachmentBuffer: {
  next: ConsultationImageAttachment[];
} = {
  next: [],
};

/** Ephemeral Body Explorer context for the next user turn only. */
export const consultationSpatialContextBuffer: {
  next: ConsultationSpatialContext | null;
} = {
  next: null,
};
