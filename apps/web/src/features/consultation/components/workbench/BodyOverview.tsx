import { Activity, ShieldAlert } from "lucide-react";
import type { BodyStateSnapshot } from "../../types/consultation";
import {
  selectBodyZoneSummaries,
  type BodyZone,
  type BodyZoneSummary,
} from "../../model/bodyMap";
import { cn } from "@/lib/utils";

interface BodyOverviewProps {
  snapshot: BodyStateSnapshot | null;
  className?: string;
}

const zoneCoordinates: Record<BodyZone, { x: number; y: number }> = {
  head: { x: 130, y: 66 },
  neck: { x: 130, y: 126 },
  torso: { x: 130, y: 230 },
  pelvis: { x: 130, y: 322 },
  arms: { x: 208, y: 218 },
  legs: { x: 166, y: 418 },
};

const trendLabel: Record<BodyZoneSummary["trend"], string> = {
  worsening: "加重",
  improving: "改善",
  stable: "稳定",
  unknown: "待观察",
};

function markerClass(trend: BodyZoneSummary["trend"]): string {
  switch (trend) {
    case "worsening":
      return "text-destructive";
    case "improving":
      return "text-primary";
    case "stable":
      return "text-muted-foreground";
    default:
      return "text-accent-foreground";
  }
}

function hasSafetyReview(snapshot: BodyStateSnapshot | null): boolean {
  const safety = snapshot?.safety_state;
  return Boolean(
    safety?.has_red_flags &&
    (safety?.status === "requires_review" || safety?.status === "active"),
  );
}

export function BodyOverview({ snapshot, className }: BodyOverviewProps) {
  const summaries = selectBodyZoneSummaries(snapshot);
  const safetyReview = hasSafetyReview(snapshot);

  return (
    <section
      aria-labelledby="body-overview-title"
      className={cn(
        "flex min-h-[420px] flex-col rounded-2xl border border-border bg-card p-4 shadow-sm",
        className,
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-medium text-muted-foreground">
            BodyState R{snapshot?.current_revision ?? 0}
          </p>
          <h2
            id="body-overview-title"
            className="mt-1 text-base font-semibold text-card-foreground"
          >
            身体状态地图
          </h2>
        </div>
        <span
          className={cn(
            "inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium",
            safetyReview
              ? "border-destructive/30 bg-destructive/10 text-destructive"
              : "border-primary/20 bg-primary/8 text-primary",
          )}
        >
          {safetyReview ? (
            <ShieldAlert className="size-3.5" aria-hidden="true" />
          ) : (
            <Activity className="size-3.5" aria-hidden="true" />
          )}
          {safetyReview ? "优先安全复核" : "当前投影"}
        </span>
      </div>

      <div className="relative mx-auto mt-3 w-full max-w-[280px] flex-1">
        <svg
          role="img"
          aria-labelledby="body-map-svg-title body-map-svg-description"
          viewBox="0 0 260 500"
          className="h-full min-h-[320px] w-full"
        >
          <title id="body-map-svg-title">当前 BodyState 身体区域概览</title>
          <desc id="body-map-svg-description">
            标记仅表示当前状态中存在相关事实或已确认观察，不表示医学诊断。
          </desc>
          <g
            fill="color-mix(in oklch, var(--muted) 76%, transparent)"
            stroke="color-mix(in oklch, var(--foreground) 38%, transparent)"
            strokeWidth="4"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <circle cx="130" cy="62" r="38" />
            <path d="M118 101 L118 119 M142 101 L142 119" fill="none" />
            <path d="M94 126 Q130 111 166 126 Q186 181 178 272 Q173 314 156 330 L104 330 Q87 314 82 272 Q74 181 94 126Z" />
            <path
              d="M90 151 Q55 170 40 218 Q34 239 52 250 Q66 250 72 230 L100 187"
              fill="none"
              strokeWidth="18"
            />
            <path
              d="M170 151 Q205 170 220 218 Q226 239 208 250 Q194 250 188 230 L160 187"
              fill="none"
              strokeWidth="18"
            />
            <path
              d="M111 330 Q101 376 92 435 Q88 468 105 480 Q125 462 130 416 L130 340"
              fill="none"
              strokeWidth="24"
            />
            <path
              d="M149 330 Q159 376 168 435 Q172 468 155 480 Q135 462 130 416 L130 340"
              fill="none"
              strokeWidth="24"
            />
          </g>

          {summaries.map((summary) => {
            const point = zoneCoordinates[summary.zone];
            return (
              <g
                key={summary.zone}
                className={markerClass(summary.trend)}
                aria-hidden="true"
              >
                <circle
                  cx={point.x}
                  cy={point.y}
                  r="17"
                  fill="currentColor"
                  opacity="0.16"
                />
                <circle cx={point.x} cy={point.y} r="8" fill="currentColor" />
                <text
                  x={point.x}
                  y={point.y + 3.5}
                  textAnchor="middle"
                  fontSize="9"
                  fontWeight="700"
                  fill="var(--background)"
                >
                  {summary.count}
                </text>
              </g>
            );
          })}
        </svg>
      </div>

      {summaries.length === 0 ? (
        <p className="rounded-xl border border-dashed border-border bg-muted/35 px-3 py-3 text-center text-xs leading-5 text-muted-foreground">
          继续对话或记录事实后，相关身体区域会在这里形成可复核标记。
        </p>
      ) : (
        <ul aria-label="身体区域状态摘要" className="mt-2 space-y-1.5">
          {summaries.slice(0, 4).map((summary) => (
            <li
              key={summary.zone}
              className="flex items-center justify-between gap-3 rounded-lg bg-muted/45 px-3 py-2 text-xs"
            >
              <span className="min-w-0 truncate font-medium text-foreground">
                {summary.label}
                <span className="ml-1 text-muted-foreground">
                  · {summary.count} 项
                </span>
              </span>
              <span className={cn("font-medium", markerClass(summary.trend))}>
                {trendLabel[summary.trend]}
              </span>
            </li>
          ))}
        </ul>
      )}

      <p className="mt-3 text-[11px] leading-5 text-muted-foreground">
        区域标记来自当前有效 Fact 与已确认
        Observation，仅用于组织信息，不构成医疗诊断。
      </p>
    </section>
  );
}
