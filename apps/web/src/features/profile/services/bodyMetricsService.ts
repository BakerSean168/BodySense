import { getBodyMetrics, updateBodyMetrics } from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

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

type GeneratedBodyMetrics = Awaited<ReturnType<typeof getBodyMetrics>>;

function projectBodyMetrics(
  snapshot: GeneratedBodyMetrics,
): BodyMetricsSnapshot {
  const metric = (
    value: GeneratedBodyMetrics["height"],
  ): BodyMetricValue | undefined =>
    value
      ? { value: value.value, unit: value.unit, observed_at: value.observed_at }
      : undefined;
  return {
    current_revision: snapshot.current_revision,
    height: metric(snapshot.height),
    weight: metric(snapshot.weight),
    bmi: snapshot.bmi,
  };
}

export const bodyMetricsService = {
  get: async (): Promise<BodyMetricsSnapshot> =>
    withOpenApiError(async () =>
      projectBodyMetrics(await getBodyMetrics(undefined, openApiAuthFetch)),
    ),

  update: async (input: {
    expected_revision: number;
    height_cm?: number;
    weight_kg?: number;
  }): Promise<BodyMetricsSnapshot> =>
    withOpenApiError(async () =>
      projectBodyMetrics(
        await updateBodyMetrics(input, undefined, openApiAuthFetch),
      ),
    ),
};
