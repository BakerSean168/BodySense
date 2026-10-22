import { toast } from "sonner";
import { create } from "zustand";

/** How long a removal can be undone before it is committed. */
export const UNDO_WINDOW_MS = 5000;

interface PendingRemovalState {
  hidden: ReadonlySet<string>;
  hide: (id: string) => void;
  show: (id: string) => void;
}

/**
 * Ids removed in the UI whose server write is still waiting for the undo
 * window. Module-level so a remounted list keeps hiding them.
 */
export const usePendingRemovals = create<PendingRemovalState>((set) => ({
  hidden: new Set(),
  hide: (id) => set((state) => ({ hidden: new Set(state.hidden).add(id) })),
  show: (id) =>
    set((state) => {
      const hidden = new Set(state.hidden);
      hidden.delete(id);
      return { hidden };
    }),
}));

/**
 * Remove now, commit later: the item disappears at once and a toast offers
 * 撤销. The write runs only when the toast closes without undo, so a removal
 * the server treats as terminal can still be taken back. Use it only for
 * low-risk removals; erasure keeps its explicit confirmation.
 */
export function removeWithUndo({
  id,
  message,
  commit,
  onError,
}: {
  id: string;
  message: string;
  commit: () => Promise<unknown>;
  onError?: (cause: unknown) => void;
}) {
  const { hide, show } = usePendingRemovals.getState();
  let settled = false;
  hide(id);
  const run = () => {
    if (settled) return;
    settled = true;
    commit()
      .catch((cause: unknown) => {
        onError?.(cause);
      })
      .finally(() => show(id));
  };
  toast(message, {
    duration: UNDO_WINDOW_MS,
    action: {
      label: "撤销",
      onClick: () => {
        if (settled) return;
        settled = true;
        show(id);
      },
    },
    onAutoClose: run,
    onDismiss: run,
  });
}
