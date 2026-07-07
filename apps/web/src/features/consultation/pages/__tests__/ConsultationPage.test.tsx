import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ConsultationPage } from '../ConsultationPage';
import type { ConsultationThread, Conversation } from '../../types/consultation';

const mockUseConversationsQuery = vi.fn();
const mockUseConsultationThreadQuery = vi.fn();

vi.mock('../../components/AssistantChatPanel', () => ({
  AssistantChatPanel: ({
    conversationId,
  }: {
    conversationId: string;
  }) => <div data-testid="assistant-chat-panel">{conversationId}</div>,
}));

vi.mock('../../components/InfoPanel', () => ({
  InfoPanel: () => <div data-testid="info-panel">info-panel</div>,
}));

vi.mock('../../components/DiagnosisPanel', () => ({
  DiagnosisPanel: () => <div data-testid="diagnosis-panel">diagnosis-panel</div>,
}));

vi.mock('../../hooks/useConversationsQuery', () => ({
  useConversationsQuery: () => mockUseConversationsQuery(),
}));

vi.mock('../../hooks/useConsultationThreadQuery', () => ({
  useConsultationThreadQuery: (conversationId: string | null) =>
    mockUseConsultationThreadQuery(conversationId),
}));

function makeConversation(overrides: Partial<Conversation> = {}): Conversation {
  return {
    id: 'conv-1',
    title: '肩颈咨询',
    title_status: 'generated',
    status: 'active',
    pinned: false,
    pinned_at: null,
    default_model: null,
    last_message_at: null,
    message_count: 0,
    metadata: {},
    created_at: '2026-07-06T00:00:00Z',
    updated_at: '2026-07-06T00:00:00Z',
    ...overrides,
  };
}

function makeThread(overrides: Partial<ConsultationThread> = {}): ConsultationThread {
  const conversation = makeConversation(overrides.conversation);

  return {
    conversation,
    conversation_id: conversation.id,
    phase: 'collecting',
    extracted_info: [],
    health_features: {
      posture_findings: [],
      discomforts: [],
      negative_findings: [],
      movement_limitations: [],
      red_flags: [],
      user_answers: [],
    },
    diagnosis: null,
    treatment_plan: null,
    pending_interactions: [],
    interaction_history: [],
    created_at: '2026-07-06T00:00:00Z',
    updated_at: '2026-07-06T00:00:00Z',
    ended_at: null,
    messages: [],
    tool_calls: [],
    ...overrides,
  };
}

function renderConsultationPage(path = '/consultation/conv-1') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
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

describe('ConsultationPage', () => {
  beforeEach(() => {
    mockUseConversationsQuery.mockReset();
    mockUseConsultationThreadQuery.mockReset();
  });

  it('keeps the workbench shell visible while the target thread is loading', () => {
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

    expect(screen.getByText('智能问诊工作台')).toBeInTheDocument();
    expect(screen.getByTestId('chat-panel-skeleton')).toBeInTheDocument();
    expect(screen.getByTestId('info-panel-skeleton')).toBeInTheDocument();
    expect(screen.queryByText('正在建立 AI 问诊连接...')).not.toBeInTheDocument();
  });

  it('keeps the previous thread mounted and shows panel overlays while switching conversations', async () => {
    mockUseConversationsQuery.mockReturnValue({
      data: [
        makeConversation({ id: 'conv-1', title: '旧会话' }),
        makeConversation({ id: 'conv-2', title: '目标会话' }),
      ],
      isPending: false,
    });
    mockUseConsultationThreadQuery.mockReturnValue({
      data: makeThread({
        conversation: makeConversation({ id: 'conv-1', title: '旧会话' }),
      }),
      isPending: false,
      isFetching: true,
      isError: false,
    });

    renderConsultationPage('/consultation/conv-2');

    expect(await screen.findByTestId('assistant-chat-panel')).toHaveTextContent('conv-1');
    expect(screen.getByTestId('info-panel')).toBeInTheDocument();
    expect(screen.getAllByText('正在切换会话')).toHaveLength(2);
    expect(screen.getAllByText('目标会话')).toHaveLength(3);
    expect(screen.queryByTestId('chat-panel-skeleton')).not.toBeInTheDocument();
    expect(screen.queryByTestId('info-panel-skeleton')).not.toBeInTheDocument();
  });
});
