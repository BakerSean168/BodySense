import {
  acceptLifestyleCandidate,
  getLifestyle,
  rejectLifestyleCandidate,
  updateLifestyle,
} from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

export type LifestyleSectionKey =
  "activity" | "sleep" | "exercise" | "nutrition" | "substances" | "recovery";

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
> & { expected_revision: number };

type GeneratedLifestyleSnapshot = Awaited<ReturnType<typeof getLifestyle>>;

function projectSection(
  section: GeneratedLifestyleSnapshot["activity"],
): LifestyleSection {
  return {
    kind: section.kind,
    fact_id: section.fact_id,
    summary: section.summary,
    details: section.details,
    valid_from: section.valid_from,
    updated_at: section.updated_at,
    review_state: section.review_state,
  };
}

function projectLifestyleSnapshot(
  snapshot: GeneratedLifestyleSnapshot,
): LifestyleSnapshot {
  return {
    current_revision: snapshot.current_revision,
    activity: projectSection(snapshot.activity),
    sleep: projectSection(snapshot.sleep),
    exercise: projectSection(snapshot.exercise),
    nutrition: projectSection(snapshot.nutrition),
    substances: projectSection(snapshot.substances),
    recovery: projectSection(snapshot.recovery),
    pending_updates: snapshot.pending_updates.map((candidate) => ({
      fact_id: candidate.fact_id,
      kind: candidate.kind,
      summary: candidate.summary,
      details: candidate.details,
      created_at: candidate.created_at,
    })),
  };
}

async function reviewCandidate(
  factId: string,
  expectedRevision: number,
  action: "accept" | "reject",
): Promise<LifestyleSnapshot> {
  return withOpenApiError(async () => {
    const request = { expected_revision: expectedRevision };
    const response =
      action === "accept"
        ? await acceptLifestyleCandidate(
            factId,
            request,
            undefined,
            openApiAuthFetch,
          )
        : await rejectLifestyleCandidate(
            factId,
            request,
            undefined,
            openApiAuthFetch,
          );
    return projectLifestyleSnapshot(response);
  });
}

export const lifestyleService = {
  get: async (): Promise<LifestyleSnapshot> =>
    withOpenApiError(async () =>
      projectLifestyleSnapshot(await getLifestyle(undefined, openApiAuthFetch)),
    ),

  update: async (input: LifestyleUpdate): Promise<LifestyleSnapshot> =>
    withOpenApiError(async () =>
      projectLifestyleSnapshot(
        await updateLifestyle(input, undefined, openApiAuthFetch),
      ),
    ),

  acceptCandidate: async (factId: string, expectedRevision: number) =>
    reviewCandidate(factId, expectedRevision, "accept"),

  rejectCandidate: async (factId: string, expectedRevision: number) =>
    reviewCandidate(factId, expectedRevision, "reject"),
};
