import { describe, it, expect, vi, beforeEach } from "vitest";

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
  metadata: {},
  created_at: "2026-09-13T12:00:00Z",
  updated_at: "2026-09-13T12:00:00Z",
};

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

  describe("getConversation", () => {
    it("resolves the single conversation through the generated client", async () => {
      mockAuthFetch.mockResolvedValue(
        mockResponse({
          conversation: {
            id: conversationWire.id,
            status: "active",
            metadata: {},
            created_at: conversationWire.created_at,
            updated_at: conversationWire.updated_at,
          },
          messages: [],
        }),
      );

      const result = await consultationApi.getConversation();

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/v1/conversation", {
        method: "GET",
      });
      expect(result).toEqual({ conversationId: conversationWire.id });
    });

    it("throws on non-ok response", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse({}, false, 500));
      await expect(consultationApi.getConversation()).rejects.toThrow(
        "API 500",
      );
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

    it("rejects schema-invalid durable replay events through the shared StreamEvent boundary", async () => {
      mockAuthFetch.mockResolvedValue(
        mockResponse({
          events: [
            {
              seq: 4,
              channel: "message",
              type: "message.text.delta",
              ids: {
                conversation_id: conversationWire.id,
                run_id: "22222222-2222-4222-8222-222222222222",
              },
              payload: {},
              created_at: "2026-09-13T12:00:00Z",
            },
          ],
          hasMore: false,
          nextAfterSeq: 4,
        }),
      );

      await expect(
        consultationApi.listRunEvents(
          conversationWire.id,
          "22222222-2222-4222-8222-222222222222",
        ),
      ).rejects.toThrow("canonical v1 schema");
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
    it("POSTs through the generated diagnosis client", async () => {
      const analysis = {
        analysis_id: "33333333-3333-4333-8333-333333333333",
        body_state_revision: 7,
        status: "completed" as const,
        scope: "full_body",
        summary: "Current durable diagnosis projection",
        candidates: [
          {
            candidate_id: "44444444-4444-4444-8444-444444444444",
            concern_key: "shoulder.right",
            name: "Test",
            confidence: "高" as const,
            severity: "轻度" as const,
            basis: "confirmed fact",
            typical_symptoms: "",
            basis_fact_ids: [],
            basis_observation_ids: [],
            supporting_evidence_ids: [],
            counterevidence_ids: [],
            reasoning_summary: "",
            missing_information: [],
            safety_notes: [],
          },
        ],
        cross_concern_patterns: [],
        information_gaps: [],
        safety_summary: {},
        citations: [],
        governance: {},
        agent_configuration_id: "diag-config-4a517fea19cb6c49",
        agent_configuration: {},
        decision_trace: {},
        execution_provenance: {},
        evidence_acquisition_trace: {},
        created_at: "2026-09-13T12:00:00Z",
      };
      mockAuthFetch.mockResolvedValue(mockResponse(analysis));

      const result = await consultationApi.analyzeDiagnosis(
        conversationWire.id,
      );

      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/consultations/${conversationWire.id}/diagnosis`,
        { method: "POST" },
      );
      expect(result.candidates).toEqual(analysis.candidates);
    });

    it("rejects a response that does not satisfy the canonical durable diagnosis schema", async () => {
      mockAuthFetch.mockResolvedValue(
        mockResponse({
          status: "mystery",
          candidates: [{ name: "Test", confidence: "certain", basis: "" }],
        }),
      );

      await expect(
        consultationApi.analyzeDiagnosis(conversationWire.id),
      ).rejects.toThrow();
    });

    it("throws on non-ok response", async () => {
      mockAuthFetch.mockResolvedValue(mockResponse({}, false, 500));
      await expect(
        consultationApi.analyzeDiagnosis(conversationWire.id),
      ).rejects.toThrow("API 500");
    });
  });

  describe("assessDiagnosisCandidates", () => {
    it("PUTs the generated candidate-assessment request", async () => {
      const analysisId = "33333333-3333-4333-8333-333333333333";
      const candidateId = "44444444-4444-4444-8444-444444444444";
      mockAuthFetch.mockResolvedValue(
        mockResponse({
          analysis_id: analysisId,
          candidate_assessments: [
            {
              id: "55555555-5555-4555-8555-555555555555",
              analysis_id: analysisId,
              candidate_id: candidateId,
              state: "confirmed",
              assessed_at: "2026-09-13T12:20:00Z",
            },
          ],
        }),
      );

      await consultationApi.assessDiagnosisCandidates(analysisId, [
        { candidate_id: candidateId, state: "confirmed" },
      ]);

      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/api/v1/diagnosis-analyses/${analysisId}/assessment`,
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({
            candidates: [{ candidate_id: candidateId, state: "confirmed" }],
          }),
        }),
      );
    });
  });

  describe("listDiagnosisHistory", () => {
    it("validates durable history with the generated projection", async () => {
      const analysisId = "33333333-3333-4333-8333-333333333333";
      const candidateId = "44444444-4444-4444-8444-444444444444";
      mockAuthFetch.mockResolvedValue(
        mockResponse({
          analyses: [
            {
              analysis_id: analysisId,
              body_state_revision: 7,
              status: "completed",
              scope: "full_body",
              summary: "Shoulder pattern",
              candidates: [
                {
                  candidate_id: candidateId,
                  concern_key: "shoulder.right",
                  name: "肩胛控制不足",
                  confidence: "中",
                  basis: "BodyState observations",
                  typical_symptoms: "抬手不适",
                  basis_fact_ids: [],
                  basis_observation_ids: [],
                  supporting_evidence_ids: [],
                  counterevidence_ids: [],
                  reasoning_summary: "pattern",
                  missing_information: [],
                  safety_notes: [],
                },
              ],
              cross_concern_patterns: [],
              information_gaps: [],
              safety_summary: {},
              citations: [],
              governance: {},
              agent_configuration_id: "diagnosis-v3",
              agent_configuration: {},
              decision_trace: {},
              execution_provenance: {},
              evidence_acquisition_trace: {},
              created_at: "2026-09-13T12:00:00Z",
              freshness: {
                analysis_id: analysisId,
                state: "fresh",
                evaluated_against_revision: 7,
                reasons: [],
                checked_at: "2026-09-13T12:01:00Z",
              },
            },
          ],
        }),
      );

      const result = await consultationApi.listDiagnosisHistory(10);

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/v1/diagnosis-analyses?limit=10",
        { method: "GET" },
      );
      expect(result.analyses[0]?.analysis_id).toBe(analysisId);
      expect(result.analyses[0]?.freshness?.state).toBe("fresh");
      expect(result.analyses[0]?.candidates[0]?.name).toBe("肩胛控制不足");
    });
  });
});
