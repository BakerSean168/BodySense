import type { UserProfile } from "@/stores/profileStore";

interface ProfileViewProps {
  profile: UserProfile;
  onEdit: () => void;
}

const GENDER_LABELS: Record<string, string> = {
  male: "男",
  female: "女",
};

const EXERCISE_FREQUENCY_LABELS: Record<string, string> = {
  never: "从不运动",
  occasional: "偶尔运动",
  "1-2": "每周 1-2 次",
  "3-4": "每周 3-4 次",
  "5+": "每周 5 次以上",
};

export function ProfileView({ profile, onEdit }: ProfileViewProps) {
  return (
    <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
      {/* Title / Action */}
      <div className="flex justify-between items-center pb-4 border-b border-[#E5E3DF]">
        <h2 className="text-lg font-display font-semibold text-[#2E3C36]">
          我的身体档案
        </h2>
        <button
          type="button"
          onClick={onEdit}
          className="px-5 py-2 rounded-full text-sm font-semibold text-white bg-primary-700 hover:bg-primary-800 transition-colors duration-300 cursor-pointer"
        >
          编辑档案
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Card 1: 基础体征 */}
        <div className="bg-[#FBFBFA] border border-[#E5E3DF] rounded-3xl p-6 space-y-4">
          <h3 className="text-sm font-bold text-[#709a83] uppercase tracking-wider mb-2">
            基础体征
          </h3>
          <div className="grid grid-cols-2 gap-4">
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50">
              <span className="text-xs text-slate-400 block mb-1 font-semibold">
                性别
              </span>
              <span className="text-base font-bold text-[#2E3C36]">
                {profile.gender ? GENDER_LABELS[profile.gender] : "未填写"}
              </span>
            </div>
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50">
              <span className="text-xs text-slate-400 block mb-1 font-semibold">
                年龄
              </span>
              <span className="text-base font-bold text-[#2E3C36]">
                {profile.age ? `${profile.age} 岁` : "未填写"}
              </span>
            </div>
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50">
              <span className="text-xs text-slate-400 block mb-1 font-semibold">
                身高
              </span>
              <span className="text-base font-bold text-[#2E3C36]">
                {profile.height_cm ? `${profile.height_cm} cm` : "未填写"}
              </span>
            </div>
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50">
              <span className="text-xs text-slate-400 block mb-1 font-semibold">
                体重
              </span>
              <span className="text-base font-bold text-[#2E3C36]">
                {profile.weight_kg ? `${profile.weight_kg} kg` : "未填写"}
              </span>
            </div>
            <div className="col-span-2 bg-[#F7F5F0]/50 p-4 rounded-2xl border border-[#E5E3DF]/80">
              <div className="flex justify-between items-center">
                <div>
                  <span className="text-xs text-[#709a83] font-semibold block mb-1">
                    身体质量指数 (BMI)
                  </span>
                  <span className="text-xl font-display font-bold text-[#2E3C36]">
                    {profile.bmi || "暂无评分"}
                  </span>
                </div>
                {profile.bmi && (
                  <span
                    className={`px-2.5 py-1 rounded-full text-xs font-semibold ${
                      profile.bmi < 18.5
                        ? "bg-yellow-50 text-yellow-800"
                        : profile.bmi < 24
                          ? "bg-emerald-50 text-emerald-800"
                          : profile.bmi < 28
                            ? "bg-yellow-50 text-yellow-800"
                            : "bg-red-50 text-red-800"
                    }`}
                  >
                    {profile.bmi < 18.5
                      ? "体重过轻"
                      : profile.bmi < 24
                        ? "体重正常"
                        : profile.bmi < 28
                          ? "超重"
                          : "肥胖"}
                  </span>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Card 2: 生活习惯 */}
        <div className="bg-[#FBFBFA] border border-[#E5E3DF] rounded-3xl p-6 space-y-4">
          <h3 className="text-sm font-bold text-[#709a83] uppercase tracking-wider mb-2">
            生活习惯
          </h3>
          <div className="space-y-3">
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50 flex justify-between items-center">
              <span className="text-sm text-slate-500 font-semibold">职业</span>
              <span className="text-sm font-bold text-[#2E3C36]">
                {profile.occupation || "未填写"}
              </span>
            </div>
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50 flex justify-between items-center">
              <span className="text-sm text-slate-500 font-semibold">
                日常活动强度
              </span>
              <span className="text-sm font-bold text-[#2E3C36]">
                {profile.exercise_frequency
                  ? EXERCISE_FREQUENCY_LABELS[profile.exercise_frequency]
                  : "未填写"}
              </span>
            </div>
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50 flex justify-between items-center">
              <span className="text-sm text-slate-500 font-semibold">
                睡眠时段
              </span>
              <span className="text-sm font-bold text-[#2E3C36]">
                {profile.sleep_time && profile.wake_time
                  ? `${profile.sleep_time} 至 ${profile.wake_time}`
                  : "未填写"}
              </span>
            </div>
          </div>
        </div>

        {/* Card 3: 运动习惯 */}
        <div className="bg-[#FBFBFA] border border-[#E5E3DF] rounded-3xl p-6 space-y-4">
          <h3 className="text-sm font-bold text-[#709a83] uppercase tracking-wider mb-2">
            运动习惯
          </h3>
          <div className="space-y-3">
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50 flex justify-between items-center">
              <span className="text-sm text-slate-500 font-semibold">
                运动类型
              </span>
              <span className="text-sm font-bold text-[#2E3C36]">
                {profile.exercise_type || "未填写"}
              </span>
            </div>
            <div className="bg-white/60 p-4 rounded-2xl border border-[#E5E3DF]/50 flex justify-between items-center">
              <span className="text-sm text-slate-500 font-semibold">
                运动频率
              </span>
              <span className="text-sm font-bold text-[#2E3C36]">
                {profile.exercise_frequency
                  ? EXERCISE_FREQUENCY_LABELS[profile.exercise_frequency]
                  : "未填写"}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Detailed Notes */}
      {(profile.injury_history || profile.self_description) && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-2">
          {profile.injury_history && (
            <div className="bg-[#FBFBFA] border border-red-100 rounded-3xl p-6 space-y-3">
              <h3 className="text-sm font-bold text-[#B65E49] uppercase tracking-wider flex items-center gap-1.5">
                <svg
                  className="w-4.5 h-4.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                伤病史
              </h3>
              <p className="text-sm text-[#5D6B63] leading-relaxed whitespace-pre-wrap pl-1 font-semibold">
                {profile.injury_history}
              </p>
            </div>
          )}
          {profile.self_description && (
            <div className="bg-[#FBFBFA] border border-[#E5E3DF] rounded-3xl p-6 space-y-3">
              <h3 className="text-sm font-bold text-[#709a83] uppercase tracking-wider">
                自我描述
              </h3>
              <p className="text-sm text-[#5D6B63] leading-relaxed whitespace-pre-wrap pl-1 font-semibold">
                {profile.self_description}
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
