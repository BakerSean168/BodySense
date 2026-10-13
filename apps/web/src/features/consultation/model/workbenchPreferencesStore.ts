import { create } from "zustand";
import { persist } from "zustand/middleware";

export type MobileWorkbenchSurface = "chat" | "workspace";

interface WorkbenchPreferencesState {
  chatOpen: boolean;
  chatSize: number;
  mobileSurface: MobileWorkbenchSurface;
  setChatOpen: (open: boolean) => void;
  toggleChat: () => void;
  setChatSize: (size: number) => void;
  setMobileSurface: (surface: MobileWorkbenchSurface) => void;
}

export function clampChatSize(size: number): number {
  if (!Number.isFinite(size)) return 38;
  return Math.min(52, Math.max(24, Math.round(size * 10) / 10));
}

export const useWorkbenchPreferencesStore = create<WorkbenchPreferencesState>()(
  persist(
    (set) => ({
      chatOpen: false,
      chatSize: 38,
      mobileSurface: "chat",
      setChatOpen: (chatOpen) => set({ chatOpen }),
      toggleChat: () => set((state) => ({ chatOpen: !state.chatOpen })),
      setChatSize: (chatSize) => set({ chatSize: clampChatSize(chatSize) }),
      setMobileSurface: (mobileSurface) => set({ mobileSurface }),
    }),
    {
      name: "bodysense-workbench-preferences",
      version: 2,
      migrate: (persistedState) => {
        const state = persistedState as Partial<WorkbenchPreferencesState> | undefined;
        return {
          ...state,
          // V3 is body-first: start from the full canvas once after the layout migration.
          chatOpen: false,
        } as WorkbenchPreferencesState;
      },
      partialize: ({ chatOpen, chatSize, mobileSurface }) => ({
        chatOpen,
        chatSize,
        mobileSurface,
      }),
    },
  ),
);
