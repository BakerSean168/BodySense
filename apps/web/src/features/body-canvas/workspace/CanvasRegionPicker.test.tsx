import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BODY_REGION_IDS } from "@/features/body-explorer/model/bodyRegionOntology";
import { CanvasRegionPicker } from "./CanvasRegionPicker";

function defaults() {
  return {
    visible: true,
    selection: { regions: [], history: [] },
    themeClass: "body-canvas-theme",
    onSelect: vi.fn(),
    onVisibleChange: vi.fn(),
    onCardOpenChange: vi.fn(),
  };
}

describe("Canvas region palette", () => {
  it("filters by Chinese label and selects on Enter", async () => {
    const user = userEvent.setup();
    const props = defaults();
    render(<CanvasRegionPicker {...props} />);
    const search = screen.getByRole("combobox", { name: "搜索身体区域" });
    expect(screen.getAllByRole("option")).toHaveLength(BODY_REGION_IDS.length);

    await user.type(search, "右肩");
    expect(screen.getByRole("option", { name: "右肩" })).toHaveAttribute(
      "aria-selected",
      "false",
    );

    await user.keyboard("{Enter}");
    expect(props.onSelect).toHaveBeenCalledWith(["shoulder.right"], false);
    expect(props.onVisibleChange).toHaveBeenCalledWith(false);
    expect(props.onCardOpenChange).toHaveBeenCalledWith(true);
  });

  it("matches the canonical id and the English label", async () => {
    const user = userEvent.setup();
    const props = defaults();
    render(<CanvasRegionPicker {...props} />);
    const search = screen.getByRole("combobox", { name: "搜索身体区域" });

    await user.type(search, "right shoulder");
    expect(screen.getByRole("option", { name: "右肩" })).toBeVisible();

    await user.clear(search);
    await user.type(search, "shoulder.left");
    expect(screen.getByRole("option", { name: "左肩" })).toBeVisible();
  });

  it("appends with Shift+Enter and keeps the palette open", async () => {
    const user = userEvent.setup();
    const props = defaults();
    render(<CanvasRegionPicker {...props} />);
    await user.type(
      screen.getByRole("combobox", { name: "搜索身体区域" }),
      "左肩",
    );
    await user.keyboard("{Shift>}{Enter}{/Shift}");
    expect(props.onSelect).toHaveBeenCalledWith(["shoulder.left"], true);
    expect(props.onVisibleChange).not.toHaveBeenCalled();
    expect(props.onCardOpenChange).not.toHaveBeenCalled();
  });

  it("moves the active option with arrow keys and marks selected regions", async () => {
    const user = userEvent.setup();
    const props = {
      ...defaults(),
      selection: { regions: ["head" as const], history: [] },
    };
    render(<CanvasRegionPicker {...props} />);
    const search = screen.getByRole("combobox", { name: "搜索身体区域" });
    const options = screen.getAllByRole("option");
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    expect(search).toHaveAttribute("aria-activedescendant", options[0].id);

    await user.keyboard("{ArrowDown}");
    expect(search).toHaveAttribute("aria-activedescendant", options[1].id);
    await user.keyboard("{Enter}");
    expect(props.onSelect).toHaveBeenCalledWith(["neck"], false);
  });

  it("shows one short line when nothing matches", async () => {
    const user = userEvent.setup();
    const props = defaults();
    render(<CanvasRegionPicker {...props} />);
    await user.type(
      screen.getByRole("combobox", { name: "搜索身体区域" }),
      "不存在",
    );
    expect(screen.getByText("没有匹配的区域")).toBeVisible();
    expect(screen.queryAllByRole("option")).toHaveLength(0);
  });
});
