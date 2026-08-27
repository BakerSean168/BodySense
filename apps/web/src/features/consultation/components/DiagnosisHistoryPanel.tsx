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

const FRESHNESS_LABELS: Record<string, string> = {
  fresh: "与当前状态一致",
  potentially_stale: "可能需复核",
  stale: "需要重新分析",
};

export function DiagnosisHistoryPanel({
  analyses,
  currentAnalysisId,
}: DiagnosisHistoryPanelProps) {
  if (analyses.length === 0) return null;

  return (
    <section className="space-y-3 border-t border-border/55 pt-5">
      <h3 className="text-sm font-semibold text-foreground">分析记录</h3>

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
              className={`rounded-xl border p-3 ${
                isCurrent
                  ? "border-primary/25 bg-primary/[0.055]"
                  : "border-border bg-card/55"
              }`}
            >
              <div className="flex items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <span className="font-semibold text-foreground/85">
                    {createdAt}
                  </span>
                  <span className="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
                    {STATUS_LABELS[analysis.status ?? "completed"] ??
                      analysis.status}
                  </span>
                  {analysis.freshness && analysis.freshness.state !== "fresh" ? (
                    <span
                      className={`rounded-full px-2 py-0.5 ${
                        analysis.freshness.state === "potentially_stale"
                          ? "bg-amber-400/10 text-amber-200"
                          : "bg-red-400/10 text-red-200"
                      }`}
                    >
                      {FRESHNESS_LABELS[analysis.freshness.state] ??
                        analysis.freshness.state}
                    </span>
                  ) : null}
                  {isCurrent ? (
                    <span className="rounded-full bg-primary/12 px-2 py-0.5 font-semibold text-primary">
                      当前
                    </span>
                  ) : null}
                </div>
                <span className="shrink-0 text-xs text-muted-foreground">
                  {analysis.candidates.length} 项可能因素
                </span>
              </div>
              {analysis.summary ? (
                <p className="mt-2 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
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
