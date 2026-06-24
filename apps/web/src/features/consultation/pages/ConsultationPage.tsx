import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router';
import { AssistantChatPanel } from '../components/AssistantChatPanel';
import { InfoPanel } from '../components/InfoPanel';
import {
  consultationApi,
  type ConsultationSession,
  type ExtractedInfo,
} from '../services/consultationService';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { MainLayout } from '@/components/layout/MainLayout';

type MobileTab = 'chat' | 'info';

export function ConsultationPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [session, setSession] = useState<ConsultationSession | null>(null);
  const [extractedInfo, setExtractedInfo] = useState<ExtractedInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mobileTab, setMobileTab] = useState<MobileTab>('chat');
  const [sessions, setSessions] = useState<ConsultationSession[]>([]);
  const [isSessionsLoading, setIsSessionsLoading] = useState(true);
  const [isMobileHistoryOpen, setIsMobileHistoryOpen] = useState(false);

  const isSessionFetching = session?.id !== id;

  useEffect(() => {
    const loadSession = async () => {
      if (!id) {
        try {
          const newSession = await consultationApi.createSession();
          navigate(`/consultation/${newSession.id}`, { replace: true });
          return;
        } catch {
          setError('Failed to create session');
          setIsLoading(false);
          return;
        }
      }

      try {
        const data = await consultationApi.getSession(id);
        setSession(data);
        setExtractedInfo(data.extracted_info || []);
      } catch {
        setError('Failed to load session');
      } finally {
        setIsLoading(false);
      }
    };

    const loadSessionsList = async () => {
      try {
        const data = await consultationApi.listSessions(50, 0);
        setSessions(data.sessions);
      } catch (err) {
        console.error('Failed to load sessions list:', err);
      } finally {
        setIsSessionsLoading(false);
      }
    };

    loadSession();
    loadSessionsList();
  }, [id, navigate]);

  const handleConfirmInfo = useCallback((info: ExtractedInfo) => {
    console.log('Confirmed:', info);
  }, []);

  const handleModifyInfo = useCallback(
    (index: number, info: ExtractedInfo) => {
      setExtractedInfo((prev) => {
        const updated = [...prev];
        updated[index] = info;
        return updated;
      });
    },
    [],
  );

  const handleStartNewConsultation = useCallback(() => {
    if (session && (!session.messages || session.messages.length === 0)) {
      return;
    }
    navigate('/consultation');
  }, [session, navigate]);

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

  if (error || !session) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-[400px] py-12">
          <Card className="max-w-md w-full p-8 text-center bg-white border border-[#E5E3DF]">
            <div className="w-16 h-16 rounded-full bg-red-50 text-[#B65E49] flex items-center justify-center mx-auto mb-4 border border-red-100">
              <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <p className="text-lg font-display font-semibold text-[#1A221E] mb-2">{error || '未找到当前问诊会话'}</p>
            <p className="text-[#4A554E] text-sm font-medium mb-6">加载您的健康咨询会话时遇到问题。</p>
            <Button onClick={() => navigate('/dashboard')} className="w-full">
              返回首页
            </Button>
          </Card>
        </div>
      </MainLayout>
    );
  }

  return (
    <MainLayout fullHeight={true}>
      <div className="h-full w-full flex flex-col bg-[#FBFBFA] relative overflow-hidden">
        {/* Background decorations */}
        <div className="absolute top-0 right-0 w-96 h-96 bg-primary-100 rounded-full mix-blend-multiply filter blur-3xl opacity-20 -translate-y-1/2 translate-x-1/2 pointer-events-none" />
        <div className="absolute bottom-0 left-0 w-96 h-96 bg-primary-100 rounded-full mix-blend-multiply filter blur-3xl opacity-20 translate-y-1/2 -translate-x-1/2 pointer-events-none" />

        {/* Header (Layout overlap fixed using relative z-20) */}
        <div className="flex items-center justify-between px-6 py-4 bg-[#FBFBFA] border-b border-[#E5E3DF] z-20 relative shadow-sm">
          <div className="flex items-center gap-4">
            {/* History Toggle Button on mobile */}
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
                {session.status === 'in_progress' ? (
                  <span className="relative flex h-3 w-3">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
                  </span>
                ) : null}
              </h1>
              <p className="text-xs font-semibold text-[#709a83] uppercase tracking-wider">
                {session.status === 'in_progress' ? '会话已激活' : '会话已结束'}
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

        {/* Main content area (Layout overlap fixed using relative z-10 and px-6) */}
        <div className="flex-1 flex min-h-0 overflow-hidden relative z-10 w-full px-6">
          {/* Desktop Left Sidebar: Sessions list */}
          <div className="w-64 border-r border-[#E5E3DF] flex flex-col min-h-0 bg-[#FBFBFA]/50 shrink-0 hidden lg:flex pr-4 py-4 md:py-6">
            <Button
              onClick={handleStartNewConsultation}
              className="w-full bg-primary-700 hover:bg-primary-800 text-white flex items-center justify-center gap-2 rounded-full py-2.5 mb-4 shrink-0 shadow-sm"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M12 4v16m8-8H4" />
              </svg>
              开始新咨询
            </Button>
            <div className="flex-1 overflow-y-auto space-y-2.5 custom-scrollbar min-h-0 pr-1">
              {sessions.length === 0 && !isSessionsLoading ? (
                <p className="text-xs text-gray-400 text-center py-8">暂无历史咨询</p>
              ) : (
                sessions.map((s) => {
                  const isActive = s.id === id;
                  const dateStr = new Date(s.created_at).toLocaleDateString('zh-CN', {
                    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
                  });
                  const summary = s.extracted_info?.map(i => i.body_part).join('、') || '体态健康咨询';
                  return (
                    <button
                      key={s.id}
                      onClick={() => navigate(`/consultation/${s.id}`)}
                      className={`w-full text-left p-3.5 rounded-[20px] border transition-all duration-300 flex flex-col gap-1.5 cursor-pointer ${
                        isActive
                          ? 'bg-primary-50 border-primary-200 text-primary-900 shadow-sm shadow-[#2a4b3a]/5'
                          : 'bg-white border-[#E5E3DF] hover:border-primary-300 text-slate-700 hover:bg-[#FBFBFA] hover:shadow-[0_2px_12px_rgba(0,0,0,0.02)]'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span className={`text-[10px] px-2 py-0.5 rounded-full font-bold border ${
                          s.status === 'in_progress'
                            ? 'bg-emerald-50 text-emerald-700 border-emerald-100'
                            : 'bg-slate-100 text-slate-600 border-slate-200'
                        }`}>
                          {s.status === 'in_progress' ? '进行中' : '已完成'}
                        </span>
                        <span className="text-[10px] text-slate-400 font-semibold">{dateStr}</span>
                      </div>
                      <span className="text-xs font-bold truncate text-[#2E3C36]">{summary}</span>
                    </button>
                  );
                })
              )}
            </div>
          </div>

          {/* Chat area */}
          <div
            className={`flex-1 flex flex-col md:px-4 py-4 md:py-6 min-h-0 ${
              mobileTab !== 'chat' ? 'hidden md:flex' : ''
            }`}
          >
            <Card className="flex-1 flex flex-col overflow-hidden bg-white/95 backdrop-blur-md border border-[#E5E3DF]">
              {isSessionFetching ? (
                <div className="flex-1 flex items-center justify-center">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-700"></div>
                </div>
              ) : (
                <AssistantChatPanel
                  key={session.id}
                  sessionId={session.id}
                  initialMessages={session.messages}
                  initialExtractedInfo={session.extracted_info || []}
                  onExtractedInfoUpdate={(info) => {
                    setExtractedInfo(info);
                    setSession((prev) => (prev ? { ...prev, extracted_info: info } : null));
                    setSessions((prev) =>
                      prev.map((s) => (s.id === session.id ? { ...s, extracted_info: info } : s))
                    );
                  }}
                />
              )}
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
                {isSessionFetching ? (
                  <div className="h-full flex items-center justify-center">
                    <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-700"></div>
                  </div>
                ) : (
                  <InfoPanel
                    key={session.id}
                    extractedInfo={extractedInfo}
                    onConfirm={handleConfirmInfo}
                    onModify={handleModifyInfo}
                  />
                )}
              </div>
            </Card>
          </div>
        </div>
      </div>

      {/* Mobile History Drawer */}
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
            <div className="p-4 border-b border-[#E5E3DF]">
              <Button
                onClick={() => {
                  handleStartNewConsultation();
                  setIsMobileHistoryOpen(false);
                }}
                className="w-full bg-primary-700 hover:bg-primary-800 text-white flex items-center justify-center gap-2 rounded-full py-2.5"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M12 4v16m8-8H4" />
                </svg>
                开始新咨询
              </Button>
            </div>
            <div className="flex-1 overflow-y-auto p-3 space-y-1.5 custom-scrollbar">
              {sessions.length === 0 && !isSessionsLoading ? (
                <p className="text-xs text-gray-400 text-center py-8">暂无历史咨询</p>
              ) : (
                sessions.map((s) => {
                  const isActive = s.id === id;
                  const dateStr = new Date(s.created_at).toLocaleDateString('zh-CN', {
                    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
                  });
                  const summary = s.extracted_info?.map(i => i.body_part).join('、') || '体态健康咨询';
                  return (
                    <button
                      key={s.id}
                      onClick={() => {
                        navigate(`/consultation/${s.id}`);
                        setIsMobileHistoryOpen(false);
                      }}
                      className={`w-full text-left p-3.5 rounded-[20px] border transition-all duration-300 flex flex-col gap-1.5 cursor-pointer ${
                        isActive
                          ? 'bg-primary-50 border-primary-200 text-primary-900 shadow-sm shadow-[#2a4b3a]/5'
                          : 'bg-white border-[#E5E3DF] hover:border-primary-300 text-slate-700'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span className={`text-[10px] px-2 py-0.5 rounded-full font-bold border ${
                          s.status === 'in_progress'
                            ? 'bg-emerald-50 text-emerald-700 border-emerald-100'
                            : 'bg-slate-100 text-slate-600 border-slate-200'
                        }`}>
                          {s.status === 'in_progress' ? '进行中' : '已完成'}
                        </span>
                        <span className="text-[10px] text-slate-400 font-semibold">{dateStr}</span>
                      </div>
                      <span className="text-xs font-bold truncate text-[#2E3C36]">{summary}</span>
                    </button>
                  );
                })
              )}
            </div>
          </div>
        </div>
      )}
    </MainLayout>
  );
}

