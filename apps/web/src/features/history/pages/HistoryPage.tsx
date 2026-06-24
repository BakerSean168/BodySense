import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router';
import { consultationApi, type ConsultationSession } from '@/features/consultation';
import { MainLayout } from '@/components/layout/MainLayout';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';

export function HistoryPage() {
  const navigate = useNavigate();
  const [sessions, setSessions] = useState<ConsultationSession[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadSessions = async () => {
      try {
        const data = await consultationApi.listSessions(50, 0);
        setSessions(data.sessions);
      } catch {
        setError('Failed to load history');
      } finally {
        setIsLoading(false);
      }
    };

    loadSessions();
  }, []);

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'in_progress':
        return '进行中';
      case 'completed':
        return '已完成';
      default:
        return status;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'in_progress':
        return 'bg-emerald-100 text-emerald-700 border-emerald-200';
      case 'completed':
        return 'bg-slate-100 text-slate-700 border-slate-200';
      default:
        return 'bg-slate-100 text-slate-800 border-slate-200';
    }
  };

  const getSummary = (session: ConsultationSession) => {
    const parts = session.extracted_info?.map((info) => info.body_part) || [];
    if (parts.length === 0) return '暂无提取信息';
    return `涉及部位：${[...new Set(parts)].join('、')}`;
  };

  if (isLoading) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-[60vh]">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
            <p className="mt-4 text-slate-500 font-medium">加载中...</p>
          </div>
        </div>
      </MainLayout>
    );
  }

  return (
    <MainLayout>
      <div className="w-full space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-white p-6 rounded-3xl shadow-sm border border-slate-100 relative overflow-hidden">
          <div className="absolute top-0 right-0 w-32 h-32 bg-indigo-100 rounded-full mix-blend-multiply filter blur-3xl opacity-50 translate-x-1/2 -translate-y-1/2"></div>
          
          <div className="relative z-10 flex items-center gap-4">
            <div>
              <h1 className="text-2xl font-bold text-slate-900 tracking-tight">历史记录</h1>
              <p className="text-slate-500 mt-1">查看您过往的 AI 健康咨询记录。</p>
            </div>
          </div>
          <Button onClick={() => navigate('/consultation')} className="relative z-10">
            开始新咨询
          </Button>
        </div>

        {error && (
          <div className="rounded-xl bg-red-50 p-4 border border-red-100">
            <p className="text-sm font-medium text-red-800">{error}</p>
          </div>
        )}

        {/* Content */}
        {sessions.length === 0 ? (
          <Card className="p-12 text-center border-dashed border-2 border-slate-200 bg-slate-50/50">
            <div className="w-20 h-20 bg-white rounded-full shadow-sm flex items-center justify-center mx-auto mb-6">
              <svg className="w-10 h-10 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <h3 className="text-lg font-bold text-slate-900 mb-2">暂无咨询记录</h3>
            <p className="text-slate-500 max-w-sm mx-auto mb-6">
              开始您的第一次 AI 健康咨询，记录将显示在这里。
            </p>
            <Button onClick={() => navigate('/consultation')} variant="outline">
              开始新的咨询
            </Button>
          </Card>
        ) : (
          <div className="space-y-4 relative">
            <div className="absolute top-8 bottom-8 left-8 w-0.5 bg-slate-200 z-0 hidden sm:block"></div>
            {sessions.map((session) => (
              <Card
                key={session.id}
                onClick={() => navigate(`/consultation/${session.id}`)}
                className={`group cursor-pointer hover:shadow-lg transition-all duration-300 relative z-10 sm:ml-16 overflow-hidden ${
                  session.status === 'in_progress' ? 'border-primary-200 bg-primary-50/10' : ''
                }`}
              >
                {/* Timeline dot */}
                <div className={`absolute top-1/2 -translate-y-1/2 -left-12 w-8 h-8 rounded-full border-4 border-white shadow-sm flex items-center justify-center hidden sm:flex ${
                  session.status === 'in_progress' ? 'bg-primary-500' : 'bg-slate-300'
                }`}>
                  <div className="w-2 h-2 rounded-full bg-white"></div>
                </div>

                <div className="p-6">
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-3 gap-2">
                    <div className="flex items-center gap-3">
                      <span
                        className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-bold border ${getStatusColor(session.status)}`}
                      >
                        {getStatusLabel(session.status)}
                      </span>
                      <span className="text-sm font-medium text-slate-500 flex items-center gap-1.5">
                        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        {new Date(session.created_at).toLocaleString('zh-CN', {
                          year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
                        })}
                      </span>
                    </div>
                    <div className="w-8 h-8 rounded-full bg-slate-50 flex items-center justify-center text-slate-400 group-hover:bg-primary-50 group-hover:text-primary-500 transition-colors self-end sm:self-auto">
                      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                      </svg>
                    </div>
                  </div>
                  
                  <div className="bg-slate-50/80 rounded-xl p-4 mt-4 group-hover:bg-white transition-colors border border-slate-100">
                    <p className="text-slate-700 font-medium">{getSummary(session)}</p>
                    {session.diagnosis && (
                      <div className="mt-3 flex items-center gap-2">
                        <span className="flex h-2 w-2 relative">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                        </span>
                        <p className="text-xs font-semibold text-emerald-600 uppercase tracking-wide">
                          已生成诊断方案
                        </p>
                      </div>
                    )}
                  </div>
                </div>
                {session.status === 'in_progress' && (
                  <div className="absolute top-0 left-0 w-1 h-full bg-primary-500"></div>
                )}
              </Card>
            ))}
          </div>
        )}
      </div>
    </MainLayout>
  );
}
