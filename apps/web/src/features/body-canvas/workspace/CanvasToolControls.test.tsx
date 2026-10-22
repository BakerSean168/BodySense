import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DEFAULT_BODY_LAYERS } from "../model/bodyCanvas";
import { CanvasToolControls } from "./CanvasToolControls";

function defaults() {
  return {
    tool: "orbit" as const,
    onSelectTool: vi.fn(),
    onExitTool: vi.fn(),
    onReset: vi.fn(),
    anatomyMode: false,
    onToggleAnatomy: vi.fn(),
    bodyLayers: DEFAULT_BODY_LAYERS,
    skeletonStatus: "idle" as const,
    onToggleLayer: vi.fn(),
  };
}

describe("Canvas tool controls", () => {
  it("folds and restores the toolbar without losing focus or its tool choices", async () => {
    const user = userEvent.setup();
    const props = defaults();
    render(<CanvasToolControls {...props} />);
    await user.click(screen.getByRole("button", { name: "收起工具栏" }));
    const expand = screen.getByRole("button", { name: "展开工具栏" });
    expect(expand).toHaveAttribute("aria-expanded", "false");
    expect(expand).toHaveFocus();
    await user.click(expand);
    expect(
      screen.getByRole("toolbar", { name: "身体画布工具" }),
    ).toBeInTheDocument();
    const brush = screen.getByRole("button", { name: "标注 · 涂抹选区" });
    await user.click(brush);
    expect(props.onSelectTool).toHaveBeenCalledWith("brush");
  });

  it("routes anatomy layer toggles through the caller without changing the selection", async () => {
    const user = userEvent.setup();
    const props = {
      ...defaults(),
      anatomyMode: true,
      tool: "brush" as const,
      skeletonStatus: "loading" as const,
    };
    render(<CanvasToolControls {...props} />);
    const skeleton = screen.getByRole("button", { name: "骨骼" });
    expect(skeleton).toHaveAttribute("aria-busy", "true");
    await user.click(skeleton);
    expect(props.onToggleLayer).toHaveBeenCalledWith("skeleton");
    await user.click(screen.getByRole("button", { name: "退出选区模式" }));
    expect(props.onExitTool).toHaveBeenCalledOnce();
    await user.click(screen.getByRole("button", { name: "关闭图层" }));
    expect(props.onToggleAnatomy).toHaveBeenCalledOnce();
  });
});
