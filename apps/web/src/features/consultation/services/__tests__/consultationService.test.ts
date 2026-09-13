import { describe, it, expect, vi, beforeEach } from "vitest";
import type { DiagnosisAnalysis } from "../../types/consultation";

// Mock authFetch before importing the module
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: vi.fn(),
}));

import { authFetch } from "@/features/auth/services/authService";
import { consultationApi } from "../consultationService";

const mockAuthFetch = vi.mocked(authFetch);

const conversationWire = {
  id: "11111111-1111-4111-8111-111111111111",
  title: "Conversation",
  title_status: "generated" as const,
  status: "active" as const,
  pinned: false,
  metadata: {},
  created_at: "2026-09-13T12:00:00Z",
  updated_at: "2026-09-13T12:00:00Z",
};

const mutationWire = { message: "ok" };

function mockResponse(body: unknown, ok = true, status = 200): Response {
  const resolvedStatus = ok
    ? status >= 200 && status < 300
      ? status
      : 200
    : status >= 400
      ? status
      : 500;
  return new Response(JSON.stringify(body), {
    status: resolvedStatus,
    headers: { "content-type": "application/json" },
  });
}

describe("consultationApi", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ===== General Conversation API =====

  describe("startConsultationRun", () => {
    it("preserves Body Explorer context as user-message metadata", async () => {
      const raw = new Response("stream");
      mockAuthFetch.mockResolvedValue(raw);
      const params = {
        conversationId: conversationWire.id,
        clientMessageId: "client-1",
        requestId: "req-1",
        message: {
          role: "user",
          parts: [{ type: "text", text: "这里为什么疼？" }],
          metadata: {
            body_explorer_context: {
              body_region_id: "shoulder.right",
              body_region_label: "右肩",
              anatomy_id: "appendicular-skeleton-clavicle-right",
              anatomy_name: "Right clavicle",
            },
          },
        },
      };

      const result = await consultationApi.startConsultationRun(params);

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/v1/consultation-runs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(params),
      });
      expect(result).toBe(raw);
    });
  });

  describe("cancelRun", () => {
    it("POSTs an explicit user cancellation command", async () => {
      mockAuthFetch.mockResolvedValue(
        mockResponse({
          status: "cancelled",
          run_id: "22222222-2222-4222-8222-222222222222",
        }),
      );

      const result = await consultationApi.cancelRun(
        "22222222-2222-4222-8222-222222222222",
      );

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/consultation-runs/22222222-2222-4222-8222-222222222222/cancel",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reason: "cancelled_by_user" }),
        },
      );
      expect(result).toEqual({
        status: "cancelled",
        run_id: "22222222-2222-4222-8222-222222222222",
      });
    });
  });

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
    it("uses the generated OpenAPI client and maps pagination to the feature domain", async () => {
      mockAuthFetch.mockResolvedValue(
        mockResponse({ conversations: [], hasMore: false }),
      );

      const result = await consultationApi.listConversations({
        cursor: "2026-09-13T12:00:00Z",
        limit: 10,
      });

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/conversations?cursor=2026-09-13T12%3A00%3A00Z&limit=10",
        { method: "GET" },
      );
      expect(result).toEqual({
        conversations: [],
        next_cursor: null,
        has_more: false,
      });
    });

    it("validates and projects public conversation fields", async () => {
      mockAuthFetch.mockResolvedValue(
        mockResponse({
          conversations: [conversationWire],
          hasMore: true,
          nextCursor: "2026-09-13T12:00:00Z",
        }),
      );

      const result = await consultationApi.listConversations();

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/v1/conversations", {
        method: "GET",
      });
      expect(result).toEqual({
        conversations: [
          {
            id: conversationWire.id,
            title: "Conversation",
            title_status: "generated",
            status: "active",
            pinned: false,
            pinned_at: null,
            default_model: null,
            last_message_at: null,
            message_count: 0,
            metadata: {},
            created_at: conversationWire.created_at,
            updated_at: conversationWire.updated_at,
          },
        ],
        next_cursor: "2026-09-13T12:00:00Z",
        has_more: true,
      });
    });

    it("throws on non-ok response", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse({}, false, 500));
      await expect(consultationApi.listConversations()).rejects.toThrow(
        "API 500",
      );
    });
  });

  describe("getConversation", () => {
    it("uses the generated detail client", async () => {
      mockAuthFetch.mockResolvedValue(
        mockResponse({ conversation: conversationWire, messages: [] }),
      );

      const result = await consultationApi.getConversation(conversationWire.id);

      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/conversations/${conversationWire.id}`,
        { method: "GET" },
      );
      expect(result.conversation.id).toBe(conversationWire.id);
      expect(result.messages).toEqual([]);
    });

    it("throws on non-ok response", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse({}, false, 404));
      await expect(consultationApi.getConversation("bad-id")).rejects.toThrow(
        "API 404",
      );
    });
  });

  describe("deleteConversation", () => {
    it("DELETEs through the generated client", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(mutationWire));
      await consultationApi.deleteConversation(conversationWire.id);
      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/conversations/${conversationWire.id}`,
        { method: "DELETE" },
      );
    });
  });

  describe("pinConversation", () => {
    it("PATCHes through the generated client", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(mutationWire));
      await consultationApi.pinConversation(conversationWire.id, true);
      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/conversations/${conversationWire.id}/pin`,
        expect.objectContaining({
          method: "PATCH",
          body: JSON.stringify({ pinned: true }),
        }),
      );
    });
  });

  describe("generateTitle", () => {
    it("POSTs through the generated client", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(mutationWire, true, 202));
      await consultationApi.generateTitle(conversationWire.id);
      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/conversations/${conversationWire.id}/title`,
        { method: "POST" },
      );
    });
  });

  describe("renameTitle", () => {
    it("PUTs through the generated client", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(mutationWire));
      await consultationApi.renameTitle(conversationWire.id, "New Title");
      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/conversations/${conversationWire.id}/title`,
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ title: "New Title" }),
        }),
      );
    });
  });

  describe("shareConversation", () => {
    it("POSTs through the authenticated generated client", async () => {
      const share = { shareToken: "tok", shareUrl: "https://x" };
      mockAuthFetch.mockResolvedValue(mockResponse(share, true, 201));

      const result = await consultationApi.shareConversation(
        conversationWire.id,
      );
      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/conversations/${conversationWire.id}/share`,
        { method: "POST" },
      );
      expect(result).toEqual(share);
    });
  });

  describe("unshareConversation", () => {
    it("DELETEs through the authenticated generated client", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse(mutationWire));
      await consultationApi.unshareConversation(conversationWire.id);
      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/conversations/${conversationWire.id}/share`,
        { method: "DELETE" },
      );
    });
  });

  describe("getSharedConversation", () => {
    it("uses the public generated client without bearer auth", async () => {
      const shared = { title: "Shared", messages: [] };
      const fetchSpy = vi
        .spyOn(globalThis, "fetch")
        .mockResolvedValue(mockResponse(shared) as unknown as Response);

      const result = await consultationApi.getSharedConversation("tok-123");
      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/v1/conversations/share/tok-123",
        { credentials: "include", method: "GET" },
      );
      expect(mockAuthFetch).not.toHaveBeenCalled();
      expect(result).toEqual(shared);

      fetchSpy.mockRestore();
    });

    it("normalizes public generated-client errors", async () => {
      vi.spyOn(globalThis, "fetch").mockResolvedValue(
        mockResponse({}, false, 404) as unknown as Response,
      );
      await expect(
        consultationApi.getSharedConversation("bad"),
      ).rejects.toThrow("API 404");
    });
  });

  describe("listRunEvents", () => {
    it("uses the generated durable-event client and parses the StreamEvent contract", async () => {
      mockAuthFetch.mockResolvedValue(
        mockResponse({
          events: [
            {
              seq: 3,
              channel: "run",
              type: "run.started",
              ids: {
                conversation_id: conversationWire.id,
                run_id: "22222222-2222-4222-8222-222222222222",
              },
              payload: { status: "running", source: "start_turn" },
              created_at: "2026-09-13T12:00:00Z",
            },
          ],
          hasMore: false,
          nextAfterSeq: 3,
        }),
      );

      const result = await consultationApi.listRunEvents(
        conversationWire.id,
        "22222222-2222-4222-8222-222222222222",
        { afterSeq: 2, limit: 20 },
      );

      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/conversations/${conversationWire.id}/runs/22222222-2222-4222-8222-222222222222/events?after_seq=2&limit=20`,
        { method: "GET" },
      );
      expect(result.hasMore).toBe(false);
      expect(result.nextAfterSeq).toBe(3);
      expect(result.events[0]?.seq).toBe(3);
    });
  });

  // ===== Consultation Domain API =====

  describe("getConsultation", () => {
    it("uses the generated client and maps the session projection", async () => {
      const session = {
        conversation_id: conversationWire.id,
        phase: "collecting",
        extracted_info: [],
        pending_interactions: [],
        created_at: "2026-09-13T12:00:00Z",
        updated_at: "2026-09-13T12:01:00Z",
        ended_at: null,
      };
      mockAuthFetch.mockResolvedValue(mockResponse(session));

      const result = await consultationApi.getConsultation(conversationWire.id);

      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/consultations/${conversationWire.id}`,
        { method: "GET" },
      );
      expect(result).toEqual({
        ...session,
        diagnosis: null,
      });
    });
  });

  describe("getConsultationThread", () => {
    it("validates and maps the durable thread projection", async () => {
      const thread = {
        conversation_id: conversationWire.id,
        conversation: { ...conversationWire, message_count: 0 },
        phase: "collecting",
        extracted_info: [],
        body_state: null,
        pending_interactions: [],
        interaction_history: [],
        active_turn_run_id: null,
        active_turn_events: [],
        messages: [],
        tool_calls: [],
        created_at: "2026-09-13T12:00:00Z",
        updated_at: "2026-09-13T12:01:00Z",
        ended_at: null,
      };
      mockAuthFetch.mockResolvedValue(mockResponse(thread));

      const result = await consultationApi.getConsultationThread(
        conversationWire.id,
      );

      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/consultations/${conversationWire.id}/thread`,
        { method: "GET" },
      );
      expect(result.conversation.id).toBe(conversationWire.id);
      expect(result.diagnosis).toBeNull();
      expect(result.active_turn_events).toEqual([]);
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
