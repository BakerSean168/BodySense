import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { BodyRegionNavigator } from "./BodyRegionNavigator";

const snapshot = {
  user_id: "user-1",
  current_revision: 2,
  safety_state: {},
  facts: [
    {
      id: "fact-1",
      kind: "discomfort",
      body_region: "右肩",
      body_region_id: "shoulder.right",
      value: "抬手时疼",
      origin: "user_reported",
      review_state: "confirmed",
      lifecycle_state: "active",
      trend: "worsening",
      updated_revision: 2,
    },
  ],
  observations: [],
  pending_observations: [],
  hypotheses: [],
};

describe("BodyRegionNavigator", () => {
  it("exposes canonical regions and durable status through an accessible select", async () => {
    const user = userEvent.setup();
    const onSelectRegion = vi.fn();
    render(
      <BodyRegionNavigator
        snapshot={snapshot}
        selectedRegionId={null}
        onSelectRegion={onSelectRegion}
      />,
    );

    const select = screen.getByRole("combobox", { name: "选择身体区域" });
    expect(select).toHaveTextContent("右肩 · 加重 1");
    expect(screen.getByText("1 个区域有记录")).toBeInTheDocument();

    await user.selectOptions(select, "shoulder.right");
    expect(onSelectRegion).toHaveBeenCalledWith("shoulder.right");
    await user.selectOptions(select, "");
    expect(onSelectRegion).toHaveBeenLastCalledWith(null);
  });
});
