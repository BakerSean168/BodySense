export const consultationKeys = {
  all: ["consultation"] as const,
  conversations: () => [...consultationKeys.all, "conversations"] as const,
  conversation: (id: string) =>
    [...consultationKeys.all, "conversation", id] as const,
  conversationEmpty: () =>
    [...consultationKeys.all, "conversation", "empty"] as const,
  session: (id: string) => [...consultationKeys.all, "session", id] as const,
  sessionEmpty: () => [...consultationKeys.all, "session", "empty"] as const,
  thread: (id: string) => [...consultationKeys.all, "thread", id] as const,
  threadEmpty: () => [...consultationKeys.all, "thread", "empty"] as const,
  diagnosisHistoryAll: () =>
    [...consultationKeys.all, "diagnosis-history"] as const,
  diagnosisHistory: (limit: number) =>
    [...consultationKeys.diagnosisHistoryAll(), limit] as const,
};
