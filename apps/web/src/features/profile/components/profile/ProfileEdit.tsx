import { useState } from "react";
import type { UserProfile } from "@/stores/profileStore";
import { Button } from "@/components/ui/Button";

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

function localDateString(value: Date) {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function ProfileEdit({ profile, onSave, onCancel, isLoading }: ProfileEditProps) {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const earliestBirthDate = new Date(today);
  earliestBirthDate.setFullYear(earliestBirthDate.getFullYear() - 150);
  const [gender, setGender] = useState(profile.gender || "");
  const [birthDate, setBirthDate] = useState(profile.birth_date || "");
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (birthDate) {
      const parsed = new Date(`${birthDate}T00:00:00`);
      if (Number.isNaN(parsed.getTime()) || parsed > today || parsed < earliestBirthDate) {
        setError("请选择有效的出生日期");
        return;
      }
    }
    setError(null);
    await onSave({ gender: gender || undefined, birth_date: birthDate || undefined });
  };

  return (
    <form onSubmit={submit} className="space-y-5">
      <div>
        <h2 className="text-lg font-semibold text-foreground">编辑基本身份</h2>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">
          可变化的身体和生活信息不会在这里编辑；它们分别进入身体测量、生活方式与 BodyState。
        </p>
      </div>

      <div>
        <div className="mb-2 text-sm font-medium text-foreground">性别</div>
        <div className="grid grid-cols-2 gap-3">
          {GENDER_OPTIONS.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => setGender(option.value)}
              className={`rounded-xl border px-3 py-3 text-sm font-medium transition-colors ${
                gender === option.value
                  ? "border-primary bg-primary/10 text-primary"
                  : "border-border text-foreground hover:bg-muted/50"
              }`}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <div>
        <label htmlFor="profile-birth-date" className="mb-2 block text-sm font-medium text-foreground">
          出生日期
        </label>
        <input
          id="profile-birth-date"
          type="date"
          value={birthDate}
          onChange={(event) => setBirthDate(event.target.value)}
          min={localDateString(earliestBirthDate)}
          max={localDateString(today)}
          className="w-full rounded-xl border border-border bg-background px-3 py-2.5 text-sm outline-none focus:border-primary"
        />
        {error ? <p className="mt-2 text-xs text-destructive">{error}</p> : null}
      </div>

      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" disabled={isLoading} onClick={onCancel}>
          取消
        </Button>
        <Button type="submit" isLoading={isLoading}>
          保存
        </Button>
      </div>
    </form>
  );
}
