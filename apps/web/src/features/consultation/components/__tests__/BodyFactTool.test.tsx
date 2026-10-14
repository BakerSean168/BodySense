import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ToolCallItem } from "../ToolCallItem";

const id = "12345678-1234-4234-9234-123456789abc";
const receipt = {
  id,
  user_id: id,
  revision: 1,
  revision_id: id,
  event_id: id,
  fact_ids: [id],
  committed_at: "2026-10-13T00:00:00Z",
};

describe("BodyFact tool receipt status", () => {
  it("shows saved only for a schema-valid authoritative receipt", () => {
    render(
      <ToolCallItem
        toolCall={{
          id: "t1",
          tool: "record_body_state",
          args: {},
          status: "completed",
          result: { status: "persisted", receipt },
        }}
      />,
    );
    expect(screen.getByText("身体记录已保存")).toBeVisible();
    expect(screen.queryByText(id)).not.toBeInTheDocument();
  });
  it.each([
    { success: true },
    { status: "proposed", facts: [] },
    { status: "persisted", receipt: {} },
    { error: "failed" },
  ])(
    "never labels transport/extraction/failure as a saved record (%j)",
    (result) => {
      render(
        <ToolCallItem
          toolCall={{
            id: "t1",
            tool: "record_body_state",
            args: {},
            status: "completed",
            result,
          }}
        />,
      );
      expect(screen.getByText("身体记录尚未确认保存")).toBeVisible();
      expect(screen.queryByText("身体记录已保存")).not.toBeInTheDocument();
    },
  );
});
