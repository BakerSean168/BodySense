import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

vi.mock("@/lib/clientDiagnostics", () => ({
  createClientDiagnosticId: () => "consultation-test-session",
  reportClientDiagnostic: vi.fn(),
}));

import { ConsultationPage } from "../ConsultationPage";
import type {
  ConsultationThread,
  Conversation,
} from "../../types/consultation";

const mockUseConversationsQuery = vi.fn();
const mockUseConsultationThreadQuery = vi.fn();
const mockUseHealthWorkspaceQuery = vi.fn();

vi.mock("../../components/AssistantChatPanel", () => ({
  AssistantChatPanel: ({ conversationId }: { conversationId: string }) => (
    <div data-testid="assistant-chat-panel">{conversationId}</div>
  ),
}));

vi.mock("../../hooks/useConversationsQuery", () => ({
  useConversationsQuery: () => mockUseConversationsQuery(),
}));

vi.mock("../../hooks/useConsultationThreadQuery", () => ({
  useConsultationThreadQuery: (conversationId: string | null) =>
    mockUseConsultationThreadQuery(conversationId),
}));

vi.mock("@/features/workspace/hooks/useHealthWorkspaceQuery", () => ({
  useHealthWorkspaceQuery: () => mockUseHealthWorkspaceQuery(),
}));

function makeConversation(overrides: Partial<Conversation> = {}): Conversation {
  return {
    id: "conv-1",
    title: "肩颈咨询",
    title_status: "generated",
    status: "active",
    pinned: false,
    pinned_at: null,
    default_model: null,
    last_message_at: null,
    message_count: 0,
    metadata: {},
    created_at: "2026-07-06T00:00:00Z",
    updated_at: "2026-07-06T00:00:00Z",
    ...overrides,
  };
}

function makeThread(
  overrides: Partial<ConsultationThread> = {},
): ConsultationThread {
  const conversation = makeConversation(overrides.conversation);
  return {
    conversation,
    conversation_id: conversation.id,
    phase: "collecting",
    extracted_info: [],
    diagnosis: null,
    pending_interactions: [],
    interaction_history: [],
    created_at: "2026-07-06T00:00:00Z",
    updated_at: "2026-07-06T00:00:00Z",
    ended_at: null,
    messages: [],
    tool_calls: [],
    ...overrides,
  };
}

function renderConsultationPage(path = "/consultation/conv-1") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/consultation/:id" element={<ConsultationPage />} />
          <Route path="/consultation" element={<ConsultationPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ConsultationPage", () => {
  beforeEach(() => {
    mockUseConversationsQuery.mockReset();
    mockUseConsultationThreadQuery.mockReset();
    mockUseHealthWorkspaceQuery.mockReset();
    mockUseHealthWorkspaceQuery.mockReturnValue({
      data: undefined,
      isPending: false,
      isError: false,
    });
  });

  it("keeps the body canvas visible while the target thread is loading", () => {
    mockUseConversationsQuery.mockReturnValue({
      data: [makeConversation()],
      isPending: false,
    });
    mockUseConsultationThreadQuery.mockReturnValue({
      data: undefined,
      isPending: true,
      isFetching: true,
      isError: false,
    });

    renderConsultationPage();

    expect(screen.getByTestId("body-canvas-workspace")).toBeInTheDocument();
    expect(screen.getByText("BodySense")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "我的身体" }),
    ).toBeInTheDocument();
    // Body-first: a loading thread must not force a business window open.
    expect(screen.queryByTestId("diagnosis-window")).not.toBeInTheDocument();
    expect(screen.queryByTestId("training-window")).not.toBeInTheDocument();
    expect(
      screen.queryByText("正在建立 AI 问诊连接..."),
    ).not.toBeInTheDocument();
  });

  it("opens the deep-linked business window from ?view=", () => {
    mockUseConversationsQuery.mockReturnValue({
      data: [makeConversation()],
      isPending: false,
    });
    mockUseConsultationThreadQuery.mockReturnValue({
      data: makeThread(),
      isPending: false,
      isFetching: false,
      isError: false,
    });

    renderConsultationPage("/consultation/conv-1?view=diagnosis");

    expect(screen.getByTestId("body-canvas-workspace")).toBeInTheDocument();
    expect(screen.getByTestId("diagnosis-window")).toBeInTheDocument();
  });

  it("ignores an unknown ?view= value and stays body-first", () => {
    mockUseConversationsQuery.mockReturnValue({
      data: [makeConversation()],
      isPending: false,
    });
    mockUseConsultationThreadQuery.mockReturnValue({
      data: makeThread(),
      isPending: false,
      isFetching: false,
      isError: false,
    });

    renderConsultationPage("/consultation/conv-1?view=unknown-surface");

    expect(screen.getByTestId("body-canvas-workspace")).toBeInTheDocument();
    expect(screen.queryByTestId("diagnosis-window")).not.toBeInTheDocument();
    expect(screen.queryByTestId("training-window")).not.toBeInTheDocument();
  });

  it("keeps the previous thread mounted while switching conversations", async () => {
    mockUseConversationsQuery.mockReturnValue({
      data: [
        makeConversation({ id: "conv-1", title: "旧会话" }),
        makeConversation({ id: "conv-2", title: "目标会话" }),
      ],
      isPending: false,
    });
    mockUseConsultationThreadQuery.mockReturnValue({
      data: makeThread({
        conversation: makeConversation({ id: "conv-1", title: "旧会话" }),
      }),
      isPending: false,
      isFetching: true,
      isError: false,
    });

    renderConsultationPage("/consultation/conv-2");

    expect(screen.getByTestId("body-canvas-workspace")).toBeInTheDocument();
    expect(screen.queryByText("目标会话")).not.toBeInTheDocument();
    // A route change alone must not force a business window open; the body stage
    // stays in charge of which surface is shown.
    expect(screen.queryByTestId("diagnosis-window")).not.toBeInTheDocument();
  });
});
