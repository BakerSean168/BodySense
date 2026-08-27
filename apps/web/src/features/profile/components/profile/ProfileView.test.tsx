import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ProfileView } from "./ProfileView";
import type { UserProfile } from "@/stores/profileStore";

const profile: UserProfile = {
  id: "profile-1",
  user_id: "user-1",
  gender: "male",
  birth_date: "1998-05-20",
  height_cm: 178,
  weight_kg: 72,
  bmi: 22.7,
  activity_pattern: "久坐为主，每次连续坐 2-3 小时；每天会步行通勤。",
  sleep_pattern: "白班和夜班交替，平均每天睡 6-7 小时。",
  exercise_type: "力量训练",
  exercise_frequency: "3-4",
  injury_history: "2024 年左膝扭伤，跑量大时偶尔酸胀。",
  created_at: "2026-08-27T00:00:00Z",
  updated_at: "2026-08-27T00:00:00Z",
};

describe("ProfileView", () => {
  it("shows health-relevant context instead of occupation, fixed sleep times, or self description", () => {
    render(<ProfileView profile={profile} onEdit={vi.fn()} />);

    expect(screen.getByText("出生日期")).toBeInTheDocument();
    expect(screen.getByText("1998 年 5 月 20 日")).toBeInTheDocument();
    expect(screen.getByText("日常活动与工作习惯")).toBeInTheDocument();
    expect(screen.getByText("睡眠与作息")).toBeInTheDocument();
    expect(screen.getByText(profile.activity_pattern!)).toBeInTheDocument();
    expect(screen.getByText(profile.sleep_pattern!)).toBeInTheDocument();

    expect(screen.queryByText("职业")).not.toBeInTheDocument();
    expect(screen.queryByText("睡眠时段")).not.toBeInTheDocument();
    expect(screen.queryByText("自我描述")).not.toBeInTheDocument();
  });
});
