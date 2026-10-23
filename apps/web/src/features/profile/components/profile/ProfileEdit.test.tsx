import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UserProfile } from "@/stores/profileStore";
import { ProfileEdit } from "./ProfileEdit";

const profile: UserProfile = {
  id: "profile-1",
  user_id: "user-1",
  gender: "male",
  birth_date: "2000-01-01",
  preferences: {},
  revision: 1,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("ProfileEdit", () => {
  it("拒绝未来的出生日期", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);

    render(
      <ProfileEdit
        profile={profile}
        onSave={onSave}
        onCancel={vi.fn()}
        isLoading={false}
      />,
    );

    fireEvent.change(screen.getByLabelText("出生日期"), {
      target: { value: "2099-01-01" },
    });

    await userEvent.click(
      screen.getByRole("button", { name: "保存" }),
    );

    expect(
      screen.getByText("请选择有效的出生日期"),
    ).toBeInTheDocument();

    expect(onSave).not.toHaveBeenCalled();
  });

  it("保存有效的出生日期", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);

    render(
      <ProfileEdit
        profile={profile}
        onSave={onSave}
        onCancel={vi.fn()}
        isLoading={false}
      />,
    );

    fireEvent.change(screen.getByLabelText("出生日期"), {
      target: { value: "1990-01-01" },
    });

    await userEvent.click(
      screen.getByRole("button", { name: "保存" }),
    );

    expect(onSave).toHaveBeenCalledWith({
      gender: "male",
    birth_date: "1990-01-01",
    });
  });
});
