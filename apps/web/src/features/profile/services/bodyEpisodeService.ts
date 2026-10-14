import { z } from "zod";
import {
  admitBodyEpisodeStatus,
  createBodyEpisode,
  getBodyEpisode,
  linkBodyEpisodeFact,
  listBodyEpisodes,
  reportBodyEpisodeRecurrence,
  listBodyEpisodeRelations,
} from "@/generated/api/bodysense";
import { ListBodyEpisodesParams } from "@/generated/api/model/listBodyEpisodesParams.zod";
import { ListBodyEpisodeRelationsParams } from "@/generated/api/model/listBodyEpisodeRelationsParams.zod";
import {
  BodyEpisodeRecurrenceRequest,
  type BodyEpisodeRecurrenceRequest as EpisodeRecurrenceInput,
} from "@/generated/api/model/bodyEpisodeRecurrenceRequest.zod";
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
export type BodyEpisodeRecurrenceInput = EpisodeRecurrenceInput;
export type BodyEpisodeRecurrenceReceipt = Awaited<
  ReturnType<typeof reportBodyEpisodeRecurrence>
>;
export type BodyEpisodeRelationPage = Awaited<
  ReturnType<typeof listBodyEpisodeRelations>
>;

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
  // The user explicitly relates a previously ended Episode to a distinct
  // new Episode. No medical etiology or arbitrary relation status is sent.
  reportRecurrence: (
    oldEpisodeId: string,
    input: EpisodeRecurrenceInput,
  ): Promise<BodyEpisodeRecurrenceReceipt> =>
    withOpenApiError(() =>
      reportBodyEpisodeRecurrence(
        z.uuid().parse(oldEpisodeId),
        BodyEpisodeRecurrenceRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
  listRelations: (
    episodeId: string,
    beforeRevision?: number,
  ): Promise<BodyEpisodeRelationPage> =>
    withOpenApiError(() =>
      listBodyEpisodeRelations(
        z.uuid().parse(episodeId),
        ListBodyEpisodeRelationsParams.parse(
          beforeRevision === undefined
            ? {}
            : { before_revision: beforeRevision },
        ),
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
