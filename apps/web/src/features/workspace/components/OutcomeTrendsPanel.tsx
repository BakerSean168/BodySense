import { BarChart3, Clock3 } from "lucide-react";
import { Card } from "@/components/ui/Card";
import type { Outcome, WorkspaceTrend } from "../types/workspace";

interface OutcomeTrendsPanelProps {
  trends: WorkspaceTrend[];
  outcomes: Outcome[];
}

const trendLabels: Record<string, string> = {
  improving: "改善",
  stable: "稳定",
  worsening: "加重",
  unknown: "不确定",
};

export function OutcomeTrendsPanel({
  trends,
  outcomes,
}: OutcomeTrendsPanelProps) {
  if (trends.length === 0 && outcomes.length === 0) return null;

  return (
    <Card className="space-y-4 p-4">
      <div>
        <div className="flex items-center gap-2">
          <BarChart3 className="h-4 w-4 text-primary" />
          <h3 className="font-semibold text-foreground">Outcome 与趋势</h3>
        </div>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">
          时间序列用于复评；association_only 明确表示相关性不等于因果。
        </p>
      </div>

      {trends.length > 0 && (
        <div className="grid gap-2 sm:grid-cols-2">
          {trends.map((trend) => (
            <div
              key={trend.key}
              className="rounded-xl border border-border p-3 text-xs"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium text-foreground">
                  {trend.body_region || trend.concern_key || trend.kind}
                </span>
                <span className="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
                  {trendLabels[trend.current_trend] ||
                    trend.current_trend ||
                    "未分类"}
                </span>
              </div>
              <p className="mt-1 text-muted-foreground">
                {trend.kind} · {trend.points.length} 个时间点
              </p>
            </div>
          ))}
        </div>
      )}

      {outcomes.length > 0 && (
        <details className="rounded-xl border border-border p-3">
          <summary className="cursor-pointer text-xs font-semibold text-muted-foreground">
            最近 Outcome · {outcomes.length}
          </summary>
          <div className="mt-3 space-y-2">
            {outcomes.slice(0, 12).map((outcome) => (
              <div
                key={outcome.id}
                className="rounded-lg bg-muted/40 p-2.5 text-xs"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="font-medium text-foreground">
                    {outcome.body_region || outcome.concern_key || outcome.kind}
                  </span>
                  <span className="inline-flex items-center gap-1 text-muted-foreground">
                    <Clock3 className="h-3 w-3" />
                    {new Date(outcome.occurred_at).toLocaleDateString("zh-CN")}
                  </span>
                </div>
                <p className="mt-1 text-muted-foreground">
                  {outcome.notes ||
                    String(outcome.value.description || outcome.kind)}
                </p>
                <p className="mt-1 text-[11px] text-muted-foreground/80">
                  {outcome.causality_level}
                </p>
              </div>
            ))}
          </div>
        </details>
      )}
    </Card>
  );
}
