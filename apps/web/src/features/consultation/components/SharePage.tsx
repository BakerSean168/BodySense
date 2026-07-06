import { useEffect, useState } from 'react';
import { useParams } from 'react-router';
import { consultationApi } from '../services/consultationService';
import type { SharedConversation, Message, Diagnosis } from '../types/consultation';

export function SharePage() {
  const { token } = useParams<{ token: string }>();
  const [data, setData] = useState<SharedConversation | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    consultationApi.getSharedConversation(token)
      .then(setData)
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [token]);

  if (loading) return <div className="flex justify-center p-8">加载中...</div>;
  if (error) return <div className="flex justify-center p-8 text-red-500">{error}</div>;
  if (!data) return null;

  // Try to parse diagnosis from metadata
  const diagnosisData = data.metadata?.diagnosis as { diagnoses?: Diagnosis[] } | Diagnosis[] | undefined;
  const diagnoses: Diagnosis[] = Array.isArray(diagnosisData)
    ? diagnosisData
    : diagnosisData?.diagnoses ?? [];

  return (
    <div className="max-w-2xl mx-auto p-6">
      <div className="text-center mb-8">
        <h1 className="text-2xl font-bold">BodySense 问诊分享</h1>
        <p className="text-gray-500 mt-1">{data.title || '问诊记录'}</p>
      </div>

      <div className="space-y-4">
        {data.messages.map((msg: Message) => (
          <div
            key={msg.id}
            className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
          >
            <div
              className={`max-w-[80%] rounded-2xl px-4 py-2 ${
                msg.role === 'user'
                  ? 'bg-primary-600 text-white'
                  : 'bg-gray-100 text-gray-900'
              }`}
            >
              {msg.parts
                .filter(p => p.type === 'text')
                .map((p, i) => (
                  <p key={i}>{(p as { type: 'text'; text: string }).text}</p>
                ))}
            </div>
          </div>
        ))}
      </div>

      {diagnoses.length > 0 && (
        <div className="mt-8 p-4 bg-blue-50 rounded-lg">
          <h2 className="font-semibold mb-3">诊断摘要</h2>
          <div className="space-y-3">
            {diagnoses.map((d, i) => (
              <div key={i} className="rounded-lg bg-white p-3 border border-blue-100">
                <div className="flex items-center justify-between mb-1">
                  <span className="font-medium text-gray-900">{d.name}</span>
                  <span className="text-xs px-2 py-0.5 rounded-full bg-blue-100 text-blue-800">
                    置信度: {d.confidence}
                  </span>
                </div>
                <p className="text-sm text-gray-600">{d.basis}</p>
                {d.typical_symptoms && (
                  <p className="text-xs text-gray-500 mt-1">典型症状: {d.typical_symptoms}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="mt-8 text-center text-sm text-gray-400">
        由 BodySense 智能问诊生成
      </div>
    </div>
  );
}
