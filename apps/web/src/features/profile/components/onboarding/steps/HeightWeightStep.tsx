import { useState } from "react";

interface HeightWeightStepProps {
  height: number | undefined;
  weight: number | undefined;
  onHeightChange: (value: number) => void;
  onWeightChange: (value: number) => void;
}

export function HeightWeightStep({
  height,
  weight,
  onHeightChange,
  onWeightChange,
}: HeightWeightStepProps) {
  const [heightInput, setHeightInput] = useState(height?.toString() || "");
  const [weightInput, setWeightInput] = useState(weight?.toString() || "");
  const [errors, setErrors] = useState<{ height?: string; weight?: string }>(
    {},
  );

  const validateHeight = (val: string) => {
    const num = parseFloat(val);
    if (isNaN(num)) {
      return "请输入有效的身高";
    }
    if (num < 50 || num > 250) {
      return "身高必须在 50-250 cm 之间";
    }
    return "";
  };

  const validateWeight = (val: string) => {
    const num = parseFloat(val);
    if (isNaN(num)) {
      return "请输入有效的体重";
    }
    if (num < 20 || num > 300) {
      return "体重必须在 20-300 kg 之间";
    }
    return "";
  };

  const handleHeightChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setHeightInput(val);

    const error = validateHeight(val);
    setErrors((prev) => ({ ...prev, height: error }));

    if (!error) {
      onHeightChange(parseFloat(val));
    }
  };

  const handleWeightChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setWeightInput(val);

    const error = validateWeight(val);
    setErrors((prev) => ({ ...prev, weight: error }));

    if (!error) {
      onWeightChange(parseFloat(val));
    }
  };

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">
        您的身高和体重是？
      </h2>
      <p className="text-sm text-gray-500 mb-6">
        身高体重用于计算 BMI，帮助我们评估您的身体状况。
      </p>

      <div className="space-y-4">
        <div>
          <label
            htmlFor="height"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            身高 (cm)
          </label>
          <input
            id="height"
            type="number"
            value={heightInput}
            onChange={handleHeightChange}
            min={50}
            max={250}
            step={0.1}
            placeholder="170"
            className="w-full rounded-md border border-gray-300 px-4 py-3 text-lg shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          {errors.height && (
            <p className="mt-1 text-sm text-red-600">{errors.height}</p>
          )}
        </div>

        <div>
          <label
            htmlFor="weight"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            体重 (kg)
          </label>
          <input
            id="weight"
            type="number"
            value={weightInput}
            onChange={handleWeightChange}
            min={20}
            max={300}
            step={0.1}
            placeholder="65"
            className="w-full rounded-md border border-gray-300 px-4 py-3 text-lg shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          {errors.weight && (
            <p className="mt-1 text-sm text-red-600">{errors.weight}</p>
          )}
        </div>
      </div>
    </div>
  );
}
