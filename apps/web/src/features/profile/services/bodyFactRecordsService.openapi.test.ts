import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api-client";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { bodyFactRecordsService } from "./bodyFactRecordsService";

const id = "12345678-1234-4234-9234-123456789abc";
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json" },
  });
const command = {
  idempotency_key: "same-user-command",
  expected_revision: 0,
  facts: [
    {
      kind: "lifestyle" as const,
      statement: "I sleep seven hours",
      body_region_ids: [],
      temporal: { precision: "unknown" as const },
      details: { habit_type: "sleep" },
    },
  ],
};
const receipt = {
  id,
  user_id: id,
  revision: 1,
  revision_id: id,
  event_id: id,
  fact_ids: [id],
  committed_at: "2026-10-13T00:00:00Z",
};

describe("Person BodyFact generated service", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("only acknowledges a validated persisted Go receipt", async () => {
    authFetchMock.mockResolvedValue(json(receipt));
    expect(await bodyFactRecordsService.record(command)).toEqual(receipt);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-state/records",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(command),
      }),
    );
  });

  it("never manufactures a successful result from a tool or 2xx response with no receipt", async () => {
    authFetchMock.mockResolvedValue(json({ success: true }));
    await expect(bodyFactRecordsService.record(command)).rejects.toThrow();
  });

  it("rejects a forged message source and legacy lifecycle before making a request", async () => {
    // A malicious runtime payload deliberately bypasses TypeScript only here.
    // The generated strict request parser must still reject it before fetch.
    const forged = {
      ...command,
      facts: [
        {
          ...command.facts[0],
          source_message_id: id,
          lifecycle_state: "resolved",
        },
      ],
    };
    await expect(
      bodyFactRecordsService.record(
        forged as Parameters<typeof bodyFactRecordsService.record>[0],
      ),
    ).rejects.toThrow();
    expect(authFetchMock).not.toHaveBeenCalled();
  });

  it("preserves a revision conflict rather than claiming the user statement was saved", async () => {
    authFetchMock.mockResolvedValue(
      json(
        {
          error: { code: "REVISION_CONFLICT", message: "changed concurrently" },
        },
        409,
      ),
    );
    await expect(bodyFactRecordsService.record(command)).rejects.toMatchObject({
      status: 409,
      code: "REVISION_CONFLICT",
    } satisfies Partial<ApiRequestError>);
    expect(authFetchMock).toHaveBeenCalledTimes(1);
    const sent = authFetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(JSON.parse(sent.body as string).idempotency_key).toBe(
      command.idempotency_key,
    );
  });

  it("reads the authoritative page and rejects a malformed projection", async () => {
    authFetchMock.mockResolvedValue(
      json({
        user_id: id,
        current_revision: 1,
        facts: [],
        has_more: false,
      }),
    );
    const page = await bodyFactRecordsService.list();
    expect(page.facts).toEqual([]);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-state/records",
      expect.objectContaining({ method: "GET" }),
    );
    authFetchMock.mockResolvedValue(
      json({
        user_id: id,
        current_revision: 1,
        facts: null,
        has_more: false,
      }),
    );
    await expect(bodyFactRecordsService.list()).rejects.toThrow();
  });
});

it("validates exact correction and withdrawal requests through generated routes", async () => {
  authFetchMock.mockReset();
  authFetchMock.mockImplementation(async () => json(receipt));
  const correction = {
    idempotency_key: "correction-key",
    expected_revision: 3,
    replacement: command.facts[0],
  };
  await bodyFactRecordsService.correct(id, correction);
  expect(authFetchMock).toHaveBeenLastCalledWith(
    `/api/v1/body-state/records/${id}/correct`,
    expect.objectContaining({
      method: "POST",
      body: JSON.stringify(correction),
    }),
  );
  const withdrawal = { idempotency_key: "withdraw-key", expected_revision: 4 };
  await bodyFactRecordsService.withdraw(id, withdrawal);
  expect(authFetchMock).toHaveBeenLastCalledWith(
    `/api/v1/body-state/records/${id}/withdraw`,
    expect.objectContaining({
      method: "POST",
      body: JSON.stringify(withdrawal),
    }),
  );
  const forged = { ...withdrawal, replacement: command.facts[0] };
  authFetchMock.mockClear();
  await expect(bodyFactRecordsService.withdraw(id, forged)).rejects.toThrow();
  expect(authFetchMock).not.toHaveBeenCalled();
});
