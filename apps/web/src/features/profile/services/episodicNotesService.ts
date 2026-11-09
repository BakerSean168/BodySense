import {
  deleteEpisodicNote,
  listEpisodicNotes,
} from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// The generated contract is the only browser DTO authority for S6-B. Notes are
// non-authoritative conversational memory, never BodyState or clinical facts.
export type EpisodicNotesPage = Awaited<ReturnType<typeof listEpisodicNotes>>;
export type EpisodicNote = EpisodicNotesPage["notes"][number];
export type EpisodicNoteDeleteReceipt = Awaited<
  ReturnType<typeof deleteEpisodicNote>
>;

export const episodicNotesKeys = {
  all: ["episodic-notes"] as const,
};

export const episodicNotesService = {
  list: (): Promise<EpisodicNotesPage> =>
    withOpenApiError(() =>
      listEpisodicNotes({ cache: "no-store" }, openApiAuthFetch),
    ),
  delete: (noteId: string): Promise<EpisodicNoteDeleteReceipt> =>
    withOpenApiError(() =>
      deleteEpisodicNote(noteId, { cache: "no-store" }, openApiAuthFetch),
    ),
};
