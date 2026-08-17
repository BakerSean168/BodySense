import type { DiagnosisAnalysis } from "../types/consultation";

interface DiagnosisHistoryPanelProps {
  analyses: DiagnosisAnalysis[];
  currentAnalysisId?: string;
}

const STATUS_LABELS: Record<string, string> = {
  completed: "已完成",
  partial: "部分完成",
  insufficient_information: "信息不足",
  safety_blocked: "安全限制",
};

/**
 * Compact analytical timeline for the longitudinal BodyState model.
 *
 * Historical DiagnosisAnalysis rows are immutable snapshots pinned to an exact
 * BodyState revision. This component intentionally does not create a parallel
 * "MedicalRecord" document concept.
 */
export function DiagnosisHistoryPanel({
  analyses,
  currentAnalysisId,
}: DiagnosisHistoryPanelProps) {
  if (analyses.length === 0) return null;

  return (
    <section className="space-y-2 border-t border-gray-100 pt-4">
      <div>
        <h3 className="text-sm font-semibold text-[#1A221E]">诊断历史</h3>
        <p className="mt-0.5 text-xs text-gray-500">
          每次分析都保留当时使用的 BodyState
          版本，后续身体状态变化不会改写旧分析。
        </p>
      </div>

      <div className="space-y-2">
        {analyses.map((analysis) => {
          const isCurrent = Boolean(
            currentAnalysisId && analysis.analysis_id === currentAnalysisId,
          );
          const createdAt = analysis.created_at
            ? new Date(analysis.created_at).toLocaleString("zh-CN", {
                month: "2-digit",
                day: "2-digit",
                hour: "2-digit",
                minute: "2-digit",
              })
            : "时间未知";
          return (
            <div
              key={
                analysis.analysis_id ??
                `${analysis.body_state_revision}-${createdAt}`
              }
              className={`rounded-lg border p-3 ${
                isCurrent
                  ? "border-primary-200 bg-primary-50/50"
                  : "border-gray-200 bg-white"
              }`}
            >
              <div className="flex items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <span className="font-semibold text-gray-800">
                    {createdAt}
                  </span>
                  {analysis.body_state_revision != null ? (
                    <span className="rounded-full bg-slate-100 px-2 py-0.5 text-slate-600">
                      R{analysis.body_state_revision}
                    </span>
                  ) : null}
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-gray-600">
                    {STATUS_LABELS[analysis.status ?? "completed"] ??
                      analysis.status}
                  </span>
                  {analysis.freshness ? (
                    <span
                      className={`rounded-full px-2 py-0.5 ${
                        analysis.freshness.state === "fresh"
                          ? "bg-emerald-100 text-emerald-700"
                          : analysis.freshness.state === "potentially_stale"
                            ? "bg-amber-100 text-amber-700"
                            : "bg-red-100 text-red-700"
                      }`}
                    >
                      {analysis.freshness.state === "fresh"
                        ? "fresh"
                        : analysis.freshness.state === "potentially_stale"
                          ? "可能需复核"
                          : "stale"}
                    </span>
                  ) : null}
                  {isCurrent ? (
                    <span className="rounded-full bg-primary-100 px-2 py-0.5 font-semibold text-primary-800">
                      当前
                    </span>
                  ) : null}
                </div>
                <span className="shrink-0 text-xs text-gray-500">
                  {analysis.candidates.length} 个候选
                </span>
              </div>
              {analysis.summary ? (
                <p className="mt-2 line-clamp-2 text-xs leading-relaxed text-gray-600">
                  {analysis.summary}
                </p>
              ) : null}
            </div>
          );
        })}
      </div>
    </section>
  );
}
