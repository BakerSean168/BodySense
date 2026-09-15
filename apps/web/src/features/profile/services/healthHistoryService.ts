import {
  getInjuryHistory,
  updateInjuryHistory,
} from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

export interface InjuryHistorySnapshot {
  current_revision: number;
  fact_id?: string;
  summary: string;
  valid_from?: string;
  updated_at?: string;
}

function projectInjuryHistory(
  snapshot: Awaited<ReturnType<typeof getInjuryHistory>>,
): InjuryHistorySnapshot {
  return {
    current_revision: snapshot.current_revision,
    fact_id: snapshot.fact_id,
    summary: snapshot.summary,
    valid_from: snapshot.valid_from,
    updated_at: snapshot.updated_at,
  };
}

export const healthHistoryService = {
  getInjuryHistory: async (): Promise<InjuryHistorySnapshot> =>
    withOpenApiError(async () =>
      projectInjuryHistory(await getInjuryHistory(undefined, openApiAuthFetch)),
    ),

  updateInjuryHistory: async (input: {
    expected_revision: number;
    summary: string;
  }): Promise<InjuryHistorySnapshot> =>
    withOpenApiError(async () =>
      projectInjuryHistory(
        await updateInjuryHistory(input, undefined, openApiAuthFetch),
      ),
    ),
};
