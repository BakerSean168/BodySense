import { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate, useParams } from 'react-router';
import { useQueryClient, useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { AssistantChatPanel } from '../components/AssistantChatPanel';
import { SessionHistorySidebar } from '../components/SessionHistorySidebar';
import { InfoPanel } from '../components/InfoPanel';
import { DiagnosisPanel } from '../components/DiagnosisPanel';
import { consultationApi } from '../services/consultationService';
import { consultationKeys } from '../services/consultationQueryKeys';
import { useConversationsQuery } from '../hooks/useConversationsQuery';
import { useConsultationThreadQuery } from '../hooks/useConsultationThreadQuery';
import type {
  ConversationListResponse,
  ExtractedInfo,
  HealthFeatures,
  HealthFeatureItem,
  InteractionHistoryItem,
  ConsultationPhase,
  Diagnosis,
  ConsultationThread,
} from '../types/consultation';
import { buildActiveTurnSeedFromRuntimeEvents, toInitialThreadTimeline } from '../runtime/threadMessageMapping';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { MainLayout } from '@/components/layout/MainLayout';

type MobileTab = 'chat' | 'info';

const PHASE_LABELS: Record<ConsultationPhase, string> = {
  collecting: '问诊收集中',
  ready_for_analysis: '可生成分析',
  analysis_ready: '等待确认诊断',
  diagnosis_confirmed: '诊断已确认',
  plan_ready: '方案已生成',
  completed: '已完成',
};

export function ConsultationPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  // --- Derived identity: URL is the single source of truth for "which conversation" ---
  const routeConversationId = id && id !== 'new' ? id : null;

  const [chatSessionKey, setChatSessionKey] = useState<string>('new');
  const [isPageLoading, setIsPageLoading] = useState(false);
  const justCreatedRef = useRef<string | null>(null);
  // Tracks the active conversation ID synchronously, updated before navigate().
  // Needed because SSE callbacks in the same event batch fire before React re-renders
  // with the new routeConversationId.
  const activeConversationIdRef = useRef<string | null>(null);

  // Sync chatSessionKey and isPageLoading with URL id, skipping if it was just created lazily
  useEffect(() => {
    if (!id || id === 'new') {
      setChatSessionKey('new');
      setIsPageLoading(false);
      activeConversationIdRef.current = null;
      return;
    }

    if (justCreatedRef.current === id) {
      // Clear the ref and skip updating key/loading to keep the active stream alive
      justCreatedRef.current = null;
      return;
    }

    // Navigated to a different conversation — clear the active ref
    activeConversationIdRef.current = null;
    setChatSessionKey(`conversation:${id}`);
    setIsPageLoading(true);
  }, [id]);

  // --- Server state via TanStack Query ---
  const { data: conversations = [] } = useConversationsQuery();
  const threadQuery = useConsultationThreadQuery(routeConversationId);
  const threadData = threadQuery.data;
  const currentConversation = threadData?.conversation ?? null;
  const messages = threadData?.messages ?? [];
  const extractedInfo = threadData?.extracted_info ?? [];
  const healthFeatures = threadData?.health_features ?? {
    posture_findings: [],
    discomforts: [],
    negative_findings: [],
    movement_limitations: [],
    red_flags: [],
    user_answers: [],
  };
  const phase = threadData?.phase ?? 'collecting';
  const diagnoses = threadData?.diagnosis?.diagnoses ?? [];
  const treatmentPlan = threadData?.treatment_plan ?? null;
  const pendingInteractions = threadData?.pending_interactions ?? [];
  const interactionHistory = threadData?.interaction_history ?? [];

  // Reset isPageLoading when query completes
  useEffect(() => {
    if (routeConversationId && !threadQuery.isPending) {
      setIsPageLoading(false);
    }
  }, [threadQuery.isPending, routeConversationId]);

  // UI state
  const [mobileTab, setMobileTab] = useState<MobileTab>('chat');
  const [isMobileHistoryOpen, setIsMobileHistoryOpen] = useState(false);
  const [analysisError, setAnalysisError] = useState<string | null>(null);

  // --- Handlers ---

  const handleNewConsultation = useCallback(() => {
    if (!routeConversationId) {
      return;
    }
    navigate('/consultation', { replace: true });
  }, [routeConversationId, navigate]);

  const handleSelectConversation = useCallback(
    (conversationId: string) => {
      setIsMobileHistoryOpen(false);
      navigate(`/consultation/${conversationId}`, { replace: true });
    },
    [navigate],
  );

  const handleDeleteConversation = useCallback(
    async (conversationId: string) => {
      try {
        await consultationApi.deleteConversation(conversationId);

        // Update conversations list cache
        queryClient.setQueryData<ConversationListResponse>(
          consultationKeys.conversations(),
          (old) =>
            old
              ? { ...old, conversations: old.conversations.filter((c) => c.id !== conversationId) }
              : old,
        );

        // Remove detail caches for the deleted conversation
        queryClient.removeQueries({ queryKey: consultationKeys.thread(conversationId) });

        // If we just deleted the currently-viewed conversation, navigate away.
        // The route change will cause query to become disabled → data resets.
        if (routeConversationId === conversationId) {
          navigate('/consultation', { replace: true });
        }
      } catch (err) {
        console.error('Failed to delete conversation:', err);
      }
    },
    [routeConversationId, navigate, queryClient],
  );

  const handleDeleteAll = useCallback(async () => {
    try {
      await Promise.all(
        conversations.map((c) => consultationApi.deleteConversation(c.id)),
      );

      // Clear the list cache
      queryClient.setQueryData<ConversationListResponse>(
        consultationKeys.conversations(),
        { conversations: [], next_cursor: null, has_more: false },
      );

      // Remove all thread detail caches
      queryClient.removeQueries({
        queryKey: [...consultationKeys.all, 'thread'],
      });

      navigate('/consultation', { replace: true });
    } catch (err) {
      console.error('Failed to delete all conversations:', err);
    }
  }, [conversations, navigate, queryClient]);

  const handlePinConversation = useCallback(
    async (conversationId: string, pinned: boolean) => {
      try {
        await consultationApi.pinConversation(conversationId, pinned);

        // Update conversations list cache
        queryClient.setQueryData<ConversationListResponse>(
          consultationKeys.conversations(),
          (old) =>
            old
              ? {
                  ...old,
                  conversations: old.conversations.map((c) =>
                    c.id === conversationId
                      ? { ...c, pinned, pinned_at: pinned ? new Date().toISOString() : null }
                      : c,
                  ),
                }
              : old,
        );

        // Update conversation detail cache if loaded
        queryClient.setQueryData<ConsultationThread>(
          consultationKeys.thread(conversationId),
          (old) =>
            old
              ? {
                  ...old,
                  conversation: {
                    ...old.conversation,
                    pinned,
                    pinned_at: pinned ? new Date().toISOString() : null,
                  },
                }
              : old,
        );
      } catch (err) {
        console.error('Failed to pin conversation:', err);
      }
    },
    [queryClient],
  );

  const handleRenameConversation = useCallback(
    async (conversationId: string, title: string) => {
      try {
        await consultationApi.renameTitle(conversationId, title);

        // Update conversations list cache
        queryClient.setQueryData<ConversationListResponse>(
          consultationKeys.conversations(),
          (old) =>
            old
              ? {
                  ...old,
                  conversations: old.conversations.map((c) =>
                    c.id === conversationId ? { ...c, title } : c,
                  ),
                }
              : old,
        );

        // Update conversation detail cache if loaded
        queryClient.setQueryData<ConsultationThread>(
          consultationKeys.thread(conversationId),
          (old) =>
            old
              ? { ...old, conversation: { ...old.conversation, title, title_status: 'generated' } }
              : old,
        );
      } catch (err) {
        console.error('Failed to rename conversation:', err);
      }
    },
    [queryClient],
  );

  const handleShareConversation = useCallback(async (conversationId: string) => {
    try {
      const result = await consultationApi.shareConversation(conversationId);
      await navigator.clipboard.writeText(result.shareUrl);
      toast.success('分享链接已复制到剪贴板');
    } catch (err) {
      console.error('Failed to share conversation:', err);
      toast.error('分享失败，请稍后重试');
    }
  }, []);

  const handleUnshareConversation = useCallback(
    async (conversationId: string) => {
      try {
        await consultationApi.unshareConversation(conversationId);
        toast.success('已取消分享');
      } catch (err) {
        console.error('Failed to unshare conversation:', err);
        toast.error('取消分享失败，请稍后重试');
      }
    },
    [],
  );

  // --- SSE Callbacks ---

  const handleTitleGenerated = useCallback(
    (title: string) => {
      const convId = activeConversationIdRef.current ?? routeConversationId;
      console.debug('[SSE] ⑤ ConsultationPage.handleTitleGenerated 开始执行', {
        title,
        convId,
        activeRef: activeConversationIdRef.current,
        routeConversationId,
      });
      if (!convId) {
        console.warn('[SSE] ⚠️ handleTitleGenerated: convId 为空，跳过更新');
        return;
      }

      // Update conversation detail cache
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(convId),
        (old) =>
          old
            ? { ...old, conversation: { ...old.conversation, title, title_status: 'generated' } }
            : old,
      );
      console.debug('[SSE] ⑤ handleTitleGenerated → 已更新 thread 详情缓存', { convId, title });

      // Update conversations list cache
      queryClient.setQueryData<ConversationListResponse>(
        consultationKeys.conversations(),
        (old) =>
          old
            ? {
                ...old,
                conversations: old.conversations.map((c) =>
                  c.id === convId ? { ...c, title, title_status: 'generated' as const } : c,
                ),
              }
            : old,
      );
      console.debug('[SSE] ⑤ handleTitleGenerated → 已更新会话列表缓存 ✅ 完成');
    },
    [routeConversationId, queryClient],
  );

  const handleMessagePersisted = useCallback(
    (clientMessageId: string, messageId: string) => {
      const convId = activeConversationIdRef.current ?? routeConversationId;
      if (!convId) return;

      // Update messages in the conversation detail cache
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(convId),
        (old) =>
          old
            ? {
                ...old,
                messages: old.messages.map((m) =>
                  m.id === clientMessageId ? { ...m, id: messageId } : m,
                ),
              }
            : old,
      );
    },
    [routeConversationId, queryClient],
  );

  const handleExtractedInfoUpdate = useCallback(
    async (info: ExtractedInfo[]) => {
      const convId = activeConversationIdRef.current ?? routeConversationId;
      if (!convId) return;
      await queryClient.cancelQueries({
        queryKey: consultationKeys.thread(convId),
      });
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(convId),
        (old) => (old ? { ...old, extracted_info: info } : old),
      );
    },
    [routeConversationId, queryClient],
  );

  const handleHealthFeaturesUpdate = useCallback(
    async (nextHealthFeatures: HealthFeatures) => {
      const convId = activeConversationIdRef.current ?? routeConversationId;
      if (!convId) return;
      await queryClient.cancelQueries({
        queryKey: consultationKeys.thread(convId),
      });
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(convId),
        (old) => (old ? { ...old, health_features: nextHealthFeatures } : old),
      );
    },
    [routeConversationId, queryClient],
  );

  const handleHealthFeaturesUpsert = useCallback(
    async (incomingHealthFeatures: HealthFeatures) => {
      const convId = activeConversationIdRef.current ?? routeConversationId;
      if (!convId) return;
      await queryClient.cancelQueries({
        queryKey: consultationKeys.thread(convId),
      });
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(convId),
        (old) =>
          old
            ? {
                ...old,
                health_features: mergeHealthFeatures(old.health_features, incomingHealthFeatures),
              }
            : old,
      );
    },
    [routeConversationId, queryClient],
  );

  const handlePhaseChange = useCallback(
    async (newPhase: ConsultationPhase) => {
      const convId = activeConversationIdRef.current ?? routeConversationId;
      if (!convId) return;

      // Skip if phase hasn't changed — avoids unnecessary cancelQueries + setQueryData
      const current = queryClient.getQueryData<ConsultationThread>(
        consultationKeys.thread(convId),
      );
      if (current?.phase === newPhase) return;

      await queryClient.cancelQueries({
        queryKey: consultationKeys.thread(convId),
      });
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(convId),
        (old) => (old ? { ...old, phase: newPhase } : old),
      );
    },
    [routeConversationId, queryClient],
  );

  const handleStreamFinished = useCallback(() => {
    const convId = activeConversationIdRef.current ?? routeConversationId;
    if (!convId) return;
    // Invalidate the durable thread projection and conversation list after the stream completes.
    queryClient.invalidateQueries({
      queryKey: consultationKeys.thread(convId),
    });
    queryClient.invalidateQueries({
      queryKey: consultationKeys.conversations(),
    });
  }, [routeConversationId, queryClient]);

  // --- Diagnosis / Treatment Mutations ---

  const analyzeDiagnosisMutation = useMutation({
    mutationFn: () => {
      if (!routeConversationId) throw new Error('No active conversation');
      return consultationApi.analyzeDiagnosis(routeConversationId);
    },
    onMutate: () => {
      setAnalysisError(null);
    },
    onSuccess: (result) => {
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(routeConversationId!),
        (old) =>
          old
            ? {
                ...old,
                diagnosis: result,
                phase: 'analysis_ready',
              }
            : old,
      );
    },
    onError: () => {
      setAnalysisError('生成可能性分析失败，请稍后重试');
    },
  });

  const confirmAndGenerateTreatmentMutation = useMutation({
    mutationFn: async (diagnosis: Diagnosis) => {
      if (!routeConversationId) throw new Error('No active conversation');

      // Check current session phase from query cache
      const thread = queryClient.getQueryData<ConsultationThread>(
        consultationKeys.thread(routeConversationId),
      );
      const currentPhase = thread?.phase ?? 'collecting';

      if (currentPhase !== 'diagnosis_confirmed' && currentPhase !== 'plan_ready') {
        await consultationApi.confirmDiagnosis(routeConversationId, diagnosis);
      }

      return consultationApi.generateTreatment(routeConversationId, diagnosis);
    },
    onMutate: () => {
      setAnalysisError(null);
    },
    onSuccess: (result) => {
      queryClient.setQueryData<ConsultationThread>(
        consultationKeys.thread(routeConversationId!),
        (old) =>
          old
            ? {
                ...old,
                treatment_plan: result,
                phase: 'plan_ready',
              }
            : old,
      );
    },
    onError: () => {
      setAnalysisError('生成改善方案失败，请点击按钮重试');
    },
  });

  const handleAnalyzeDiagnosis = useCallback(() => {
    if (!routeConversationId || extractedInfo.length === 0 || analyzeDiagnosisMutation.isPending) return;
    analyzeDiagnosisMutation.mutate();
  }, [routeConversationId, extractedInfo.length, analyzeDiagnosisMutation]);

  const handleConfirmAndGenerateTreatment = useCallback(
    (diagnosis: Diagnosis) => {
      if (!routeConversationId || confirmAndGenerateTreatmentMutation.isPending) return;
      confirmAndGenerateTreatmentMutation.mutate(diagnosis);
    },
    [routeConversationId, confirmAndGenerateTreatmentMutation],
  );

  const isAnalyzingDiagnosis = analyzeDiagnosisMutation.isPending;
  const isGeneratingTreatment = confirmAndGenerateTreatmentMutation.isPending;

  // --- Loading / Error States ---

  const error = threadQuery.isError ? '加载会话失败' : null;

  if (isPageLoading) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-[400px] py-12">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-700 mx-auto"></div>
            <p className="mt-4 text-[#709a83] font-semibold">正在建立 AI 问诊连接...</p>
          </div>
        </div>
      </MainLayout>
    );
  }

  if (error) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-[400px] py-12">
          <Card className="max-w-md w-full p-8 text-center bg-white border border-[#E5E3DF]">
            <div className="w-16 h-16 rounded-full bg-red-50 text-[#B65E49] flex items-center justify-center mx-auto mb-4 border border-red-100">
              <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <p className="text-lg font-display font-semibold text-[#1A221E] mb-2">{error}</p>
            <p className="text-[#4A554E] text-sm font-medium mb-6">加载您的健康咨询会话时遇到问题。</p>
            <Button onClick={() => navigate('/dashboard')} className="w-full">
              返回首页
            </Button>
          </Card>
        </div>
      </MainLayout>
    );
  }

  // --- Render ---

  const replayedTurnSeed = buildActiveTurnSeedFromRuntimeEvents(
    threadData?.active_turn_events ?? [],
    pendingInteractions,
  );
  const initialTurnSeed = replayedTurnSeed;
  const historicalMessages = initialTurnSeed?.consumedMessageId
    ? messages.filter((message) => message.id !== initialTurnSeed.consumedMessageId)
    : messages;

  return (
    <MainLayout fullHeight={true}>
      <div className="h-full w-full flex flex-col bg-[#FBFBFA] relative overflow-hidden">
        {/* Background decorations */}
        <div className="absolute top-0 right-0 w-96 h-96 bg-primary-100 rounded-full mix-blend-multiply filter blur-3xl opacity-20 -translate-y-1/2 translate-x-1/2 pointer-events-none" />
        <div className="absolute bottom-0 left-0 w-96 h-96 bg-primary-100 rounded-full mix-blend-multiply filter blur-3xl opacity-20 translate-y-1/2 -translate-x-1/2 pointer-events-none" />

        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 bg-[#FBFBFA] border-b border-[#E5E3DF] z-20 relative shadow-sm">
          <div className="flex items-center gap-4">
            {/* Mobile history toggle */}
            <button
              onClick={() => setIsMobileHistoryOpen(true)}
              className="lg:hidden w-10 h-10 rounded-full bg-[#e2ebe5]/50 flex items-center justify-center text-primary-700 hover:bg-[#e2ebe5] transition-colors border border-[#c5d7cc]/25"
            >
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <div>
              <h1 className="text-xl font-display font-semibold text-[#1A221E] flex items-center gap-2">
                智能问诊工作台
                {currentConversation && (
                  <span className="relative flex h-3 w-3">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
                  </span>
                )}
              </h1>
              <p className="text-xs font-semibold text-[#709a83] uppercase tracking-wider">
                {currentConversation ? '会话已激活' : '准备咨询'}
                {' · '}
                {PHASE_LABELS[phase]}
              </p>
            </div>
          </div>
        </div>

        {/* Mobile tab switcher */}
        <div className="flex bg-[#FBFBFA] border-b border-[#E5E3DF] md:hidden z-10">
          <button
            onClick={() => setMobileTab('chat')}
            className={`flex-1 py-3 text-sm font-semibold transition-all ${
              mobileTab === 'chat'
                ? 'text-primary-700 border-b-2 border-primary-700 bg-primary-50/50'
                : 'text-[#4A554E] hover:text-[#1A221E] hover:bg-primary-50/20'
            }`}
          >
            咨询对话
          </button>
          <button
            onClick={() => setMobileTab('info')}
            className={`flex-1 py-3 text-sm font-semibold transition-all flex items-center justify-center gap-2 ${
              mobileTab === 'info'
                ? 'text-primary-700 border-b-2 border-primary-700 bg-primary-50/50'
                : 'text-[#4A554E] hover:text-[#1A221E] hover:bg-primary-50/20'
            }`}
          >
            健康特征
            {countHealthFeatures(healthFeatures) > 0 && (
              <span className={`inline-flex items-center justify-center px-2 py-0.5 text-xs rounded-full ${
                mobileTab === 'info' ? 'bg-primary-700 text-[#FBFBFA]' : 'bg-[#e2ebe5] text-primary-900'
              }`}>
                {countHealthFeatures(healthFeatures)}
              </span>
            )}
          </button>
        </div>

        {/* Main content area */}
        <div className="flex-1 flex min-h-0 overflow-hidden relative z-10 w-full px-3 md:px-6">
          {/* Desktop left sidebar */}
          <div className="w-64 border-r border-[#E5E3DF] flex-col min-h-0 bg-[#FBFBFA]/50 shrink-0 hidden lg:flex pr-4 py-4 md:py-6">
            <SessionHistorySidebar
              conversations={conversations}
              activeId={routeConversationId}
              onSelect={handleSelectConversation}
              onNew={handleNewConsultation}
              onDelete={handleDeleteConversation}
              onDeleteAll={handleDeleteAll}
              onPin={handlePinConversation}
              onRename={handleRenameConversation}
              onShare={handleShareConversation}
              onUnshare={handleUnshareConversation}
            />
          </div>

          {/* Chat area */}
          <div
            className={`flex-1 flex flex-col md:px-4 py-4 md:py-6 min-h-0 ${
              mobileTab !== 'chat' ? 'hidden md:flex' : ''
            }`}
          >
            <Card className="flex-1 flex flex-col overflow-hidden bg-white/95 backdrop-blur-md border border-[#E5E3DF]">
              <AssistantChatPanel
                key={chatSessionKey}
                conversationId={id || 'new'}
                onConversationCreated={(newId) => {
                  console.debug('[SSE] ⑤ ConsultationPage.onConversationCreated 开始执行', {
                    newId,
                    currentRouteId: id,
                    currentActiveRef: activeConversationIdRef.current,
                  });
                  // Set refs synchronously before navigate() — SSE callbacks in the same
                  // event batch need this ID before React re-renders with the new route.
                  justCreatedRef.current = newId;
                  activeConversationIdRef.current = newId;

                  // Optimistic insert: show the new conversation in the sidebar immediately
                  queryClient.setQueryData<ConversationListResponse>(
                    consultationKeys.conversations(),
                    (old) => {
                      if (!old) return old;
                      const now = new Date().toISOString();
                      return {
                        ...old,
                        conversations: [
                          {
                            id: newId,
                            title: '',
                            title_status: 'pending' as const,
                            status: 'active' as const,
                            pinned: false,
                            pinned_at: null,
                            default_model: null,
                            last_message_at: now,
                            message_count: 0,
                            metadata: {},
                            created_at: now,
                            updated_at: now,
                          },
                          ...old.conversations.filter((c) => c.id !== newId),
                        ],
                      };
                    },
                  );

                  console.debug('[SSE] ⑤ ConsultationPage.onConversationCreated → 已乐观插入缓存，准备导航', {
                    newId,
                    targetUrl: `/consultation/${newId}`,
                  });
                  // Navigate to the new conversation URL (silent, no history entry)
                  navigate(`/consultation/${newId}`, { replace: true });
                  console.debug('[SSE] ⑤ ConsultationPage.onConversationCreated ✅ 完成');
                }}
                initialMessages={toInitialThreadTimeline(
                  historicalMessages,
                  interactionHistory as InteractionHistoryItem[],
                )}
                initialActiveTurn={initialTurnSeed?.activeTurn ?? null}
                initialExtractedInfo={extractedInfo}
                onExtractedInfoUpdate={handleExtractedInfoUpdate}
                onHealthFeaturesUpdate={handleHealthFeaturesUpsert}
                onPhaseChange={handlePhaseChange}
                onTitleGenerated={handleTitleGenerated}
                onMessagePersisted={handleMessagePersisted}
                onStreamFinished={handleStreamFinished}
              />
            </Card>
          </div>

          {/* Info panel */}
          <div
            className={`w-full md:w-[380px] lg:w-[420px] flex-shrink-0 flex flex-col md:pl-4 py-4 md:py-6 min-h-0 ${
              mobileTab !== 'info' ? 'hidden md:flex' : ''
            }`}
          >
            <Card className="flex-1 overflow-hidden flex flex-col bg-white/95 backdrop-blur-md border border-[#E5E3DF]">
              <div className="p-4 border-b border-[#E5E3DF] bg-[#F7F5F0]/50 flex items-center justify-between">
                <h3 className="font-display font-semibold text-[#1A221E] flex items-center gap-2">
                  <svg className="w-5 h-5 text-primary-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                  提取健康特征
                </h3>
                <span className="text-xs font-semibold px-2.5 py-1 bg-primary-100 text-primary-900 rounded-full border border-primary-200/50">
                  已提取 {countHealthFeatures(healthFeatures)} 项
                </span>
              </div>
              <div className="flex-1 overflow-y-auto p-4 bg-slate-50/10 custom-scrollbar">
                <div>
                  <InfoPanel
                    key={routeConversationId ?? 'draft'}
                    healthFeatures={healthFeatures}
                    onConfirm={async (category, index) => {
                      if (!routeConversationId) return;
                      try {
                        const updated = markHealthFeatureConfirmed(healthFeatures, category, index);
                        await handleHealthFeaturesUpdate(updated);
                        await consultationApi.updateHealthFeatures(routeConversationId, updated);
                      } catch {
                        setAnalysisError('保存确认信息失败，请稍后重试');
                        queryClient.invalidateQueries({
                          queryKey: consultationKeys.thread(routeConversationId),
                        });
                      }
                    }}
                    onModify={(category, index, item) => {
                      if (!routeConversationId) return;
                      const updated = updateHealthFeature(healthFeatures, category, index, {
                        ...item,
                        confirmed: !item.confirmed,
                      });
                      handleHealthFeaturesUpdate(updated);

                      consultationApi.updateHealthFeatures(routeConversationId, updated).catch(() => {
                        setAnalysisError('保存修改失败，请稍后重试');
                        queryClient.invalidateQueries({
                          queryKey: consultationKeys.thread(routeConversationId),
                        });
                      });
                    }}
                    onDelete={(category, index) => {
                      if (!routeConversationId) return;
                      const updated = deleteHealthFeature(healthFeatures, category, index);
                      handleHealthFeaturesUpdate(updated);

                      consultationApi.updateHealthFeatures(routeConversationId, updated).catch(() => {
                        setAnalysisError('删除失败，请稍后重试');
                        queryClient.invalidateQueries({
                          queryKey: consultationKeys.thread(routeConversationId),
                        });
                      });
                    }}
                  />
                  <div className="mt-4 rounded-lg border border-[#E5E3DF] bg-white p-4">
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <div>
                        <h3 className="text-sm font-semibold text-[#1A221E]">可能性分析与方案</h3>
                        <p className="text-xs font-medium text-[#709a83]">
                          {PHASE_LABELS[phase]} · 基于已提取信息生成诊断候选和改善方案
                        </p>
                      </div>
                      {!treatmentPlan && diagnoses.length === 0 ? (
                        <Button
                          onClick={handleAnalyzeDiagnosis}
                          disabled={extractedInfo.length === 0 || isAnalyzingDiagnosis}
                          className="shrink-0 rounded-full px-4 py-2 text-xs"
                        >
                          {isAnalyzingDiagnosis ? '分析中...' : '生成分析'}
                        </Button>
                      ) : null}
                    </div>
                    {analysisError ? (
                      <p className="mb-3 rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-xs font-medium text-red-700">
                        {analysisError}
                      </p>
                    ) : null}
                    {diagnoses.length === 0 && !treatmentPlan ? (
                      <p className="text-xs text-gray-400">
                        提取至少一项症状信息后，可以生成可能性分析。
                      </p>
                    ) : (
                      <DiagnosisPanel
                        diagnoses={diagnoses}
                        citations={
                          diagnoses.length > 0
                            ? diagnoses[0]?.name
                              ? undefined
                              : undefined
                            : undefined
                        }
                        treatmentPlan={treatmentPlan}
                        onConfirmAndGenerateTreatment={handleConfirmAndGenerateTreatment}
                        isGeneratingTreatment={isGeneratingTreatment}
                      />
                    )}
                  </div>
                </div>
              </div>
            </Card>
          </div>
        </div>
      </div>

      {/* Mobile history drawer */}
      {isMobileHistoryOpen && (
        <div className="fixed inset-0 z-50 lg:hidden flex">
          {/* Overlay */}
          <div
            className="fixed inset-0 bg-slate-900/50 backdrop-blur-sm"
            onClick={() => setIsMobileHistoryOpen(false)}
          />
          {/* Drawer content */}
          <div className="relative w-72 max-w-xs bg-[#FBFBFA] h-full flex flex-col border-r border-[#E5E3DF] animate-in slide-in-from-left duration-300">
            <div className="p-4 border-b border-[#E5E3DF] flex justify-between items-center bg-white">
              <h3 className="font-display font-semibold text-[#2E3C36] flex items-center gap-1.5">
                <svg className="w-5 h-5 text-[#709a83]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                咨询历史
              </h3>
              <button
                onClick={() => setIsMobileHistoryOpen(false)}
                className="w-8 h-8 rounded-full flex items-center justify-center text-slate-400 hover:bg-slate-100 hover:text-slate-700"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="flex-1 overflow-hidden">
              <SessionHistorySidebar
                conversations={conversations}
                activeId={routeConversationId}
                onSelect={handleSelectConversation}
                onNew={handleNewConsultation}
                onDelete={handleDeleteConversation}
                onDeleteAll={handleDeleteAll}
                onPin={handlePinConversation}
                onRename={handleRenameConversation}
                onShare={handleShareConversation}
                onUnshare={handleUnshareConversation}
              />
            </div>
          </div>
        </div>
      )}
    </MainLayout>
  );
}

