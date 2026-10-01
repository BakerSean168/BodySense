import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const viewerHarness = vi.hoisted(() => ({
  modelFailuresRemaining: 0,
  renders: 0,
}));

vi.mock("@/lib/clientDiagnostics", () => ({
  createClientDiagnosticId: () => "body3d-test-session",
  reportClientDiagnostic: vi.fn(),
}));

vi.mock("./BodyExplorer3D", async () => {
  const React = await import("react");
  return {
    default: ({
      onFatalError,
    }: {
      onFatalError: (error: {
        kind: "model";
        message: string;
        retryable: boolean;
      }) => void;
    }) => {
      React.useEffect(() => {
        viewerHarness.renders += 1;
        if (viewerHarness.modelFailuresRemaining > 0) {
          viewerHarness.modelFailuresRemaining -= 1;
          onFatalError({
            kind: "model",
            message: "transient model fetch failure",
            retryable: true,
          });
        }
      }, [onFatalError]);
      return <div>mock 3D viewer</div>;
    },
  };
});

import { reportClientDiagnostic } from "@/lib/clientDiagnostics";
import { BodyExplorer, detectWebGLSupport } from "./BodyExplorer";

describe("BodyExplorer fallback boundary", () => {
  afterEach(() => {
    viewerHarness.modelFailuresRemaining = 0;
    viewerHarness.renders = 0;
    vi.mocked(reportClientDiagnostic).mockClear();
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

    expect(
      await screen.findByText("3D 身体视图暂时不可用"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "选择右肩" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/当前选择已保留/)).toBeInTheDocument();
  });

  it("detects an available WebGL context without importing the lazy viewer", () => {
    const context = {} as WebGLRenderingContext;
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(((
      kind: string,
    ) =>
      kind === "webgl2"
        ? context
        : null) as typeof HTMLCanvasElement.prototype.getContext);

    expect(detectWebGLSupport()).toBe(true);
  });

  it("automatically recreates the viewer once after a retryable model failure", async () => {
    const context = {} as WebGLRenderingContext;
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(((
      kind: string,
    ) =>
      kind === "webgl2"
        ? context
        : null) as typeof HTMLCanvasElement.prototype.getContext);
    viewerHarness.modelFailuresRemaining = 1;

    render(<BodyExplorer snapshot={null} />);

    await waitFor(() => {
      expect(viewerHarness.renders).toBeGreaterThanOrEqual(2);
    });
    expect(await screen.findByText("mock 3D viewer")).toBeInTheDocument();
    expect(reportClientDiagnostic).toHaveBeenCalledWith(
      expect.objectContaining({
        category: "body3d.viewer",
        event: "model_auto_retry",
        phase: "viewer_recreate",
      }),
    );
    expect(screen.queryByText("3D 身体视图暂时不可用")).not.toBeInTheDocument();
  });
});
