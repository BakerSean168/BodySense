import { describe, it, expect, vi, beforeEach } from "vitest";
import type { DiagnosisAnalysis } from "../../types/consultation";

// Mock authFetch before importing the module
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: vi.fn(),
}));

import { authFetch } from "@/features/auth/services/authService";
import { consultationApi } from "../consultationService";

const mockAuthFetch = vi.mocked(authFetch);

function mockResponse(body: unknown, ok = true, status = 200): Response {
  return {
    ok,
    status,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
    statusText: "OK",
  } as unknown as Response;
}

describe("consultationApi", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ===== General Conversation API =====

  describe("resumeInteractionStream", () => {
    it("POSTs to the consultation interrupt answer endpoint and returns raw Response", async () => {
      const raw = new Response("stream");
      mockAuthFetch.mockResolvedValue(raw);

      const params = {
        requestId: "req-2",
        answer: { text: "是" },
      };

      const result = await consultationApi.resumeInteractionStream(
        "conv-1",
        "interrupt-1",
        params,
      );

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/consultations/conv-1/interrupts/interrupt-1/answers",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(params),
        },
      );
      expect(result).toBe(raw);
    });
  });

  describe("listConversations", () => {
    it("GETs /api/v1/conversations with pagination params", async () => {
      const data = { conversations: [], nextCursor: null, hasMore: false };
      mockAuthFetch.mockResolvedValue(mockResponse(data));

      const result = await consultationApi.listConversations({
        cursor: "abc",
        limit: 10,
      });

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/conversations?cursor=abc&limit=10",
      );
      expect(result).toEqual(data);
    });

    it("GETs /api/v1/conversations without params", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse({ conversations: [] }));
      await consultationApi.listConversations();
      expect(mockAuthFetch).toHaveBeenCalledWith("/api/v1/conversations");
    });

    it("throws on non-ok response", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse({}, false, 500));
      await expect(consultationApi.listConversations()).rejects.toThrow(
        "API 500",
      );
    });
  });

  describe("getConversation", () => {
    it("GETs /api/v1/conversations/:id", async () => {
      const data = { conversation: { id: "conv-1" }, messages: [] };
      mockAuthFetch.mockResolvedValue(mockResponse(data));

      const result = await consultationApi.getConversation("conv-1");

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/conversations/conv-1",
      );
      expect(result).toEqual(data);
    });

    it("throws on non-ok response", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse({}, false, 404));
      await expect(consultationApi.getConversation("bad-id")).rejects.toThrow(
        "API 404",
      );
    });
  });

  describe("deleteConversation", () => {
    it("DELETEs /api/v1/conversations/:id", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(null));
      await consultationApi.deleteConversation("conv-1");
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/conversations/conv-1",
        {
          method: "DELETE",
        },
      );
    });
  });

  describe("pinConversation", () => {
    it("PATCHes /api/v1/conversations/:id/pin", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(null));
      await consultationApi.pinConversation("conv-1", true);
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/conversations/conv-1/pin",
        {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ pinned: true }),
        },
      );
    });
  });

  describe("generateTitle", () => {
    it("POSTs to /api/v1/conversations/:id/title", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(null));
      await consultationApi.generateTitle("conv-1");
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/conversations/conv-1/title",
        {
          method: "POST",
        },
      );
    });
  });

  describe("renameTitle", () => {
    it("PUTs to /api/v1/conversations/:id/title", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(null));
      await consultationApi.renameTitle("conv-1", "New Title");
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/conversations/conv-1/title",
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ title: "New Title" }),
        },
      );
    });
  });

  describe("shareConversation", () => {
    it("POSTs to /api/v1/conversations/:id/share", async () => {
      const share = { shareToken: "tok", shareUrl: "https://x" };
      mockAuthFetch.mockResolvedValue(mockResponse(share));

      const result = await consultationApi.shareConversation("conv-1");
      expect(result).toEqual(share);
    });
  });

  describe("unshareConversation", () => {
    it("DELETEs /api/v1/conversations/:id/share", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(null));
      await consultationApi.unshareConversation("conv-1");
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/conversations/conv-1/share",
        {
          method: "DELETE",
        },
      );
    });
  });

  describe("getSharedConversation", () => {
    it("fetches public shared conversation without auth", async () => {
      const shared = { title: "Shared", messages: [] };
      const fetchSpy = vi
        .spyOn(globalThis, "fetch")
        .mockResolvedValue(mockResponse(shared) as unknown as Response);

      const result = await consultationApi.getSharedConversation("tok-123");
      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/v1/conversations/share/tok-123",
      );
      expect(result).toEqual(shared);

      fetchSpy.mockRestore();
    });

    it("throws on non-ok response", async () => {
      vi.spyOn(globalThis, "fetch").mockResolvedValue(
        mockResponse({}, false, 404) as unknown as Response,
      );
      await expect(
        consultationApi.getSharedConversation("bad"),
      ).rejects.toThrow("API 404");
    });
  });

  // ===== Consultation Domain API =====

  describe("getConsultation", () => {
    it("GETs /api/v1/consultations/:id", async () => {
      const session = { conversationId: "conv-1", phase: "collecting" };
      mockAuthFetch.mockResolvedValue(mockResponse(session));

      const result = await consultationApi.getConsultation("conv-1");

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/consultations/conv-1",
      );
      expect(result).toEqual(session);
    });
  });

  describe("analyzeDiagnosis", () => {
    it("POSTs to the diagnosis endpoint", async () => {
      const analysis: DiagnosisAnalysis = {
        candidates: [
          {
            name: "Test",
            confidence: "高",
            severity: "轻度",
            basis: "",
          },
        ],
      };
      mockAuthFetch.mockResolvedValue(mockResponse(analysis));

      const result = await consultationApi.analyzeDiagnosis("conv-1");

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/consultations/conv-1/diagnosis",
        { method: "POST" },
      );
      expect(result).toEqual(analysis);
    });

    it("throws on non-ok response", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse({}, false, 500));
      await expect(consultationApi.analyzeDiagnosis("conv-1")).rejects.toThrow(
        "API 500",
      );
    });
  });
});
