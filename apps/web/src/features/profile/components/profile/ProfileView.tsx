import type { UserProfile } from '@/stores/profileStore';

interface ProfileViewProps {
  profile: UserProfile;
  onEdit: () => void;
}

const GENDER_LABELS: Record<string, string> = {
  male: '男',
  female: '女',
};

const EXERCISE_FREQUENCY_LABELS: Record<string, string> = {
  never: '从不运动',
  occasional: '偶尔运动',
  '1-2': '每周 1-2 次',
  '3-4': '每周 3-4 次',
  '5+': '每周 5 次以上',
};

interface InfoItemProps {
  label: string;
  value: string | number | undefined | null;
  unit?: string;
}

function InfoItem({ label, value, unit }: InfoItemProps) {
  if (value === undefined || value === null || value === '') {
    return (
      <div className="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
        <dt className="text-sm font-medium text-gray-500">{label}</dt>
        <dd className="mt-1 text-sm text-gray-400 sm:col-span-2 sm:mt-0">未填写</dd>
      </div>
    );
  }

  return (
    <div className="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
      <dt className="text-sm font-medium text-gray-500">{label}</dt>
      <dd className="mt-1 text-sm text-gray-900 sm:col-span-2 sm:mt-0">
        {value}{unit && <span className="text-gray-500 ml-1">{unit}</span>}
      </dd>
    </div>
  );
}

export function ProfileView({ profile, onEdit }: ProfileViewProps) {
  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-lg font-medium text-gray-900">我的身体档案</h2>
        <button
          type="button"
          onClick={onEdit}
          className="px-4 py-2 text-sm font-medium text-blue-600 bg-blue-50 border border-blue-200 rounded-md hover:bg-blue-100 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          编辑
        </button>
      </div>

      <dl className="divide-y divide-gray-200">
        <InfoItem label="性别" value={profile.gender ? GENDER_LABELS[profile.gender] : undefined} />
        <InfoItem label="年龄" value={profile.age} unit="岁" />
        <InfoItem label="身高" value={profile.height_cm} unit="cm" />
        <InfoItem label="体重" value={profile.weight_kg} unit="kg" />
        <InfoItem label="BMI" value={profile.bmi} />
        <InfoItem label="职业" value={profile.occupation} />
        <InfoItem label="日常活动强度" value={profile.exercise_frequency ? EXERCISE_FREQUENCY_LABELS[profile.exercise_frequency] : undefined} />
        <InfoItem label="入睡时间" value={profile.sleep_time} />
        <InfoItem label="起床时间" value={profile.wake_time} />
        <InfoItem label="运动类型" value={profile.exercise_type} />
        <InfoItem label="运动频率" value={profile.exercise_frequency ? EXERCISE_FREQUENCY_LABELS[profile.exercise_frequency] : undefined} />

        {profile.injury_history && (
          <div className="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
            <dt className="text-sm font-medium text-gray-500">伤病史</dt>
            <dd className="mt-1 text-sm text-gray-900 sm:col-span-2 sm:mt-0 whitespace-pre-wrap">
              {profile.injury_history}
            </dd>
          </div>
        )}

        {profile.self_description && (
          <div className="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
            <dt className="text-sm font-medium text-gray-500">自我描述</dt>
            <dd className="mt-1 text-sm text-gray-900 sm:col-span-2 sm:mt-0 whitespace-pre-wrap">
              {profile.self_description}
            </dd>
          </div>
        )}
      </dl>
    </div>
  );
}
