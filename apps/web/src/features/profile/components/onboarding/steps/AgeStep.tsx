import { useState } from 'react';

interface AgeStepProps {
  value: number | undefined;
  onChange: (value: number) => void;
}

export function AgeStep({ value, onChange }: AgeStepProps) {
  const [inputValue, setInputValue] = useState(value?.toString() || '');
  const [error, setError] = useState('');

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setInputValue(val);

    const num = parseInt(val, 10);
    if (isNaN(num)) {
      setError('请输入有效的年龄');
    } else if (num < 1 || num > 150) {
      setError('年龄必须在 1-150 之间');
    } else {
      setError('');
      onChange(num);
    }
  };

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">您的年龄是？</h2>
      <p className="text-sm text-gray-500 mb-6">
        年龄是评估身体状况和制定训练计划的重要参考因素。
      </p>

      <div>
        <input
          type="number"
          value={inputValue}
          onChange={handleChange}
          min={1}
          max={150}
          placeholder="请输入年龄"
          className="w-full rounded-md border border-gray-300 px-4 py-3 text-lg shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
      </div>
    </div>
  );
}
