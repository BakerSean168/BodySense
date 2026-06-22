interface GenderStepProps {
  value: string;
  onChange: (value: string) => void;
}

const GENDER_OPTIONS = [
  { value: 'male', label: '男' },
  { value: 'female', label: '女' },
];

export function GenderStep({ value, onChange }: GenderStepProps) {
  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">您的性别是？</h2>
      <p className="text-sm text-gray-500 mb-6">
        了解性别有助于我们为您提供更准确的身体评估和训练建议。
      </p>

      <div className="grid grid-cols-2 gap-4">
        {GENDER_OPTIONS.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            className={`p-4 rounded-lg border-2 text-center font-medium transition-colors ${
              value === option.value
                ? 'border-blue-500 bg-blue-50 text-blue-700'
                : 'border-gray-200 hover:border-gray-300 text-gray-700'
            }`}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}
