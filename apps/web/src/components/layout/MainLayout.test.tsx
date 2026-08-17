import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";
import { MainLayout } from "./MainLayout";

function renderLayout(pathname = "/profile") {
  return render(
    <MemoryRouter initialEntries={[pathname]}>
      <MainLayout>
        <div>route content</div>
      </MainLayout>
    </MemoryRouter>,
  );
}

describe("MainLayout", () => {
  it("renders the compact application shell and active section", () => {
    renderLayout();

    expect(screen.getByLabelText("主导航")).toBeInTheDocument();
    expect(screen.getAllByText("身体档案")).toHaveLength(2);
    expect(screen.getByText("route content")).toBeInTheDocument();
    expect(screen.getByLabelText("打开用户菜单")).toBeInTheDocument();
  });

  it("can render immersive content without duplicate global chrome", () => {
    render(
      <MemoryRouter initialEntries={["/consultation/conversation-1"]}>
        <MainLayout chrome="immersive" fullHeight>
          <div>immersive workbench</div>
        </MainLayout>
      </MemoryRouter>,
    );

    expect(screen.getByText("immersive workbench")).toBeInTheDocument();
    expect(screen.queryByLabelText("主导航")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("打开用户菜单")).not.toBeInTheDocument();
  });
});
