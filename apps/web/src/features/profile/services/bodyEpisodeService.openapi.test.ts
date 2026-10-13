import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api-client";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { bodyEpisodeService } from "./bodyEpisodeService";

const owner = "12345678-1234-4234-9234-123456789abc";
const episodeID = "23456789-2345-4345-9345-23456789abcd";
const factID = "3456789a-3456-4456-9456-3456789abcde";
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json" },
  });
const episode = {
  id: episodeID,
  user_id: owner,
  symptom_type: "reported shoulder discomfort",
  body_region_ids: [],
  laterality: "unknown" as const,
  identity_status: "current" as const,
  created_revision: 1,
  created_at: "2026-10-13T00:00:00Z",
  links: [
    {
      fact_id: factID,
      role: "onset" as const,
      association_method: "explicit" as const,
      is_primary: true,
      linked_revision: 1,
    },
  ],
  projection: {
    state: "unknown" as const,
    reason: "insufficient_evidence" as const,
    basis_fact_ids: [],
    based_on_revision: 1,
  },
};

describe("BodyState Episode generated read service", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("reads the authenticated, source-scoped Episode page", async () => {
    authFetchMock.mockResolvedValue(
      json({
        user_id: owner,
        current_revision: 1,
        episodes: [episode],
        has_more: false,
      }),
    );
    expect((await bodyEpisodeService.list()).episodes[0]).toEqual(episode);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-state/episodes",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("passes the generated cursor contract without silently rebasing revisions", async () => {
    authFetchMock.mockResolvedValue(
      json({
        user_id: owner,
        current_revision: 2,
        episodes: [],
        has_more: false,
      }),
    );
    await bodyEpisodeService.list(2);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-state/episodes?before_revision=2",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("rejects malformed cursors and foreign or malformed IDs before network", async () => {
    await expect(bodyEpisodeService.list(-1)).rejects.toThrow();
    await expect(bodyEpisodeService.get("../other-user")).rejects.toThrow();
    expect(authFetchMock).not.toHaveBeenCalled();
  });

  it("only renders a valid source-based projection, not a success-shaped mock", async () => {
    authFetchMock.mockResolvedValue(
      json({
        user_id: owner,
        current_revision: 1,
        episodes: [{ ...episode, projection: { state: "resolved" } }],
        has_more: false,
      }),
    );
    await expect(bodyEpisodeService.list()).rejects.toThrow();
  });

  it("uses the generated detail route and preserves the server's 404 denial", async () => {
    authFetchMock.mockResolvedValueOnce(json(episode));
    expect(await bodyEpisodeService.get(episodeID)).toEqual(episode);
    expect(authFetchMock).toHaveBeenLastCalledWith(
      `/api/v1/body-state/episodes/${episodeID}`,
      expect.objectContaining({ method: "GET" }),
    );
    authFetchMock.mockResolvedValueOnce(
      json(
        { error: { code: "EPISODE_NOT_FOUND", message: "unavailable" } },
        404,
      ),
    );
    await expect(bodyEpisodeService.get(episodeID)).rejects.toMatchObject({
      status: 404,
      code: "EPISODE_NOT_FOUND",
    } satisfies Partial<ApiRequestError>);
  });
});

describe("BodyState Episode explicit user commands", () => {
  beforeEach(() => authFetchMock.mockReset());
  const command = {
    idempotency_key: "user-create-episode",
    expected_revision: 1,
    fact_id: factID,
  };
  const receipt = {
    id: "456789ab-4567-4567-9567-456789abcdef",
    user_id: owner,
    operation: "create",
    episode_id: episodeID,
    fact_id: factID,
    revision: 2,
    revision_id: "56789abc-5678-4678-9678-56789abcdef0",
    event_id: "6789abcd-6789-4789-9789-6789abcdef01",
    committed_at: "2026-10-13T00:00:00Z",
  };

  it("only accepts a real committed create receipt", async () => {
    authFetchMock.mockResolvedValueOnce(json(receipt));
    expect(await bodyEpisodeService.create(command)).toEqual(receipt);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-state/episodes",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(command),
      }),
    );
    authFetchMock.mockResolvedValueOnce(json({ success: true }));
    await expect(bodyEpisodeService.create(command)).rejects.toThrow();
  });

  it("uses the exact Episode and Fact IDs for explicit linking", async () => {
    authFetchMock.mockResolvedValue(json({ ...receipt, operation: "link" }));
    const response = await bodyEpisodeService.linkFact(episodeID, command);
    expect(response.operation).toBe("link");
    expect(response.episode_id).toBe(episodeID);
    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/body-state/episodes/${episodeID}/facts`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(command),
      }),
    );
  });

  it("admits only a source Fact as explicit Episode evidence, never client-supplied state", async () => {
    authFetchMock.mockResolvedValueOnce(
      json({ ...receipt, operation: "status" }),
    );
    const admitted = await bodyEpisodeService.admitStatus(episodeID, command);
    expect(admitted.operation).toBe("status");
    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/body-state/episodes/${episodeID}/status`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(command),
      }),
    );
    await expect(
      bodyEpisodeService.admitStatus(episodeID, {
        ...command,
        state: "ended",
        claim: "explicit_resolution",
      } as Parameters<typeof bodyEpisodeService.admitStatus>[1]),
    ).rejects.toThrow();
    expect(authFetchMock).toHaveBeenCalledTimes(1);
  });

  it("rejects model-injected source, owner, symptom status and extra fields before fetch", async () => {
    const forged = {
      ...command,
      user_id: owner,
      source_evidence_id: factID,
      diagnosis: "migraine",
      state: "ended",
      role: "resolution",
    };
    await expect(
      bodyEpisodeService.create(
        forged as Parameters<typeof bodyEpisodeService.create>[0],
      ),
    ).rejects.toThrow();
    await expect(
      bodyEpisodeService.linkFact("../foreign", command),
    ).rejects.toThrow();
    await expect(
      bodyEpisodeService.create({ ...command, expected_revision: -1 }),
    ).rejects.toThrow();
    expect(authFetchMock).not.toHaveBeenCalled();
  });

  it("preserves optimistic conflicts and source-unavailable denials rather than saying saved", async () => {
    authFetchMock.mockResolvedValueOnce(
      json(
        {
          error: {
            code: "REVISION_CONFLICT",
            message: "reload before retry",
          },
        },
        409,
      ),
    );
    await expect(bodyEpisodeService.create(command)).rejects.toMatchObject({
      status: 409,
      code: "REVISION_CONFLICT",
    } satisfies Partial<ApiRequestError>);
    authFetchMock.mockResolvedValueOnce(
      json(
        {
          error: {
            code: "EPISODE_SOURCE_UNAVAILABLE",
            message: "source no longer available",
          },
        },
        404,
      ),
    );
    await expect(
      bodyEpisodeService.linkFact(episodeID, command),
    ).rejects.toMatchObject({
      status: 404,
      code: "EPISODE_SOURCE_UNAVAILABLE",
    } satisfies Partial<ApiRequestError>);
    expect(authFetchMock).toHaveBeenCalledTimes(2);
  });
});
