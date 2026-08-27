interface BirthDateStepProps {
  value: string;
  onChange: (value: string) => void;
}

function dateInputBounds() {
  const today = new Date();
  const max = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  const min = new Date(max);
  min.setFullYear(min.getFullYear() - 150);

  const format = (value: Date) => {
    const year = value.getFullYear();
    const month = String(value.getMonth() + 1).padStart(2, "0");
    const day = String(value.getDate()).padStart(2, "0");
    return `${year}-${month}-${day}`;
  };

  return { min: format(min), max: format(max) };
}

export function BirthDateStep({ value, onChange }: BirthDateStepProps) {
  const { min, max } = dateInputBounds();

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">
        您的出生日期是？
      </h2>
      <p className="text-sm text-gray-500 mb-6">
        我们会根据出生日期计算随时间变化的年龄，您不需要以后再手动更新年龄。
      </p>

      <input
        type="date"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        min={min}
        max={max}
        aria-label="出生日期"
        className="w-full rounded-md border border-gray-300 px-4 py-3 text-lg shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      />
    </div>
  );
}
