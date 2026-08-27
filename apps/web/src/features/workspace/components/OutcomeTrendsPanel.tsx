import { BarChart3, Clock3 } from "lucide-react";
import type { Outcome, WorkspaceTrend } from "../types/workspace";

interface OutcomeTrendsPanelProps {
  trends: WorkspaceTrend[];
  outcomes: Outcome[];
}

const trendLabels: Record<string, string> = {
  improving: "改善",
  stable: "稳定",
  worsening: "加重",
  unknown: "待观察",
};

const causalityLabels: Record<Outcome["causality_level"], string> = {
  association_only: "时间上相关，仅作参考",
  user_attributed: "你认为可能相关",
  clinician_attributed: "专业人员认为可能相关",
};

export function OutcomeTrendsPanel({
  trends,
  outcomes,
}: OutcomeTrendsPanelProps) {
  if (trends.length === 0 && outcomes.length === 0) return null;

  return (
    <div className="space-y-5">
      <div className="flex items-center gap-2">
        <BarChart3 className="h-4 w-4 text-primary" />
        <h3 className="font-semibold text-foreground">变化趋势</h3>
      </div>

      {trends.length > 0 ? (
        <div className="divide-y divide-border/55">
          {trends.map((trend) => (
            <div key={trend.key} className="py-3 text-xs first:pt-0">
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium text-foreground">
                  {trend.body_region || trend.concern_key || "身体状态"}
                </span>
                <span className="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
                  {trendLabels[trend.current_trend] || "待观察"}
                </span>
              </div>
              <p className="mt-1 text-muted-foreground">
                已记录 {trend.points.length} 个时间点
              </p>
            </div>
          ))}
        </div>
      ) : null}

      {outcomes.length > 0 ? (
        <details className="border-t border-border/55 pt-4">
          <summary className="cursor-pointer list-none text-xs font-semibold text-muted-foreground">
            最近变化 · {outcomes.length}
          </summary>
          <div className="mt-3 space-y-2">
            {outcomes.slice(0, 12).map((outcome) => (
              <div
                key={outcome.id}
                className="rounded-lg bg-muted/40 p-2.5 text-xs"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="font-medium text-foreground">
                    {outcome.body_region || outcome.concern_key || "身体变化"}
                  </span>
                  <span className="inline-flex items-center gap-1 text-muted-foreground">
                    <Clock3 className="h-3 w-3" />
                    {new Date(outcome.occurred_at).toLocaleDateString("zh-CN")}
                  </span>
                </div>
                <p className="mt-1 text-muted-foreground">
                  {outcome.notes ||
                    String(outcome.value.description || "记录了一次身体变化")}
                </p>
                <p className="mt-1 text-[11px] text-muted-foreground/70">
                  {causalityLabels[outcome.causality_level]}
                </p>
              </div>
            ))}
          </div>
        </details>
      ) : null}
    </div>
  );
}
