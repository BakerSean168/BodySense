import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { CanvasWorkspaceDialogs } from "./CanvasWorkspaceDialogs";
import { regionPaletteShortcut } from "../model/bodyCanvas";

function renderHelp() {
  render(
    <MemoryRouter>
      <CanvasWorkspaceDialogs
        canvasOverlay="help"
        onCanvasOverlayChange={vi.fn()}
        accountDialog={null}
        onAccountDialogChange={vi.fn()}
        themePreference="system"
        onThemePreferenceChange={vi.fn()}
        themeClass="body-canvas-theme"
      />
    </MemoryRouter>,
  );
}

describe("Canvas workspace help", () => {
  it("lists the palette shortcut without the old region picker entry", () => {
    renderHelp();
    expect(screen.getByRole("dialog", { name: "快捷键" })).toBeVisible();
    expect(screen.getByText(regionPaletteShortcut())).toBeVisible();
    expect(screen.getByText("按名称选择身体区域")).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "按名称选择身体区域" }),
    ).toBeNull();
    expect(screen.queryByText(/每个快捷操作都有可见按钮/)).toBeNull();
  });
});
