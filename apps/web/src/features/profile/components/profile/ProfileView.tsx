import type { UserProfile } from "@/stores/profileStore";

interface ProfileViewProps {
  profile: UserProfile;
  onEdit: () => void;
}

const GENDER_LABELS: Record<string, string> = {
  male: "男",
  female: "女",
};

function formatBirthDate(value?: string) {
  if (!value) return "未填写";
  const [year, month, day] = value.slice(0, 10).split("-");
  if (!year || !month || !day) return value;
  return `${year} 年 ${Number(month)} 月 ${Number(day)} 日`;
}

export function ProfileView({ profile, onEdit }: ProfileViewProps) {
  return (
    <div className="space-y-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-foreground">基本身份</h2>
          <p className="mt-1 max-w-2xl text-xs leading-5 text-muted-foreground">
            这里只保存相对稳定、不会频繁变化的身份背景。身体测量、生活方式、伤病与症状由 BodyState
            持续记录并保留变化历史。
          </p>
        </div>
        <button
          type="button"
          onClick={onEdit}
          className="rounded-full bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground transition-opacity hover:opacity-90"
        >
          编辑基本信息
        </button>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <div className="rounded-2xl border border-border bg-muted/20 p-4">
          <div className="text-xs font-medium text-muted-foreground">性别</div>
          <div className="mt-2 text-base font-semibold text-foreground">
            {profile.gender ? GENDER_LABELS[profile.gender] || profile.gender : "未填写"}
          </div>
        </div>
        <div className="rounded-2xl border border-border bg-muted/20 p-4">
          <div className="text-xs font-medium text-muted-foreground">出生日期</div>
          <div className="mt-2 text-base font-semibold text-foreground">
            {formatBirthDate(profile.birth_date)}
          </div>
        </div>
        <div className="rounded-2xl border border-border bg-muted/20 p-4">
          <div className="text-xs font-medium text-muted-foreground">当前年龄</div>
          <div className="mt-2 text-base font-semibold text-foreground">
            {profile.age_years != null ? `${profile.age_years} 岁` : "未计算"}
          </div>
        </div>
      </div>
    </div>
  );
}
