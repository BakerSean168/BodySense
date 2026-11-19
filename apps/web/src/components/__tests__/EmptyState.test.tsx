import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { EmptyState } from "../ui/EmptyState";

describe("EmptyState", () => {
  it("renders title, description and action", () => {
    render(
      <EmptyState
        data-testid="empty"
        title="还没有记录"
        description="在对话里描述身体感受。"
        action={<button type="button">去对话</button>}
      />,
    );

    expect(screen.getByTestId("empty")).toHaveAttribute(
      "data-slot",
      "empty-state",
    );
    expect(screen.getByText("还没有记录")).toBeVisible();
    expect(screen.getByText("在对话里描述身体感受。")).toBeVisible();
    expect(screen.getByRole("button", { name: "去对话" })).toBeVisible();
  });

  it("keeps the icon decorative and allows a compact size", () => {
    render(
      <EmptyState
        size="sm"
        icon={<svg data-testid="empty-icon" />}
        title="暂无相关记录"
      />,
    );

    expect(screen.getByTestId("empty-icon").parentElement).toHaveAttribute(
      "aria-hidden",
      "true",
    );
    expect(screen.queryByText(/描述/)).not.toBeInTheDocument();
  });

  it("forwards role and test id to the root", () => {
    render(
      <EmptyState
        data-testid="status-empty"
        role="status"
        title="还没有诊断决定。"
      />,
    );

    expect(screen.getByRole("status")).toHaveAttribute(
      "data-testid",
      "status-empty",
    );
  });
});
