import { authFetch } from "@/features/auth/services/authService";
import { expectJson } from "@/lib/api-client";

export interface InjuryHistorySnapshot {
  current_revision: number;
  fact_id?: string;
  summary: string;
  valid_from?: string;
  updated_at?: string;
}

export const healthHistoryService = {
  getInjuryHistory: async () =>
    expectJson<InjuryHistorySnapshot>(
      await authFetch("/api/v1/health-history/injury"),
    ),
  updateInjuryHistory: async (input: {
    expected_revision?: number;
    summary: string;
  }) =>
    expectJson<InjuryHistorySnapshot>(
      await authFetch("/api/v1/health-history/injury", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      }),
    ),
};
