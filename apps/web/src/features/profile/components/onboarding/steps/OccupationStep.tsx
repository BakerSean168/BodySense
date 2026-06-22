interface OccupationStepProps {
  occupation: string;
  activityType: string;
  onOccupationChange: (value: string) => void;
  onActivityTypeChange: (value: string) => void;
}

const ACTIVITY_TYPES = [
  { value: 'sedentary', label: '久坐为主' },
  { value: 'light', label: '轻度活动' },
  { value: 'moderate', label: '中度活动' },
  { value: 'active', label: '重度活动' },
];

export function OccupationStep({
  occupation,
  activityType,
  onOccupationChange,
  onActivityTypeChange,
}: OccupationStepProps) {
  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">您的职业和日常活动类型？</h2>
      <p className="text-sm text-gray-500 mb-6">
        了解您的工作性质有助于判断日常能量消耗和肌肉使用情况。
      </p>

      <div className="space-y-6">
        <div>
          <label htmlFor="occupation" className="block text-sm font-medium text-gray-700 mb-1">
            职业
          </label>
          <input
            id="occupation"
            type="text"
            value={occupation}
            onChange={(e) => onOccupationChange(e.target.value)}
            placeholder="例如：程序员、教师、销售..."
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-3">日常活动强度</label>
          <div className="grid grid-cols-2 gap-3">
            {ACTIVITY_TYPES.map((type) => (
              <button
                key={type.value}
                type="button"
                onClick={() => onActivityTypeChange(type.value)}
                className={`p-3 rounded-lg border-2 text-center text-sm font-medium transition-colors ${
                  activityType === type.value
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-200 hover:border-gray-300 text-gray-700'
                }`}
              >
                {type.label}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
