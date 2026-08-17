import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useWorkbenchPreferencesStore } from "../../model/workbenchPreferencesStore";
import { ConsultationWorkbenchShell } from "./ConsultationWorkbenchShell";

function renderShell(onWorkspaceViewChange = vi.fn()) {
  render(
    <MemoryRouter>
      <ConsultationWorkbenchShell
        title="智能问诊工作台"
        phaseLabel="问诊收集中"
        bodyStateRevision={4}
        bodyStateItemCount={3}
        workspaceView="state"
        onWorkspaceViewChange={onWorkspaceViewChange}
        onOpenHistory={vi.fn()}
        chat={<div>chat content</div>}
        workspace={<div>workspace content</div>}
      />
    </MemoryRouter>,
  );
  return onWorkspaceViewChange;
}

describe("ConsultationWorkbenchShell", () => {
  beforeEach(() => {
    localStorage.removeItem("bodysense-workbench-preferences");
    useWorkbenchPreferencesStore.setState({
      chatOpen: true,
      chatSize: 38,
      mobileSurface: "chat",
    });
  });

  it("keeps chat and workspace mounted in an accessible resizable group", () => {
    renderShell();

    expect(screen.getByText("chat content")).toBeInTheDocument();
    expect(screen.getByText("workspace content")).toBeInTheDocument();
    expect(
      screen.getByRole("separator", {
        name: "调整对话区与健康工作区宽度",
      }),
    ).toBeInTheDocument();
  });

  it("collapses chat without changing the selected workspace", () => {
    const onWorkspaceViewChange = renderShell();

    fireEvent.click(screen.getByRole("button", { name: "收起对话区" }));
    expect(useWorkbenchPreferencesStore.getState().chatOpen).toBe(false);
    expect(
      screen.getByRole("button", { name: "展开对话区" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "分析" }));
    expect(onWorkspaceViewChange).toHaveBeenCalledWith("diagnosis");
  });

  it("supports the desktop keyboard shortcut", () => {
    renderShell();

    fireEvent.keyDown(window, { key: "b", ctrlKey: true });
    expect(useWorkbenchPreferencesStore.getState().chatOpen).toBe(false);
  });
});
