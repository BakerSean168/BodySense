import { useState } from "react";
import type { UserProfile } from "@/stores/profileStore";

interface ProfileEditProps {
  profile: UserProfile;
  onSave: (data: Partial<UserProfile>) => Promise<void>;
  onCancel: () => void;
  isLoading: boolean;
}

const GENDER_OPTIONS = [
  { value: "male", label: "男" },
  { value: "female", label: "女" },
];

const EXERCISE_FREQUENCY_OPTIONS = [
  { value: "never", label: "从不运动" },
  { value: "occasional", label: "偶尔运动" },
  { value: "1-2", label: "每周 1-2 次" },
  { value: "3-4", label: "每周 3-4 次" },
  { value: "5+", label: "每周 5 次以上" },
];

function localDateString(value: Date) {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function ProfileEdit({
  profile,
  onSave,
  onCancel,
  isLoading,
}: ProfileEditProps) {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const earliestBirthDate = new Date(today);
  earliestBirthDate.setFullYear(earliestBirthDate.getFullYear() - 150);

  const [formData, setFormData] = useState({
    gender: profile.gender || "",
    birth_date: profile.birth_date || "",
    height_cm: profile.height_cm?.toString() || "",
    weight_kg: profile.weight_kg?.toString() || "",
    activity_pattern: profile.activity_pattern || "",
    sleep_pattern: profile.sleep_pattern || "",
    exercise_frequency: profile.exercise_frequency || "",
    exercise_type: profile.exercise_type || "",
    injury_history: profile.injury_history || "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (formData.birth_date) {
      const birthDate = new Date(`${formData.birth_date}T00:00:00`);
      if (
        Number.isNaN(birthDate.getTime()) ||
        birthDate > today ||
        birthDate < earliestBirthDate
      ) {
        newErrors.birth_date = "请选择有效的出生日期";
      }
    }

    if (formData.height_cm) {
      const height = parseFloat(formData.height_cm);
      if (isNaN(height) || height < 50 || height > 250) {
        newErrors.height_cm = "身高必须在 50-250 cm 之间";
      }
    }

    if (formData.weight_kg) {
      const weight = parseFloat(formData.weight_kg);
      if (isNaN(weight) || weight < 20 || weight > 300) {
        newErrors.weight_kg = "体重必须在 20-300 kg 之间";
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!validate()) return;

    await onSave({
      gender: formData.gender || undefined,
      birth_date: formData.birth_date || undefined,
      height_cm: formData.height_cm
        ? parseFloat(formData.height_cm)
        : undefined,
      weight_kg: formData.weight_kg
        ? parseFloat(formData.weight_kg)
        : undefined,
      activity_pattern: formData.activity_pattern || undefined,
      sleep_pattern: formData.sleep_pattern || undefined,
      exercise_frequency: formData.exercise_frequency || undefined,
      exercise_type: formData.exercise_type || undefined,
      injury_history: formData.injury_history || undefined,
    });
  };

  const handleChange = (field: string, value: string) => {
    setFormData((previous) => ({ ...previous, [field]: value }));
    if (errors[field]) {
      setErrors((previous) => {
        const next = { ...previous };
        delete next[field];
        return next;
      });
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-lg font-medium text-gray-900">编辑身体档案</h2>
          <p className="mt-1 text-xs text-gray-500">
            只保留对身体健康判断真正有帮助的信息，不需要提供具体职业或重复描述当前症状。
          </p>
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={onCancel}
            disabled={isLoading}
            className="px-5 py-2 rounded-full text-sm font-semibold text-[#4A554E] bg-[#F7F5F0] border border-[#E5E3DF] hover:bg-primary-50 transition-colors duration-300 disabled:opacity-50"
          >
            取消
          </button>
          <button
            type="submit"
            disabled={isLoading}
            className="px-5 py-2 rounded-full text-sm font-semibold text-white bg-primary-700 hover:bg-primary-800 transition-colors duration-300 disabled:opacity-50"
          >
            {isLoading ? "保存中..." : "保存"}
          </button>
        </div>
      </div>

      <div className="space-y-6">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            性别
          </label>
          <div className="grid grid-cols-2 gap-3">
            {GENDER_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => handleChange("gender", option.value)}
                className={`p-3 rounded-lg border-2 text-center text-sm font-semibold transition-colors ${
                  formData.gender === option.value
                    ? "border-primary-700 bg-primary-50/50 text-primary-800"
                    : "border-gray-200 hover:border-gray-300 text-gray-700"
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        <div>
          <label
            htmlFor="edit-birth-date"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            出生日期
          </label>
          <input
            id="edit-birth-date"
            type="date"
            value={formData.birth_date}
            onChange={(event) => handleChange("birth_date", event.target.value)}
            min={localDateString(earliestBirthDate)}
            max={localDateString(today)}
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-primary-600 focus:outline-none focus:ring-1 focus:ring-primary-600"
          />
          {errors.birth_date && (
            <p className="mt-1 text-sm text-red-600">{errors.birth_date}</p>
          )}
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label
              htmlFor="edit-height"
              className="block text-sm font-medium text-gray-700 mb-1"
            >
              身高 (cm)
            </label>
            <input
              id="edit-height"
              type="number"
              value={formData.height_cm}
              onChange={(event) =>
                handleChange("height_cm", event.target.value)
              }
              min={50}
              max={250}
              step={0.1}
              placeholder="170"
              className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-primary-600 focus:outline-none focus:ring-1 focus:ring-primary-600"
            />
            {errors.height_cm && (
              <p className="mt-1 text-sm text-red-600">{errors.height_cm}</p>
            )}
          </div>
          <div>
            <label
              htmlFor="edit-weight"
              className="block text-sm font-medium text-gray-700 mb-1"
            >
              体重 (kg)
            </label>
            <input
              id="edit-weight"
              type="number"
              value={formData.weight_kg}
              onChange={(event) =>
                handleChange("weight_kg", event.target.value)
              }
              min={20}
              max={300}
              step={0.1}
              placeholder="65"
              className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-primary-600 focus:outline-none focus:ring-1 focus:ring-primary-600"
            />
            {errors.weight_kg && (
              <p className="mt-1 text-sm text-red-600">{errors.weight_kg}</p>
            )}
          </div>
        </div>

        <div>
          <label
            htmlFor="edit-activity-pattern"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            日常活动与工作习惯
          </label>
          <p className="mb-2 text-xs text-gray-500">
            不需要写职业名称，描述久坐、久站、走动、搬抬、重复动作或轮班等身体使用模式即可。
          </p>
          <textarea
            id="edit-activity-pattern"
            value={formData.activity_pattern}
            onChange={(event) =>
              handleChange("activity_pattern", event.target.value)
            }
            rows={4}
            placeholder="例如：工作日久坐为主，每次连续坐 2-3 小时；每天步行约 40 分钟。"
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-primary-600 focus:outline-none focus:ring-1 focus:ring-primary-600"
          />
        </div>

        <div>
          <label
            htmlFor="edit-sleep-pattern"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            睡眠与作息
          </label>
          <p className="mb-2 text-xs text-gray-500">
            可直接描述作息是否规律、轮班、通常睡多久、夜醒或缺觉，不要求固定时间表。
          </p>
          <textarea
            id="edit-sleep-pattern"
            value={formData.sleep_pattern}
            onChange={(event) =>
              handleChange("sleep_pattern", event.target.value)
            }
            rows={4}
            placeholder="例如：白班和夜班交替，起床时间不固定；平均每天睡 6-7 小时。"
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-primary-600 focus:outline-none focus:ring-1 focus:ring-primary-600"
          />
        </div>

        <div>
          <label
            htmlFor="edit-exercise-type"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            常做的运动
          </label>
          <input
            id="edit-exercise-type"
            type="text"
            value={formData.exercise_type}
            onChange={(event) =>
              handleChange("exercise_type", event.target.value)
            }
            placeholder="例如：跑步、游泳、力量训练、瑜伽..."
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-primary-600 focus:outline-none focus:ring-1 focus:ring-primary-600"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            运动频率
          </label>
          <div className="grid grid-cols-2 gap-3">
            {EXERCISE_FREQUENCY_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => handleChange("exercise_frequency", option.value)}
                className={`p-3 rounded-lg border-2 text-center text-sm font-semibold transition-colors ${
                  formData.exercise_frequency === option.value
                    ? "border-primary-700 bg-primary-50/50 text-primary-800"
                    : "border-gray-200 hover:border-gray-300 text-gray-700"
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        <div>
          <label
            htmlFor="edit-injury"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            既往伤病与手术史
          </label>
          <textarea
            id="edit-injury"
            value={formData.injury_history}
            onChange={(event) =>
              handleChange("injury_history", event.target.value)
            }
            rows={4}
            placeholder="例如：2024 年左膝扭伤，未手术，跑步量大时偶尔酸胀。"
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-primary-600 focus:outline-none focus:ring-1 focus:ring-primary-600"
          />
        </div>
      </div>
    </form>
  );
}
