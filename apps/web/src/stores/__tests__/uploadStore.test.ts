import { act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "../authStore";
import { useUploadStore } from "../uploadStore";

const uploadWire = {
  id: "11111111-1111-4111-8111-111111111111",
  file_type: "consultation_photo" as const,
  original_name: "photo.png",
  file_size: 3,
  mime_type: "image/png",
  ocr_status: "pending" as const,
  analysis_status: "none" as const,
  created_at: "2026-09-13T15:00:00Z",
  updated_at: "2026-09-13T15:00:00Z",
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  act(() => {
    useAuthStore.setState({
      accessToken: "upload-token",
      isAuthenticated: true,
      hasHydrated: true,
      isAuthResolved: true,
    });
    useUploadStore.setState({ uploads: [], isLoading: false, error: null });
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  act(() => {
    useAuthStore.setState({ accessToken: null, isAuthenticated: false });
    useUploadStore.setState({ uploads: [], isLoading: false, error: null });
  });
});

describe("uploadStore OpenAPI boundary", () => {
  it("loads the generated UploadListResponse and keeps persistence identity out of feature state", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse({ uploads: [uploadWire] }));
    vi.stubGlobal("fetch", fetchMock);

    await useUploadStore.getState().fetchUploads();

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/uploads"),
      expect.objectContaining({ method: "GET" }),
    );
    const [upload] = useUploadStore.getState().uploads;
    expect(upload?.id).toBe(uploadWire.id);
    expect(upload && "user_id" in upload).toBe(false);
    expect(upload && "file_path" in upload).toBe(false);
    expect(useUploadStore.getState().error).toBeNull();
  });

  it("fails closed when a nominal 200 reintroduces persistence/storage compatibility fields", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse({
          uploads: [
            {
              ...uploadWire,
              user_id: "22222222-2222-4222-8222-222222222222",
              file_path: "private/storage/key.png",
            },
          ],
        }),
      ),
    );

    await useUploadStore.getState().fetchUploads();

    expect(useUploadStore.getState().uploads).toEqual([]);
    expect(useUploadStore.getState().error).toBeTruthy();
  });

  it("uploads multipart data through the generated client", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(uploadWire, 201));
    vi.stubGlobal("fetch", fetchMock);
    const file = new File(["png"], "photo.png", { type: "image/png" });

    const upload = await useUploadStore
      .getState()
      .uploadFile(file, "consultation_photo");

    expect(upload.id).toBe(uploadWire.id);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/api/v1/uploads");
    expect(init.method).toBe("POST");
    expect(init.body).toBeInstanceOf(FormData);
    const form = init.body as FormData;
    expect(form.get("file_type")).toBe("consultation_photo");
    expect(form.get("file")).toBeInstanceOf(Blob);
  });
});
