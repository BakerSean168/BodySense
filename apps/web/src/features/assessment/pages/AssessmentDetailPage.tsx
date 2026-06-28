import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router';
import {
  assessmentApi,
  type AssessmentReport,
  type DimensionScores,
  type IdentifiedIssue,
} from '../services/assessmentService';
import { MainLayout } from '@/components/layout/MainLayout';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/button';

const GRADE_COLORS: Record<string, string> = {
  A: 'from-emerald-400 to-green-500 text-white shadow-emerald-500/30',
  B: 'from-blue-400 to-indigo-500 text-white shadow-blue-500/30',
  C: 'from-amber-400 to-orange-500 text-white shadow-orange-500/30',
  D: 'from-rose-400 to-red-500 text-white shadow-red-500/30',
};

const GRADE_LABELS: Record<string, string> = {
  A: '优秀 (Excellent)',
  B: '良好 (Good)',
  C: '一般 (Fair)',
  D: '需改善 (Needs Improvement)',
};

const DIMENSION_LABELS: Record<keyof DimensionScores, string> = {
  posture: '体态 (Posture)',
  exercise: '运动能力 (Exercise)',
  lifestyle: '生活习惯 (Lifestyle)',
  injury_risk: '伤病风险 (Injury Risk)',
  overall: '综合 (Overall)',
};

const SEVERITY_COLORS: Record<string, string> = {
  '轻度': 'bg-emerald-100 text-emerald-800 border-emerald-200',
  '中度': 'bg-amber-100 text-amber-800 border-amber-200',
  '重度': 'bg-rose-100 text-rose-800 border-rose-200',
};

function ScoreBar({ label, score }: { label: string; score: number }) {
  const color =
    score >= 80
      ? 'from-emerald-400 to-emerald-500'
      : score >= 60
        ? 'from-amber-400 to-orange-500'
        : 'from-rose-400 to-red-500';

  return (
    <div className="flex items-center gap-4 py-2">
      <span className="w-40 text-sm font-medium text-slate-700 truncate" title={label}>{label}</span>
      <div className="flex-1 h-3 bg-slate-100 rounded-full overflow-hidden shadow-inner">
        <div
          className={`h-full rounded-full bg-gradient-to-r ${color} transition-all duration-1000 ease-out`}
          style={{ width: `${score}%` }}
        />
      </div>
      <span className="w-12 text-sm font-bold text-slate-800 text-right">
        {score}
      </span>
    </div>
  );
}

