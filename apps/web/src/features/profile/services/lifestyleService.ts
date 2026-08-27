import { authFetch } from "@/features/auth/services/authService";
import { expectJson } from "@/lib/api-client";

export type LifestyleSectionKey =
  | "activity"
  | "sleep"
  | "exercise"
  | "nutrition"
  | "substances"
  | "recovery";

export interface LifestyleSection {
  kind: string;
  fact_id?: string;
  summary: string;
  details: Record<string, unknown>;
  valid_from?: string;
  updated_at?: string;
  review_state?: string;
}

export interface LifestyleCandidate {
  fact_id: string;
  kind: string;
  summary: string;
  details: Record<string, unknown>;
  created_at: string;
}

export interface LifestyleSnapshot {
  current_revision: number;
  activity: LifestyleSection;
  sleep: LifestyleSection;
  exercise: LifestyleSection;
  nutrition: LifestyleSection;
  substances: LifestyleSection;
  recovery: LifestyleSection;
  pending_updates: LifestyleCandidate[];
}

export interface LifestyleSectionInput {
  summary: string;
  details?: Record<string, unknown>;
}

export type LifestyleUpdate = Partial<
  Record<LifestyleSectionKey, LifestyleSectionInput>
> & { expected_revision?: number };

const jsonHeaders = { "Content-Type": "application/json" };

async function reviewCandidate(
  factId: string,
  expectedRevision: number,
  action: "accept" | "reject",
) {
  return expectJson<LifestyleSnapshot>(
    await authFetch(`/api/v1/lifestyle/candidates/${factId}/${action}`, {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({ expected_revision: expectedRevision }),
    }),
  );
}

export const lifestyleService = {
  get: async () =>
    expectJson<LifestyleSnapshot>(await authFetch("/api/v1/lifestyle")),
  update: async (input: LifestyleUpdate) =>
    expectJson<LifestyleSnapshot>(
      await authFetch("/api/v1/lifestyle", {
        method: "PUT",
        headers: jsonHeaders,
        body: JSON.stringify(input),
      }),
    ),
  acceptCandidate: async (factId: string, expectedRevision: number) =>
    reviewCandidate(factId, expectedRevision, "accept"),
  rejectCandidate: async (factId: string, expectedRevision: number) =>
    reviewCandidate(factId, expectedRevision, "reject"),
};
