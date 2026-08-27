interface SleepStepProps {
  value: string;
  onChange: (value: string) => void;
}

const SLEEP_EXAMPLES = [
  "作息比较规律",
  "经常晚睡",
  "轮班",
  "入睡时间不固定",
  "起床时间不固定",
  "睡眠经常中断",
];

export function SleepStep({ value, onChange }: SleepStepProps) {
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
      <h2 className="text-lg font-medium text-gray-900 mb-2">睡眠与作息情况</h2>
      <p className="text-sm text-gray-500 mb-5">
        不要求填写固定的入睡和起床时间。请描述规律性、轮班情况、通常睡多久，以及是否经常夜醒或明显缺觉。
      </p>

      <div className="mb-4 flex flex-wrap gap-2" aria-label="常见作息描述">
        {SLEEP_EXAMPLES.map((example) => (
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

      <label htmlFor="sleepPattern" className="sr-only">
        睡眠与作息描述
      </label>
      <textarea
        id="sleepPattern"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        rows={6}
        placeholder="例如：白班时通常 23:30 左右睡、7:00 起；每月有几次夜班，夜班后可能下午才起床。平均每天睡 6-7 小时，换班后容易睡不够。"
        className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      />
      <p className="mt-2 text-xs text-gray-400">
        选填。作息不规律本身就是有价值的信息。
      </p>
    </div>
  );
}
