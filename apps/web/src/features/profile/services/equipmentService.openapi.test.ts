import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { availabilityChange, equipmentService } from "./equipmentService";

const asset = {
  id: "4df4fc34-371e-49a3-8e74-06bad9dbe64e",
  user_id: "54dc58a5-a2fa-4883-b35a-818057f60cdd",
  display_name: "弹力带",
  notes: "中等阻力",
  access_kind: "owned",
  availability: "available",
  valid_from: null,
  valid_until: null,
  revision: 2,
  created_at: "2026-10-15T08:00:00Z",
  updated_at: "2026-10-15T09:00:00Z",
} as const;

function respond(body: unknown, status = 200) {
  authFetchMock.mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

describe("equipmentService OpenAPI boundary", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("lists the owner's equipment through the generated contract", async () => {
    respond({ items: [asset] });
    const result = await equipmentService.list();
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/equipment",
      expect.objectContaining({ method: "GET" }),
    );
    expect(result.items[0].display_name).toBe("弹力带");
  });

  it("creates with an idempotency key and rejects owner fields client-side", async () => {
    respond({ ...asset, revision: 1 });
    await equipmentService.create({
      idempotency_key: "eq-1",
      display_name: "弹力带",
      notes: "中等阻力",
      access_kind: "owned",
      availability: "available",
    });
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/equipment",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          idempotency_key: "eq-1",
          display_name: "弹力带",
          notes: "中等阻力",
          access_kind: "owned",
          availability: "available",
        }),
      }),
    );
    await expect(
      equipmentService.create({
        idempotency_key: "eq-2",
        display_name: "x",
        access_kind: "owned",
        availability: "available",
        user_id: asset.user_id,
      } as never),
    ).rejects.toThrow();
  });

  it("changes only availability under the asset revision", async () => {
    respond({ ...asset, availability: "unavailable", revision: 3 });
    const result = await equipmentService.update(
      asset.id,
      availabilityChange(asset, "unavailable"),
    );
    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/equipment/${asset.id}`,
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          expected_revision: 2,
          display_name: "弹力带",
          notes: "中等阻力",
          access_kind: "owned",
          availability: "unavailable",
          valid_from: null,
          valid_until: null,
        }),
      }),
    );
    expect(result.revision).toBe(3);
  });

  it("normalizes conflicts and fails closed on an invalid response", async () => {
    respond(
      { error: { code: "EQUIPMENT_DUPLICATE_NAME", message: "dup" } },
      409,
    );
    await expect(equipmentService.list()).rejects.toMatchObject({
      name: "ApiRequestError",
      status: 409,
      code: "EQUIPMENT_DUPLICATE_NAME",
    });
    respond({ items: [{ id: "nope" }] });
    await expect(equipmentService.list()).rejects.toThrow();
  });
});
