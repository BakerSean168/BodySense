import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PersonCategoryMenu } from "./PersonCategoryMenu";

afterEach(() => vi.unstubAllEnvs());

async function openMenu() {
  const user = userEvent.setup();
  render(<PersonCategoryMenu value="records" onChange={() => {}} />);
  await user.click(screen.getByRole("button", { name: /身体记录/ }));
  return user;
}

describe("PersonCategoryMenu S6-B flag", () => {
  it("keeps exactly the five established categories while the flag is off", async () => {
    await openMenu();
    expect(screen.getAllByRole("option")).toHaveLength(5);
    expect(
      screen.queryByRole("option", { name: "记忆" }),
    ).not.toBeInTheDocument();
  });

  it("adds the memory preview only when the flag is explicitly on", async () => {
    vi.stubEnv("VITE_EPISODIC_NOTES_CONTROLS_ENABLED", "true");
    await openMenu();
    expect(screen.getAllByRole("option")).toHaveLength(6);
    expect(screen.getByRole("option", { name: "记忆" })).toBeVisible();
  });
});
