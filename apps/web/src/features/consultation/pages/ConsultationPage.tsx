import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router';
import { ChatPanel } from '../components/ChatPanel';
import { InfoPanel } from '../components/InfoPanel';
import {
  consultationApi,
  type ConsultationSession,
  type ExtractedInfo,
} from '../services/consultationService';

type MobileTab = 'chat' | 'info';

export function ConsultationPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [session, setSession] = useState<ConsultationSession | null>(null);
  const [extractedInfo, setExtractedInfo] = useState<ExtractedInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mobileTab, setMobileTab] = useState<MobileTab>('chat');

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

  const handleConfirmInfo = useCallback((info: ExtractedInfo) => {
    // Mark as confirmed (could update backend in the future)
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

      {/* Mobile tab switcher */}
      <div className="flex border-b md:hidden">
        <button
          onClick={() => setMobileTab('chat')}
          className={`flex-1 py-2 text-sm font-medium text-center transition-colors ${
            mobileTab === 'chat'
              ? 'text-blue-600 border-b-2 border-blue-600'
              : 'text-gray-500 hover:text-gray-700'
          }`}
        >
          对话
        </button>
        <button
          onClick={() => setMobileTab('info')}
          className={`flex-1 py-2 text-sm font-medium text-center transition-colors ${
            mobileTab === 'info'
              ? 'text-blue-600 border-b-2 border-blue-600'
              : 'text-gray-500 hover:text-gray-700'
          }`}
        >
          信息面板
          {extractedInfo.length > 0 && (
            <span className="ml-1 inline-flex items-center justify-center w-5 h-5 text-xs bg-blue-100 text-blue-600 rounded-full">
              {extractedInfo.length}
            </span>
          )}
        </button>
      </div>

      {/* Main content area */}
      <div className="flex-1 flex overflow-hidden">
        {/* Chat area - left side (hidden on mobile when info tab is active) */}
        <div
          className={`flex-1 flex flex-col border-r ${
            mobileTab !== 'chat' ? 'hidden md:flex' : ''
          }`}
        >
          <ChatPanel
            session={session}
            onSessionUpdate={handleSessionUpdate}
            onExtractedInfoUpdate={handleExtractedInfoUpdate}
          />
        </div>

        {/* Info panel - right side (hidden on mobile when chat tab is active) */}
        <div
          className={`w-full md:w-80 flex-shrink-0 overflow-y-auto p-4 bg-gray-50 ${
            mobileTab !== 'info' ? 'hidden md:block' : ''
          }`}
        >
          <InfoPanel
            extractedInfo={extractedInfo}
            onConfirm={handleConfirmInfo}
            onModify={handleModifyInfo}
          />
        </div>
      </div>
    </div>
  );
}
