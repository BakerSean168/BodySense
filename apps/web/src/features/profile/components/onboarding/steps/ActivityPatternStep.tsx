interface ActivityPatternStepProps {
  value: string;
  onChange: (value: string) => void;
}

const ACTIVITY_EXAMPLES = [
  "久坐为主",
  "久站为主",
  "经常走动",
  "体力活动较多",
  "姿势经常变化",
  "轮班或节奏不规律",
];

export function ActivityPatternStep({
  value,
  onChange,
}: ActivityPatternStepProps) {
  const useExample = (example: string) => {
    if (!value.trim()) {
      onChange(example);
      return;
    }
    if (!value.includes(example)) {
      onChange(`${value.trim()}；${example}`);
    }
  };

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">
        日常活动与工作习惯
      </h2>
      <p className="text-sm text-gray-500 mb-5">
        不需要填写职业名称。请描述一天里身体通常怎么活动，例如连续久坐多久、是否久站、走动频率、搬抬负荷或轮班情况。
      </p>

      <div className="mb-4 flex flex-wrap gap-2" aria-label="常见活动描述">
        {ACTIVITY_EXAMPLES.map((example) => (
          <button
            key={example}
            type="button"
            onClick={() => useExample(example)}
            className="rounded-full border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700"
          >
            {example}
          </button>
        ))}
      </div>

      <label htmlFor="activityPattern" className="sr-only">
        日常活动与工作习惯描述
      </label>
      <textarea
        id="activityPattern"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        rows={6}
        placeholder="例如：工作日大部分时间坐着，每次会连续坐 2-3 小时；通勤每天步行约 40 分钟；偶尔需要搬重物。周末活动量会更大。"
        className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      />
      <p className="mt-2 text-xs text-gray-400">
        选填。按真实情况自然描述即可，不必套用固定格式。
      </p>
    </div>
  );
}
