import { beforeEach, describe, expect, it, vi } from "vitest";
import { bodyObservationService } from "./bodyObservationService";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

const owner = "12345678-1234-4234-9234-123456789abc";
const target = "23456789-2345-4345-9345-23456789abcd";
const replacementId = "3456789a-3456-4456-9456-3456789abcde";
const evidence = "456789ab-4567-4567-9567-456789abcdef";
const initial = {
  idempotency_key: "correct-reading",
  expected_revision: 1,
  replacement: {
    kind: "body_weight",
    statement: "I misread the scale; it displayed 65.7 kg",
    raw_value: "65.7",
    raw_unit: "kg",
    observed_at: { precision: "day" as const, start: "2026-10-13" },
  },
};
const receipt = (observationId: string, revision: number) => ({
  id: "56789abc-5678-4678-9678-56789abcdef0",
  user_id: owner,
  observation_id: observationId,
  source_evidence_id: evidence,
  revision,
  revision_id: "6789abcd-6789-4789-9789-6789abcdef01",
  event_id: "789abcde-789a-489a-989a-789abcdef012",
  committed_at: "2026-10-13T00:00:00Z",
});
const json = (value: unknown) =>
  new Response(JSON.stringify(value), {
    status: 200,
    headers: { "content-type": "application/json" },
  });

describe("direct-only source-backed Observation amendments", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("corrects an exact target and carries only the explicitly authored new value", async () => {
    authFetchMock.mockResolvedValueOnce(json(receipt(replacementId, 2)));
    const saved = await bodyObservationService.correct(target, initial);
    expect(saved.revision).toBe(2);
    expect(saved.observation_id).toBe(replacementId);
    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/body-state/observation-records/${target}/correct`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(initial),
      }),
    );
    expect(saved).not.toHaveProperty("clinical_normalized_value");
    expect(saved).not.toHaveProperty("device_confidence");
  });

  it("rejects unverified provenance, forged owner, missing replacement and no source statement before HTTP", async () => {
    await expect(
      bodyObservationService.correct(target, {
        ...initial,
        replacement: {
          ...initial.replacement,
          source_kind: "device",
          normalized_value: 65.7,
        },
      } as typeof initial),
    ).rejects.toThrow();
    await expect(
      bodyObservationService.correct(target, {
        ...initial,
        replacement: { ...initial.replacement, statement: "" },
      }),
    ).rejects.toThrow();
    await expect(
      bodyObservationService.correct(target, {
        ...initial,
        expected_revision: -1,
      }),
    ).rejects.toThrow();
    await expect(
      bodyObservationService.correct(target, {
        ...initial,
        user_id: owner,
      } as typeof initial),
    ).rejects.toThrow();
    expect(authFetchMock).not.toHaveBeenCalled();
  });

  it("withdraws one target with no replacement, date or fabricated medical resolution", async () => {
    const input = {
      idempotency_key: "withdraw-corrected",
      expected_revision: 2,
    };
    authFetchMock.mockResolvedValueOnce(json(receipt(replacementId, 3)));
    const saved = await bodyObservationService.withdraw(replacementId, input);
    expect(saved.revision).toBe(3);
    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/body-state/observation-records/${replacementId}/withdraw`,
      expect.objectContaining({ method: "POST", body: JSON.stringify(input) }),
    );
    await expect(
      bodyObservationService.withdraw(replacementId, {
        ...input,
        replacement: initial.replacement,
      } as typeof input),
    ).rejects.toThrow();
    expect(authFetchMock).toHaveBeenCalledTimes(1);
  });

  it("rejects server-declared success lacking a real durable commit identity", async () => {
    authFetchMock.mockResolvedValueOnce(json({ status: "saved", revision: 3 }));
    await expect(
      bodyObservationService.withdraw(target, {
        idempotency_key: "missing-receipt",
        expected_revision: 2,
      }),
    ).rejects.toThrow();
  });
});
