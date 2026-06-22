import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router';
import {
  assessmentApi,
  type AssessmentReport,
  type DimensionScores,
  type IdentifiedIssue,
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

const DIMENSION_LABELS: Record<keyof DimensionScores, string> = {
  posture: '体态',
  exercise: '运动能力',
  lifestyle: '生活习惯',
  injury_risk: '伤病风险',
  overall: '综合',
};

const SEVERITY_COLORS: Record<string, string> = {
  '轻度': 'bg-green-100 text-green-800',
  '中度': 'bg-yellow-100 text-yellow-800',
  '重度': 'bg-red-100 text-red-800',
};

function ScoreBar({ label, score }: { label: string; score: number }) {
  const color =
    score >= 80
      ? 'bg-green-500'
      : score >= 60
        ? 'bg-yellow-500'
        : 'bg-red-500';

  return (
    <div className="flex items-center gap-3">
      <span className="w-20 text-sm text-gray-600">{label}</span>
      <div className="flex-1 h-3 bg-gray-200 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full ${color} transition-all`}
          style={{ width: `${score}%` }}
        />
      </div>
      <span className="w-10 text-sm font-medium text-gray-700 text-right">
        {score}
      </span>
    </div>
  );
}

function IssueCard({ issue }: { issue: IdentifiedIssue }) {
  return (
    <div className="rounded-lg border p-4">
      <div className="flex items-center gap-2 mb-2">
        <h4 className="font-medium text-gray-900">{issue.issue}</h4>
        <span
          className={`rounded-full px-2 py-0.5 text-xs font-medium ${SEVERITY_COLORS[issue.severity] || 'bg-gray-100 text-gray-800'}`}
        >
          {issue.severity}
        </span>
      </div>
      <p className="text-sm text-gray-600">{issue.description}</p>
    </div>
  );
}

export function AssessmentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [report, setReport] = useState<AssessmentReport | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) {
      navigate('/assessment');
      return;
    }

    const loadReport = async () => {
      try {
        const data = await assessmentApi.getReport(id);
        setReport(data);
      } catch {
        setError('Failed to load report');
      } finally {
        setIsLoading(false);
      }
    };

    loadReport();
  }, [id, navigate]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-gray-500">加载中...</div>
      </div>
    );
  }

  if (error || !report) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <p className="text-red-500 mb-4">{error || 'Report not found'}</p>
          <button
            onClick={() => navigate('/assessment')}
            className="text-blue-600 hover:underline"
          >
            返回报告列表
          </button>
        </div>
      </div>
    );
  }

  const scores = report.dimension_scores;
  const issues = report.identified_issues || [];
  const summary = report.improvement_summary;

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-4 sm:px-6 lg:px-8">
          <div className="flex items-center gap-3">
            <button
              onClick={() => navigate('/assessment')}
              className="text-gray-500 hover:text-gray-700"
            >
              ← 返回
            </button>
            <h1 className="text-xl font-bold text-gray-900">评估报告详情</h1>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="space-y-6">
          {/* Grade Card */}
          <div className="rounded-lg bg-white p-6 shadow">
            <div className="flex items-center gap-6">
              <div
                className={`flex h-24 w-24 items-center justify-center rounded-xl border-2 text-4xl font-bold ${GRADE_COLORS[report.health_grade] || ''}`}
              >
                {report.health_grade}
              </div>
              <div>
                <h2 className="text-2xl font-bold text-gray-900">
                  健康等级：{report.health_grade}
                </h2>
                <p className="text-gray-600 mt-1">
                  {GRADE_LABELS[report.health_grade] || '未知'}
                </p>
                <p className="text-sm text-gray-400 mt-2">
                  生成时间：{new Date(report.created_at).toLocaleString('zh-CN')}
                </p>
              </div>
            </div>
          </div>

          {/* Dimension Scores */}
          <div className="rounded-lg bg-white p-6 shadow">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">维度评分</h3>
            <div className="space-y-3">
              {(Object.entries(scores) as [keyof DimensionScores, number][]).map(
                ([key, value]) => (
                  <ScoreBar
                    key={key}
                    label={DIMENSION_LABELS[key] || key}
                    score={value}
                  />
                ),
              )}
            </div>
          </div>

          {/* Identified Issues */}
          {issues.length > 0 && (
            <div className="rounded-lg bg-white p-6 shadow">
              <h3 className="text-lg font-semibold text-gray-900 mb-4">
                识别的问题
              </h3>
              <div className="space-y-3">
                {issues
                  .sort((a, b) => (a.priority || 0) - (b.priority || 0))
                  .map((issue, i) => (
                    <IssueCard key={i} issue={issue} />
                  ))}
              </div>
            </div>
          )}

          {/* Improvement Summary */}
          <div className="rounded-lg bg-white p-6 shadow">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">
              改善方案概要
            </h3>
            <div className="space-y-4">
              {summary.general && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-1">总体方向</h4>
                  <p className="text-sm text-gray-600">{summary.general}</p>
                </div>
              )}
              {summary.exercise && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-1">运动建议</h4>
                  <p className="text-sm text-gray-600">{summary.exercise}</p>
                </div>
              )}
              {summary.lifestyle && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-1">生活习惯</h4>
                  <p className="text-sm text-gray-600">{summary.lifestyle}</p>
                </div>
              )}
              {summary.nutrition && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-1">饮食建议</h4>
                  <p className="text-sm text-gray-600">{summary.nutrition}</p>
                </div>
              )}
            </div>
          </div>

          {/* Disclaimer */}
          <div className="rounded-lg bg-yellow-50 border border-yellow-200 p-4">
            <p className="text-xs text-yellow-800">
              本报告仅供参考，不构成医疗诊断。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
