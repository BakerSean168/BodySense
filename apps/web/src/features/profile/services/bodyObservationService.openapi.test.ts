import { beforeEach, describe, expect, it, vi } from "vitest";
import { bodyObservationService } from "./bodyObservationService";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

const owner = "12345678-1234-4234-9234-123456789abc";
const observationId = "23456789-2345-4345-9345-23456789abcd";
const evidenceId = "3456789a-3456-4456-9456-3456789abcde";
const commitId = "456789ab-4567-4567-9567-456789abcdef";
const revisionId = "56789abc-5678-4678-9678-56789abcdef0";
const eventId = "6789abcd-6789-4789-9789-6789abcdef01";
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json" },
  });
const command = {
  idempotency_key: "weight-2026-10-13",
  expected_revision: 0,
  observation: {
    kind: "body_weight",
    statement: "I measured 67.5 kilograms today",
    raw_value: "67.5",
    raw_unit: "kg",
    method: "home scale",
    observed_at: { precision: "day" as const, start: "2026-10-13" },
  },
};
const receipt = {
  id: commitId,
  user_id: owner,
  observation_id: observationId,
  source_evidence_id: evidenceId,
  revision: 1,
  revision_id: revisionId,
  event_id: eventId,
  committed_at: "2026-10-13T00:00:00Z",
};

describe("BodyObservation generated direct-user adapter", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("only posts the explicit user's original value without clinical interpretation", async () => {
    authFetchMock.mockResolvedValueOnce(json(receipt));
    expect(await bodyObservationService.record(command)).toEqual(receipt);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-state/observation-records",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(command),
      }),
    );
    const forged = {
      ...command,
      observation: {
        ...command.observation,
        normalized_value: 67.5,
        source_kind: "device",
      },
    };
    await expect(
      bodyObservationService.record(forged as typeof command),
    ).rejects.toThrow();
    expect(authFetchMock).toHaveBeenCalledTimes(1);
  });

  it("cannot supply a clinical, device or document source as the observation kind", async () => {
    await expect(
      bodyObservationService.record({
        ...command,
        observation: {
          ...command.observation,
          kind: "diagnosis:confirmed",
        },
      }),
    ).rejects.toThrow();
    await expect(
      bodyObservationService.record({ ...command, expected_revision: -1 }),
    ).rejects.toThrow();
    await expect(
      bodyObservationService.record({
        ...command,
        observation: {
          ...command.observation,
          statement: "",
        },
      }),
    ).rejects.toThrow();
    expect(authFetchMock).not.toHaveBeenCalled();
  });

  it("reads source availability and user_report attribution, not a medical confidence guess", async () => {
    const sourceRecord = {
      id: observationId,
      user_id: owner,
      kind: "body_weight",
      statement: "I measured 67.5 kilograms today",
      raw_value: "67.5",
      raw_unit: "kg",
      observed_at: { precision: "day", start: "2026-10-13" },
      source: { kind: "user_report", id: evidenceId, version: 1 },
      assertion_status: "asserted",
      created_revision: 1,
      recorded_at: "2026-10-13T00:00:00Z",
    };
    const page = {
      user_id: owner,
      current_revision: 1,
      observations: [sourceRecord],
      has_more: false,
    };
    authFetchMock.mockResolvedValueOnce(json(page));
    const loaded = await bodyObservationService.list();
    expect(loaded.observations[0]).toMatchObject({
      source: { kind: "user_report" },
    });
    expect(authFetchMock).toHaveBeenLastCalledWith(
      "/api/v1/body-state/observation-records",
      expect.objectContaining({ method: "GET" }),
    );
    authFetchMock.mockResolvedValueOnce(json(page));
    await bodyObservationService.list(1);
    expect(authFetchMock).toHaveBeenLastCalledWith(
      "/api/v1/body-state/observation-records?before_revision=1",
      expect.objectContaining({ method: "GET" }),
    );
    await expect(bodyObservationService.list(-1)).rejects.toThrow();
    expect(authFetchMock).toHaveBeenCalledTimes(2);
  });

  it("rejects a success-shaped response with invented clinical provenance", async () => {
    const fake = {
      user_id: owner,
      current_revision: 1,
      has_more: false,
      observations: [
        {
          id: observationId,
          user_id: owner,
          kind: "body_weight",
          statement: "I weighed myself",
          source: { kind: "laboratory_confirmed", id: evidenceId, version: 1 },
          observed_at: { precision: "unknown" },
          assertion_status: "asserted",
          created_revision: 1,
          recorded_at: "2026-10-13T00:00:00Z",
        },
      ],
    };
    authFetchMock.mockResolvedValueOnce(json(fake));
    await expect(bodyObservationService.list()).rejects.toThrow();
  });
});
