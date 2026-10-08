import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { BodyStateProjection } from "@/features/consultation/types/consultation";
import { BodyRegionStatusSummary } from "./BodyRegionStatusSummary";

const snapshot = {
  current_revision: 2,
  safety_state: {},
  observations: [],
  facts: [
    {
      id: "fact-right",
      concern_key: "region:shoulder.right",
      kind: "discomfort",
      body_region: "右肩",
      body_region_id: "shoulder.right",
      value: "抬高手臂时右肩疼",
      details: {},
      origin: "user_reported",
      review_state: "confirmed",
      lifecycle_state: "active",
      trend: "stable",
      updated_revision: 2,
    },
    {
      id: "fact-left",
      concern_key: "region:shoulder.left",
      kind: "discomfort",
      body_region: "左肩",
      body_region_id: "shoulder.left",
      value: "左肩测试记录",
      details: {},
      origin: "user_reported",
      review_state: "confirmed",
      lifecycle_state: "active",
      trend: "stable",
      updated_revision: 2,
    },
  ],
} as BodyStateProjection;

describe("BodyRegionStatusSummary", () => {
  it("shows only durable records for the selected canonical region", () => {
    render(
      <BodyRegionStatusSummary
        snapshot={snapshot}
        regionId="shoulder.right"
      />,
    );

    expect(screen.getByText("抬高手臂时右肩疼")).toBeInTheDocument();
    expect(screen.queryByText("左肩测试记录")).not.toBeInTheDocument();
    expect(screen.getByText("已确认记录")).toBeInTheDocument();
  });
});
