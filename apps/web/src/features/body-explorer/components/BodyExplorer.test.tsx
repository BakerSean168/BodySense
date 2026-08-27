import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BodyExplorer, detectWebGLSupport } from "./BodyExplorer";

describe("BodyExplorer fallback boundary", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("keeps State usable with deterministic SVG fallback when WebGL is unavailable", async () => {
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);

    render(<BodyExplorer snapshot={null} />);

    expect(
      await screen.findByText("3D 身体视图暂时不可用"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: /当前身体区域概览/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /重试/ })).toBeInTheDocument();
    expect(screen.getByText("还没有身体记录")).toBeInTheDocument();
  });

  it("keeps the semantic region slot outside the WebGL implementation", async () => {
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);

    render(
      <BodyExplorer
        snapshot={null}
        semanticBridge={{
          selectedRegionLabel: "右肩",
          semanticRegionTree: (
            <button type="button" aria-label="选择右肩">
              右肩区域
            </button>
          ),
        }}
      />,
    );

    expect(await screen.findByText("3D 身体视图暂时不可用")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "选择右肩" })).toBeInTheDocument();
    expect(screen.getByText(/当前选择已保留/)).toBeInTheDocument();
  });

  it("detects an available WebGL context without importing the lazy viewer", () => {
    const context = {} as WebGLRenderingContext;
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(
      ((kind: string) => (kind === "webgl2" ? context : null)) as typeof HTMLCanvasElement.prototype.getContext,
    );

    expect(detectWebGLSupport()).toBe(true);
  });
});
