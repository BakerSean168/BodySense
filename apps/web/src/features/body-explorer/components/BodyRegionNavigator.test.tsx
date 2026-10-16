import { bodyRecordsFixture, symptomFact } from "@/features/profile/services/__fixtures__/bodyRecordsFixture";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { BodyRegionNavigator } from "./BodyRegionNavigator";

const snapshot = bodyRecordsFixture();
snapshot.facts = [symptomFact()];

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
    expect(select).toHaveTextContent("右肩 · 有记录 1");
    expect(screen.getByText("1 个区域有记录")).toBeInTheDocument();

    await user.selectOptions(select, "shoulder.right");
    expect(onSelectRegion).toHaveBeenCalledWith("shoulder.right");
    await user.selectOptions(select, "");
    expect(onSelectRegion).toHaveBeenLastCalledWith(null);
  });
});
