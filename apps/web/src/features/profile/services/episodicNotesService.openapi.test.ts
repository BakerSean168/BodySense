import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ApiRequestError } from "@/lib/api-client";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { episodicNotesService } from "./episodicNotesService";

const noteId = "12345678-1234-4234-9234-123456789abc";
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json" },
  });
const note = {
  id: noteId,
  contentText: "用户偏好早上训练。",
  fromSeq: 3,
  toSeq: 8,
  status: "shadow" as const,
  generatorVersion: "episodic-generator-v1",
  retentionPolicyVersion: "episodic-retention-v1",
  createdAt: "2026-11-05T12:00:00Z",
  updatedAt: "2026-11-05T12:00:00Z",
};

describe("episodic notes generated service", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("reads the owner projection through the generated route", async () => {
    authFetchMock.mockResolvedValue(json({ notes: [note] }));
    const page = await episodicNotesService.list();
    expect(page.notes).toEqual([note]);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/profile/episodic-notes",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("rejects a projection that smuggles source or owner identity", async () => {
    authFetchMock.mockResolvedValue(
      json({
        notes: [
          {
            ...note,
            userId: noteId,
            conversationId: noteId,
            sourceMessageIds: [noteId],
          },
        ],
      }),
    );
    await expect(episodicNotesService.list()).rejects.toThrow();
  });

  it("deletes one note through the generated route", async () => {
    const receipt = {
      noteId,
      deleted: true,
      message: "episodic note removed",
    };
    authFetchMock.mockResolvedValue(json(receipt));
    expect(await episodicNotesService.delete(noteId)).toEqual(receipt);
    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/profile/episodic-notes/${noteId}`,
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("preserves an owner-masked 404 instead of claiming success", async () => {
    authFetchMock.mockResolvedValue(
      json(
        { error: { code: "NOT_FOUND", message: "episodic note not found" } },
        404,
      ),
    );
    await expect(episodicNotesService.delete(noteId)).rejects.toMatchObject({
      status: 404,
      code: "NOT_FOUND",
    } satisfies Partial<ApiRequestError>);
  });
});
