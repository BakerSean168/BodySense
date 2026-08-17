interface ExerciseStepProps {
  exerciseType: string;
  exerciseFrequency: string;
  onExerciseTypeChange: (value: string) => void;
  onExerciseFrequencyChange: (value: string) => void;
}

const EXERCISE_FREQUENCIES = [
  { value: "never", label: "从不运动" },
  { value: "occasional", label: "偶尔运动" },
  { value: "1-2", label: "每周 1-2 次" },
  { value: "3-4", label: "每周 3-4 次" },
  { value: "5+", label: "每周 5 次以上" },
];

export function ExerciseStep({
  exerciseType,
  exerciseFrequency,
  onExerciseTypeChange,
  onExerciseFrequencyChange,
}: ExerciseStepProps) {
  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">
        您的运动习惯是？
      </h2>
      <p className="text-sm text-gray-500 mb-6">
        了解运动习惯有助于评估您的体能水平和制定合适的训练计划。
      </p>

      <div className="space-y-6">
        <div>
          <label
            htmlFor="exerciseType"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            常做的运动类型
          </label>
          <input
            id="exerciseType"
            type="text"
            value={exerciseType}
            onChange={(e) => onExerciseTypeChange(e.target.value)}
            placeholder="例如：跑步、游泳、健身、瑜伽..."
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-3">
            运动频率
          </label>
          <div className="grid grid-cols-2 gap-3">
            {EXERCISE_FREQUENCIES.map((freq) => (
              <button
                key={freq.value}
                type="button"
                onClick={() => onExerciseFrequencyChange(freq.value)}
                className={`p-3 rounded-lg border-2 text-center text-sm font-medium transition-colors ${
                  exerciseFrequency === freq.value
                    ? "border-blue-500 bg-blue-50 text-blue-700"
                    : "border-gray-200 hover:border-gray-300 text-gray-700"
                }`}
              >
                {freq.label}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
