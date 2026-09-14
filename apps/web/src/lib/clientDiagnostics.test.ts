import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { reportClientDiagnostic } from "./clientDiagnostics";

const noContent = () => new Response(null, { status: 204 });

describe("client diagnostics OpenAPI boundary", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("sends the canonical v1 scalar envelope through the authenticated generated client", async () => {
    authFetchMock.mockResolvedValue(noContent());

    reportClientDiagnostic({
      category: "body3d.viewer",
      event: "asset_failed",
      severity: "warn",
      resource: "https://assets.example.test/model.glb?token=secret",
      elapsedMs: 12.345,
      attributes: { retry: 2, cached: false, optional: null },
    });

    await vi.waitFor(() => expect(authFetchMock).toHaveBeenCalledTimes(1));
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/client-diagnostics",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          schemaVersion: 1,
          category: "body3d.viewer",
          event: "asset_failed",
          severity: "warn",
          resource: "https://assets.example.test/model.glb?token=secret",
          elapsedMs: 12.3,
          attributes: { retry: 2, cached: false, optional: null },
        }),
      }),
    );
  });

  it("clamps invalid elapsed time out of the wire payload and remains best-effort on transport failure", async () => {
    authFetchMock.mockResolvedValue(new Response(null, { status: 503 }));

    expect(() =>
      reportClientDiagnostic({
        category: "chat.transport",
        event: "offline",
        elapsedMs: Number.NaN,
      }),
    ).not.toThrow();

    await vi.waitFor(() => expect(authFetchMock).toHaveBeenCalledTimes(1));
    const [, init] = authFetchMock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(String(init.body)) as Record<string, unknown>;
    expect(body).not.toHaveProperty("elapsedMs");
    expect(body).toMatchObject({
      schemaVersion: 1,
      category: "chat.transport",
      event: "offline",
      severity: "info",
    });
  });
});
