import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BodyCanvasDock } from "./BodyCanvasDock";

describe("Penpot V2 Body Canvas Dock", () => {
  it("offers exactly four always-present module entrances, not a duplicate Trainer shortcut", () => {
    render(
      <BodyCanvasDock
        activeWindow={null}
        confirmedDiagnosisCount={0}
        onToggle={vi.fn()}
      />,
    );
    const navigation = screen.getByRole("navigation", {
      name: "身体工作区导航",
    });
    expect(navigation.querySelectorAll("button")).toHaveLength(4);
    expect(screen.getByRole("button", { name: "个人档案" })).toBeVisible();
    expect(screen.getByRole("button", { name: "训练计划" })).toBeVisible();
    expect(screen.getByRole("button", { name: "综合诊断" })).toBeVisible();
    expect(screen.getByRole("button", { name: "对话" })).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "训练执行" }),
    ).not.toBeInTheDocument();
  });

  it("shows the number of adopted/confirmed conditions on the Diagnosis Dock icon", async () => {
    const onToggle = vi.fn();
    render(
      <BodyCanvasDock
        activeWindow="diagnosis"
        confirmedDiagnosisCount={2}
        onToggle={onToggle}
      />,
    );
    const diagnosis = screen.getByRole("button", {
      name: "综合诊断，2 项已确认病情",
    });
    expect(diagnosis).toHaveAttribute("aria-pressed", "true");
    expect(diagnosis.querySelector(".bc-dock-count")).toHaveTextContent("2");
    await userEvent.click(diagnosis);
    expect(onToggle).toHaveBeenCalledWith("diagnosis");
    await userEvent.click(screen.getByRole("button", { name: "训练计划" }));
    expect(onToggle).toHaveBeenCalledWith("training");
  });

  it("hides the numeric badge for empty or untrusted counts and caps very large values", () => {
    const { rerender } = render(
      <BodyCanvasDock
        activeWindow={null}
        confirmedDiagnosisCount={0}
        onToggle={vi.fn()}
      />,
    );
    expect(document.querySelector(".bc-dock-count")).toBeNull();
    rerender(
      <BodyCanvasDock
        activeWindow={null}
        confirmedDiagnosisCount={Number.NaN}
        onToggle={vi.fn()}
      />,
    );
    expect(document.querySelector(".bc-dock-count")).toBeNull();
    rerender(
      <BodyCanvasDock
        activeWindow={null}
        confirmedDiagnosisCount={123}
        onToggle={vi.fn()}
      />,
    );
    expect(document.querySelector(".bc-dock-count")).toHaveTextContent("99+");
  });
});