function countHealthFeatures(healthFeatures: HealthFeatures) {
  return Object.values(healthFeatures).reduce((total, items) => total + items.length, 0);
}

function markHealthFeatureConfirmed(
  healthFeatures: HealthFeatures,
  category: keyof HealthFeatures,
  index: number,
) {
  const item = healthFeatures[category][index];
  return updateHealthFeature(healthFeatures, category, index, {
    ...item,
    confirmed: true,
  });
}

function updateHealthFeature(
  healthFeatures: HealthFeatures,
  category: keyof HealthFeatures,
  index: number,
  item: HealthFeatureItem,
): HealthFeatures {
  return {
    ...healthFeatures,
    [category]: healthFeatures[category].map((entry, entryIndex) =>
      entryIndex === index ? item : entry,
    ),
  };
}

function deleteHealthFeature(
  healthFeatures: HealthFeatures,
  category: keyof HealthFeatures,
  index: number,
): HealthFeatures {
  return {
    ...healthFeatures,
    [category]: healthFeatures[category].filter((_, entryIndex) => entryIndex !== index),
  };
}

function mergeHealthFeatures(current: HealthFeatures, incoming: HealthFeatures): HealthFeatures {
  return {
    posture_findings: mergeHealthFeatureItems(current.posture_findings, incoming.posture_findings),
    discomforts: mergeHealthFeatureItems(current.discomforts, incoming.discomforts),
    negative_findings: mergeHealthFeatureItems(current.negative_findings, incoming.negative_findings),
    movement_limitations: mergeHealthFeatureItems(current.movement_limitations, incoming.movement_limitations),
    red_flags: mergeHealthFeatureItems(current.red_flags, incoming.red_flags),
    user_answers: mergeHealthFeatureItems(current.user_answers, incoming.user_answers),
  };
}

function mergeHealthFeatureItems(
  current: HealthFeatureItem[],
  incoming: HealthFeatureItem[],
): HealthFeatureItem[] {
  const merged = [...current];
  const seen = new Set(current.map((item) => `${item.label}|${item.body_part ?? ''}|${item.value ?? ''}|${item.details ?? ''}|${item.source ?? ''}`));
  for (const item of incoming) {
    const key = `${item.label}|${item.body_part ?? ''}|${item.value ?? ''}|${item.details ?? ''}|${item.source ?? ''}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    merged.push(item);
  }
  return merged;
}
