import { authFetch } from "@/features/auth/services/authService";
import { expectJson } from "@/lib/api-client";

export interface BodyMetricValue {
  value: number;
  unit: string;
  observed_at?: string;
}

export interface BodyMetricsSnapshot {
  current_revision: number;
  height?: BodyMetricValue;
  weight?: BodyMetricValue;
  bmi?: number;
}

export const bodyMetricsService = {
  get: async () =>
    expectJson<BodyMetricsSnapshot>(await authFetch("/api/v1/body-metrics")),
  update: async (input: {
    expected_revision?: number;
    height_cm?: number;
    weight_kg?: number;
  }) =>
    expectJson<BodyMetricsSnapshot>(
      await authFetch("/api/v1/body-metrics", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      }),
    ),
};
