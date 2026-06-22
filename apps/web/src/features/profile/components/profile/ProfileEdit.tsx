import { useState } from 'react';
import type { UserProfile } from '@/stores/profileStore';

interface ProfileEditProps {
  profile: UserProfile;
  onSave: (data: Partial<UserProfile>) => Promise<void>;
  onCancel: () => void;
  isLoading: boolean;
}

const GENDER_OPTIONS = [
  { value: 'male', label: '男' },
  { value: 'female', label: '女' },
];

const EXERCISE_FREQUENCY_OPTIONS = [
  { value: 'never', label: '从不运动' },
  { value: 'occasional', label: '偶尔运动' },
  { value: '1-2', label: '每周 1-2 次' },
  { value: '3-4', label: '每周 3-4 次' },
  { value: '5+', label: '每周 5 次以上' },
];

export function ProfileEdit({ profile, onSave, onCancel, isLoading }: ProfileEditProps) {
  const [formData, setFormData] = useState({
    gender: profile.gender || '',
    age: profile.age?.toString() || '',
    height_cm: profile.height_cm?.toString() || '',
    weight_kg: profile.weight_kg?.toString() || '',
    occupation: profile.occupation || '',
    exercise_frequency: profile.exercise_frequency || '',
    sleep_time: profile.sleep_time || '',
    wake_time: profile.wake_time || '',
    exercise_type: profile.exercise_type || '',
    injury_history: profile.injury_history || '',
    self_description: profile.self_description || '',
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (formData.age) {
      const age = parseInt(formData.age, 10);
      if (isNaN(age) || age < 1 || age > 150) {
        newErrors.age = '年龄必须在 1-150 之间';
      }
    }

    if (formData.height_cm) {
      const height = parseFloat(formData.height_cm);
      if (isNaN(height) || height < 50 || height > 250) {
        newErrors.height_cm = '身高必须在 50-250 cm 之间';
      }
    }

    if (formData.weight_kg) {
      const weight = parseFloat(formData.weight_kg);
      if (isNaN(weight) || weight < 20 || weight > 300) {
        newErrors.weight_kg = '体重必须在 20-300 kg 之间';
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) return;

    await onSave({
      gender: formData.gender || undefined,
      age: formData.age ? parseInt(formData.age, 10) : undefined,
      height_cm: formData.height_cm ? parseFloat(formData.height_cm) : undefined,
      weight_kg: formData.weight_kg ? parseFloat(formData.weight_kg) : undefined,
      occupation: formData.occupation || undefined,
      exercise_frequency: formData.exercise_frequency || undefined,
      sleep_time: formData.sleep_time || undefined,
      wake_time: formData.wake_time || undefined,
      exercise_type: formData.exercise_type || undefined,
      injury_history: formData.injury_history || undefined,
      self_description: formData.self_description || undefined,
    });
  };

  const handleChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    // Clear error when user starts typing
    if (errors[field]) {
      setErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-lg font-medium text-gray-900">编辑身体档案</h2>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={onCancel}
            disabled={isLoading}
            className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
          >
            取消
          </button>
          <button
            type="submit"
            disabled={isLoading}
            className="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
          >
            {isLoading ? '保存中...' : '保存'}
          </button>
        </div>
      </div>

      <div className="space-y-6">
        {/* Gender */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">性别</label>
          <div className="grid grid-cols-2 gap-3">
            {GENDER_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => handleChange('gender', option.value)}
                className={`p-3 rounded-lg border-2 text-center text-sm font-medium transition-colors ${
                  formData.gender === option.value
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-200 hover:border-gray-300 text-gray-700'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {/* Age */}
        <div>
          <label htmlFor="edit-age" className="block text-sm font-medium text-gray-700 mb-1">
            年龄
          </label>
          <input
            id="edit-age"
            type="number"
            value={formData.age}
            onChange={(e) => handleChange('age', e.target.value)}
            min={1}
            max={150}
            placeholder="请输入年龄"
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          {errors.age && <p className="mt-1 text-sm text-red-600">{errors.age}</p>}
        </div>

        {/* Height & Weight */}
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label htmlFor="edit-height" className="block text-sm font-medium text-gray-700 mb-1">
              身高 (cm)
            </label>
            <input
              id="edit-height"
              type="number"
              value={formData.height_cm}
              onChange={(e) => handleChange('height_cm', e.target.value)}
              min={50}
              max={250}
              step={0.1}
              placeholder="170"
              className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
            {errors.height_cm && <p className="mt-1 text-sm text-red-600">{errors.height_cm}</p>}
          </div>
          <div>
            <label htmlFor="edit-weight" className="block text-sm font-medium text-gray-700 mb-1">
              体重 (kg)
            </label>
            <input
              id="edit-weight"
              type="number"
              value={formData.weight_kg}
              onChange={(e) => handleChange('weight_kg', e.target.value)}
              min={20}
              max={300}
              step={0.1}
              placeholder="65"
              className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
            {errors.weight_kg && <p className="mt-1 text-sm text-red-600">{errors.weight_kg}</p>}
          </div>
        </div>

        {/* Occupation */}
        <div>
          <label htmlFor="edit-occupation" className="block text-sm font-medium text-gray-700 mb-1">
            职业
          </label>
          <input
            id="edit-occupation"
            type="text"
            value={formData.occupation}
            onChange={(e) => handleChange('occupation', e.target.value)}
            placeholder="例如：程序员、教师、销售..."
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        {/* Exercise Frequency */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">日常活动强度</label>
          <div className="grid grid-cols-2 gap-3">
            {EXERCISE_FREQUENCY_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => handleChange('exercise_frequency', option.value)}
                className={`p-3 rounded-lg border-2 text-center text-sm font-medium transition-colors ${
                  formData.exercise_frequency === option.value
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-200 hover:border-gray-300 text-gray-700'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {/* Sleep Time */}
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label htmlFor="edit-sleep" className="block text-sm font-medium text-gray-700 mb-1">
              入睡时间
            </label>
            <input
              id="edit-sleep"
              type="time"
              value={formData.sleep_time}
              onChange={(e) => handleChange('sleep_time', e.target.value)}
              className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>
          <div>
            <label htmlFor="edit-wake" className="block text-sm font-medium text-gray-700 mb-1">
              起床时间
            </label>
            <input
              id="edit-wake"
              type="time"
              value={formData.wake_time}
              onChange={(e) => handleChange('wake_time', e.target.value)}
              className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>
        </div>

        {/* Exercise Type */}
        <div>
          <label htmlFor="edit-exercise-type" className="block text-sm font-medium text-gray-700 mb-1">
            运动类型
          </label>
          <input
            id="edit-exercise-type"
            type="text"
            value={formData.exercise_type}
            onChange={(e) => handleChange('exercise_type', e.target.value)}
            placeholder="例如：跑步、游泳、健身、瑜伽..."
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        {/* Injury History */}
        <div>
          <label htmlFor="edit-injury" className="block text-sm font-medium text-gray-700 mb-1">
            伤病史
          </label>
          <textarea
            id="edit-injury"
            value={formData.injury_history}
            onChange={(e) => handleChange('injury_history', e.target.value)}
            rows={3}
            placeholder="例如：膝盖半月板损伤、腰椎间盘突出..."
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        {/* Self Description */}
        <div>
          <label htmlFor="edit-description" className="block text-sm font-medium text-gray-700 mb-1">
            自我描述
          </label>
          <textarea
            id="edit-description"
            value={formData.self_description}
            onChange={(e) => handleChange('self_description', e.target.value)}
            rows={3}
            placeholder="简单描述您的身体状况、健身目标等..."
            className="w-full rounded-md border border-gray-300 px-4 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>
      </div>
    </form>
  );
}
