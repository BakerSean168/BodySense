import { z } from "zod";
import {
  admitBodyEpisodeStatus,
  createBodyEpisode,
  getBodyEpisode,
  linkBodyEpisodeFact,
  listBodyEpisodes,
} from "@/generated/api/bodysense";
import { ListBodyEpisodesParams } from "@/generated/api/model/listBodyEpisodesParams.zod";
import {
  BodyEpisodeAssociationRequest,
  type BodyEpisodeAssociationRequest as EpisodeAssociationInput,
} from "@/generated/api/model/bodyEpisodeAssociationRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// The generated API is the authoritative DTO owner. UI ViewModel mappings
// must not interpret absent state claims as symptom resolution.
export type BodyEpisodePage = Awaited<ReturnType<typeof listBodyEpisodes>>;
export type BodyEpisode = Awaited<ReturnType<typeof getBodyEpisode>>;
export type BodyEpisodeCommandReceipt = Awaited<
  ReturnType<typeof createBodyEpisode>
>;
export type BodyEpisodeAssociationInput = EpisodeAssociationInput;

export const bodyEpisodeService = {
  // Both commands require an actual user action. This browser adapter is
  // not a callable AI Episode classifier or a guessed state transition.
  create: (
    input: EpisodeAssociationInput,
  ): Promise<BodyEpisodeCommandReceipt> =>
    withOpenApiError(() =>
      createBodyEpisode(
        BodyEpisodeAssociationRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  // Commit a status EVIDENCE basis, never client-authored "ongoing/ended".
  // The exact linked Fact must contain a previously saved explicit course report.
  admitStatus: (
    episodeId: string,
    input: EpisodeAssociationInput,
  ): Promise<BodyEpisodeCommandReceipt> =>
    withOpenApiError(() =>
      admitBodyEpisodeStatus(
        z.uuid().parse(episodeId),
        BodyEpisodeAssociationRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  linkFact: (
    episodeId: string,
    input: EpisodeAssociationInput,
  ): Promise<BodyEpisodeCommandReceipt> =>
    withOpenApiError(() =>
      linkBodyEpisodeFact(
        z.uuid().parse(episodeId),
        BodyEpisodeAssociationRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
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
