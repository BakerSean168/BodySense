import { ShieldAlert } from "lucide-react";
import type { RedFlag } from "../types/consultation";

interface RedFlagBannerProps {
  redFlags: RedFlag[];
  onAcknowledge?: () => void;
}

const CATEGORY_LABELS: Record<string, string> = {
  severe_pain: "严重疼痛",
  radiating_pain: "放射痛",
  numbness: "神经症状",
  neurological: "神经系统",
  trauma: "外伤",
  worsening: "症状加重",
  infection: "感染征兆",
  systemic: "全身症状",
  severe_symptom: "重度症状",
};

export function RedFlagBanner({ redFlags, onAcknowledge }: RedFlagBannerProps) {
  if (!redFlags || redFlags.length === 0) return null;

  return (
    <div className="rounded-xl border border-red-300/15 bg-red-300/[0.055] p-4 text-red-100/80">
      <div className="flex items-start gap-3">
        <ShieldAlert
          className="mt-0.5 size-5 shrink-0 text-red-300/85"
          aria-hidden="true"
        />
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-semibold text-red-100/90">
            建议优先关注这些情况
          </h3>
          <div className="mt-2 space-y-2">
            {redFlags.map((flag, index) => (
              <div key={index} className="text-sm leading-6 text-red-100/70">
                <span className="mr-2 inline-block rounded-md bg-red-300/10 px-1.5 py-0.5 text-xs font-medium text-red-200/85">
                  {CATEGORY_LABELS[flag.category] || flag.category}
                </span>
                {flag.message}
              </div>
            ))}
          </div>
          <p className="mt-3 text-xs leading-5 text-red-100/48">
            这些信息可能需要专业医疗评估。BodySense 的分析仅用于辅助理解身体情况，不构成医疗诊断；如有紧急情况，请及时就医。
          </p>
          {onAcknowledge ? (
            <button
              type="button"
              onClick={onAcknowledge}
              className="mt-3 rounded-lg bg-red-100/90 px-3 py-1.5 text-xs font-semibold text-red-950 transition-colors hover:bg-red-50"
            >
              我已了解
            </button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