function IssueCard({ issue }: { issue: IdentifiedIssue }) {
  return (
    <div className="rounded-2xl border border-slate-100 bg-slate-50/50 p-5 hover:bg-slate-50 hover:shadow-sm transition-all duration-200">
      <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-3">
        <h4 className="font-bold text-slate-900 text-lg">{issue.issue}</h4>
        <span
          className={`inline-flex self-start sm:self-auto items-center px-2.5 py-0.5 rounded-full text-xs font-bold border ${SEVERITY_COLORS[issue.severity] || 'bg-slate-100 text-slate-800 border-slate-200'}`}
        >
          {issue.severity} Severity
        </span>
      </div>
      <p className="text-sm text-slate-600 leading-relaxed">{issue.description}</p>
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
      <MainLayout>
        <div className="flex items-center justify-center min-h-[60vh]">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
            <p className="mt-4 text-slate-500 font-medium">加载中 (Loading)...</p>
          </div>
        </div>
      </MainLayout>
    );
  }

  if (error || !report) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-[60vh]">
          <Card className="max-w-md w-full p-8 text-center shadow-xl">
            <div className="w-16 h-16 rounded-full bg-red-50 text-red-500 flex items-center justify-center mx-auto mb-4">
              <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <p className="text-lg font-bold text-slate-900 mb-2">{error || 'Report not found'}</p>
            <Button onClick={() => navigate('/assessment')} className="w-full mt-4">
              返回报告列表 (Return to List)
            </Button>
          </Card>
        </div>
      </MainLayout>
    );
  }

  const scores = report.dimension_scores;
  const issues = report.identified_issues || [];
  const summary = report.improvement_summary;

  return (
    <MainLayout>
      <div className="w-full space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
        {/* Header */}
        <div className="flex items-center gap-4">
          <button
            onClick={() => navigate('/assessment')}
            className="w-10 h-10 rounded-xl bg-white shadow-sm border border-slate-100 flex items-center justify-center text-slate-500 hover:bg-slate-50 hover:text-slate-900 transition-colors"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
            </svg>
          </button>
          <div>
            <h1 className="text-2xl font-bold text-slate-900 tracking-tight">评估报告详情 (Assessment Details)</h1>
            <p className="text-slate-500 text-sm mt-1">Detailed breakdown of your health metrics</p>
          </div>
        </div>

        {/* Grade Card */}
        <Card className="p-8 relative overflow-hidden bg-slate-900 text-white shadow-2xl border-none">
          <div className="absolute top-0 right-0 w-64 h-64 bg-primary-500 rounded-full mix-blend-multiply filter blur-3xl opacity-20 -translate-y-1/2 translate-x-1/2" />
          <div className="absolute bottom-0 left-0 w-64 h-64 bg-indigo-500 rounded-full mix-blend-multiply filter blur-3xl opacity-20 translate-y-1/2 -translate-x-1/2" />
          
          <div className="relative z-10 flex flex-col md:flex-row items-center md:items-start gap-8">
            <div
              className={`flex h-32 w-32 shrink-0 items-center justify-center rounded-[2rem] bg-gradient-to-br text-6xl font-black shadow-2xl ring-4 ring-white/10 ${GRADE_COLORS[report.health_grade] || 'from-slate-400 to-slate-500 text-white'}`}
            >
              {report.health_grade}
            </div>
            <div className="text-center md:text-left pt-2">
              <h2 className="text-3xl font-bold text-white mb-2 tracking-tight">
                健康等级：{report.health_grade}
              </h2>
              <p className="text-xl text-primary-200 font-medium mb-4">
                {GRADE_LABELS[report.health_grade] || '未知 (Unknown)'}
              </p>
              <div className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-white/10 backdrop-blur-md border border-white/10">
                <svg className="w-5 h-5 text-primary-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
                <span className="text-sm font-medium text-slate-300">
                  {new Date(report.created_at).toLocaleString('zh-CN', {
                    year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit'
                  })}
                </span>
              </div>
            </div>
          </div>
        </Card>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          {/* Dimension Scores */}
          <Card className="p-8">
            <h3 className="text-xl font-bold text-slate-900 mb-6 flex items-center gap-2">
              <svg className="w-6 h-6 text-primary-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
              维度评分 (Dimension Scores)
            </h3>
            <div className="space-y-2">
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
          </Card>

          {/* Identified Issues */}
          {issues.length > 0 && (
            <Card className="p-8">
              <h3 className="text-xl font-bold text-slate-900 mb-6 flex items-center gap-2">
                <svg className="w-6 h-6 text-rose-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                识别的问题 (Identified Issues)
              </h3>
              <div className="space-y-4">
                {issues
                  .sort((a, b) => (a.priority || 0) - (b.priority || 0))
                  .map((issue, i) => (
                    <IssueCard key={i} issue={issue} />
                  ))}
              </div>
            </Card>
          )}
        </div>

        {/* Improvement Summary */}
        <Card className="p-8">
          <h3 className="text-xl font-bold text-slate-900 mb-8 flex items-center gap-2">
            <svg className="w-6 h-6 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
            </svg>
            改善方案概要 (Improvement Summary)
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
            {summary.general && (
              <div className="bg-slate-50/50 p-5 rounded-2xl border border-slate-100">
                <h4 className="text-sm font-bold text-indigo-600 uppercase tracking-wider mb-2">总体方向 (General)</h4>
                <p className="text-slate-700 leading-relaxed">{summary.general}</p>
              </div>
            )}
            {summary.exercise && (
              <div className="bg-slate-50/50 p-5 rounded-2xl border border-slate-100">
                <h4 className="text-sm font-bold text-emerald-600 uppercase tracking-wider mb-2">运动建议 (Exercise)</h4>
                <p className="text-slate-700 leading-relaxed">{summary.exercise}</p>
              </div>
            )}
            {summary.lifestyle && (
              <div className="bg-slate-50/50 p-5 rounded-2xl border border-slate-100">
                <h4 className="text-sm font-bold text-amber-600 uppercase tracking-wider mb-2">生活习惯 (Lifestyle)</h4>
                <p className="text-slate-700 leading-relaxed">{summary.lifestyle}</p>
              </div>
            )}
            {summary.nutrition && (
              <div className="bg-slate-50/50 p-5 rounded-2xl border border-slate-100">
                <h4 className="text-sm font-bold text-blue-600 uppercase tracking-wider mb-2">饮食建议 (Nutrition)</h4>
                <p className="text-slate-700 leading-relaxed">{summary.nutrition}</p>
              </div>
            )}
          </div>
        </Card>

        {/* Disclaimer */}
        <div className="rounded-2xl bg-amber-50 border border-amber-100 p-5 flex gap-4 items-start">
          <div className="p-2 bg-amber-100 text-amber-600 rounded-xl shrink-0">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <div>
            <h4 className="font-semibold text-amber-800 mb-1">免责声明 (Disclaimer)</h4>
            <p className="text-sm text-amber-700 leading-relaxed">
              本报告仅供参考，不构成医疗诊断。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。<br/>
              (This report is for reference only and does not constitute a medical diagnosis. Please seek professional medical help if you experience persistent pain or severe discomfort.)
            </p>
          </div>
        </div>
      </div>
    </MainLayout>
  );
}
