import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { RouteErrorBoundary } from "./RouteErrorBoundary";

function Broken(): never {
  throw new Error("render failed");
}

describe("RouteErrorBoundary", () => {
  it("contains workbench rendering failures without hiding durability guidance", () => {
    const errorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    render(
      <MemoryRouter initialEntries={["/consultation/conv-1"]}>
        <RouteErrorBoundary variant="consultation">
          <Broken />
        </RouteErrorBoundary>
      </MemoryRouter>,
      { onCaughtError: () => undefined },
    );

    expect(
      screen.getByRole("heading", { name: "健康工作区暂时无法显示" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/已保留服务器中的长期数据/)).toBeInTheDocument();
    errorSpy.mockRestore();
  });
});
