import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ProfileView } from "./ProfileView";
import type { UserProfile } from "@/stores/profileStore";

const profile: UserProfile = {
  id: "profile-1",
  user_id: "user-1",
  gender: "male",
  birth_date: "1998-05-20",
  age_years: 28,
  created_at: "2026-08-27T00:00:00Z",
  updated_at: "2026-08-27T00:00:00Z",
};

describe("ProfileView", () => {
  it("renders stable identity only and keeps mutable health state out of Profile", () => {
    render(<ProfileView profile={profile} onEdit={vi.fn()} />);

    expect(screen.getByText("基本身份")).toBeInTheDocument();
    expect(screen.getByText("1998 年 5 月 20 日")).toBeInTheDocument();
    expect(screen.getByText("28 岁")).toBeInTheDocument();
    expect(screen.queryByText("日常活动与工作习惯")).not.toBeInTheDocument();
    expect(screen.queryByText("睡眠与作息")).not.toBeInTheDocument();
    expect(screen.queryByText("体重")).not.toBeInTheDocument();
    expect(screen.queryByText("职业")).not.toBeInTheDocument();
  });
});
