import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  episodicNotesKeys,
  episodicNotesService,
  type EpisodicNote,
} from "@/features/profile/services/episodicNotesService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";

/**
 * Owner-scoped reads and permanent removal for the S6-B preview. A successful
 * delete hides the row immediately and then refreshes the authoritative list;
 * a 404 means the note is already gone and is treated the same way, never as a
 * successful clinical change.
 */
export function useEpisodicNotes() {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: episodicNotesKeys.all,
    queryFn: episodicNotesService.list,
  });
  const [hiddenIds, setHiddenIds] = useState<readonly string[]>([]);
  const [removingId, setRemovingId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const notes = (query.data?.notes ?? []).filter(
    (note) => !hiddenIds.includes(note.id),
  );
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: episodicNotesKeys.all });
  const hide = (id: string) =>
    setHiddenIds((current) =>
      current.includes(id) ? current : [...current, id],
    );

  const remove = async (note: EpisodicNote): Promise<boolean> => {
    if (removingId) return false;
    setRemovingId(note.id);
    setActionError(null);
    try {
      await episodicNotesService.delete(note.id);
      hide(note.id);
      await refresh();
      return true;
    } catch (cause) {
      if (cause instanceof ApiRequestError && cause.status === 404) {
        // The list only shows the caller's notes, so 404 means it was already
        // removed. Reflect that without inventing a deletion result.
        hide(note.id);
        await refresh();
        return true;
      }
      setActionError(errorMessage(cause, "记忆没有删除，请重试。"));
      return false;
    } finally {
      setRemovingId(null);
    }
  };

  return {
    notes,
    isPending: query.isPending,
    isError: query.isError,
    error: query.error,
    removingId,
    actionError,
    remove,
    retry: () => void query.refetch(),
  };
}
