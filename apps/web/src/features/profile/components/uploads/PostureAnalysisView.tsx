import type {
  PostureAnalysis,
  PostureView,
  Severity,
} from "../../types/upload.types";
import { CONFIDENCE_LABELS, SEVERITY_LABELS } from "../../types/upload.types";

const VIEW_LABELS: Record<PostureView, string> = {
  front: "正面",
  side: "侧面",
  back: "背面",
};

const SEVERITY_STYLES: Record<Severity, string> = {
  mild: "bg-success/15 text-success-foreground border-success/30",
  moderate: "bg-warning/15 text-warning-foreground border-warning/40",
  marked: "bg-destructive/10 text-destructive border-destructive/25",
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
    "本分析基于照片视觉判断，仅供参考，不构成医疗诊断。";

  return (
    <div className="rounded-xl border border-border bg-card p-5 space-y-5">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-bold text-foreground">AI 体态分析</h3>
        <span className="text-[10px] uppercase tracking-widest text-muted-foreground/70">
          三视图 · {valid.length} 项已完成
        </span>
      </div>

      {redFlags.length > 0 && (
        <div className="rounded-lg border border-destructive/25 bg-destructive/10 p-3 space-y-1.5">
          <div className="flex items-center gap-2 text-destructive text-xs font-semibold">
            <svg
              className="w-4 h-4 shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            <span>健康提示</span>
          </div>
          {redFlags.map((rf, i) => (
            <p
              key={`${rf.category}-${i}`}
              className="text-xs text-destructive leading-relaxed"
            >
              {rf.message}
            </p>
          ))}
        </div>
      )}

      <div className="space-y-4">
        {valid.map((a) => (
          <div key={a.view} className="space-y-2">
            <div className="flex items-center gap-2">
              <span className="px-2 py-0.5 rounded bg-muted text-muted-foreground text-[10px] font-bold tracking-wider">
                {VIEW_LABELS[a.view]}
              </span>
              <span className="text-[10px] text-muted-foreground/70">
                置信度{" "}
                {CONFIDENCE_LABELS[a.overall_confidence] ??
                  a.overall_confidence}
              </span>
            </div>

            {a.findings.length === 0 ? (
              <p className="text-xs text-muted-foreground">
                未检出明显体态偏差。
              </p>
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
                      <span className="font-semibold text-foreground">
                        {f.label}
                      </span>
                      {f.metric && (
                        <span className="ml-1 text-muted-foreground">
                          （{f.metric.name} {f.metric.value}
                          {f.metric.unit}）
                        </span>
                      )}
                      {f.evidence && (
                        <p className="text-muted-foreground mt-0.5 leading-relaxed">
                          {f.evidence}
                        </p>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}

            {a.summary_markdown && (
              <p className="text-xs text-muted-foreground leading-relaxed whitespace-pre-line">
                {a.summary_markdown}
              </p>
            )}
          </div>
        ))}
      </div>

      <p className="text-[10px] text-muted-foreground/70 leading-relaxed border-t border-border pt-3">
        {disclaimer}
      </p>
    </div>
  );
}
