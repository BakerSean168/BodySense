import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router';
import { toast } from 'sonner';
import { AssistantChatPanel } from '../components/AssistantChatPanel';
import { SessionHistorySidebar } from '../components/SessionHistorySidebar';
import { InfoPanel } from '../components/InfoPanel';
import { DiagnosisPanel } from '../components/DiagnosisPanel';
import { consultationApi } from '../services/consultationService';
import type {
  Conversation,
  Message,
  ExtractedInfo,
  ConsultationPhase,
  Diagnosis,
  DiagnosisAnalysis,
  TreatmentPlan,
} from '../types/consultation';
import { Button } from '@/components/ui/button';
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

  // Core state
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [currentConversation, setCurrentConversation] = useState<Conversation | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [isDraft, setIsDraft] = useState(true);
  const [clientDraftId, setClientDraftId] = useState<string | null>(null);

  // Consultation domain state
  const [extractedInfo, setExtractedInfo] = useState<ExtractedInfo[]>([]);
  const [phase, setPhase] = useState<ConsultationPhase>('collecting');
  const [diagnoses, setDiagnoses] = useState<Diagnosis[]>([]);
  const [treatmentPlan, setTreatmentPlan] = useState<TreatmentPlan | null>(null);

  // UI state
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mobileTab, setMobileTab] = useState<MobileTab>('chat');
  const [isMobileHistoryOpen, setIsMobileHistoryOpen] = useState(false);
  const [isAnalyzingDiagnosis, setIsAnalyzingDiagnosis] = useState(false);
  const [isGeneratingTreatment, setIsGeneratingTreatment] = useState(false);
  const [analysisError, setAnalysisError] = useState<string | null>(null);

  // Load conversations list
  const loadConversations = useCallback(async () => {
    try {
      const res = await consultationApi.listConversations({ limit: 50 });
      setConversations(res.conversations);
    } catch (err) {
      console.error('Failed to load conversations:', err);
    }
  }, []);

  // Load a specific conversation
  const loadConversation = useCallback(async (conversationId: string) => {
    try {
      const res = await consultationApi.getConversation(conversationId);
      setCurrentConversation(res.conversation);
      setMessages(res.messages);
      setIsDraft(false);
      setClientDraftId(null);

      // Load consultation domain data
      try {
        const consultation = await consultationApi.getConsultation(conversationId);
        setExtractedInfo(consultation.extracted_info || []);
        setPhase(consultation.phase || 'collecting');
        setDiagnoses(extractDiagnoses(consultation.diagnosis));
        setTreatmentPlan(extractTreatmentPlan(consultation.treatment_plan));
      } catch {
        // Consultation record may not exist yet
        setExtractedInfo([]);
        setPhase('collecting');
        setDiagnoses([]);
        setTreatmentPlan(null);
      }
    } catch {
      setError('Failed to load conversation');
    }
  }, []);

  // Initialize: load conversations and handle route
  useEffect(() => {
    const init = async () => {
      setIsLoading(true);
      setError(null);
      await loadConversations();

      if (id && id !== 'new') {
        await loadConversation(id);
      } else {
        // New draft conversation
        setCurrentConversation(null);
        setMessages([]);
        setExtractedInfo([]);
        setPhase('collecting');
        setDiagnoses([]);
        setTreatmentPlan(null);
        setIsDraft(true);
        setClientDraftId(crypto.randomUUID());
      }

      setIsLoading(false);
    };

    init();
  }, [id, loadConversations, loadConversation]);

  // --- Handlers ---

  const handleNewConsultation = useCallback(() => {
    setCurrentConversation(null);
    setMessages([]);
    setExtractedInfo([]);
    setPhase('collecting');
    setDiagnoses([]);
    setTreatmentPlan(null);
    setIsDraft(true);
    setClientDraftId(crypto.randomUUID());
    navigate('/consultation/new', { replace: true });
  }, [navigate]);

  const handleSelectConversation = useCallback(
    async (conversationId: string) => {
      setIsMobileHistoryOpen(false);
      await loadConversation(conversationId);
      navigate(`/consultation/${conversationId}`, { replace: true });
    },
    [navigate, loadConversation],
  );

  const handleDeleteConversation = useCallback(
    async (conversationId: string) => {
      try {
        await consultationApi.deleteConversation(conversationId);
        setConversations((prev) => prev.filter((c) => c.id !== conversationId));
        if (currentConversation?.id === conversationId) {
          handleNewConsultation();
        }
      } catch (err) {
        console.error('Failed to delete conversation:', err);
      }
    },
    [currentConversation, handleNewConsultation],
  );

  const handleDeleteAll = useCallback(async () => {
    try {
      // Delete all conversations sequentially (no bulk endpoint)
      await Promise.all(
        conversations.map((c) => consultationApi.deleteConversation(c.id)),
      );
      setConversations([]);
      handleNewConsultation();
    } catch (err) {
      console.error('Failed to delete all conversations:', err);
    }
  }, [conversations, handleNewConsultation]);

  const handlePinConversation = useCallback(
    async (conversationId: string, pinned: boolean) => {
      try {
        await consultationApi.pinConversation(conversationId, pinned);
        setConversations((prev) =>
          prev.map((c) =>
            c.id === conversationId ? { ...c, pinned, pinned_at: pinned ? new Date().toISOString() : null } : c,
          ),
        );
        if (currentConversation?.id === conversationId) {
          setCurrentConversation((prev) =>
            prev ? { ...prev, pinned, pinned_at: pinned ? new Date().toISOString() : null } : null,
          );
        }
      } catch (err) {
        console.error('Failed to pin conversation:', err);
      }
    },
    [currentConversation],
  );

  const handleRenameConversation = useCallback(
    async (conversationId: string, title: string) => {
      try {
        await consultationApi.renameTitle(conversationId, title);
        setConversations((prev) =>
          prev.map((c) => (c.id === conversationId ? { ...c, title } : c)),
        );
        if (currentConversation?.id === conversationId) {
          setCurrentConversation((prev) => (prev ? { ...prev, title } : null));
        }
      } catch (err) {
        console.error('Failed to rename conversation:', err);
      }
    },
    [currentConversation],
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

  const handleConversationCreated = useCallback(
    (conversationId: string) => {
      setIsDraft(false);
      setClientDraftId(null);
      navigate(`/consultation/${conversationId}`, { replace: true });
      loadConversations();
    },
    [navigate, loadConversations],
  );

  const handleTitleGenerated = useCallback(
    (title: string) => {
      if (currentConversation) {
        setCurrentConversation((prev) => (prev ? { ...prev, title } : null));
      }
      // Update in conversations list too
      setConversations((prev) =>
        prev.map((c) =>
          c.id === currentConversation?.id ? { ...c, title } : c,
        ),
      );
    },
    [currentConversation],
  );

  const handleMessagePersisted = useCallback(
    (clientMessageId: string, messageId: string) => {
      // Replace temporary client message ID with the server-assigned ID
      setMessages((prev) =>
        prev.map((m) => (m.id === clientMessageId ? { ...m, id: messageId } : m)),
      );
    },
    [],
  );

  // --- Diagnosis / Treatment ---

  const handleAnalyzeDiagnosis = useCallback(async () => {
    if (!currentConversation || extractedInfo.length === 0 || isAnalyzingDiagnosis) return;

    setIsAnalyzingDiagnosis(true);
    setAnalysisError(null);
    try {
      const result = await consultationApi.analyzeDiagnosis(currentConversation.id);
      setDiagnoses(result.diagnoses || []);
      setPhase('analysis_ready');
    } catch {
      setAnalysisError('生成可能性分析失败，请稍后重试');
    } finally {
      setIsAnalyzingDiagnosis(false);
    }
  }, [currentConversation, extractedInfo.length, isAnalyzingDiagnosis]);

  const handleConfirmAndGenerateTreatment = useCallback(
    async (diagnosis: Diagnosis) => {
      if (!currentConversation || isGeneratingTreatment) return;

      const previousPhase = phase;
      setIsGeneratingTreatment(true);
      setAnalysisError(null);
      try {
        // Confirm diagnosis
        if (phase !== 'diagnosis_confirmed' && phase !== 'plan_ready') {
          await consultationApi.confirmDiagnosis(currentConversation.id, diagnosis);
          setPhase('diagnosis_confirmed');
        }
        // Generate treatment plan
        const result = await consultationApi.generateTreatment(currentConversation.id, diagnosis);
        setTreatmentPlan(result);
        setPhase('plan_ready');
      } catch {
        setAnalysisError('生成改善方案失败，请点击按钮重试');
        // Roll back phase on failure
        setPhase(previousPhase);
      } finally {
        setIsGeneratingTreatment(false);
      }
    },
    [currentConversation, isGeneratingTreatment, phase],
  );

  // --- Loading / Error States ---

  if (isLoading) {
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
                {!isDraft && currentConversation && (
                  <span className="relative flex h-3 w-3">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
                  </span>
                )}
              </h1>
              <p className="text-xs font-semibold text-[#709a83] uppercase tracking-wider">
                {isDraft ? '新咨询草稿' : '会话已激活'}
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
            症状信息
            {extractedInfo.length > 0 && (
              <span className={`inline-flex items-center justify-center px-2 py-0.5 text-xs rounded-full ${
                mobileTab === 'info' ? 'bg-primary-700 text-[#FBFBFA]' : 'bg-[#e2ebe5] text-primary-900'
              }`}>
                {extractedInfo.length}
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
              activeId={currentConversation?.id ?? null}
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
                key={currentConversation?.id ?? 'draft'}
                conversationId={currentConversation?.id ?? null}
                initialMessages={messages.map((m) => ({
                  role: m.role as 'user' | 'assistant',
                  content: m.content_text || m.parts.filter((p) => p.type === 'text').map((p) => (p as { type: 'text'; text: string }).text).join(''),
                }))}
                initialExtractedInfo={extractedInfo}
                isDraft={isDraft}
                clientDraftId={clientDraftId}
                onExtractedInfoUpdate={(info) => {
                  setExtractedInfo(info);
                }}
                onPhaseChange={(newPhase) => {
                  setPhase(newPhase);
                }}
                onConversationCreated={handleConversationCreated}
                onTitleGenerated={handleTitleGenerated}
                onMessagePersisted={handleMessagePersisted}
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
                  已提取 {extractedInfo.length} 项
                </span>
              </div>
              <div className="flex-1 overflow-y-auto p-4 bg-slate-50/10 custom-scrollbar">
                <div>
                  <InfoPanel
                    key={currentConversation?.id ?? 'draft'}
                    extractedInfo={extractedInfo}
                    onConfirm={async (_info) => {
                      if (!currentConversation) return;
                      try {
                        await consultationApi.updateExtractedInfo(currentConversation.id, extractedInfo);
                      } catch {
                        setAnalysisError('保存确认信息失败，请稍后重试');
                      }
                    }}
                    onModify={(index, info) => {
                      setExtractedInfo((prev) => {
                        const updated = [...prev];
                        updated[index] = info;
                        return updated;
                      });
                      if (currentConversation) {
                        const updated = [...extractedInfo];
                        updated[index] = info;
                        consultationApi.updateExtractedInfo(currentConversation.id, updated).catch(() => {
                          setAnalysisError('保存修改失败，请稍后重试');
                        });
                      }
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
                activeId={currentConversation?.id ?? null}
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

// --- Helpers ---

function extractDiagnoses(diagnosis: DiagnosisAnalysis | null): Diagnosis[] {
  if (!diagnosis) return [];
  return diagnosis.diagnoses || [];
}

function extractTreatmentPlan(treatmentPlan: TreatmentPlan | null): TreatmentPlan | null {
  return treatmentPlan ?? null;
}
