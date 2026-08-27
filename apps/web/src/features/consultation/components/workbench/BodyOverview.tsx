import { ShieldAlert } from "lucide-react";
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

/**
 * Deterministic legacy body projection retained for BodyExplorer 2D/WebGL
 * fallback only. The primary State path is the lazy Vanatome viewer.
 */
export function BodyOverview({ snapshot, className }: BodyOverviewProps) {
  const summaries = selectBodyZoneSummaries(snapshot);
  const safetyReview = hasSafetyReview(snapshot);

  return (
    <section
      aria-labelledby="body-overview-title"
      className={cn("relative flex min-h-0 flex-col", className)}
    >
      <h2 id="body-overview-title" className="sr-only">
        身体区域状态
      </h2>

      {safetyReview ? (
        <span className="absolute right-0 top-0 inline-flex items-center gap-1 rounded-full border border-destructive/30 bg-destructive/10 px-2.5 py-1 text-xs font-medium text-destructive">
          <ShieldAlert className="size-3.5" aria-hidden="true" />
          需优先确认
        </span>
      ) : null}

      <div className="relative mx-auto mt-2 w-full max-w-[252px]">
        <svg
          role="img"
          aria-labelledby="body-map-svg-title body-map-svg-description"
          viewBox="0 0 260 500"
          className="h-auto w-full"
        >
          <title id="body-map-svg-title">当前身体区域概览</title>
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
        <div className="mx-auto mt-3 w-full max-w-[300px] px-3 pb-2 text-center">
          <p className="text-[13px] font-medium text-foreground/85">
            还没有身体记录
          </p>
          <p className="mx-auto mt-1 max-w-[240px] text-xs leading-[1.55] text-muted-foreground">
            和 BodySense 说说最近哪里不舒服，相关区域会逐渐出现在这里。
          </p>
        </div>
      ) : (
        <ul
          aria-label="身体区域状态摘要"
          className="mx-auto mt-3 w-full max-w-[340px] divide-y divide-border/50"
        >
          {summaries.slice(0, 4).map((summary) => (
            <li
              key={summary.zone}
              className="flex items-center justify-between gap-3 px-1 py-2.5 text-xs"
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
    </section>
  );
}
