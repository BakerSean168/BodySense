import { z } from "zod";
import { getBodyEpisode, listBodyEpisodes } from "@/generated/api/bodysense";
import { ListBodyEpisodesParams } from "@/generated/api/model/listBodyEpisodesParams.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// The generated API is the authoritative DTO owner. UI ViewModel mappings
// must not interpret absent state claims as symptom resolution.
export type BodyEpisodePage = Awaited<ReturnType<typeof listBodyEpisodes>>;
export type BodyEpisode = Awaited<ReturnType<typeof getBodyEpisode>>;

export const bodyEpisodeService = {
  list: (beforeRevision?: number): Promise<BodyEpisodePage> =>
    withOpenApiError(() =>
      listBodyEpisodes(
        ListBodyEpisodesParams.parse(
          beforeRevision === undefined
            ? {}
            : { before_revision: beforeRevision },
        ),
        undefined,
        openApiAuthFetch,
      ),
    ),
  get: (episodeId: string): Promise<BodyEpisode> =>
    withOpenApiError(() =>
      getBodyEpisode(z.uuid().parse(episodeId), undefined, openApiAuthFetch),
    ),
};
