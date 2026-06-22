import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router';
import {
  assessmentApi,
  type AssessmentReport,
} from '../services/assessmentService';

const GRADE_COLORS: Record<string, string> = {
  A: 'text-green-600 bg-green-50 border-green-200',
  B: 'text-blue-600 bg-blue-50 border-blue-200',
  C: 'text-yellow-600 bg-yellow-50 border-yellow-200',
  D: 'text-red-600 bg-red-50 border-red-200',
};

const GRADE_LABELS: Record<string, string> = {
  A: '优秀',
  B: '良好',
  C: '一般',
  D: '需改善',
};

export function AssessmentListPage() {
  const navigate = useNavigate();
  const [reports, setReports] = useState<AssessmentReport[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isGenerating, setIsGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadReports();
  }, []);

  const loadReports = async () => {
    try {
      const data = await assessmentApi.listReports();
      setReports(data.reports);
    } catch {
      setError('Failed to load reports');
    } finally {
      setIsLoading(false);
    }
  };

  const handleGenerate = async () => {
    setIsGenerating(true);
    setError(null);

    try {
      const report = await assessmentApi.generate();
      navigate(`/assessment/${report.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate report');
    } finally {
      setIsGenerating(false);
    }
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
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <button
                onClick={() => navigate('/dashboard')}
                className="text-gray-500 hover:text-gray-700"
              >
                ← 返回
              </button>
              <h1 className="text-xl font-bold text-gray-900">健康评估报告</h1>
            </div>
            <button
              onClick={handleGenerate}
              disabled={isGenerating}
              className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white
                         hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed
                         transition-colors"
            >
              {isGenerating ? '生成中...' : '生成新报告'}
            </button>
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

        {reports.length === 0 ? (
          <div className="rounded-lg bg-white p-8 text-center shadow">
            <p className="text-gray-500 mb-4">暂无评估报告</p>
            <p className="text-sm text-gray-400">
              点击"生成新报告"按钮，基于您的身体档案生成健康评估
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {reports.map((report) => (
              <button
                key={report.id}
                onClick={() => navigate(`/assessment/${report.id}`)}
                className="w-full rounded-lg bg-white p-6 shadow text-left hover:shadow-md transition-shadow"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div
                      className={`flex h-16 w-16 items-center justify-center rounded-lg border-2 text-2xl font-bold ${GRADE_COLORS[report.health_grade] || ''}`}
                    >
                      {report.health_grade}
                    </div>
                    <div>
                      <h3 className="font-medium text-gray-900">
                        健康等级：{report.health_grade}（
                        {GRADE_LABELS[report.health_grade] || '未知'}）
                      </h3>
                      <p className="text-sm text-gray-500 mt-1">
                        综合评分：{report.dimension_scores?.overall || '-'}
                      </p>
                      <p className="text-xs text-gray-400 mt-1">
                        {new Date(report.created_at).toLocaleString('zh-CN')}
                      </p>
                    </div>
                  </div>
                  <div className="text-gray-400">→</div>
                </div>
              </button>
            ))}
          </div>
        )}

        {/* Disclaimer */}
        <div className="mt-8 rounded-lg bg-yellow-50 border border-yellow-200 p-4">
          <p className="text-xs text-yellow-800">
            本报告仅供参考，不构成医疗诊断。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。
          </p>
        </div>
      </div>
    </div>
  );
}
