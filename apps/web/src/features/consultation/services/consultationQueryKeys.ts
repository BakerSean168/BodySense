export const consultationKeys = {
  all: ["consultation"] as const,
  conversation: () => [...consultationKeys.all, "conversation"] as const,
  session: (id: string) => [...consultationKeys.all, "session", id] as const,
  sessionEmpty: () => [...consultationKeys.all, "session", "empty"] as const,
  thread: (id: string) => [...consultationKeys.all, "thread", id] as const,
  threadEmpty: () => [...consultationKeys.all, "thread", "empty"] as const,
  diagnosisHistoryAll: () =>
    [...consultationKeys.all, "diagnosis-history"] as const,
  diagnosisHistory: (limit: number) =>
    [...consultationKeys.diagnosisHistoryAll(), limit] as const,
};
