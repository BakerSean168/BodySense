import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface ConsultationState {
  draftMessage: string;
  setDraftMessage: (message: string) => void;
  clearDraftMessage: () => void;
}

export const useConsultationStore = create<ConsultationState>()(
  persist(
    (set) => ({
      draftMessage: '',
      setDraftMessage: (message: string) => set({ draftMessage: message }),
      clearDraftMessage: () => set({ draftMessage: '' }),
    }),
    {
      name: 'bodysense-consultation-storage',
    }
  )
);
