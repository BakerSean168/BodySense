import { beforeEach, expect, it, vi } from "vitest";
import { bodyHypothesisService as api } from "./bodyHypothesisService";
const { authFetch } = vi.hoisted(() => ({ authFetch: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({ authFetch }));
const id = "12345678-1234-4234-9234-123456789abc";
const command = {
  idempotency_key: "explicit-user-command",
  expected_revision: 3,
  statement: "我猜可能与久坐有关",
  related_fact_ids: [id],
  related_observation_ids: [id],
};
const receipt = {
  id,
  user_id: id,
  operation: "record",
  hypothesis_id: id,
  source_evidence_id: id,
  revision: 4,
  revision_id: id,
  event_id: id,
  committed_at: "2026-10-17T00:00:00Z",
};
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
beforeEach(() => authFetch.mockReset());
it("uses the native endpoint, strict user command and generated persisted receipt parser", async () => {
  authFetch.mockResolvedValue(json(receipt));
  expect(await api.record(command)).toEqual(receipt);
  expect(authFetch).toHaveBeenCalledWith(
    "/api/v1/body-state/hypothesis-records",
    expect.objectContaining({ method: "POST", body: JSON.stringify(command) }),
  );
});
it("rejects 2xx without a commit and rejects agent provenance before fetch", async () => {
  authFetch.mockResolvedValue(json({ saved: true }));
  await expect(api.record(command)).rejects.toThrow();
  authFetch.mockClear();
  await expect(
    api.record({ ...command, ...{ source: { actor: "agent" } } }),
  ).rejects.toThrow();
  expect(authFetch).not.toHaveBeenCalled();
});
it("sends complete correction links and exact withdrawal target", async () => {
  const replacement = {
    statement: "我猜与运动有关",
    related_fact_ids: [],
    related_observation_ids: [id],
  };
  const correction = {
    idempotency_key: "correct",
    expected_revision: 4,
    replacement,
  };
  authFetch.mockResolvedValue(
    json({
      ...receipt,
      operation: "correct",
      target_hypothesis_id: id,
      revision: 5,
    }),
  );
  await api.correct(id, correction);
  expect(authFetch).toHaveBeenLastCalledWith(
    `/api/v1/body-state/hypothesis-records/${id}/correct`,
    expect.objectContaining({ body: JSON.stringify(correction) }),
  );
  authFetch.mockResolvedValue(
    json({
      ...receipt,
      operation: "withdraw",
      target_hypothesis_id: id,
      revision: 6,
    }),
  );
  await api.withdraw(id, { idempotency_key: "withdraw", expected_revision: 5 });
  expect(authFetch).toHaveBeenLastCalledWith(
    `/api/v1/body-state/hypothesis-records/${id}/withdraw`,
    expect.objectContaining({ method: "POST" }),
  );
});
it.each([404, 409])(
  "preserves owner-masked/stale %s without success",
  async (status) => {
    authFetch.mockResolvedValue(
      json({ error: { code: "CONFLICT", message: "unavailable" } }, status),
    );
    await expect(
      api.withdraw(id, { idempotency_key: "w", expected_revision: 3 }),
    ).rejects.toMatchObject({ status });
  },
);
it("validates native read-only groups and rejects malformed projections", async () => {
  const groups = {
    user_id: id,
    current_revision: 3,
    groups: [
      {
        id,
        display_title: "相关记录",
        latest_revision: 3,
        related_fact_ids: [id],
        related_hypothesis_ids: [id],
        related_observation_ids: [id],
        related_episode_ids: [],
      },
    ],
  };
  authFetch.mockResolvedValue(json(groups));
  expect(await api.groups()).toEqual(groups);
  expect(authFetch).toHaveBeenCalledWith(
    "/api/v1/body-state/concern-groups",
    expect.objectContaining({ method: "GET" }),
  );
  authFetch.mockResolvedValue(
    json({
      ...groups,
      groups: [{ ...groups.groups[0], clinical_diagnosis: "invented" }],
    }),
  );
  await expect(api.groups()).rejects.toThrow();
});
