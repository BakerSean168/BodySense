import { act, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { WorkbenchProfileGate } from "./WorkbenchProfileGate";
import { useProfileStore, type UserProfile } from "@/stores/profileStore";

const profile: UserProfile = {
  id: "profile-1",
  user_id: "user-1",
  age: 30,
  created_at: "2026-08-26T00:00:00Z",
  updated_at: "2026-08-26T00:00:00Z",
};

function renderGate() {
  return render(
    <MemoryRouter initialEntries={["/consultation"]}>
      <Routes>
        <Route
          path="/consultation"
          element={
            <WorkbenchProfileGate>
              <div>健康工作台</div>
            </WorkbenchProfileGate>
          }
        />
        <Route path="/onboarding" element={<div>初始化身体档案</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

afterEach(() => {
  act(() => {
    useProfileStore.setState({
      profile: null,
      isLoading: false,
      error: null,
    });
  });
});

describe("WorkbenchProfileGate", () => {
  it("enters the single workbench when a durable profile exists", async () => {
    const fetchProfile = vi.fn(async () => {
      useProfileStore.setState({ profile, error: null, isLoading: false });
    });
    act(() => useProfileStore.setState({ fetchProfile }));

    renderGate();

    expect(await screen.findByText("健康工作台")).toBeInTheDocument();
    expect(fetchProfile).toHaveBeenCalledTimes(1);
  });

  it("routes first-time users to onboarding without a dashboard hop", async () => {
    const fetchProfile = vi.fn(async () => {
      useProfileStore.setState({ profile: null, error: null, isLoading: false });
    });
    act(() => useProfileStore.setState({ fetchProfile }));

    renderGate();

    await waitFor(() =>
      expect(screen.getByText("初始化身体档案")).toBeInTheDocument(),
    );
    expect(screen.queryByText("健康工作台")).not.toBeInTheDocument();
  });
});
