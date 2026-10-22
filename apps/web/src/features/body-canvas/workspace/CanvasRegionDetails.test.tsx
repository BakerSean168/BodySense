import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { getReferenceMotion } from "../model/bodyCanvas";
import { CanvasRegionDetails } from "./CanvasRegionDetails";

function defaults() {
  return {
    primary: "shoulder.right" as const,
    selection: { regions: ["shoulder.right" as const], history: [] },
    regionFacts: [],
    hasBodyState: true,
    loading: false,
    error: false,
    regionCardOpen: true,
    regionListOpen: false,
    activeWindow: false,
    motionMode: false,
    definition: getReferenceMotion("stand"),
    phase: 0,
    selectionLabel: "右肩",
    onCardOpenChange: vi.fn(),
    onRemoveRegion: vi.fn(),
    onUndo: vi.fn(),
    onClear: vi.fn(),
    onAsk: vi.fn(),
    onOpenPerson: vi.fn(),
  };
}

describe("Canvas region details", () => {
  it("offers actions without inventing body facts when records are empty", async () => {
    const user = userEvent.setup();
    const props = defaults();
    render(<CanvasRegionDetails {...props} />);
    const panel = screen.getByRole("complementary", {
      name: "已选区域当前状况",
    });
    expect(panel).toHaveTextContent("这里还没有已记录的信息");
    await user.click(screen.getByRole("button", { name: /围绕.*提问/ }));
    expect(props.onAsk).toHaveBeenCalledOnce();
    await user.click(screen.getByRole("button", { name: "身体记录" }));
    expect(props.onOpenPerson).toHaveBeenCalledOnce();
    await user.click(screen.getByRole("button", { name: /取消选择/ }));
    expect(props.onRemoveRegion).toHaveBeenCalledWith("shoulder.right");
  });

  it("distinguishes failed loading from a successful empty record", () => {
    const props = { ...defaults(), hasBodyState: false, error: true };
    const { rerender } = render(<CanvasRegionDetails {...props} />);
    expect(screen.getByText(/暂时无法读取记录/)).toBeVisible();
    expect(screen.queryByText("这里还没有已记录的信息")).toBeNull();
    rerender(<CanvasRegionDetails {...props} regionCardOpen={false} />);
    expect(
      screen.queryByRole("complementary", { name: "已选区域当前状况" }),
    ).toBeNull();
    expect(screen.getByRole("button", { name: "右肩" })).toBeVisible();
  });
});
