import { listBodyHypothesisRecords } from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import {
  bodyFactRecordsService,
  type BodyFactRecord,
} from "./bodyFactRecordsService";
import {
  bodyObservationService,
  type BodyObservationPage,
} from "./bodyObservationService";
import { bodyEpisodeService, type BodyEpisodePage } from "./bodyEpisodeService";

/** Native ledger records; no clinical state or legacy BodyState projection. */
export interface BodyRecords {
  user_id: string;
  current_revision: number;
  facts: BodyFactRecord[];
  observations: BodyObservationPage["observations"];
  episodes: BodyEpisodePage["episodes"];
  hypotheses: Awaited<
    ReturnType<typeof listBodyHypothesisRecords>
  >["hypotheses"];
}

type Page = {
  user_id: string;
  current_revision: number;
  has_more: boolean;
  next_before_revision?: number;
};

export async function readBodyRecords(owner: string): Promise<BodyRecords> {
  let revision: number | undefined;
  const check = (
    page: { user_id: string; current_revision: number },
    rows: { user_id: string }[],
  ) => {
    if (page.user_id !== owner || rows.some((row) => row.user_id !== owner)) {
      throw new Error("Body records owner mismatch");
    }
    revision ??= page.current_revision;
    if (page.current_revision !== revision) {
      throw new Error("Body records revision changed; retry the read");
    }
  };
  async function readPages<T extends { user_id: string }, P extends Page>(
    list: (cursor?: number) => Promise<P>,
    rows: (page: P) => T[],
  ): Promise<T[]> {
    const result: T[] = [];
    let cursor: number | undefined;
    for (let count = 0; count < 50; count += 1) {
      const page = await list(cursor);
      const items = rows(page);
      check(page, items);
      result.push(...items);
      if (!page.has_more) return result;
      const next = page.next_before_revision;
      if (
        next === undefined ||
        next < 1 ||
        next >= (cursor ?? page.current_revision + 1)
      ) {
        throw new Error("Body records invalid page cursor");
      }
      cursor = next;
    }
    throw new Error("Body records page limit exceeded");
  }
  const [facts, observations, episodes, hypotheses] = await Promise.all([
    readPages(bodyFactRecordsService.list, (page) => page.facts),
    readPages(bodyObservationService.list, (page) => page.observations),
    readPages(bodyEpisodeService.list, (page) => page.episodes),
    withOpenApiError(async () => {
      const page = await listBodyHypothesisRecords(undefined, openApiAuthFetch);
      check(page, page.hypotheses);
      return page.hypotheses;
    }),
  ]);
  // Every source returned at least one page, including empty ledgers.
  if (revision === undefined)
    throw new Error("Body records revision unavailable");
  return {
    user_id: owner,
    current_revision: revision,
    facts,
    observations,
    episodes,
    hypotheses,
  };
}
