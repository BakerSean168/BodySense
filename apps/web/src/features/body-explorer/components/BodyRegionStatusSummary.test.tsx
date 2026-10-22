import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  bodyRecordsFixture,
  symptomFact,
} from "@/features/profile/services/__fixtures__/bodyRecordsFixture";
import { BodyRegionStatusSummary } from "./BodyRegionStatusSummary";

describe("BodyRegionStatusSummary", () => {
  it("shows original user statements for every linked region without claiming confirmation", () => {
    const records = bodyRecordsFixture();
    records.facts = [
      symptomFact({ body_region_ids: ["shoulder.right", "shoulder.left"] }),
      symptomFact({
        id: "old",
        statement: "旧陈述",
        assertion_status: "superseded",
      }),
    ];
    render(
      <BodyRegionStatusSummary snapshot={records} regionId="shoulder.left" />,
    );
    expect(screen.getByText("右肩抬手时疼")).toBeInTheDocument();
    expect(screen.queryByText("旧陈述")).not.toBeInTheDocument();
    expect(screen.queryByText("已确认记录")).not.toBeInTheDocument();
  });
});
