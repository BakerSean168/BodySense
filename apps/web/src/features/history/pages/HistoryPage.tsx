import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router';
import { consultationApi, type ConsultationSession } from '@/features/consultation';

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
        return 'bg-blue-100 text-blue-800';
      case 'completed':
        return 'bg-green-100 text-green-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const getSummary = (session: ConsultationSession) => {
    const parts = session.extracted_info?.map((info) => info.body_part) || [];
    if (parts.length === 0) return '暂无提取信息';
    return `涉及部位：${[...new Set(parts)].join('、')}`;
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-gray-500">加载中...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-4 sm:px-6 lg:px-8">
          <div className="flex items-center gap-3">
            <button
              onClick={() => navigate('/dashboard')}
              className="text-gray-500 hover:text-gray-700"
            >
              ← 返回
            </button>
            <h1 className="text-xl font-bold text-gray-900">历史记录</h1>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {error && (
          <div className="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">
            {error}
          </div>
        )}

        {sessions.length === 0 ? (
          <div className="rounded-lg bg-white p-8 text-center shadow">
            <p className="text-gray-500 mb-4">暂无咨询记录</p>
            <button
              onClick={() => navigate('/consultation')}
              className="text-blue-600 hover:underline text-sm"
            >
              开始新的咨询
            </button>
          </div>
        ) : (
          <div className="space-y-4">
            {sessions.map((session) => (
              <button
                key={session.id}
                onClick={() => navigate(`/consultation/${session.id}`)}
                className="w-full rounded-lg bg-white p-5 shadow text-left hover:shadow-md transition-shadow"
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-3">
                    <span
                      className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${getStatusColor(session.status)}`}
                    >
                      {getStatusLabel(session.status)}
                    </span>
                    <span className="text-sm text-gray-500">
                      {new Date(session.created_at).toLocaleString('zh-CN')}
                    </span>
                  </div>
                  <span className="text-gray-400">→</span>
                </div>
                <p className="text-sm text-gray-700">{getSummary(session)}</p>
                {session.diagnosis && (
                  <p className="text-xs text-gray-500 mt-1">
                    已生成诊断方案
                  </p>
                )}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
