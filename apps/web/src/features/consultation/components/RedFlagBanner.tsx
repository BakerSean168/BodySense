import type { RedFlag } from '../types/consultation';

interface RedFlagBannerProps {
  redFlags: RedFlag[];
  onAcknowledge?: () => void;
}

const CATEGORY_LABELS: Record<string, string> = {
  severe_pain: '严重疼痛',
  radiating_pain: '放射痛',
  numbness: '神经症状',
  neurological: '神经系统',
  trauma: '外伤',
  worsening: '症状加重',
  infection: '感染征兆',
  systemic: '全身症状',
  severe_symptom: '重度症状',
};

export function RedFlagBanner({ redFlags, onAcknowledge }: RedFlagBannerProps) {
  if (!redFlags || redFlags.length === 0) return null;

  return (
    <div className="rounded-lg border-2 border-red-300 bg-red-50 p-4">
      <div className="flex items-start gap-3">
        <div className="flex-shrink-0">
          <svg
            className="h-6 w-6 text-red-600"
            fill="none"
            viewBox="0 0 24 24"
            strokeWidth={2}
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"
            />
          </svg>
        </div>
        <div className="flex-1">
          <h3 className="text-sm font-semibold text-red-800">
            ⚠️ 安全提醒
          </h3>
          <div className="mt-2 space-y-2">
            {redFlags.map((flag, i) => (
              <div key={i} className="text-sm text-red-700">
                <span className="inline-block rounded bg-red-100 px-1.5 py-0.5 text-xs font-medium text-red-800 mr-2">
                  {CATEGORY_LABELS[flag.category] || flag.category}
                </span>
                {flag.message}
              </div>
            ))}
          </div>
          <p className="mt-3 text-xs text-red-600">
            以上症状可能需要专业医疗评估。本系统提供的分析仅供参考，不构成医疗诊断。如有紧急情况，请立即就医。
          </p>
          {onAcknowledge && (
            <button
              onClick={onAcknowledge}
              className="mt-3 rounded-md bg-red-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-700 transition-colors"
            >
              我已了解
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
