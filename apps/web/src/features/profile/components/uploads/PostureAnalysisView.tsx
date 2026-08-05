import type {
  PostureAnalysis,
  PostureView,
  Severity,
} from '../../types/upload.types';
import {
  CONFIDENCE_LABELS,
  SEVERITY_LABELS,
} from '../../types/upload.types';

const VIEW_LABELS: Record<PostureView, string> = {
  front: '正面',
  side: '侧面',
  back: '背面',
};

const SEVERITY_STYLES: Record<Severity, string> = {
  mild: 'bg-emerald-50 text-emerald-700 border-emerald-200',
  moderate: 'bg-amber-50 text-amber-700 border-amber-200',
  marked: 'bg-rose-50 text-rose-700 border-rose-200',
};

interface PostureAnalysisViewProps {
  analyses: PostureAnalysis[];
}

/**
 * Aggregated posture-analysis card for the three-view photo set.
 * Renders red-flag warnings, per-view findings with severity/confidence
 * badges, plain-language summaries, and the mandatory medical disclaimer.
 */
export function PostureAnalysisView({ analyses }: PostureAnalysisViewProps) {
  const valid = analyses.filter((a) => a && a.findings);
  if (valid.length === 0) return null;

  const redFlags = valid.flatMap((a) => a.red_flags ?? []);
  const disclaimer =
    valid.find((a) => a.disclaimer)?.disclaimer ??
    '本分析基于照片视觉判断，仅供参考，不构成医疗诊断。';

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 space-y-5">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-bold text-slate-900">AI 体态分析</h3>
        <span className="text-[10px] uppercase tracking-widest text-slate-400">
          三视图 · {valid.length} 项已完成
        </span>
      </div>

      {redFlags.length > 0 && (
        <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 space-y-1.5">
          <div className="flex items-center gap-2 text-rose-800 text-xs font-semibold">
            <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <span>健康提示</span>
          </div>
          {redFlags.map((rf, i) => (
            <p key={`${rf.category}-${i}`} className="text-xs text-rose-700 leading-relaxed">
              {rf.message}
            </p>
          ))}
        </div>
      )}

      <div className="space-y-4">
        {valid.map((a) => (
          <div key={a.view} className="space-y-2">
            <div className="flex items-center gap-2">
              <span className="px-2 py-0.5 rounded bg-slate-100 text-slate-600 text-[10px] font-bold tracking-wider">
                {VIEW_LABELS[a.view]}
              </span>
              <span className="text-[10px] text-slate-400">
                置信度 {CONFIDENCE_LABELS[a.overall_confidence] ?? a.overall_confidence}
              </span>
            </div>

            {a.findings.length === 0 ? (
              <p className="text-xs text-slate-500">未检出明显体态偏差。</p>
            ) : (
              <ul className="space-y-1.5">
                {a.findings.map((f) => (
                  <li
                    key={`${a.view}-${f.key}`}
                    className="flex items-start gap-2 text-xs"
                  >
                    <span
                      className={`px-1.5 py-0.5 rounded border text-[10px] font-semibold shrink-0 ${
                        SEVERITY_STYLES[f.severity] ?? SEVERITY_STYLES.mild
                      }`}
                    >
                      {SEVERITY_LABELS[f.severity] ?? f.severity}
                    </span>
                    <div className="min-w-0">
                      <span className="font-semibold text-slate-800">{f.label}</span>
                      {f.metric && (
                        <span className="ml-1 text-slate-500">
                          （{f.metric.name} {f.metric.value}
                          {f.metric.unit}）
                        </span>
                      )}
                      {f.evidence && (
                        <p className="text-slate-500 mt-0.5 leading-relaxed">{f.evidence}</p>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}

            {a.summary_markdown && (
              <p className="text-xs text-slate-600 leading-relaxed whitespace-pre-line">
                {a.summary_markdown}
              </p>
            )}
          </div>
        ))}
      </div>

      <p className="text-[10px] text-slate-400 leading-relaxed border-t border-slate-100 pt-3">
        {disclaimer}
      </p>
    </div>
  );
}
