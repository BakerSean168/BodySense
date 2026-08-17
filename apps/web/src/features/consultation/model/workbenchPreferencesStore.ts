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
      chatOpen: true,
      chatSize: 38,
      mobileSurface: "chat",
      setChatOpen: (chatOpen) => set({ chatOpen }),
      toggleChat: () => set((state) => ({ chatOpen: !state.chatOpen })),
      setChatSize: (chatSize) => set({ chatSize: clampChatSize(chatSize) }),
      setMobileSurface: (mobileSurface) => set({ mobileSurface }),
    }),
    {
      name: "bodysense-workbench-preferences",
      partialize: ({ chatOpen, chatSize, mobileSurface }) => ({
        chatOpen,
        chatSize,
        mobileSurface,
      }),
    },
  ),
);
