import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router';
import { ChatPanel } from '../components/ChatPanel';
import {
  consultationApi,
  type ConsultationSession,
  type ExtractedInfo,
} from '../services/consultationService';

export function ConsultationPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [session, setSession] = useState<ConsultationSession | null>(null);
  const [extractedInfo, setExtractedInfo] = useState<ExtractedInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadSession = async () => {
      if (!id) {
        // No ID provided, create a new session
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

    loadSession();
  }, [id, navigate]);

  const handleSessionUpdate = useCallback((updated: ConsultationSession) => {
    setSession(updated);
  }, []);

  const handleExtractedInfoUpdate = useCallback(
    (info: ExtractedInfo[] | ((prev: ExtractedInfo[]) => ExtractedInfo[])) => {
      if (typeof info === 'function') {
        setExtractedInfo(info);
      } else {
        setExtractedInfo(info);
      }
    },
    [],
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-gray-500">加载中...</div>
      </div>
    );
  }

  if (error || !session) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <p className="text-red-500 mb-4">{error || 'Session not found'}</p>
          <button
            onClick={() => navigate('/dashboard')}
            className="text-blue-600 hover:underline"
          >
            返回首页
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="h-screen flex flex-col bg-white">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate('/dashboard')}
            className="text-gray-500 hover:text-gray-700"
          >
            ← 返回
          </button>
          <h1 className="text-lg font-semibold">咨询工作台</h1>
        </div>
        <div className="text-sm text-gray-500">
          {session.status === 'in_progress' ? '进行中' : '已结束'}
        </div>
      </div>

      {/* Main content area */}
      <div className="flex-1 flex overflow-hidden">
        {/* Chat area - left side */}
        <div className="flex-1 flex flex-col border-r">
          <ChatPanel
            session={session}
            onSessionUpdate={handleSessionUpdate}
            onExtractedInfoUpdate={handleExtractedInfoUpdate}
          />
        </div>

        {/* Info panel - right side */}
        <div className="w-80 flex-shrink-0 overflow-y-auto p-4 bg-gray-50 hidden md:block">
          <h2 className="text-sm font-semibold text-gray-700 mb-3">
            提取的信息
          </h2>

          {extractedInfo.length === 0 ? (
            <p className="text-sm text-gray-400">
              对话中提到的症状信息会在这里显示
            </p>
          ) : (
            <div className="space-y-3">
              {extractedInfo.map((info, i) => (
                <div
                  key={i}
                  className="bg-white rounded-lg p-3 shadow-sm border"
                >
                  <div className="font-medium text-sm text-blue-700">
                    {info.body_part}
                  </div>
                  {info.symptom_type && (
                    <div className="text-xs text-gray-600 mt-1">
                      症状：{info.symptom_type}
                    </div>
                  )}
                  {info.duration && (
                    <div className="text-xs text-gray-600">
                      持续时间：{info.duration}
                    </div>
                  )}
                  {info.trigger && (
                    <div className="text-xs text-gray-600">
                      触发场景：{info.trigger}
                    </div>
                  )}
                  {info.severity && (
                    <div className="text-xs text-gray-600">
                      严重程度：{info.severity}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Medical disclaimer */}
          <div className="mt-6 p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
            <p className="text-xs text-yellow-800">
              本分析仅供参考，不构成医疗诊断。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
