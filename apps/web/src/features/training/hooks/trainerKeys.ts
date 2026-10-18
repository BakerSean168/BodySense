/** Query keys of the Trainer window and the Home training chip. */
export const trainerKeys = {
  all: ["training", "trainer"] as const,
  plan: (planId: string) => [...trainerKeys.all, "plan", planId] as const,
  sessions: () => [...trainerKeys.all, "sessions"] as const,
  revision: (planId: string, revisionId: string | null) =>
    [...trainerKeys.all, "revision", planId, revisionId] as const,
};
