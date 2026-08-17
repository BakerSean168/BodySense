import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { consultationApi } from "../services/consultationService";
import { consultationKeys } from "../services/consultationQueryKeys";
import type {
  Conversation,
  ConversationListResponse,
  ConsultationThread,
} from "../types/consultation";
import { useConversationActions } from "./useConversationActions";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

afterEach(() => vi.restoreAllMocks());

const conversation: Conversation = {
  id: "conversation-1",
  title: "Neck state",
  title_status: "generated",
  status: "active",
  pinned: false,
  pinned_at: null,
  default_model: null,
  last_message_at: null,
  message_count: 0,
  metadata: {},
  created_at: "2026-08-17T00:00:00Z",
  updated_at: "2026-08-17T00:00:00Z",
};

describe("useConversationActions", () => {
  it("owns list and thread cache reconciliation for pinning", async () => {
    vi.spyOn(consultationApi, "pinConversation").mockResolvedValue();
    const queryClient = new QueryClient();
    queryClient.setQueryData<ConversationListResponse>(
      consultationKeys.conversations(),
      { conversations: [conversation], next_cursor: null, has_more: false },
    );
    queryClient.setQueryData<ConsultationThread>(
      consultationKeys.thread(conversation.id),
      {
        conversation,
        conversation_id: conversation.id,
        phase: "collecting",
        extracted_info: [],
        diagnosis: null,
        pending_interactions: [],
        interaction_history: [],
        created_at: conversation.created_at,
        updated_at: conversation.updated_at,
        ended_at: null,
        messages: [],
        tool_calls: [],
      },
    );
    const wrapper = ({ children }: PropsWithChildren) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(
      () =>
        useConversationActions({
          conversations: [conversation],
          routeConversationId: conversation.id,
          navigateTo: vi.fn(),
        }),
      { wrapper },
    );

    await act(async () => {
      await result.current.pinConversation(conversation.id, true);
    });

    expect(
      queryClient.getQueryData<ConversationListResponse>(
        consultationKeys.conversations(),
      )?.conversations[0].pinned,
    ).toBe(true);
    expect(
      queryClient.getQueryData<ConsultationThread>(
        consultationKeys.thread(conversation.id),
      )?.conversation.pinned,
    ).toBe(true);
  });
});
