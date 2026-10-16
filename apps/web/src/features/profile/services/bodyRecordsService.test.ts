import { beforeEach, describe, expect, it, vi } from "vitest";
import { bodyFactRecordsService } from "./bodyFactRecordsService";
import { bodyObservationService } from "./bodyObservationService";
import { bodyEpisodeService } from "./bodyEpisodeService";
import { listBodyHypothesisRecords } from "@/generated/api/bodysense";
import { readBodyRecords } from "./bodyRecordsService";

vi.mock("./bodyFactRecordsService", () => ({
  bodyFactRecordsService: { list: vi.fn() },
}));
vi.mock("./bodyObservationService", () => ({
  bodyObservationService: { list: vi.fn() },
}));
vi.mock("./bodyEpisodeService", () => ({
  bodyEpisodeService: { list: vi.fn() },
}));
vi.mock("@/generated/api/bodysense", () => ({
  listBodyHypothesisRecords: vi.fn(),
}));

const owner = "owner";
const head = { user_id: owner, current_revision: 4 };

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(bodyFactRecordsService.list).mockResolvedValue({
    ...head,
    facts: [],
    has_more: false,
  });
  vi.mocked(bodyObservationService.list).mockResolvedValue({
    ...head,
    observations: [],
    has_more: false,
  });
  vi.mocked(bodyEpisodeService.list).mockResolvedValue({
    ...head,
    episodes: [],
    has_more: false,
  });
  vi.mocked(listBodyHypothesisRecords).mockResolvedValue({
    ...head,
    hypotheses: [],
  });
});

describe("readBodyRecords", () => {
  it("follows V2 page cursors and retains the shared revision", async () => {
    vi.mocked(bodyFactRecordsService.list)
      .mockResolvedValueOnce({
        ...head,
        facts: [],
        has_more: true,
        next_before_revision: 3,
      })
      .mockResolvedValueOnce({ ...head, facts: [], has_more: false });
    expect(await readBodyRecords(owner)).toEqual({
      ...head,
      facts: [],
      observations: [],
      episodes: [],
      hypotheses: [],
    });
    expect(bodyFactRecordsService.list).toHaveBeenNthCalledWith(2, 3);
  });

  it("rejects an owner mismatch even on an empty page", async () => {
    vi.mocked(bodyObservationService.list).mockResolvedValue({
      ...head,
      user_id: "other",
      observations: [],
      has_more: false,
    });
    await expect(readBodyRecords(owner)).rejects.toThrow(/owner/);
  });

  it("rejects a foreign row inside an owned page", async () => {
    vi.mocked(listBodyHypothesisRecords).mockResolvedValue({
      ...head,
      hypotheses: [
        {
          id: "hypothesis",
          user_id: "other",
          statement: "自己的猜测",
          source: {
            actor: "user",
            kind: "direct_edit",
            id: "source",
            version: 1,
          },
          lineage_id: "hypothesis",
          related_fact_ids: [],
          related_observation_ids: [],
          assertion_status: "asserted",
          created_revision: 4,
          recorded_at: "2026-10-16T00:00:00Z",
        },
      ],
    });
    await expect(readBodyRecords(owner)).rejects.toThrow(/owner/);
  });

  it("rejects concurrent revision drift instead of presenting a mixed read", async () => {
    vi.mocked(bodyEpisodeService.list).mockResolvedValue({
      ...head,
      current_revision: 5,
      episodes: [],
      has_more: false,
    });
    await expect(readBodyRecords(owner)).rejects.toThrow(/revision/);
  });

  it("rejects a repeated cursor rather than showing incomplete data", async () => {
    vi.mocked(bodyObservationService.list).mockResolvedValue({
      ...head,
      observations: [],
      has_more: true,
      next_before_revision: 3,
    });
    await expect(readBodyRecords(owner)).rejects.toThrow(/cursor/);
  });

  it("propagates read failures instead of returning an empty body", async () => {
    vi.mocked(bodyFactRecordsService.list).mockRejectedValue(
      new Error("unavailable"),
    );
    await expect(readBodyRecords(owner)).rejects.toThrow("unavailable");
  });
});
