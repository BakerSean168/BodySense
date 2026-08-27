import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/stores/authStore";
import { AppUserMenu } from "./AppUserMenu";

afterEach(() => {
  useAuthStore.setState({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    hasHydrated: false,
    isAuthResolved: false,
    isVerifyingSession: false,
    isLoading: false,
    error: null,
  });
});

describe("AppUserMenu", () => {
  it("opens the avatar menu without throwing a Base UI group-context error", async () => {
    useAuthStore.setState({
      user: { id: "user-1", email: "member@example.com" },
      isAuthenticated: true,
      hasHydrated: true,
      isAuthResolved: true,
    });
    const onOpenProfile = vi.fn();
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <AppUserMenu compact onOpenProfile={onOpenProfile} />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole("button", { name: "打开用户菜单" }));

    expect(screen.getByText("member@example.com")).toBeInTheDocument();
    await user.click(screen.getByRole("menuitem", { name: /身体档案/ }));
    expect(onOpenProfile).toHaveBeenCalledTimes(1);
  });
});
